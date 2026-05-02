package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicAuth(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("環境変数未設定なら認証スキップ", func(t *testing.T) {
		t.Setenv("BASIC_AUTH_USER", "")
		t.Setenv("BASIC_AUTH_PASS", "")

		handler := basicAuth(okHandler)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("認証情報なしで401", func(t *testing.T) {
		t.Setenv("BASIC_AUTH_USER", "admin")
		t.Setenv("BASIC_AUTH_PASS", "secret")

		handler := basicAuth(okHandler)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
		if rec.Header().Get("WWW-Authenticate") == "" {
			t.Error("expected WWW-Authenticate header")
		}
	})

	t.Run("誤った認証情報で401", func(t *testing.T) {
		t.Setenv("BASIC_AUTH_USER", "admin")
		t.Setenv("BASIC_AUTH_PASS", "secret")

		handler := basicAuth(okHandler)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("admin", "wrong")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("正しい認証情報で200", func(t *testing.T) {
		t.Setenv("BASIC_AUTH_USER", "admin")
		t.Setenv("BASIC_AUTH_PASS", "secret")

		handler := basicAuth(okHandler)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("admin", "secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("skipPathsは認証不要", func(t *testing.T) {
		t.Setenv("BASIC_AUTH_USER", "admin")
		t.Setenv("BASIC_AUTH_PASS", "secret")

		handler := basicAuth(okHandler, "/health")
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}
