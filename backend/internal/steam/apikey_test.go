package steam

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestCheckAPIKey(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("key") {
		case "good":
			w.Write([]byte(`{"response":{"players":[{"steamid":"76561197960287930"}]}}`))
		case "bad":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	ctx := context.Background()
	if err := c.CheckAPIKey(ctx, "good"); err != nil {
		t.Errorf("good key: %v", err)
	}
	if err := c.CheckAPIKey(ctx, "bad"); !errors.Is(err, ErrBadAPIKey) {
		t.Errorf("bad key: err = %v, want ErrBadAPIKey", err)
	}
	if err := c.CheckAPIKey(ctx, "other"); err == nil || errors.Is(err, ErrBadAPIKey) {
		t.Errorf("server error: err = %v, want a non-key error", err)
	}

	// Checking doesn't switch keys; SetAPIKey does.
	if c.APIKey() != "test-key" {
		t.Errorf("APIKey = %q after checks", c.APIKey())
	}
	c.SetAPIKey("good")
	if c.APIKey() != "good" {
		t.Errorf("APIKey = %q after SetAPIKey", c.APIKey())
	}
}

// A request that fails before Steam answers must not put the key in the error.
func TestNetworkErrorHidesKey(t *testing.T) {
	c := New("secret-key", WithBaseURLs("http://127.0.0.1:1", "http://127.0.0.1:1"))
	err := c.CheckAPIKey(context.Background(), "another-secret")
	if err == nil {
		t.Fatal("want a connection error")
	}
	if msg := err.Error(); strings.Contains(msg, "secret") || !strings.Contains(msg, "REDACTED") {
		t.Errorf("error = %q, want the key redacted", msg)
	}
}
