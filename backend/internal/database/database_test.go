package database

import "testing"

func TestParse(t *testing.T) {
	for url, want := range map[string][2]string{
		"postgres://u:p@localhost:5432/steamlib?sslmode=disable": {"postgres", ""},
		"postgresql://localhost/steamlib":                        {"postgres", ""},
		"sqlite:steamlib.db":                                     {"sqlite", "steamlib.db"},
		"sqlite:/home/me/steamlib.db":                            {"sqlite", "/home/me/steamlib.db"},
		"sqlite:///home/me/steamlib.db":                          {"sqlite", "/home/me/steamlib.db"},
		`sqlite:C:\Users\me\steamlib.db`:                         {"sqlite", `C:\Users\me\steamlib.db`},
	} {
		engine, path, err := Parse(url)
		if err != nil || engine != want[0] || path != want[1] {
			t.Errorf("Parse(%q) = %q, %q, %v; want %q, %q", url, engine, path, err, want[0], want[1])
		}
	}
	for _, url := range []string{"", "mysql://x", "sqlite:", "sqlite:a.db?mode=ro", "steamlib.db"} {
		if _, _, err := Parse(url); err == nil {
			t.Errorf("Parse(%q) should fail", url)
		}
	}
}
