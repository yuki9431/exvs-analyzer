package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"os"
)

// basicAuth はベーシック認証ミドルウェアを返す。
// 環境変数 BASIC_AUTH_USER / BASIC_AUTH_PASS が未設定の場合は認証をスキップする。
// skipPaths に指定したパスは認証不要（ヘルスチェック用）。
func basicAuth(next http.Handler, skipPaths ...string) http.Handler {
	user := os.Getenv("BASIC_AUTH_USER")
	pass := os.Getenv("BASIC_AUTH_PASS")

	// 未設定なら認証なしで通す
	if user == "" || pass == "" {
		return next
	}

	// 比較用ハッシュを事前計算（タイミング攻撃対策）
	expectedUserHash := sha256.Sum256([]byte(user))
	expectedPassHash := sha256.Sum256([]byte(pass))

	skip := make(map[string]bool, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// スキップ対象パスは認証不要
		if skip[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		givenUser, givenPass, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		givenUserHash := sha256.Sum256([]byte(givenUser))
		givenPassHash := sha256.Sum256([]byte(givenPass))

		userMatch := subtle.ConstantTimeCompare(givenUserHash[:], expectedUserHash[:]) == 1
		passMatch := subtle.ConstantTimeCompare(givenPassHash[:], expectedPassHash[:]) == 1

		if !userMatch || !passMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
