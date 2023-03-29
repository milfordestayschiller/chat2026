package barertc

import (
	"encoding/json"
	"net/http"

	"git.kirsle.net/apps/barertc/pkg/config"
)

// Statistics (/api/statistics) returns info about the users currently logged onto the chat,
// for your website to call via CORS. The URL to your site needs to be in the CORSHosts array
// of your settings.toml.
func (s *Server) Statistics() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle the CORS header from your trusted domains.
		if origin := r.Header.Get("Origin"); origin != "" {
			var found bool
			for _, allowed := range config.Current.CORSHosts {
				if allowed == origin {
					found = true
				}
			}

			if found {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}

		var result = struct {
			UserCount int
			Usernames []string
		}{
			Usernames: []string{},
		}

		// Count all users + collect unique usernames.
		var unique = map[string]struct{}{}
		for _, sub := range s.IterSubscribers() {
			if sub.authenticated && sub.ChatStatus != "hidden" {
				result.UserCount++
				if _, ok := unique[sub.Username]; ok {
					continue
				}
				result.Usernames = append(result.Usernames, sub.Username)
				unique[sub.Username] = struct{}{}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	})
}
