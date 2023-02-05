package barertc

import (
	"html/template"
	"net/http"

	"git.kirsle.net/apps/barertc/pkg/config"
	"git.kirsle.net/apps/barertc/pkg/log"
	"git.kirsle.net/apps/barertc/pkg/util"
)

// IndexPage returns the HTML template for the chat room.
func IndexPage() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Load the template, TODO: once on server startup.
		tmpl := template.New("index")

		// Variables to give to the front-end page.
		var values = map[string]interface{}{
			// A cache-busting hash for JS and CSS includes.
			"CacheHash": util.RandomString(8),

			// The current website settings.
			"Config": config.Current,
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
