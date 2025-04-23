package util

import (
	"net"
	"net/http"
	"strings"

	"git.kirsle.net/apps/barertc/pkg/config"
)

/*
IPAddress returns the best guess at the user's IP address, as a string for logging.
*/
func IPAddress(r *http.Request) string {
	if config.Current.UseXForwardedFor {
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			return realIP
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.SplitN(xff, ",", 2)[0]
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // fallback con puerto si falla
	}
	return host
}
