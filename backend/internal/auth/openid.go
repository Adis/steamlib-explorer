// Package auth implements Sign in with Steam (OpenID 2.0) and cookie sessions.
package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	openIDNS         = "http://specs.openid.net/auth/2.0"
	identifierSelect = "http://specs.openid.net/auth/2.0/identifier_select"
	// SteamOpenIDEndpoint is Steam's OpenID provider.
	SteamOpenIDEndpoint = "https://steamcommunity.com/openid/login"
)

var claimedIDRe = regexp.MustCompile(`^https://steamcommunity\.com/openid/id/(\d{17})$`)

// ErrInvalidAssertion means the callback did not prove a Steam login.
var ErrInvalidAssertion = errors.New("auth: invalid Steam OpenID response")

// OpenID builds Steam login URLs and verifies Steam's answers.
type OpenID struct {
	Endpoint string // Steam's OpenID endpoint; replaced in tests
	HTTP     *http.Client
}

// LoginURL is where the browser goes to sign in. Steam sends it back to
// returnTo; realm is the site origin shown on Steam's login page.
func (o *OpenID) LoginURL(returnTo, realm string) string {
	q := url.Values{}
	q.Set("openid.ns", openIDNS)
	q.Set("openid.mode", "checkid_setup")
	q.Set("openid.return_to", returnTo)
	q.Set("openid.realm", realm)
	q.Set("openid.identity", identifierSelect)
	q.Set("openid.claimed_id", identifierSelect)
	return o.Endpoint + "?" + q.Encode()
}

// Verify checks the query Steam sent to the callback and returns the user's
// 64-bit SteamID. expectedReturnTo is the return_to sent in LoginURL; the
// signature is confirmed by asking Steam directly (check_authentication).
func (o *OpenID) Verify(ctx context.Context, q url.Values, expectedReturnTo string) (uint64, error) {
	if q.Get("openid.mode") != "id_res" {
		return 0, fmt.Errorf("%w: mode %q", ErrInvalidAssertion, q.Get("openid.mode"))
	}
	if q.Get("openid.return_to") != expectedReturnTo {
		return 0, fmt.Errorf("%w: return_to mismatch", ErrInvalidAssertion)
	}
	if q.Get("openid.op_endpoint") != SteamOpenIDEndpoint {
		return 0, fmt.Errorf("%w: unexpected op_endpoint", ErrInvalidAssertion)
	}
	claimed := q.Get("openid.claimed_id")
	m := claimedIDRe.FindStringSubmatch(claimed)
	if m == nil || q.Get("openid.identity") != claimed {
		return 0, fmt.Errorf("%w: bad claimed_id", ErrInvalidAssertion)
	}
	steamID, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: bad steam id", ErrInvalidAssertion)
	}

	// Ask Steam whether it really signed these fields. Steam also rejects a
	// response_nonce it has already seen, which stops replays.
	check := url.Values{}
	for k, v := range q {
		if strings.HasPrefix(k, "openid.") && len(v) > 0 {
			check.Set(k, v[0])
		}
	}
	check.Set("openid.mode", "check_authentication")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Endpoint, strings.NewReader(check.Encode()))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := o.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("auth: contact Steam: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK || !isValid(string(body)) {
		return 0, fmt.Errorf("%w: Steam did not confirm the login", ErrInvalidAssertion)
	}
	return steamID, nil
}

// isValid reads the key:value reply of check_authentication.
func isValid(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "is_valid:true" {
			return true
		}
	}
	return false
}
