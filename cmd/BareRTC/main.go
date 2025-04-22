
package main

import (
    "flag"
    "fmt"
    "math/rand"
    "time"

    barertc "git.kirsle.net/apps/barertc/pkg"
    "git.kirsle.net/apps/barertc/pkg/config"
    "git.kirsle.net/apps/barertc/pkg/log"

    "github.com/golang-jwt/jwt/v5"
)

func init() {
    rand.Seed(time.Now().UnixNano())
}

func GenerateJWT(username string, esOp bool) (string, error) {
    now := time.Now()
    claims := jwt.MapClaims{
        "sub": username,
        "username": username,
        "nick": username,
        "img": "/static/photos/" + username + ".jpg",
        "url": "/u/" + username,
        "gender": "m",
        "emoji": "🤖",
        "rules": []string{"redcam", "noimage"},
        "iss": "my own app",
        "iat": now.Unix(),
        "nbf": now.Unix(),
        "exp": now.Add(12 * time.Hour).Unix(),
    }
    if esOp {
        claims["op"] = true
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(config.Current.JWT.SecretKey))
}

func main() {
    // Command line flags.
    var (
        debug   bool
        address string
    )
    flag.BoolVar(&debug, "debug", false, "Enable debug-level logging in the app.")
    flag.StringVar(&address, "address", ":9000", "Address to listen on, like localhost:5000 or :8080")
    flag.Parse()

    if debug {
        log.SetDebug(true)
    }

    // Load configuration.
    if err := config.LoadSettings(); err != nil {
        panic(fmt.Sprintf("Error loading settings.toml: %s", err))
    }

    // Mostrar enlaces con tokens para moderadores.
    moderators := []string{"Killer", "Denisse", "Ricotera", "Cris"}
    for _, mod := range moderators {
        token, err := GenerateJWT(mod, true)
        if err != nil {
            fmt.Printf("Error generando token para %s: %s\n", mod, err)
            continue
        }
        fmt.Printf("%s puede entrar en: http://localhost:9000/?jwt=%s\n", mod, token)
    }

    app := barertc.NewServer()
    app.Setup()

    log.Info("Listening at %s", address)
    panic(app.ListenAndServe(address))
}
