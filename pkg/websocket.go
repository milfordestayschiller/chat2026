
package barertc

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"

    "git.kirsle.net/apps/barertc/pkg/config"
    "git.kirsle.net/apps/barertc/pkg/log"
    "git.kirsle.net/apps/barertc/pkg/messages"
    "git.kirsle.net/apps/barertc/pkg/util"
    "git.kirsle.net/apps/barertc/pkg/jwt"
    "nhooyr.io/websocket"
)

func ipEnAmbosArchivos(ip string) bool {
    exePath, _ := os.Executable()
    basePath := filepath.Dir(exePath)

    archivo1 := filepath.Join(basePath, "datos.txt")
    archivo2 := filepath.Join(basePath, "datos2.txt")

    enDatos1 := false
    enDatos2 := false

    if f1, err := os.ReadFile(archivo1); err == nil {
        for _, linea := range strings.Split(string(f1), "") {
            if strings.Contains(linea, ip) {
                enDatos1 = true
                break
            }
        }
    }

    if f2, err := os.ReadFile(archivo2); err == nil {
        for _, linea := range strings.Split(string(f2), "") {
            if strings.Contains(linea, ip) {
                enDatos2 = true
                break
            }
        }
    }

    return enDatos1 && enDatos2
}

// GuardaNick guarda el nick real (no los automáticos) y su IP en datos.txt
func GuardaNick(nick, ip string) {
    if strings.HasPrefix(nick, "Invitado_") {
        log.Debug("Nick automático detectado, no se guarda en datos.txt")
        return
    }

    linea := fmt.Sprintf("Nick: %s | IP: %s\n", nick, ip)
    log.Debug("Escribiendo en datos.txt: %s", linea)

    exePath, err := os.Executable()
    if err != nil {
        log.Error("No se pudo obtener ruta del ejecutable: %s", err)
        return
    }
    rutaArchivo := filepath.Join(filepath.Dir(exePath), "datos.txt")

    f, err := os.OpenFile(rutaArchivo, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Error("No se pudo abrir datos.txt: %s", err)
        return
    }
    defer f.Close()

    if _, err := f.WriteString(linea); err != nil {
        log.Error("No se pudo escribir en datos.txt: %s", err)
    } else {
        log.Info("Se escribió correctamente el nick en datos.txt")
    }
}

func (s *Server) WebSocket() http.HandlerFunc {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip := util.IPAddress(r)
        log.Info("WebSocket connection from %s - %s", ip, r.Header.Get("User-Agent"))
        log.Debug("Headers: %+v", r.Header)

        if ipEnAmbosArchivos(ip) {
            log.Warn("Conexión bloqueada: IP %s está baneada en ambos archivos", ip)
            w.WriteHeader(http.StatusForbidden)
            fmt.Fprintln(w, "Acceso denegado.")
            return
        }

        dir, _ := os.Getwd()
        log.Debug("Ruta actual de trabajo: %s", dir)

        c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
            CompressionMode: websocket.CompressionDisabled,
        })
        if err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            fmt.Fprintf(w, "Could not accept websocket connection: %s", err)
            return
        }
        defer c.Close(websocket.StatusInternalError, "the sky is falling")

        log.Debug("WebSocket: %s ha conectado", ip)
        c.SetReadLimit(config.Current.WebSocketReadLimit)

        jwtToken := r.URL.Query().Get("jwt")
        log.Debug("Token JWT recibido: %s", jwtToken)
        var claims *jwt.Claims
        if jwtToken != "" {
            parsed, authOK, err := jwt.ParseAndValidate(jwtToken)
            if err == nil && authOK {
                claims = parsed
                log.Debug("JWT válido para %s", claims.Nick)
            } else {
                log.Warn("JWT inválido: %s", err)
            }
        }

        ctx, cancel := context.WithCancel(r.Context())
        sub := s.NewWebSocketSubscriber(ctx, c, cancel)
        sub.IP = ip

        if hdr := r.Header.Get("X-User"); hdr != "" {
            sub.Username = hdr
            log.Debug("Nick por header: %s", sub.Username)
        }

        if claims != nil && claims.Nick != "" {
            sub.Username = claims.Nick
            log.Debug("Nick por JWT: %s", sub.Username)
        }

        if sub.Username == "" {
            sub.Username = fmt.Sprintf("Invitado_%s", ip)
            log.Debug("Nick asignado automáticamente: %s", sub.Username)
        }

        if r.Header.Get("X-Op") == "true" {
            sub.Op = true
        }

        if !strings.HasPrefix(sub.Username, "Invitado_") {
            log.Debug("Guardando Nick REAL: %s", sub.Username)
            GuardaNick(sub.Username, ip)
        } else {
            log.Debug("Nick automático, esperando primer mensaje login...")

            _, msg, err := c.Read(ctx)
            if err != nil {
                log.Error("Error leyendo primer mensaje WebSocket: %s", err)
                return
            }

            var loginMsg messages.Message
            if err := json.Unmarshal(msg, &loginMsg); err == nil && loginMsg.Action == messages.ActionLogin && loginMsg.Username != "" {
                sub.Username = loginMsg.Username
                log.Debug("Nick recibido del login: %s", sub.Username)
                GuardaNick(sub.Username, ip)
            } else {
                log.Warn("No se pudo extraer el nick del primer mensaje")
            }
        }

        s.AddSubscriber(sub)
        defer s.DeleteSubscriber(sub)

        go sub.ReadLoop(s)

        pinger := time.NewTicker(PingInterval)
        for {
            select {
            case msg := <-sub.messages:
                err = writeTimeout(ctx, time.Second*time.Duration(config.Current.WebSocketSendTimeout), c, msg)
                if err != nil {
                    return
                }
            case <-pinger.C:
                sub.SendJSON(messages.Message{
                    Action: messages.ActionPing,
                })
            case <-ctx.Done():
                pinger.Stop()
                return
            }
        }
    })
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg []byte) error {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    return c.Write(ctx, websocket.MessageText, msg)
}
