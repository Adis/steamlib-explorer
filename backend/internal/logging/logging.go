// Package logging sends the service's logs to the console and to a text file,
// and logs every outgoing HTTP call and incoming API request.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Setup makes the default slog logger write human-readable lines to stdout
// and, when path is set, append them to that file too. Call the returned
// function on shutdown to close the file.
func Setup(path string, level slog.Level) (*slog.Logger, func() error, error) {
	var w io.Writer = os.Stdout
	closeFn := func() error { return nil }
	if path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, nil, fmt.Errorf("create log directory: %w", err)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file: %w", err)
		}
		w = io.MultiWriter(os.Stdout, f)
		closeFn = f.Close
	}
	logger := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	return logger, closeFn, nil
}
