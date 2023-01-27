package barertc

import (
	"html/template"
	"math/rand"
	"net/http"

	"git.kirsle.net/apps/barertc/pkg/log"
)

// IndexPage returns the HTML template for the chat room.
func IndexPage() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Load the template, TODO: once on server startup.
		tmpl := template.New("index")
		tmpl.Funcs(template.FuncMap{
			// Cache busting random string for JS and CSS dependency.
			"CacheHash": func() string {
				return RandomString(8)
			},
		})
		tmpl, err := tmpl.ParseFiles("web/templates/chat.html")
		if err != nil {
			panic(err.Error())
		}
		// END load the template

		log.Info("Index route hit")
		tmpl.ExecuteTemplate(w, "index", nil)
	})
}

// RandomString returns a random string of any length.
func RandomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz"
	var result = make([]byte, n)
	for i := 0; i < n; i++ {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}
