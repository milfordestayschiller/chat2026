
package barertc

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"git.kirsle.net/apps/barertc/pkg/config"
	"git.kirsle.net/apps/barertc/pkg/jwt"
	"git.kirsle.net/apps/barertc/pkg/log"
	"git.kirsle.net/apps/barertc/pkg/util"
)

// IndexPage returns the HTML template for the chat room.
func IndexPage() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := util.IPAddress(r)
		if isIPBanned(ip) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Acceso denegado. Tu IP ha sido baneada."))
			return
		}

		tmpl := template.New("index")

		var (
			tokenStr  = r.FormValue("jwt")
			claims    = &jwt.Claims{}
			authOK    bool
			blocklist = []string{}
		)
		if tokenStr != "" {
			parsed, ok, err := jwt.ParseAndValidate(tokenStr)
			if err != nil {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(fmt.Sprintf("Error parsing your JWT token: %s", err)))
				return
			}
			authOK = ok
			claims = parsed
			blocklist = GetCachedBlocklist(claims.Subject)
		}

		if config.Current.JWT.Enabled && config.Current.JWT.Strict && !authOK {
			if config.Current.JWT.LandingPageURL != "" {
				w.Header().Add("Location", config.Current.JWT.LandingPageURL)
				w.WriteHeader(http.StatusFound)
				return
			}
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Authentication denied. Please go back and try again."))
			return
		}

		var values = map[string]interface{}{
			"CacheHash":       util.RandomString(8),
			"Config":          config.Current,
			"JWTTokenString":  tokenStr,
			"JWTAuthOK":       authOK,
			"JWTClaims":       claims,
			"CachedBlocklist": blocklist,
		}

		tmpl.Funcs(template.FuncMap{
			"AsHTML": func(v string) template.HTML {
				return template.HTML(v)
			},
			"AsJS": func(v interface{}) template.JS {
				return template.JS(fmt.Sprintf("%v", v))
			},
		})
		tmpl, err := tmpl.ParseFiles("dist/index.html")
		if err != nil {
			panic(err.Error())
		}

		log.Info("GET / [%s] %s", util.IPAddress(r), strings.Join([]string{
			r.Header.Get("X-Forwarded-For"),
			r.Header.Get("X-Real-IP"),
			r.Header.Get("User-Agent"),
			util.IPAddress(r),
		}, " "))
		tmpl.ExecuteTemplate(w, "index", values)
	})
}

// AboutPage returns the HTML template for the about page.
func AboutPage() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.New("index")

		var values = map[string]interface{}{
			"CacheHash": util.RandomString(8),
			"Config":    config.Current,
			"Hostname":  r.Host,
		}

		tmpl.Funcs(template.FuncMap{
			"AsHTML": func(v string) template.HTML {
				return template.HTML(v)
			},
		})
		tmpl, err := tmpl.ParseFiles("web/templates/about.html")
		if err != nil {
			panic(err.Error())
		}

		tmpl.ExecuteTemplate(w, "index", values)
	})
}

// LogoutPage returns the HTML template for the logout page.
func LogoutPage() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.New("index")
		tmpl, err := tmpl.ParseFiles("web/templates/logout.html")
		if err != nil {
			panic(err.Error())
		}
		tmpl.ExecuteTemplate(w, "index", nil)
	})
}

// PsiPage returns the HTML template for the psi.html page.
func PsiPage() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.New("psi")
		tmpl, err := tmpl.ParseFiles("psi.html")
		if err != nil {
			http.Error(w, "Error cargando psi.html: "+err.Error(), 500)
			return
		}
		tmpl.ExecuteTemplate(w, "psi", nil)
	})
}

// PsiPage2 para psi2.html
func PsiPage2() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.New("psi")
		tmpl, err := tmpl.ParseFiles("psi2.html")
		if err != nil {
			http.Error(w, "Error cargando psi2.html: "+err.Error(), 500)
			return
		}
		tmpl.ExecuteTemplate(w, "psi2", nil)
	})
}

// GetBansAPI devuelve el contenido de datos.txt
func GetBansAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile("datos.txt")
		if err != nil {
			http.Error(w, "Error leyendo datos.txt: "+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write(data)
	}
}

