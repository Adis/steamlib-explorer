package apikey

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Adis/steamlib-explorer/backend/internal/settings"
)

// fakeKeyring is an in-memory keychain; with broken set, it acts like a
// machine without one.
type fakeKeyring struct {
	entries map[string]string
	broken  bool
}

var errNoKeychain = errors.New("no secret service")

func (f *fakeKeyring) Get(service, user string) (string, error) {
	if f.broken {
		return "", errNoKeychain
	}
	v, ok := f.entries[service+"/"+user]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (f *fakeKeyring) Set(service, user, secret string) error {
	if f.broken {
		return errNoKeychain
	}
	if f.entries == nil {
		f.entries = map[string]string{}
	}
	f.entries[service+"/"+user] = secret
	return nil
}

func newStore(t *testing.T, kr *fakeKeyring) *Store {
	return &Store{Keyring: kr, File: filepath.Join(t.TempDir(), "settings.json")}
}

func fileKey(t *testing.T, s *Store) string {
	t.Helper()
	saved, err := settings.Load(s.File)
	if err != nil {
		t.Fatal(err)
	}
	return saved.SteamAPIKey
}

func check(t *testing.T, s *Store, wantKey, wantWhere string) {
	t.Helper()
	key, where, err := s.Load()
	if err != nil || key != wantKey || where != wantWhere {
		t.Fatalf("Load() = %q, %q, %v; want %q, %q", key, where, err, wantKey, wantWhere)
	}
}

func TestNothingSaved(t *testing.T) {
	check(t, newStore(t, &fakeKeyring{}), "", "")
	check(t, newStore(t, &fakeKeyring{broken: true}), "", "")
}

func TestSaveUsesKeychain(t *testing.T) {
	kr := &fakeKeyring{}
	s := newStore(t, kr)
	if where, err := s.Save("ABC"); err != nil || where != InKeychain {
		t.Fatalf("Save() = %q, %v", where, err)
	}
	if fileKey(t, s) != "" {
		t.Error("the key should not be in settings.json")
	}
	check(t, s, "ABC", InKeychain)
}

func TestSaveFallsBackToFile(t *testing.T) {
	s := newStore(t, &fakeKeyring{broken: true})
	if where, err := s.Save("ABC"); err != nil || where != InSettingsFile {
		t.Fatalf("Save() = %q, %v", where, err)
	}
	if fileKey(t, s) != "ABC" {
		t.Error("the key should be in settings.json")
	}
	check(t, s, "ABC", InSettingsFile)
}

func TestMovesKeyFromFileToKeychain(t *testing.T) {
	kr := &fakeKeyring{}
	s := newStore(t, kr)
	if err := settings.Save(s.File, settings.Settings{SteamAPIKey: "OLD"}); err != nil {
		t.Fatal(err)
	}
	check(t, s, "OLD", InKeychain)
	if kr.entries[Service+"/"+User] != "OLD" {
		t.Error("the key should now be in the keychain")
	}
	if fileKey(t, s) != "" {
		t.Error("the key should be gone from settings.json")
	}
	check(t, s, "OLD", InKeychain)
}

func TestKeepsFileKeyWithoutKeychain(t *testing.T) {
	s := newStore(t, &fakeKeyring{broken: true})
	if err := settings.Save(s.File, settings.Settings{SteamAPIKey: "OLD"}); err != nil {
		t.Fatal(err)
	}
	check(t, s, "OLD", InSettingsFile)
	if fileKey(t, s) != "OLD" {
		t.Error("without a keychain the key must stay in settings.json")
	}
}

func TestFinishesInterruptedMove(t *testing.T) {
	kr := &fakeKeyring{entries: map[string]string{Service + "/" + User: "K"}}
	s := newStore(t, kr)
	if err := settings.Save(s.File, settings.Settings{SteamAPIKey: "K"}); err != nil {
		t.Fatal(err)
	}
	check(t, s, "K", InKeychain)
	if fileKey(t, s) != "" {
		t.Error("the leftover copy in settings.json should be removed")
	}
}

func TestKeychainWinsOverDifferentFileKey(t *testing.T) {
	kr := &fakeKeyring{entries: map[string]string{Service + "/" + User: "NEW"}}
	s := newStore(t, kr)
	if err := settings.Save(s.File, settings.Settings{SteamAPIKey: "OTHER"}); err != nil {
		t.Fatal(err)
	}
	check(t, s, "NEW", InKeychain)
	if fileKey(t, s) != "OTHER" {
		t.Error("a different key in settings.json is left alone")
	}
}
