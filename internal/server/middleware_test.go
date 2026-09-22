package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAPIKey(t *testing.T) {
	s := &Server{cfg: Config{APIKey: "gt_secret"}, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	h := s.requireAPIKey(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	tests := []struct {
		name   string
		method string
		path   string
		auth   string
		want   int
	}{
		{"health without a key", http.MethodGet, "/v0/health", "", http.StatusNoContent},
		{"health by suffix is not exempt", http.MethodGet, "/v0/spaces/local/databases/health", "", http.StatusUnauthorized},
		{"delete named health is not exempt", http.MethodDelete, "/v0/spaces/local/databases/health", "", http.StatusUnauthorized},
		{"no key", http.MethodGet, "/v0/spaces", "", http.StatusUnauthorized},
		{"wrong key", http.MethodGet, "/v0/spaces", "Bearer gt_other", http.StatusUnauthorized},
		{"wrong scheme", http.MethodGet, "/v0/spaces", "Basic gt_secret", http.StatusUnauthorized},
		{"right key", http.MethodGet, "/v0/spaces", "Bearer gt_secret", http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("%s %s: got %d, want %d", tt.method, tt.path, rec.Code, tt.want)
			}
		})
	}
}

func TestRecoverPanics(t *testing.T) {
	s := &Server{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	h := s.recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v0/spaces", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", rec.Code)
	}
}
