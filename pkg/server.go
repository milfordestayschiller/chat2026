package barertc

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"git.kirsle.net/apps/barertc/pkg/config"
	"git.kirsle.net/apps/barertc/pkg/log"
	"git.kirsle.net/apps/barertc/pkg/models"
)

type Server struct {
	upSince                time.Time
	mux                    *http.ServeMux
	subscriberMessageBuffer int
	subscribersMu          sync.RWMutex
	subscribers            map[*Subscriber]struct{}
	logfh                  map[string]io.WriteCloser
}

func NewServer() *Server {
	return &Server{
		subscriberMessageBuffer: 32,
		subscribers:             make(map[*Subscriber]struct{}),
	}
}

func (s *Server) Setup() error {
	if config.Current.DirectMessageHistory.Enabled {
		if err := models.Initialize(config.Current.DirectMessageHistory.SQLiteDatabase); err != nil {
			log.Error("Error initializing SQLite database: %s", err)
		}
	}

	var mux = http.NewServeMux()

	// Rutas existentes
	mux.Handle("/", s.JWTMiddleware(IndexPage()))
	mux.Handle("/psi", PsiPage())
	mux.Handle("/api/bans", GetBansAPI())
	mux.Handle("/psi2", PsiPage2())
	mux.Handle("/api/bans2", GetBansAPI2())
	mux.HandleFunc("/api/ban", AddBanAPI())
	mux.HandleFunc("/api/buscar", BuscarUsuarioAPI())
	mux.HandleFunc("/api/ban2", AddBanAPI2())
	mux.HandleFunc("/api/unban2", UnbanAPI())
	mux.Handle("/about", AboutPage())
	mux.Handle("/logout", LogoutPage())
	mux.Handle("/ws", s.WebSocket())
	mux.Handle("/poll", s.PollingAPI())
	mux.Handle("/api/statistics", s.Statistics())
	mux.Handle("/api/blocklist", s.BlockList())
	mux.Handle("/api/block/now", s.BlockNow())
	mux.Handle("/api/disconnect/now", s.DisconnectNow())
	mux.Handle("/api/shutdown", s.ShutdownAPI())
	mux.Handle("/api/profile", s.UserProfile())
	mux.Handle("/api/message/history", s.MessageHistory())
	mux.Handle("/api/message/usernames", s.MessageUsernameHistory())
	mux.Handle("/api/message/clear", s.ClearMessages())
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("dist/assets"))))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("dist/static"))))

	// Nuevas rutas de autenticación
	mux.HandleFunc("/api/register", s.HandleRegister)
	mux.HandleFunc("/api/login", s.HandleLogin)

	s.mux = mux
	return nil
}

func (s *Server) ListenAndServe(address string) error {
	s.upSince = time.Now()
	go s.KickIdlePollUsers()
	go s.sendWhoListAfterReady()
	return http.ListenAndServe(address, s.mux)
}

func (s *Server) sendWhoListAfterReady() {
	time.Sleep(16 * time.Second)
	log.Info("Up 15 seconds, sending WhoList to any online chatters")
	s.SendWhoList()
}

// Middleware JWT
func (s *Server) JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !config.Current.JWT.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		jwtToken := r.URL.Query().Get("jwt")
		if jwtToken == "" {
			if config.Current.JWT.Strict {
				http.Redirect(w, r, config.Current.JWT.LandingPageURL, http.StatusSeeOther)
				return
			} else {
				next.ServeHTTP(w, r)
				return
			}
		}

		token, err := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
			return []byte(config.Current.JWT.SecretKey), nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Invalid JWT", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Invalid claims", http.StatusUnauthorized)
			return
		}

		if op, found := claims["op"].(bool); found && op {
			r.Header.Set("X-User", claims["username"].(string))
			r.Header.Set("X-Op", "true")
		}

		next.ServeHTTP(w, r)
	})
}

// Registro
func (s *Server) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		http.Error(w, "Faltan campos", http.StatusBadRequest)
		return
	}

	if userExists(username) {
		http.Error(w, "El usuario ya existe", http.StatusConflict)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error interno", http.StatusInternalServerError)
		return
	}

	f, err := os.OpenFile(".users.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		http.Error(w, "No se puede guardar", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s:%s", username, string(hashedPassword))
	if err != nil {
		http.Error(w, "Error al escribir", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Login
func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		http.Error(w, "Faltan campos", http.StatusBadRequest)
		return
	}

	file, err := os.Open(".users.txt")
	if err != nil {
		http.Error(w, "No autorizado", http.StatusUnauthorized)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		user := parts[0]
		hashed := parts[1]

		if user == username {
			err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
			if err == nil {
				moderators := map[string]bool{
					"Killer":   true,
					"Cris":     true,
					"Ricotera": true,
					"Denisse":  true,
					"Stanlydark": true,
				}

				if moderators[username] {
					token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
						"username":  username,
						"moderator": true,
					})
					tokenString, err := token.SignedString([]byte(config.Current.JWT.SecretKey))
					if err != nil {
						http.Error(w, "Error al generar token", http.StatusInternalServerError)
						return
					}
					redirectURL := fmt.Sprintf("/?jwt=%s", tokenString)
					http.Redirect(w, r, redirectURL, http.StatusSeeOther)
					return
				}

				w.WriteHeader(http.StatusOK)
				return
			} else {
				http.Error(w, "Contraseña incorrecta", http.StatusUnauthorized)
				return
			}
		}
	}

	http.Error(w, "Usuario no encontrado", http.StatusUnauthorized)
}

func userExists(username string) bool {
	file, err := os.Open(".users.txt")
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && parts[0] == username {
			return true
		}
	}
	return false
}
