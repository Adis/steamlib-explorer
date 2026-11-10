package auth

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const testSteamID = "76561197960287930"

// memStore is an in-memory Store.
type memStore struct {
	mu       sync.Mutex
	users    map[uint64]User
	sessions map[string]uint64 // hex(token hash) -> steam id
}

func newMemStore() *memStore {
	return &memStore{users: map[uint64]User{}, sessions: map[string]uint64{}}
}

func (m *memStore) UpsertUser(_ context.Context, u User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.SteamID] = u
	return nil
}

func (m *memStore) CreateSession(_ context.Context, h []byte, id uint64, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[hex.EncodeToString(h)] = id
	return nil
}

func (m *memStore) SessionUser(_ context.Context, h []byte) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.sessions[hex.EncodeToString(h)]
	if !ok {
		return User{}, ErrNoSession
	}
	return m.users[id], nil
}

func (m *memStore) DeleteSession(_ context.Context, h []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, hex.EncodeToString(h))
	return nil
}

type fakeProfiles struct{ err error }

func (f fakeProfiles) Profile(_ context.Context, id uint64) (User, error) {
	if f.err != nil {
		return User{}, f.err
	}
	return User{SteamID: id, PersonaName: "Rabscuttle", AvatarURL: "https://avatars.example/a.jpg"}, nil
}

// fakeSteam answers check_authentication like Steam does.
func fakeSteam(t *testing.T, valid bool, gotMode *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if gotMode != nil {
			*gotMode = r.PostForm.Get("openid.mode")
		}
		w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:" + map[bool]string{true: "true", false: "false"}[valid] + "\n"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newService(steamURL string, store *memStore, profiles ProfileFetcher) *Service {
	return &Service{
		OpenID:    &OpenID{Endpoint: steamURL},
		Store:     store,
		Profiles:  profiles,
		PublicURL: "http://localhost:5173",
	}
}

func newMux(s *Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/steam/login", s.Login)
	mux.HandleFunc("GET /auth/steam/callback", s.Callback)
	mux.HandleFunc("POST /auth/logout", s.Logout)
	mux.Handle("GET /api/me", s.RequireUser(http.HandlerFunc(Me)))
	return mux
}

// startLogin hits /auth/steam/login and returns the state cookie and the
// return_to URL Steam would send the browser back to.
func startLogin(t *testing.T, h http.Handler) (*http.Cookie, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/steam/login", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("login status = %d", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := loc.Query()
	if q.Get("openid.mode") != "checkid_setup" || q.Get("openid.realm") != "http://localhost:5173/" {
		t.Fatalf("unexpected login query: %v", q)
	}
	var state *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == stateCookie {
			state = c
		}
	}
	if state == nil || !state.HttpOnly {
		t.Fatal("no HttpOnly state cookie")
	}
	return state, q.Get("openid.return_to")
}

// steamAnswer is the query Steam appends to return_to after a login.
func steamAnswer(returnTo, claimedID string) url.Values {
	return url.Values{
		"openid.ns":             {openIDNS},
		"openid.mode":           {"id_res"},
		"openid.op_endpoint":    {SteamOpenIDEndpoint},
		"openid.claimed_id":     {claimedID},
		"openid.identity":       {claimedID},
		"openid.return_to":      {returnTo},
		"openid.response_nonce": {"2026-11-10T12:00:00Zabc"},
		"openid.assoc_handle":   {"1234567890"},
		"openid.signed":         {"signed,op_endpoint,claimed_id,identity,return_to,response_nonce,assoc_handle"},
		"openid.sig":            {"c2lnbmF0dXJl"},
	}
}

func callback(t *testing.T, h http.Handler, returnTo string, answer url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	u, _ := url.Parse(returnTo)
	q := u.Query()
	for k, v := range answer {
		q[k] = v
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/steam/callback?"+q.Encode(), nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookie && c.Value != "" {
			return c
		}
	}
	return nil
}

