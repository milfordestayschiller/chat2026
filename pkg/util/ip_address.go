package util

import (
	"net"
	"net/http"
	"strings"
)

/*
IPAddress returns the best guess at the user's IP address, as a string for logging.
It prioritizes common proxy headers regardless of configuration.
*/
func IPAddress(r *http.Request) string {
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.SplitN(xff, ",", 2)[0]
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // fallback with port
	}
	return host
}
