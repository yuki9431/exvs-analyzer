package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yuki9431/exvs-analyzer/internal/model"
	"github.com/yuki9431/exvs-analyzer/internal/scraper"
	"github.com/yuki9431/exvs-analyzer/internal/storage"
	"golang.org/x/time/rate"
)

// DefaultMSListPath はデフォルトのMSリストパス
const DefaultMSListPath = "data/ms_list.json"

// ジョブの状態
type jobStatus string

const (
	statusPending   jobStatus = "pending"
	statusScraping  jobStatus = "scraping"
	statusAnalyzing jobStatus = "analyzing"
	statusDone      jobStatus = "done"
	statusError     jobStatus = "error"
)

// job はバックグラウンドジョブの情報
type job struct {
	ID      string    `json:"id"`
	Status  jobStatus `json:"status"`
	Message string    `json:"message,omitempty"`
	Report  string    `json:"report,omitempty"`
	Error   string    `json:"error,omitempty"`
}

// ジョブストア（インメモリ）
var (
	jobs   = make(map[string]*job)
	jobsMu sync.RWMutex
)

type analyzeRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// runPipeline はスクレイピング→分析を実行し、レポートを返す
func runPipeline(j *job, username, password string) {
	updateStatus(j, statusScraping)

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
	}

	// スクレイピング
	log.Printf("[INFO] Scraping for user (hash: %s)", storage.UserKey(username))
	onProgress := func(msg string) {
		jobsMu.Lock()
		j.Message = msg
		jobsMu.Unlock()
	}
	datedScores := scraper.Scraping(username, password, since, onProgress)
	if len(datedScores) == 0 && !exists {
		setError(j, "戦績データが見つかりませんでした", "no scores found")
		return
	}

	// 同梱のMSリストから機体名マッピングを読み込み
	msList, err := model.LoadMSList(DefaultMSListPath)
	if err != nil {
		log.Printf("[WARN] MS list not found, MS names will be empty")
	}

	msMap := model.BuildMSNameMap(msList)
	datedScores.FillMsNames(msMap)
	datedScores.CheckUnknownMS()

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
	updateStatus(j, statusAnalyzing)
	cmd := exec.Command("python3", "scripts/analyze.py", csvPath)
	cmd.Dir = "/app"
	output, err := cmd.CombinedOutput()
	if err != nil {
		setError(j, "分析処理に失敗しました", fmt.Sprintf("analysis failed: %v\n%s", err, string(output)))
		return
	}

	// レポート読み込み
	reportPath := filepath.Join(tmpDir, "report.md")
	report, err := os.ReadFile(reportPath)
	if err != nil {
		setError(j, "内部エラーが発生しました", fmt.Sprintf("failed to read report: %v", err))
		return
	}

	jobsMu.Lock()
	j.Status = statusDone
	j.Report = string(report)
	jobsMu.Unlock()
	log.Printf("[INFO] Job %s completed", j.ID)
}

func updateStatus(j *job, s jobStatus) {
	jobsMu.Lock()
	j.Status = s
	jobsMu.Unlock()
}

func setError(j *job, clientMsg, detail string) {
	jobsMu.Lock()
	j.Status = statusError
	j.Error = clientMsg
	jobsMu.Unlock()
	log.Printf("[ERROR] Job %s failed: %s", j.ID, detail)
}

var requestLimiter = make(chan struct{}, 3)

// StartServer はHTTPサーバーを起動する
func StartServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// レート制限の設定（RATE_LIMIT環境変数: 1時間あたりの最大リクエスト数、0または未設定で無制限）
	var rl *rateLimiter
	if v := os.Getenv("RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			// n回/時間 = n/3600回/秒、バーストはnと同じ
			rl = newRateLimiter(rate.Limit(float64(n)/3600), n)
			log.Printf("[INFO] Rate limit enabled: %d requests/hour per IP", n)
		}
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	// POST /analyze → ジョブ作成、IDを返す
	http.HandleFunc("/analyze", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// レート制限チェック
		if rl != nil {
			ip := clientIP(r)
			if !rl.getLimiter(ip).Allow() {
				sendJSON(w, http.StatusTooManyRequests, map[string]string{"error": "リクエスト回数の上限に達しました。しばらく時間をおいてから再度お試しください"})
				return
			}
		}

		// リクエストボディサイズを1KBに制限
		r.Body = http.MaxBytesReader(w, r.Body, 1024)

		var req analyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
			return
		}

		if req.Username == "" || req.Password == "" {
			sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Username and password are required"})
			return
		}

		// メールアドレス形式チェック
		if _, err := mail.ParseAddress(req.Username); err != nil {
			sendJSON(w, http.StatusBadRequest, map[string]string{"error": "有効なメールアドレスを入力してください"})
			return
		}

		// 入力長の制限（メールアドレス: RFC 5321準拠254文字、パスワード: 128文字）
		if len(req.Username) > 254 || len(req.Password) > 128 {
			sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Username or password is too long"})
			return
		}

		// 同時実行数制限
		select {
		case requestLimiter <- struct{}{}:
		default:
			sendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Server is busy, please try again later"})
			return
		}

		// ジョブ作成
		j := &job{
			ID:     uuid.New().String(),
			Status: statusPending,
		}
		jobsMu.Lock()
		jobs[j.ID] = j
		jobsMu.Unlock()

		// バックグラウンドで実行
		go func() {
			defer func() { <-requestLimiter }()
			runPipeline(j, req.Username, req.Password)
		}()

		sendJSON(w, http.StatusAccepted, map[string]string{"id": j.ID})
	})

	// GET /status/{id} → ジョブ状態を返す
	http.HandleFunc("/status/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/status/"):]

		jobsMu.RLock()
		j, ok := jobs[id]
		jobsMu.RUnlock()

		if !ok {
			sendJSON(w, http.StatusNotFound, map[string]string{"error": "Job not found"})
			return
		}

		jobsMu.RLock()
		resp := map[string]string{
			"id":     j.ID,
			"status": string(j.Status),
		}
		if j.Message != "" {
			resp["message"] = j.Message
		}
		if j.Error != "" {
			resp["error"] = j.Error
		}
		jobsMu.RUnlock()

		sendJSON(w, http.StatusOK, resp)
	})

	// GET /result/{id} → レポートを返す
	http.HandleFunc("/result/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/result/"):]

		jobsMu.RLock()
		j, ok := jobs[id]
		jobsMu.RUnlock()

		if !ok {
			sendJSON(w, http.StatusNotFound, map[string]string{"error": "Job not found"})
			return
		}

		jobsMu.RLock()
		status := j.Status
		report := j.Report
		errMsg := j.Error
		jobsMu.RUnlock()

		if status != statusDone && status != statusError {
			sendJSON(w, http.StatusAccepted, map[string]string{"status": string(status)})
			return
		}

		if status == statusError {
			sendJSON(w, http.StatusInternalServerError, map[string]string{"error": errMsg})
			return
		}

		sendJSON(w, http.StatusOK, map[string]string{"report": report})
	})

	// 静的ファイル（フロントエンド）
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	log.Printf("[INFO] Server starting on port %s", port)
	handler := securityHeaders(http.DefaultServeMux)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("[ERROR] Server failed: %v", err)
	}
}

// securityHeaders は全レスポンスにセキュリティヘッダーを付与する
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

func sendJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}
