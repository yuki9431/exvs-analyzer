package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// analyzeRequest はAPIリクエストのJSON構造
type analyzeRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// analyzeResponse はAPIレスポンスのJSON構造
type analyzeResponse struct {
	Report string `json:"report"`
	Error  string `json:"error,omitempty"`
}

// runPipeline はスクレイピング→分析を実行し、レポートを返す
func runPipeline(username, password string) (string, error) {
	// ユーザーごとの一時ディレクトリを作成
	tmpDir, err := os.MkdirTemp("", "exvs-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "scores.csv")

	// スクレイピング
	log.Printf("[INFO] Scraping for user: %s", username)
	var since time.Time
	datedScores := Scraiping(username, password, since)
	if len(datedScores) == 0 {
		return "", fmt.Errorf("no scores found")
	}

	// 同梱のMSリストから機体名マッピングを読み込み
	msList, err := LoadMSList(defaultMSListPath)
	if err != nil {
		log.Printf("[WARN] MS list not found, MS names will be empty")
	}

	msMap := BuildMSNameMap(msList)
	datedScores.FillMsNames(msMap)
	datedScores.CheckUnknownMS()

	// CSV保存
	if err := SaveAllScoresCSV(datedScores, csvPath); err != nil {
		return "", fmt.Errorf("failed to save CSV: %w", err)
	}

	// Python分析実行
	cmd := exec.Command("python3", "analyze.py", csvPath)
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

// requestLimiter は同時リクエスト数を制限する
var requestLimiter = make(chan struct{}, 3)

func startServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// ヘルスチェック
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	// 分析API
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

		// 同時実行数制限
		select {
		case requestLimiter <- struct{}{}:
			defer func() { <-requestLimiter }()
		default:
			sendError(w, "Server is busy, please try again later", http.StatusServiceUnavailable)
			return
		}

		report, err := runPipeline(req.Username, req.Password)
		if err != nil {
			log.Printf("[ERROR] Pipeline failed: %v", err)
			sendError(w, "Analysis failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analyzeResponse{Report: report})
	})

	// 静的ファイル（フロントエンド）
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
