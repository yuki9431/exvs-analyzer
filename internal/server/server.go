package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"time"

	"github.com/yuki9431/exvs-analyzer/internal/pipeline"
	"github.com/yuki9431/exvs-analyzer/internal/storage"
	"golang.org/x/time/rate"
)

type analyzeRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var requestLimiter = make(chan struct{}, 3)

// StartServer はHTTPサーバーを起動する
func StartServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 完了済みジョブの定期クリーンアップ（1時間経過したジョブを削除）
	go pipeline.CleanupJobs(1 * time.Hour)

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

		// 403ブロックチェック
		userHash := storage.UserKey(req.Username)
		if forbidden403.IsBlocked(userHash) {
			sendJSON(w, http.StatusTooManyRequests, map[string]string{"error": "戦績詳細ページへのアクセスが拒否されました。ブラウザからガンダムモバイル(https://web.vsmobile.jp)にログインし、対戦履歴が閲覧できるか確認してください。"})
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
		j := pipeline.NewJob()

		// バックグラウンドで実行
		go func() {
			defer func() { <-requestLimiter }()
			pipeline.Run(j, req.Username, req.Password, forbidden403.Block)
		}()

		sendJSON(w, http.StatusAccepted, map[string]string{"id": j.ID})
	})

	// GET /status/{id} → ジョブ状態を返す
	http.HandleFunc("/status/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/status/"):]

		j, ok := pipeline.GetJob(id)
		if !ok {
			sendJSON(w, http.StatusNotFound, map[string]string{"error": "Job not found"})
			return
		}

		snap := j.Snapshot()
		resp := map[string]interface{}{
			"id":     snap.ID,
			"status": string(snap.Status),
		}
		if snap.Message != "" {
			resp["message"] = snap.Message
		}
		if snap.ProgressTotal > 0 {
			resp["progress"] = snap.Progress
			resp["progress_total"] = snap.ProgressTotal
		}
		if snap.Error != "" {
			resp["error"] = snap.Error
		}
		if snap.PreliminaryReport != "" {
			resp["has_preliminary_report"] = true
		}

		sendJSON(w, http.StatusOK, resp)
	})

	// GET /result/{id} → 分析結果(JSON)を返す
	// GET /result/{id}/period?start=...&end=... → カスタム期間で再分析
	http.HandleFunc("/result/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[len("/result/"):]

		// /result/{id}/period のパターンを判定
		if idx := len(path) - len("/period"); idx > 0 && path[idx:] == "/period" {
			id := path[:idx]
			handleCustomPeriod(w, r, id)
			return
		}

		// 既存の /result/{id} 処理
		handleResult(w, r, path)
	})

	// GET /period?user_key=...&start=...&end=... → カスタム期間で再分析（ジョブ不要）
	http.HandleFunc("/period", func(w http.ResponseWriter, r *http.Request) {
		handlePeriod(w, r)
	})

	// 静的ファイル（フロントエンド）
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	log.Printf("[INFO] Server starting on port %s", port)
	handler := basicAuth(securityHeaders(http.DefaultServeMux), "/health")
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("[ERROR] Server failed: %v", err)
	}
}

func handleResult(w http.ResponseWriter, r *http.Request, id string) {
	j, ok := pipeline.GetJob(id)
	if !ok {
		sendJSON(w, http.StatusNotFound, map[string]string{"error": "Job not found"})
		return
	}

	snap := j.Snapshot()

	if snap.Status != pipeline.StatusDone && snap.Status != pipeline.StatusError {
		if snap.PreliminaryReport != "" {
			sendRawReport(w, http.StatusOK, snap.PreliminaryReport, string(snap.Status), snap.UserKey, true)
			return
		}
		sendJSON(w, http.StatusAccepted, map[string]string{"status": string(snap.Status)})
		return
	}

	if snap.Status == pipeline.StatusError {
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": snap.Error})
		return
	}

	sendRawReport(w, http.StatusOK, snap.Report, "", snap.UserKey, false)
}

func handleCustomPeriod(w http.ResponseWriter, r *http.Request, id string) {
	j, ok := pipeline.GetJob(id)
	if !ok {
		sendJSON(w, http.StatusNotFound, map[string]string{"error": "Job not found"})
		return
	}

	snap := j.Snapshot()
	if snap.Status != pipeline.StatusDone {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Job not completed"})
		return
	}

	if snap.UserKey == "" {
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "User key not found"})
		return
	}

	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	if start == "" || end == "" {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "start and end parameters are required"})
		return
	}

	// 日時フォーマットの簡易バリデーション（YYYY-MM-DD HH:MM）
	const layout = "2006-01-02 15:04"
	if _, err := time.Parse(layout, start); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid start datetime format (expected: YYYY-MM-DD HH:MM)"})
		return
	}
	if _, err := time.Parse(layout, end); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid end datetime format (expected: YYYY-MM-DD HH:MM)"})
		return
	}

	report, err := pipeline.RunCustomPeriod(snap.UserKey, start, end)
	if err != nil {
		log.Printf("[ERROR] Custom period analysis failed: %v", err)
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "カスタム期間の分析に失敗しました"})
		return
	}

	sendRawReport(w, http.StatusOK, report, "", snap.UserKey, false)
}

func handlePeriod(w http.ResponseWriter, r *http.Request) {
	userKey := r.URL.Query().Get("user_key")
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	if userKey == "" || start == "" || end == "" {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "user_key, start and end parameters are required"})
		return
	}

	const layout = "2006-01-02 15:04"
	if _, err := time.Parse(layout, start); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid start datetime format (expected: YYYY-MM-DD HH:MM)"})
		return
	}
	if _, err := time.Parse(layout, end); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid end datetime format (expected: YYYY-MM-DD HH:MM)"})
		return
	}

	report, err := pipeline.RunCustomPeriod(userKey, start, end)
	if err != nil {
		log.Printf("[ERROR] Custom period analysis failed: %v", err)
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "カスタム期間の分析に失敗しました"})
		return
	}

	sendRawReport(w, http.StatusOK, report, "", userKey, false)
}

// securityHeaders は全レスポンスにセキュリティヘッダーを付与する
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		next.ServeHTTP(w, r)
	})
}

func sendJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// sendRawReport はJSON形式のレポートをレスポンスとして返す。
// reportJSONはanalyze.pyが生成したJSON文字列。json.RawMessageで二重エンコードを防ぐ。
func sendRawReport(w http.ResponseWriter, code int, reportJSON, status, userKey string, preliminary bool) {
	type reportResponse struct {
		Report      json.RawMessage `json:"report"`
		Status      string          `json:"status,omitempty"`
		Preliminary bool            `json:"preliminary,omitempty"`
		UserKey     string          `json:"user_key,omitempty"`
	}
	resp := reportResponse{
		Report:      json.RawMessage(reportJSON),
		Status:      status,
		Preliminary: preliminary,
		UserKey:     userKey,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}
