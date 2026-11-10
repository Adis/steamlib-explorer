package config

import "testing"

func TestLoad(t *testing.T) {
	tests := []struct {
		name      string
		dbURL     string
		key       string
		port      string
		publicURL string
		wantPort  int
		wantURL   string
		wantErr   bool
	}{
		{"defaults", "postgres://x", "k", "", "", 8080, "http://localhost:8080", false},
		{"custom port", "postgres://x", "k", "9000", "", 9000, "http://localhost:9000", false},
		{"public url trailing slash", "postgres://x", "k", "", "https://games.example.com/", 8080, "https://games.example.com", false},
		{"missing db url", "", "k", "", "", 0, "", true},
		{"missing api key", "postgres://x", "", "", "", 0, "", true},
		{"bad port", "postgres://x", "k", "abc", "", 0, "", true},
		{"port out of range", "postgres://x", "k", "70000", "", 0, "", true},
		{"public url without scheme", "postgres://x", "k", "", "example.com", 0, "", true},
		{"public url with path", "postgres://x", "k", "", "https://example.com/app", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", tt.dbURL)
			t.Setenv("STEAM_API_KEY", tt.key)
			t.Setenv("PORT", tt.port)
			t.Setenv("PUBLIC_URL", tt.publicURL)
			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if cfg.Port != tt.wantPort || cfg.PublicURL != tt.wantURL {
				t.Errorf("Port = %d, PublicURL = %q; want %d, %q", cfg.Port, cfg.PublicURL, tt.wantPort, tt.wantURL)
			}
		})
	}
}
