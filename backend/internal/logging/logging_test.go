package logging

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransportLogsAndKeepsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"response":{"game_count":2}}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := &http.Client{Transport: &Transport{Logger: slog.New(slog.NewTextHandler(&buf, nil)), Service: "steam"}}
	resp, err := client.Get(srv.URL + "/IPlayerService/GetOwnedGames/v1/?key=SECRET&steamid=1")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"response":{"game_count":2}}` {
		t.Errorf("caller got body %q", body)
	}

	line := buf.String()
	for _, want := range []string{"service=steam", "status=200", "key=REDACTED", `game_count`} {
		if !strings.Contains(line, want) {
			t.Errorf("log line missing %q: %s", want, line)
		}
	}
	if strings.Contains(line, "SECRET") {
		t.Errorf("log leaks the API key: %s", line)
	}
}

func TestOneLineTruncates(t *testing.T) {
	got := oneLine([]byte("a\n  b\tc"+strings.Repeat("x", 50)), 10)
	if got != "a b cxxx…" {
		t.Errorf("oneLine = %q", got)
	}
}