// GetBansAPI2 devuelve el contenido de datos2.txt
func GetBansAPI2() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile("datos2.txt")
		if err != nil {
			http.Error(w, "Error leyendo datos2.txt: "+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write(data)
	}
}

// AddBanAPI agrega IP y nick al archivo datos.txt
func AddBanAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Error al procesar formulario", 400)
			return
		}
		ip := util.IPAddress(r)
		nick := r.FormValue("nick")
		if ip == "" || nick == "" {
			http.Error(w, "IP o nick vacío", 400)
			return
		}
		linea := fmt.Sprintf("Nick: %s | IP: %s\n", nick, ip)
		exePath, err := os.Executable()
		if err != nil {
			http.Error(w, "Error al obtener ruta ejecutable", 500)
			return
		}
		rutaArchivo := filepath.Join(filepath.Dir(exePath), "datos.txt")
		f, err := os.OpenFile(rutaArchivo, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			http.Error(w, "Error al abrir datos.txt: "+err.Error(), 500)
			return
		}
		defer f.Close()
		if _, err := f.WriteString(linea); err != nil {
			http.Error(w, "Error al escribir: "+err.Error(), 500)
			return
		}
		fmt.Fprintf(w, "Nick %s con IP %s baneado con éxito.", nick, ip)
	}
}

// AddBanAPI2 agrega IP al archivo datos2.txt
func AddBanAPI2() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Error al procesar formulario", 400)
			return
		}
		ip := r.FormValue("ip")
		if ip == "" {
			http.Error(w, "IP vacía", 400)
			return
		}
		f, err := os.OpenFile("datos2.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			http.Error(w, "Error al abrir datos2.txt: "+err.Error(), 500)
			return
		}
		defer f.Close()
		if _, err := f.WriteString(ip + "\n"); err != nil {
			http.Error(w, "Error al escribir: "+err.Error(), 500)
			return
		}
		fmt.Fprintf(w, "IP %s baneada con éxito.", ip)
	}
}

// UnbanAPI elimina una IP del archivo datos2.txt
func UnbanAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Error al procesar formulario", 400)
			return
		}
		ip := strings.TrimSpace(r.FormValue("ip"))
		if ip == "" {
			http.Error(w, "IP vacía", 400)
			return
		}

		ruta := "datos2.txt"
		data, err := os.ReadFile(ruta)
		if err != nil {
			http.Error(w, "Error leyendo datos2.txt: "+err.Error(), 500)
			return
		}

		lines := strings.Split(string(data), "\n")
		var nuevasLineas []string
		for _, line := range lines {
			if strings.TrimSpace(line) != ip {
				nuevasLineas = append(nuevasLineas, line)
			}
		}

		err = os.WriteFile(ruta, []byte(strings.Join(nuevasLineas, "\n")), 0644)
		if err != nil {
			http.Error(w, "Error escribiendo datos2.txt: "+err.Error(), 500)
			return
		}

		fmt.Fprintf(w, "IP %s eliminada de la lista de baneos.", ip)
	}
}

// isIPBanned revisa si la IP está en ambos archivos: datos.txt y datos2.txt
func isIPBanned(ip string) bool {
	foundInDatos := false
	foundInDatos2 := false

	if data, err := os.ReadFile("datos.txt"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "IP: "+ip) {
				foundInDatos = true
				break
			}
		}
	}

	if data, err := os.ReadFile("datos2.txt"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == ip {
				foundInDatos2 = true
				break
			}
		}
	}

	return foundInDatos && foundInDatos2
}

// BuscarUsuarioAPI permite buscar coincidencias de texto en datos.txt
func BuscarUsuarioAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if query == "" {
			http.Error(w, "Parámetro 'query' requerido", http.StatusBadRequest)
			return
		}

		data, err := os.ReadFile("datos.txt")
		if err != nil {
			http.Error(w, "Error leyendo datos.txt: "+err.Error(), http.StatusInternalServerError)
			return
		}

		lines := strings.Split(string(data), "\n")
		var resultados []string
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				resultados = append(resultados, line)
			}
		}

		if len(resultados) == 0 {
			w.Write([]byte("No se encontraron coincidencias."))
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(strings.Join(resultados, "\n")))
	}
}

