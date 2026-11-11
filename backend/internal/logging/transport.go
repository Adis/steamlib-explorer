package logging

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Transport logs each outgoing request with its status, duration and the
// start of the response body. API keys in the URL are redacted.
type Transport struct {
	Base    http.RoundTripper // nil means http.DefaultTransport
	Logger  *slog.Logger      // nil means slog.Default()
	Service string            // e.g. "steam", shown on every line
	// BodyPreview is how many bytes of each response body to log (default 300).
	BodyPreview int
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	logger := t.Logger
	if logger == nil {
		logger = slog.Default()
	}
	preview := t.BodyPreview
	if preview <= 0 {
		preview = 300
	}

	start := time.Now()
	resp, err := base.RoundTrip(req)
	attrs := []any{
		"service", t.Service,
		"method", req.Method,
		"url", Redact(req.URL),
		"duration_ms", time.Since(start).Milliseconds(),
	}
	if err != nil {
		logger.Error("outgoing call failed", append(attrs, "error", err)...)
		return nil, err
	}

	// Read the whole body so it can be logged, then hand the caller a copy.
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if readErr != nil {
		logger.Error("outgoing call body read failed", append(attrs, "status", resp.StatusCode, "error", readErr)...)
		return resp, nil
	}

	attrs = append(attrs, "status", resp.StatusCode, "bytes", len(body), "body", oneLine(body, preview))
	if resp.StatusCode >= 400 {
		logger.Warn("outgoing call", attrs...)
	} else {
		logger.Info("outgoing call", attrs...)
	}
	return resp, nil
}

// Redact hides secrets in a URL's query (Steam's ?key=).
func Redact(u *url.URL) string {
	q := u.Query()
	if !q.Has("key") {
		return u.String()
	}
	c := *u
	q.Set("key", "REDACTED")
	c.RawQuery = q.Encode()
	return c.String()
}

func oneLine(b []byte, max int) string {
	s := string(b)
	if len(s) > max {
		s = strings.ToValidUTF8(s[:max], "") + "…"
	}
	return strings.Join(strings.Fields(s), " ")
}
