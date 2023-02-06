package barertc

import (
	"fmt"
	"html/template"
	"net/http"

	"git.kirsle.net/apps/barertc/pkg/config"
	"git.kirsle.net/apps/barertc/pkg/jwt"
	"git.kirsle.net/apps/barertc/pkg/log"
	"git.kirsle.net/apps/barertc/pkg/util"
)

// IndexPage returns the HTML template for the chat room.
func IndexPage() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Load the template, TODO: once on server startup.
		tmpl := template.New("index")

		// Handle a JWT authentication token.
		var (
			tokenStr = r.FormValue("jwt")
			claims   = &jwt.Claims{}
			authOK   bool
		)
		if tokenStr != "" {
			parsed, ok, err := jwt.ParseAndValidate(tokenStr)
			if err != nil {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(
					fmt.Sprintf("Error parsing your JWT token: %s", err),
				))
				return
			}

			authOK = ok
			claims = parsed
		}

		// Are we enforcing strict JWT authentication?
		if config.Current.JWT.Enabled && config.Current.JWT.Strict && !authOK {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(
				fmt.Sprintf("Authentication denied. Please go back and try again."),
			))
			return
		}

		// Variables to give to the front-end page.
		var values = map[string]interface{}{
			// A cache-busting hash for JS and CSS includes.
			"CacheHash": util.RandomString(8),

			// The current website settings.
			"Config": config.Current,

			// Authentication settings.
			"JWTTokenString": tokenStr,
			"JWTAuthOK":      authOK,
			"JWTClaims":      claims,
		}

		tmpl.Funcs(template.FuncMap{
			// Cache busting random string for JS and CSS dependency.
			// "CacheHash": func() string {
			// 	return util.RandomString(8)
			// },

			//
		})
		tmpl, err := tmpl.ParseFiles("web/templates/chat.html")
		if err != nil {
			panic(err.Error())
		}
		// END load the template

		log.Info("Index route hit")
		tmpl.ExecuteTemplate(w, "index", values)
	})
}
