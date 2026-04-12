package pipeline

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yuki9431/exvs-analyzer/internal/mslist"
	"github.com/yuki9431/exvs-analyzer/internal/scraper"
	"github.com/yuki9431/exvs-analyzer/internal/storage"
)

// DefaultMSListPath はデフォルトのMSリストパス
const DefaultMSListPath = "data/ms_list.json"

// JobStatus はジョブの状態
type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusScraping  JobStatus = "scraping"
	StatusAnalyzing JobStatus = "analyzing"
	StatusDone      JobStatus = "done"
	StatusError     JobStatus = "error"
)

// Job はバックグラウンドジョブの情報
type Job struct {
	ID                 string    `json:"id"`
	Status             JobStatus `json:"status"`
	Message            string    `json:"message,omitempty"`
	Progress           int       `json:"progress,omitempty"`
	ProgressTotal      int       `json:"progress_total,omitempty"`
	Report             string    `json:"report,omitempty"`
	PreliminaryReport  string    `json:"preliminary_report,omitempty"`
	Error              string    `json:"error,omitempty"`
	completedAt        time.Time
}

// JobSnapshot はジョブ状態のスナップショット
type JobSnapshot struct {
	ID                string
	Status            JobStatus
	Message           string
	Progress          int
	ProgressTotal     int
	Report            string
	PreliminaryReport string
	Error             string
}

// ジョブストア（インメモリ）
var (
	jobs   = make(map[string]*Job)
	jobsMu sync.RWMutex
)

// NewJob はジョブを作成してストアに登録する
func NewJob() *Job {
	j := &Job{
		ID:     uuid.New().String(),
		Status: StatusPending,
	}
	jobsMu.Lock()
	jobs[j.ID] = j
	jobsMu.Unlock()
	return j
}

// GetJob はIDからジョブを取得する
func GetJob(id string) (*Job, bool) {
	jobsMu.RLock()
	j, ok := jobs[id]
	jobsMu.RUnlock()
	return j, ok
}

// Snapshot はジョブ状態のスナップショットを返す
func (j *Job) Snapshot() JobSnapshot {
	jobsMu.RLock()
	defer jobsMu.RUnlock()
	return JobSnapshot{
		ID:                j.ID,
		Status:            j.Status,
		Message:           j.Message,
		Progress:          j.Progress,
		ProgressTotal:     j.ProgressTotal,
		Report:            j.Report,
		PreliminaryReport: j.PreliminaryReport,
		Error:             j.Error,
	}
}

// Run はスクレイピング→分析を実行し、レポートをジョブに保存する
func Run(j *Job, username, password string) {
	updateStatus(j, StatusScraping)

	tmpDir, err := os.MkdirTemp("", "exvs-*")
	if err != nil {
		setError(j, "内部エラーが発生しました", fmt.Sprintf("failed to create temp dir: %v", err))
		return
	}
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "scores.csv")

	// Cloud Storageから既存CSVをダウンロード
	var since time.Time
	exists, err := storage.DownloadCSV(username, csvPath)
	if err != nil {
		log.Printf("[WARN] Failed to download existing CSV: %v", err)
	}
	if exists {
		since, err = storage.GetLatestDatetime(csvPath)
		if err != nil {
			log.Printf("[WARN] Failed to read latest datetime: %v", err)
		}
		if !since.IsZero() {
			log.Printf("[INFO] Fetching scores after %s", since.Format("2006-01-02 15:04"))
		}

		// 前回データで即座に分析（速報レポート）
		prelimReport := runAnalysis(csvPath, tmpDir)
		if prelimReport != "" {
			jobsMu.Lock()
			j.PreliminaryReport = prelimReport
			jobsMu.Unlock()
			log.Printf("[INFO] Job %s: preliminary report ready", j.ID)
		}
	}

	// スクレイピング
	log.Printf("[INFO] Scraping for user (hash: %s)", storage.UserKey(username))
	onProgress := func(current, total int) {
		jobsMu.Lock()
		j.Message = "戦歴データを取得中"
		j.Progress = current
		j.ProgressTotal = total
		jobsMu.Unlock()
	}
	datedScores := scraper.Scraping(username, password, since, onProgress)
	if len(datedScores) == 0 && !exists {
		setError(j, "戦績データが見つかりませんでした", "no scores found")
		return
	}

	// 新規データがない場合は速報レポートをそのまま最終結果にする
	if len(datedScores) == 0 && j.PreliminaryReport != "" {
		jobsMu.Lock()
		j.Status = StatusDone
		j.Report = j.PreliminaryReport
		j.completedAt = time.Now()
		jobsMu.Unlock()
		log.Printf("[INFO] Job %s completed (no new data, using preliminary report)", j.ID)
		return
	}

	// 同梱のMSリストから機体名マッピングを読み込み
	msList, err := mslist.LoadMSList(DefaultMSListPath)
	if err != nil {
		log.Printf("[WARN] MS list not found, MS names will be empty")
	}

	msMap := mslist.BuildMSNameMap(msList)
	mslist.FillMsNames(datedScores, msMap)
	mslist.CheckUnknownMS(datedScores)

	// CSV保存（既存データに追記）
	if err := storage.SaveAllScoresCSV(datedScores, csvPath); err != nil {
		setError(j, "内部エラーが発生しました", fmt.Sprintf("failed to save CSV: %v", err))
		return
	}

	// Cloud Storageにアップロード
	if err := storage.UploadCSV(username, csvPath); err != nil {
		log.Printf("[WARN] Failed to upload CSV to Cloud Storage: %v", err)
	}

	// Python分析実行
	updateStatus(j, StatusAnalyzing)
	report := runAnalysis(csvPath, tmpDir)
	if report == "" {
		setError(j, "分析処理に失敗しました", "analysis returned empty report")
		return
	}

	jobsMu.Lock()
	j.Status = StatusDone
	j.Report = report
	j.completedAt = time.Now()
	jobsMu.Unlock()
	log.Printf("[INFO] Job %s completed", j.ID)
}

// runAnalysis はPython分析を実行してレポートを返す。失敗時は空文字を返す。
func runAnalysis(csvPath, tmpDir string) string {
	cmd := exec.Command("python3", "scripts/analyze.py", csvPath)
	cmd.Dir = "/app"
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[WARN] Analysis failed: %v\n%s", err, string(output))
		return ""
	}

	reportPath := filepath.Join(tmpDir, "report.md")
	report, err := os.ReadFile(reportPath)
	if err != nil {
		log.Printf("[WARN] Failed to read report: %v", err)
		return ""
	}
	return string(report)
}

func updateStatus(j *Job, s JobStatus) {
	jobsMu.Lock()
	j.Status = s
	jobsMu.Unlock()
}

func setError(j *Job, clientMsg, detail string) {
	jobsMu.Lock()
	j.Status = StatusError
	j.Error = clientMsg
	j.completedAt = time.Now()
	jobsMu.Unlock()
	log.Printf("[ERROR] Job %s failed: %s", j.ID, detail)
}

// CleanupJobs は完了済みジョブを定期的に削除する
func CleanupJobs(ttl time.Duration) {
	ticker := time.NewTicker(ttl)
	defer ticker.Stop()
	for range ticker.C {
		jobsMu.Lock()
		before := len(jobs)
		for id, j := range jobs {
			if !j.completedAt.IsZero() && time.Since(j.completedAt) > ttl {
				delete(jobs, id)
			}
		}
		after := len(jobs)
		jobsMu.Unlock()
		if before != after {
			log.Printf("[INFO] Job cleanup: %d -> %d jobs", before, after)
		}
	}
}
