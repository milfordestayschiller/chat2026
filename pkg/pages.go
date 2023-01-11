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
				const charset = "abcdefghijklmnopqrstuvwxyz"
				var result = make([]byte, 8)
				for i := 0; i < 8; i++ {
					result[i] = charset[rand.Intn(len(charset))]
				}
				return string(result)
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
