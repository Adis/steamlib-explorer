// Package logging sends the service's logs to the console and, when asked,
// to one text file per day, and logs every outgoing HTTP call and incoming
// API request.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Setup makes the default slog logger write human-readable lines to stdout
// and, when dir is set, also to dir/YYYY-MM-DD.log, a new file each day
// (local time). Call the returned function on shutdown to close the file.
func Setup(dir string, level slog.Level) (*slog.Logger, func() error, error) {
	var w io.Writer = os.Stdout
	closeFn := func() error { return nil }
	if dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, nil, fmt.Errorf("create log directory: %w", err)
		}
		daily := &DailyFile{Dir: dir}
		w = io.MultiWriter(os.Stdout, daily)
		closeFn = daily.Close
	}
	logger := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	return logger, closeFn, nil
}

// DailyFile appends to Dir/YYYY-MM-DD.log, switching to a new file when the
// day changes. It is safe for concurrent use.
type DailyFile struct {
	Dir string
	Now func() time.Time // nil means time.Now

	mu  sync.Mutex
	day string
	f   *os.File
}

func (d *DailyFile) Write(p []byte) (int, error) {
	now := time.Now
	if d.Now != nil {
		now = d.Now
	}
	day := now().Format("2006-01-02")

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.f == nil || day != d.day {
		if d.f != nil {
			_ = d.f.Close()
			d.f = nil
		}
		f, err := os.OpenFile(filepath.Join(d.Dir, day+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return 0, fmt.Errorf("open log file: %w", err)
		}
		d.f, d.day = f, day
	}
	return d.f.Write(p)
}

// Close closes the current file.
func (d *DailyFile) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.f == nil {
		return nil
	}
	err := d.f.Close()
	d.f = nil
	return err
}
