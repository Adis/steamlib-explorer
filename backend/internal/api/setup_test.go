package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/steam"
)

type fakeSetup struct {
	key      string
	checkErr error
	saveErr  error
}

func (f *fakeSetup) HasAPIKey() bool                           { return f.key != "" }
func (f *fakeSetup) CheckAPIKey(context.Context, string) error { return f.checkErr }
func (f *fakeSetup) SaveAPIKey(key string) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.key = key
	return nil
}

const goodKey = "0123456789ABCDEF0123456789abcdef"

func TestSetup(t *testing.T) {
	post := func(f *fakeSetup, contentType, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()
		NewRouter(Deps{DB: fakeDB{}, Auth: &auth.Service{}, Setup: f}).ServeHTTP(rec, req)
		return rec
	}
	status := func(f *fakeSetup) string {
		rec := httptest.NewRecorder()
		NewRouter(Deps{DB: fakeDB{}, Auth: &auth.Service{}, Setup: f}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/setup", nil))
		return strings.TrimSpace(rec.Body.String())
	}

	f := &fakeSetup{}
	if got := status(f); got != `{"needs_api_key":true}` {
		t.Errorf("status before = %s", got)
	}
	if rec := post(f, "application/json", `{"steam_api_key": " `+goodKey+` "}`); rec.Code != http.StatusNoContent || f.key != goodKey {
		t.Fatalf("save = %d %s, key %q", rec.Code, rec.Body, f.key)
	}
	if got := status(f); got != `{"needs_api_key":false}` {
		t.Errorf("status after = %s", got)
	}
	if rec := post(f, "application/json", `{"steam_api_key": "`+goodKey+`"}`); rec.Code != http.StatusConflict {
		t.Errorf("second save = %d, want 409: only the first run may set it without signing in", rec.Code)
	}

	for name, tt := range map[string]struct {
		f           *fakeSetup
		contentType string
		body        string
		want        int
	}{
		"form post (cross-site)":  {&fakeSetup{}, "application/x-www-form-urlencoded", "steam_api_key=" + goodKey, http.StatusUnsupportedMediaType},
		"text/plain (cross-site)": {&fakeSetup{}, "text/plain", `{"steam_api_key": "` + goodKey + `"}`, http.StatusUnsupportedMediaType},
		"not a key":               {&fakeSetup{}, "application/json", `{"steam_api_key": "hello"}`, http.StatusBadRequest},
		"bad json":                {&fakeSetup{}, "application/json", `{`, http.StatusBadRequest},
		"steam refuses it":        {&fakeSetup{checkErr: steam.ErrBadAPIKey}, "application/json", `{"steam_api_key": "` + goodKey + `"}`, http.StatusBadRequest},
		"steam unreachable":       {&fakeSetup{checkErr: errors.New("dial tcp: timeout")}, "application/json", `{"steam_api_key": "` + goodKey + `"}`, http.StatusBadGateway},
		"disk full":               {&fakeSetup{saveErr: errors.New("no space left")}, "application/json", `{"steam_api_key": "` + goodKey + `"}`, http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			if rec := post(tt.f, tt.contentType, tt.body); rec.Code != tt.want {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tt.want, rec.Body)
			}
			if tt.f.key != "" {
				t.Errorf("key was saved: %q", tt.f.key)
			}
		})
	}
}
