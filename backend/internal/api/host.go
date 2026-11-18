package api

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// CheckHost answers 403 to requests whose Host header is not an address this
// app is reached at: localhost, an IP address, or the host in publicURL.
//
// This stops DNS rebinding: a website whose domain name is made to point at
// 127.0.0.1 could otherwise talk to the app from the browser as if it were
// the same site (e.g. the setup page before a key is saved). Such requests
// carry the attacker's domain name as Host. The Vite dev server applies the
// same rule on its port.
func CheckHost(publicURL string) func(http.Handler) http.Handler {
	allowed := map[string]bool{"localhost": true}
	if u, err := url.Parse(publicURL); err == nil && u.Hostname() != "" {
		allowed[strings.ToLower(u.Hostname())] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !hostAllowed(r.Host, allowed) {
				http.Error(w, "This address isn't allowed. Open the app at its PUBLIC_URL, or set PUBLIC_URL to the address you use.", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hostAllowed(hostport string, allowed map[string]bool) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(strings.Trim(host, "[]"), "."))
	// An IP address can't be rebound: only domain names are looked up.
	return allowed[host] || net.ParseIP(host) != nil
}
