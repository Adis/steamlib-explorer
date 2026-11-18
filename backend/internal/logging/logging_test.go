package logging

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTransportLogsAndKeepsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":{"game_count":2}}`))
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

func TestDailyFileStartsANewFileEachDay(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 18, 23, 59, 0, 0, time.Local)
	d := &DailyFile{Dir: dir, Now: func() time.Time { return now }}
	defer d.Close()

	for _, line := range []string{"one\n", "two\n"} {
		if _, err := d.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	now = now.Add(2 * time.Minute)
	if _, err := d.Write([]byte("three\n")); err != nil {
		t.Fatal(err)
	}

	for name, want := range map[string]string{"2026-11-18.log": "one\ntwo\n", "2026-11-19.log": "three\n"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", name, got, err, want)
		}
	}
}
