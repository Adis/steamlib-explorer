package config

import "testing"

func TestLoad(t *testing.T) {
	tests := []struct {
		name      string
		dbURL     string
		key       string
		port      string
		host      string
		publicURL string
		wantPort  int
		wantHost  string
		wantURL   string
		wantErr   bool
	}{
		{"defaults", "postgres://x", "k", "", "", "", 8080, "127.0.0.1", "http://localhost:8080", false},
		{"custom port", "postgres://x", "k", "9000", "", "", 9000, "127.0.0.1", "http://localhost:9000", false},
		{"public url trailing slash", "postgres://x", "k", "", "", "https://games.example.com/", 8080, "127.0.0.1", "https://games.example.com", false},
		{"listen on all interfaces", "postgres://x", "k", "", "0.0.0.0", "", 8080, "0.0.0.0", "http://localhost:8080", false},
		{"no db url uses the default SQLite file", "", "k", "", "", "", 8080, "127.0.0.1", "http://localhost:8080", false},
		{"no api key is fine: the setup page asks", "postgres://x", "", "", "", "", 8080, "127.0.0.1", "http://localhost:8080", false},
		{"bad port", "postgres://x", "k", "abc", "", "", 0, "", "", true},
		{"port out of range", "postgres://x", "k", "70000", "", "", 0, "", "", true},
		{"public url without scheme", "postgres://x", "k", "", "", "example.com", 0, "", "", true},
		{"public url with path", "postgres://x", "k", "", "", "https://example.com/app", 0, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", tt.dbURL)
			t.Setenv("STEAMLIB_DATA_DIR", "/data")
			t.Setenv("STEAM_API_KEY", tt.key)
			t.Setenv("PORT", tt.port)
			t.Setenv("LISTEN_HOST", tt.host)
			t.Setenv("PUBLIC_URL", tt.publicURL)
			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.dbURL == "" && cfg.DatabaseURL != "sqlite:/data/steamlib.db" {
				t.Errorf("DatabaseURL = %q, want the default SQLite file", cfg.DatabaseURL)
			}
			if cfg.Port != tt.wantPort || cfg.ListenHost != tt.wantHost || cfg.PublicURL != tt.wantURL {
				t.Errorf("Port = %d, ListenHost = %q, PublicURL = %q; want %d, %q, %q", cfg.Port, cfg.ListenHost, cfg.PublicURL, tt.wantPort, tt.wantHost, tt.wantURL)
			}
		})
	}
}
