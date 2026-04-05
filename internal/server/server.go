package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/yuki9431/exvs-analyzer/internal/model"
	"github.com/yuki9431/exvs-analyzer/internal/scraper"
	"github.com/yuki9431/exvs-analyzer/internal/storage"
)

// DefaultMSListPath はデフォルトのMSリストパス
const DefaultMSListPath = "data/ms_list.json"

type analyzeRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type analyzeResponse struct {
	Report string `json:"report"`
	Error  string `json:"error,omitempty"`
}

// RunPipeline はスクレイピング→分析を実行し、レポートを返す
func RunPipeline(username, password string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "exvs-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
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
	datedScores := scraper.Scraiping(username, password, since)
	if len(datedScores) == 0 && !exists {
		return "", fmt.Errorf("no scores found")
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
		return "", fmt.Errorf("failed to save CSV: %w", err)
	}

	// Cloud Storageにアップロード
	if err := storage.UploadCSV(username, csvPath); err != nil {
		log.Printf("[WARN] Failed to upload CSV to Cloud Storage: %v", err)
	}

	// Python分析実行
	cmd := exec.Command("python3", "scripts/analyze.py", csvPath)
	cmd.Dir = "/app"
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("analysis failed: %w\n%s", err, string(output))
	}

	// レポート読み込み
	reportPath := filepath.Join(tmpDir, "report.md")
	report, err := os.ReadFile(reportPath)
	if err != nil {
		return "", fmt.Errorf("failed to read report: %w", err)
	}

	return string(report), nil
}

var requestLimiter = make(chan struct{}, 3)

// StartServer はHTTPサーバーを起動する
func StartServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	http.HandleFunc("/analyze", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req analyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Username == "" || req.Password == "" {
			sendError(w, "Username and password are required", http.StatusBadRequest)
			return
		}

		select {
		case requestLimiter <- struct{}{}:
			defer func() { <-requestLimiter }()
		default:
			sendError(w, "Server is busy, please try again later", http.StatusServiceUnavailable)
			return
		}

		report, err := RunPipeline(req.Username, req.Password)
		if err != nil {
			log.Printf("[ERROR] Pipeline failed: %v", err)
			sendError(w, "Analysis failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analyzeResponse{Report: report})
	})

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	log.Printf("[INFO] Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("[ERROR] Server failed: %v", err)
	}
}

func sendError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(analyzeResponse{Error: msg})
}