func TestLoginFlow(t *testing.T) {
	var mode string
	steam := fakeSteam(t, true, &mode)
	store := newMemStore()
	h := newMux(newService(steam.URL, store, fakeProfiles{}))

	state, returnTo := startLogin(t, h)
	rec := callback(t, h, returnTo, steamAnswer(returnTo, "https://steamcommunity.com/openid/id/"+testSteamID), state)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "http://localhost:5173/" {
		t.Fatalf("callback = %d %s, body %s", rec.Code, rec.Header().Get("Location"), rec.Body)
	}
	if mode != "check_authentication" {
		t.Errorf("Steam was asked with mode %q", mode)
	}
	sess := sessionCookie(rec)
	if sess == nil || !sess.HttpOnly || sess.SameSite != http.SameSiteLaxMode {
		t.Fatalf("bad session cookie: %+v", sess)
	}
	if _, stored := store.sessions[hex.EncodeToString([]byte(sess.Value))]; stored {
		t.Error("store holds the raw token, want its hash")
	}

	// /api/me with the cookie returns the user.
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(sess)
	me := httptest.NewRecorder()
	h.ServeHTTP(me, req)
	var got User
	if err := json.NewDecoder(me.Body).Decode(&got); err != nil || me.Code != http.StatusOK {
		t.Fatalf("/api/me = %d, %v", me.Code, err)
	}
	if got.PersonaName != "Rabscuttle" || got.SteamID != 76561197960287930 {
		t.Errorf("/api/me user = %+v", got)
	}

	// Logout ends the session.
	req = httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(sess)
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	if out.Code != http.StatusNoContent || len(store.sessions) != 0 {
		t.Fatalf("logout = %d, sessions left %d", out.Code, len(store.sessions))
	}
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(sess)
	me = httptest.NewRecorder()
	h.ServeHTTP(me, req)
	if me.Code != http.StatusUnauthorized {
		t.Errorf("/api/me after logout = %d, want 401", me.Code)
	}
}

func TestMeWithoutSession(t *testing.T) {
	h := newMux(newService("http://unused", newMemStore(), fakeProfiles{}))
	for _, c := range []*http.Cookie{nil, {Name: SessionCookie, Value: "made-up"}} {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		if c != nil {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("cookie %v: status %d, want 401", c, rec.Code)
		}
	}
}

func TestCallbackRejects(t *testing.T) {
	goodClaim := "https://steamcommunity.com/openid/id/" + testSteamID
	tests := []struct {
		name       string
		steamValid bool
		mutate     func(q url.Values, returnTo string) url.Values
		dropState  bool
		want       int
	}{
		{"steam says invalid", false, nil, false, http.StatusUnauthorized},
		{"no state cookie", true, nil, true, http.StatusBadRequest},
		{"claimed id on another host", true, func(q url.Values, _ string) url.Values {
			q.Set("openid.claimed_id", "https://evil.example/openid/id/"+testSteamID)
			q.Set("openid.identity", "https://evil.example/openid/id/"+testSteamID)
			return q
		}, false, http.StatusUnauthorized},
		{"identity differs from claimed id", true, func(q url.Values, _ string) url.Values {
			q.Set("openid.identity", "https://steamcommunity.com/openid/id/76561197960287931")
			return q
		}, false, http.StatusUnauthorized},
		{"other op endpoint", true, func(q url.Values, _ string) url.Values {
			q.Set("openid.op_endpoint", "https://evil.example/openid/login")
			return q
		}, false, http.StatusUnauthorized},
		{"return_to for another site", true, func(q url.Values, _ string) url.Values {
			q.Set("openid.return_to", "https://evil.example/auth/steam/callback")
			return q
		}, false, http.StatusUnauthorized},
		{"cancelled on steam", true, func(q url.Values, _ string) url.Values {
			q.Set("openid.mode", "cancel")
			return q
		}, false, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemStore()
			h := newMux(newService(fakeSteam(t, tt.steamValid, nil).URL, store, fakeProfiles{}))
			state, returnTo := startLogin(t, h)
			answer := steamAnswer(returnTo, goodClaim)
			if tt.mutate != nil {
				answer = tt.mutate(answer, returnTo)
			}
			var cookies []*http.Cookie
			if !tt.dropState {
				cookies = append(cookies, state)
			}
			rec := callback(t, h, returnTo, answer, cookies...)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tt.want, strings.TrimSpace(rec.Body.String()))
			}
			if sessionCookie(rec) != nil || len(store.sessions) != 0 {
				t.Error("a session was created")
			}
		})
	}
}

func TestCallbackSurvivesProfileFailure(t *testing.T) {
	store := newMemStore()
	h := newMux(newService(fakeSteam(t, true, nil).URL, store, fakeProfiles{err: errors.New("steam down")}))
	state, returnTo := startLogin(t, h)
	rec := callback(t, h, returnTo, steamAnswer(returnTo, "https://steamcommunity.com/openid/id/"+testSteamID), state)
	if rec.Code != http.StatusFound || sessionCookie(rec) == nil {
		t.Fatalf("status = %d, want sign-in to succeed", rec.Code)
	}
	if u := store.users[76561197960287930]; u.SteamID == 0 || u.PersonaName != "" {
		t.Errorf("stored user = %+v", u)
	}
}

func TestSecureCookieOnHTTPS(t *testing.T) {
	s := newService("http://unused", newMemStore(), fakeProfiles{})
	s.PublicURL = "https://games.example.com"
	rec := httptest.NewRecorder()
	s.Login(rec, httptest.NewRequest(http.MethodGet, "/auth/steam/login", nil))
	for _, c := range rec.Result().Cookies() {
		if !c.Secure {
			t.Errorf("cookie %s is not Secure", c.Name)
		}
	}
}
