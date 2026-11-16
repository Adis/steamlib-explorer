package paths

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDataDir(t *testing.T) {
	t.Setenv("STEAMLIB_DATA_DIR", "/custom/place")
	if dir, err := DataDir(); err != nil || dir != "/custom/place" {
		t.Errorf("override: DataDir = %q, %v", dir, err)
	}
	t.Setenv("STEAMLIB_DATA_DIR", "")

	if runtime.GOOS != "linux" {
		t.Skip("the rest checks the Linux folders")
	}
	t.Setenv("HOME", "/home/me")
	t.Setenv("XDG_DATA_HOME", "")
	if dir, _ := DataDir(); dir != "/home/me/.local/share/steamlib-explorer" {
		t.Errorf("default: DataDir = %q", dir)
	}
	t.Setenv("XDG_DATA_HOME", "/xdg/data")
	if dir, _ := DataDir(); dir != "/xdg/data/steamlib-explorer" {
		t.Errorf("XDG_DATA_HOME: DataDir = %q", dir)
	}
	t.Setenv("XDG_DATA_HOME", "relative/is/ignored")
	if db, _ := DefaultDatabase(); db != filepath.Join("/home/me/.local/share/steamlib-explorer", "steamlib.db") {
		t.Errorf("DefaultDatabase = %q", db)
	}
}
