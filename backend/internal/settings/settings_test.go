package settings

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "settings.json")
	s, err := Load(path)
	if err != nil || s.SteamAPIKey != "" {
		t.Fatalf("missing file: %+v, %v", s, err)
	}
	if err := Save(path, Settings{SteamAPIKey: "ABC"}); err != nil {
		t.Fatal(err)
	}
	s, err = Load(path)
	if err != nil || s.SteamAPIKey != "ABC" {
		t.Fatalf("after save: %+v, %v", s, err)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0o600 {
			t.Errorf("mode = %v, want 0600", info.Mode().Perm())
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("folder has %d files, want only settings.json", len(entries))
	}

	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("a broken file should be an error, not empty settings")
	}
}
