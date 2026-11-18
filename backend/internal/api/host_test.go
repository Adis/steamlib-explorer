package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckHost(t *testing.T) {
	h := CheckHost("http://my-pc.lan:5173")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for host, want := range map[string]int{
		"localhost:8080":         http.StatusOK,
		"LOCALHOST:5173":         http.StatusOK,
		"localhost":              http.StatusOK,
		"127.0.0.1:8080":         http.StatusOK,
		"[::1]:8080":             http.StatusOK,
		"192.168.1.20:5173":      http.StatusOK,
		"my-pc.lan:5173":         http.StatusOK,
		"my-pc.lan.:5173":        http.StatusOK,
		"evil.example:8080":      http.StatusForbidden,
		"localhost.evil.example": http.StatusForbidden,
		"":                       http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/setup", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %q = %d, want %d", host, rec.Code, want)
		}
	}
}
