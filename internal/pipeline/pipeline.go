package pipeline

import (
	"encoding/json"
	"errors"
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
	UserKey            string    `json:"-"`
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
	UserKey           string
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
		UserKey:           j.UserKey,
	}
}

// On403Func は403検出時に呼び出されるコールバック型
type On403Func func(userHash string)

// Run はスクレイピング→分析を実行し、レポートをジョブに保存する
func Run(j *Job, username, password string, on403 ...On403Func) {
	jobsMu.Lock()
	j.UserKey = storage.UserKey(username)
	jobsMu.Unlock()
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
	// GCSから既存タッグ相方情報をダウンロード（速報レポートで使用）
	cachedTagPartnersPath := filepath.Join(tmpDir, "cached_tag_partners.json")
	tagPartnersExists, err := storage.DownloadTagPartners(username, cachedTagPartnersPath)
	if err != nil {
		log.Printf("[WARN] Failed to download existing tag partners: %v", err)
	}
	if !tagPartnersExists {
		cachedTagPartnersPath = ""
	}

	if exists {
		since, err = storage.GetLatestDatetime(csvPath)
		if err != nil {
			log.Printf("[WARN] Failed to read latest datetime: %v", err)
		}
		if !since.IsZero() {
			log.Printf("[INFO] Fetching scores after %s", since.Format("2006-01-02 15:04"))
		}

		// 前回データで即座に分析（速報レポート、キャッシュ済みタッグ情報付き）
		prelimReport := runAnalysis(csvPath, tmpDir, cachedTagPartnersPath)
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
	datedScores, jar, err := scraper.Scraping(username, password, since, onProgress)
	if err != nil {
		switch {
		case errors.Is(err, scraper.ErrLoginFailed):
			setError(j, "ログインに失敗しました。メールアドレスとパスワードを確認してください。", err.Error())
		case errors.Is(err, scraper.ErrAccessDenied):
			setError(j, "対戦履歴ページへのアクセスが拒否されました。ブラウザからガンダムモバイル(https://web.vsmobile.jp)にログインし、対戦履歴が閲覧できるか確認してください。", err.Error())
			if len(on403) > 0 && on403[0] != nil {
				on403[0](storage.UserKey(username))
			}
		case errors.Is(err, scraper.ErrUnauthorized):
			setError(j, "認証の有効期限が切れました。再度ログインしてお試しください。", err.Error())
		case errors.Is(err, scraper.ErrNotFound):
			setError(j, "対戦履歴ページが見つかりませんでした。サイトの仕様が変更された可能性があります。", err.Error())
		case errors.Is(err, scraper.ErrServerError):
			setError(j, "ガンダムモバイルのサーバーでエラーが発生しています。しばらく時間をおいてから再度お試しください。", err.Error())
		default:
			setError(j, "データの取得に失敗しました。時間をおいて再度お試しいただき、解決しない場合は開発者までお問い合わせください。", err.Error())
		}
		return
	}
	if len(datedScores) == 0 && !exists {
		setError(j, "戦績データが見つかりませんでした", "no scores found")
		return
	}

	// 新規データがない場合はタッグ情報を付与して最終レポートにする
	if len(datedScores) == 0 && j.PreliminaryReport != "" {
		var tagPartnersPath string
		tagPartners := scraper.ScrapeTagPartners(jar)
		if len(tagPartners) > 0 {
			tagPartnersPath = filepath.Join(tmpDir, "tag_partners.json")
			if err := saveTagPartners(tagPartners, tagPartnersPath); err != nil {
				log.Printf("[WARN] Failed to save tag partners: %v", err)
				tagPartnersPath = ""
			} else {
				log.Printf("[INFO] Found %d tag partners (no new data path)", len(tagPartners))
				// タッグ相方情報をGCSにアップロード
				if err := storage.UploadTagPartners(username, tagPartnersPath); err != nil {
					log.Printf("[WARN] Failed to upload tag partners to GCS: %v", err)
				}
			}
		} else {
			log.Printf("[INFO] No tag partners found (no new data path)")
		}

		// タッグ情報がある場合は再分析、なければ速報レポートをそのまま使う
		finalReport := j.PreliminaryReport
		if tagPartnersPath != "" {
			report := runAnalysis(csvPath, tmpDir, tagPartnersPath)
			if report != "" {
				finalReport = report
			}
		}

		jobsMu.Lock()
		j.Status = StatusDone
		j.Report = finalReport
		j.completedAt = time.Now()
		jobsMu.Unlock()
		log.Printf("[INFO] Job %s completed (no new data)", j.ID)
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

	// タッグ相方名を取得
	var tagPartnersPath string
	tagPartners := scraper.ScrapeTagPartners(jar)
	if len(tagPartners) > 0 {
		tagPartnersPath = filepath.Join(tmpDir, "tag_partners.json")
		if err := saveTagPartners(tagPartners, tagPartnersPath); err != nil {
			log.Printf("[WARN] Failed to save tag partners: %v", err)
			tagPartnersPath = ""
		} else {
			log.Printf("[INFO] Found %d tag partners", len(tagPartners))
			// タッグ相方情報をGCSにアップロード
			if err := storage.UploadTagPartners(username, tagPartnersPath); err != nil {
				log.Printf("[WARN] Failed to upload tag partners to GCS: %v", err)
			}
		}
	} else {
		log.Printf("[INFO] No tag partners found")
	}

	// Python分析実行
	updateStatus(j, StatusAnalyzing)
	report := runAnalysis(csvPath, tmpDir, tagPartnersPath)
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

// RunCustomPeriod はカスタム日時範囲で再分析を実行してJSON文字列を返す
func RunCustomPeriod(userKey, start, end string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "exvs-period-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "scores.csv")

	exists, err := storage.DownloadCSVByKey(userKey, csvPath)
	if err != nil {
		return "", fmt.Errorf("failed to download CSV: %w", err)
	}
	if !exists {
		return "", fmt.Errorf("CSV not found for user")
	}

	cmd := exec.Command("python3", "scripts/analyze.py", csvPath, "--start", start, "--end", end)
	cmd.Dir = "/app"
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("analysis failed: %v\n%s", err, string(output))
	}

	reportPath := filepath.Join(tmpDir, "report.json")
	report, err := os.ReadFile(reportPath)
	if err != nil {
		return "", fmt.Errorf("failed to read report: %w", err)
	}
	return string(report), nil
}

// saveTagPartners はタッグ相方情報をJSONファイルに保存する
func saveTagPartners(partners []scraper.TagPartner, path string) error {
	type tagPartnerJSON struct {
		TeamName   string `json:"team_name"`
		PlayerName string `json:"player_name"`
	}

	data := make([]tagPartnerJSON, len(partners))
	for i, p := range partners {
		data[i] = tagPartnerJSON{TeamName: p.TeamName, PlayerName: p.PlayerName}
	}

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal tag partners: %w", err)
	}
	return os.WriteFile(path, b, 0644)
}

// runAnalysis はPython分析を実行してJSON形式のレポートを返す。失敗時は空文字を返す。
func runAnalysis(csvPath, tmpDir, tagPartnersPath string) string {
	args := []string{"scripts/analyze.py", csvPath}
	if tagPartnersPath != "" {
		args = append(args, "--tag-partners", tagPartnersPath)
	}
	cmd := exec.Command("python3", args...)
	cmd.Dir = "/app"
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[WARN] Analysis failed: %v\n%s", err, string(output))
		return ""
	}

	reportPath := filepath.Join(tmpDir, "report.json")
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
