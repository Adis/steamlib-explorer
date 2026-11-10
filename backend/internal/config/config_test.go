package config

import "testing"

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		dbURL   string
		port    string
		want    int
		wantErr bool
	}{
		{"defaults", "postgres://x", "", 8080, false},
		{"custom port", "postgres://x", "9000", 9000, false},
		{"missing db url", "", "", 0, true},
		{"bad port", "postgres://x", "abc", 0, true},
		{"port out of range", "postgres://x", "70000", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", tt.dbURL)
			t.Setenv("PORT", tt.port)
			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && cfg.Port != tt.want {
				t.Errorf("Port = %d, want %d", cfg.Port, tt.want)
			}
		})
	}
}
