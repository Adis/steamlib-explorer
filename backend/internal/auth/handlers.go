package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNoSession means the request has no valid session.
var ErrNoSession = errors.New("auth: no session")

// ProfileFetcher looks up a user's name and avatar after sign-in.
type ProfileFetcher interface {
	Profile(ctx context.Context, steamID uint64) (User, error)
}

// Service wires OpenID, the store and cookies together.
type Service struct {
	OpenID   *OpenID
	Store    Store
	Profiles ProfileFetcher
	// PublicURL is the address browsers use for the app, e.g.
	// http://localhost:5173 in development. Callbacks and redirects use it.
	PublicURL string
	Now       func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) secure() bool { return strings.HasPrefix(s.PublicURL, "https://") }

func (s *Service) callbackURL(state string) string {
	return s.PublicURL + "/auth/steam/callback?state=" + url.QueryEscape(state)
}

// Login redirects to Steam's sign-in page.
func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	state, err := newToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: stateCookie, Value: state, Path: "/auth/steam",
		MaxAge: 600, HttpOnly: true, Secure: s.secure(), SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.OpenID.LoginURL(s.callbackURL(state), s.PublicURL+"/"), http.StatusFound)
}

// Callback verifies Steam's answer, creates the user and session, and sends
// the browser back to the app.
func (s *Service) Callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	c, err := r.Cookie(stateCookie)
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		http.Error(w, "login expired or started in another browser, please try again", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Path: "/auth/steam", MaxAge: -1})

	steamID, err := s.OpenID.Verify(r.Context(), r.URL.Query(), s.callbackURL(state))
	if err != nil {
		slog.Warn("steam login rejected", "error", err)
		http.Error(w, "Steam sign-in could not be verified", http.StatusUnauthorized)
		return
	}

	user := User{SteamID: steamID}
	if p, err := s.Profiles.Profile(r.Context(), steamID); err == nil {
		user = p
		user.SteamID = steamID
	} else {
		// A missing name or avatar shouldn't block sign-in.
		slog.Warn("steam profile lookup failed", "steam_id", steamID, "error", err)
	}
	if err := s.Store.UpsertUser(r.Context(), user); err != nil {
		slog.Error("save user", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	token, err := newToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	expires := s.now().Add(SessionTTL)
	if err := s.Store.CreateSession(r.Context(), hashToken(token), steamID, expires); err != nil {
		slog.Error("create session", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: token, Path: "/", Expires: expires,
		HttpOnly: true, Secure: s.secure(), SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.PublicURL+"/", http.StatusFound)
}

// Logout deletes the session and clears the cookie.
func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookie); err == nil {
		if err := s.Store.DeleteSession(r.Context(), hashToken(c.Value)); err != nil {
			slog.Error("delete session", "error", err)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.secure(), SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

type ctxKey struct{}

// RequireUser rejects requests without a valid session with 401 and puts the
// user in the request context for the handlers behind it.
func (s *Service) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookie)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		user, err := s.Store.SessionUser(r.Context(), hashToken(c.Value))
		if errors.Is(err, ErrNoSession) {
			writeUnauthorized(w)
			return
		}
		if err != nil {
			slog.Error("load session", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, user)))
	})
}

// UserFrom returns the signed-in user set by RequireUser.
func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// Me answers GET /api/me with the signed-in user.
func Me(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFrom(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(u)
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"not signed in"}` + "\n"))
}
