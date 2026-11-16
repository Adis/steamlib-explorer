package webui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(t *testing.T, h http.Handler, path string) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Result()
}

func TestServesBuild(t *testing.T) {
	h := New(fstest.MapFS{
		"index.html":       {Data: []byte("<html>app</html>")},
		"assets/app-1a.js": {Data: []byte("console.log(1)")},
	})

	res := get(t, h, "/")
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), "app") {
		t.Fatalf("GET / = %d %q, want index.html", res.StatusCode, body)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("index Cache-Control = %q, want no-cache", cc)
	}

	res = get(t, h, "/assets/app-1a.js")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET asset = %d, want 200", res.StatusCode)
	}
	if cc := res.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("asset Cache-Control = %q, want immutable", cc)
	}

	if res := get(t, h, "/assets/missing.js"); res.StatusCode != http.StatusNotFound {
		t.Errorf("GET missing asset = %d, want 404", res.StatusCode)
	}
}

func TestNotBuilt(t *testing.T) {
	h := New(fstest.MapFS{".gitkeep": {}})
	res := get(t, h, "/")
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusNotFound || !strings.Contains(string(body), "make build") {
		t.Fatalf("GET / = %d %q, want the not-built page", res.StatusCode, body)
	}
}
