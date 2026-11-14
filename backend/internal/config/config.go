// Package config reads the service settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL string // DATABASE_URL, required
	Port        int    // PORT, default 8080
	// ListenHost is the interface the backend listens on (LISTEN_HOST, default
	// 127.0.0.1, so only this machine can connect; 0.0.0.0 opens it to the network).
	ListenHost  string
	SteamAPIKey string // STEAM_API_KEY, required
	// PublicURL is the address browsers use for the app (PUBLIC_URL, default
	// http://localhost:8080). Steam sends users back here after sign-in.
	PublicURL string
	// LogFile also receives every log line (LOG_FILE, default logs/api-calls.txt;
	// set to "-" to log to the console only).
	LogFile string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        8080,
		ListenHost:  os.Getenv("LISTEN_HOST"),
		SteamAPIKey: os.Getenv("STEAM_API_KEY"),
		PublicURL:   strings.TrimRight(os.Getenv("PUBLIC_URL"), "/"),
		LogFile:     os.Getenv("LOG_FILE"),
	}
	switch cfg.LogFile {
	case "":
		cfg.LogFile = "logs/api-calls.txt"
	case "-":
		cfg.LogFile = ""
	}
	if cfg.ListenHost == "" {
		cfg.ListenHost = "127.0.0.1"
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL is required")
	}
	if cfg.SteamAPIKey == "" {
		return cfg, errors.New("STEAM_API_KEY is required (get one at https://steamcommunity.com/dev/apikey)")
	}
	if p := os.Getenv("PORT"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return cfg, fmt.Errorf("PORT must be a port number, got %q", p)
		}
		cfg.Port = n
	}
	if cfg.PublicURL == "" {
		cfg.PublicURL = fmt.Sprintf("http://localhost:%d", cfg.Port)
	}
	if u, err := url.Parse(cfg.PublicURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || (u.Path != "" && u.Path != "/") {
		return cfg, fmt.Errorf("PUBLIC_URL must look like https://example.com, got %q", cfg.PublicURL)
	}
	return cfg, nil
}
