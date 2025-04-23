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

func GuardaNick(nick, ip string) {
    log.Debug("Guardando cualquier nick (invitados incluidos)")

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

        // Verificar si la IP está baneada
        if EstaBaneado(ip) {
            log.Warn("Intento de conexión de IP baneada: %s", ip)
            http.Error(w, "Tu IP ha sido baneada", http.StatusForbidden)
            return
        }
        log.Info("WebSocket connection from %s - %s", ip, r.Header.Get("User-Agent"))

        c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
            CompressionMode: websocket.CompressionDisabled,
        })
        if err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            fmt.Fprintf(w, "Could not accept websocket connection: %s", err)
            return
        }
        defer c.Close(websocket.StatusInternalError, "the sky is falling")

        c.SetReadLimit(config.Current.WebSocketReadLimit)

        jwtToken := r.URL.Query().Get("jwt")
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

        // Nick por header
        if hdr := r.Header.Get("X-User"); hdr != "" {
            sub.Username = hdr
            log.Debug("Nick por header: %s", sub.Username)
        }

        // Nick por JWT
        if claims != nil && claims.Nick != "" {
            sub.Username = claims.Nick
            log.Debug("Nick por JWT: %s", sub.Username)
        }

        // Nick automático si sigue vacío
        if sub.Username == "" {
            sub.Username = fmt.Sprintf("Invitado_%s", ip)
            log.Debug("Nick asignado automáticamente: %s", sub.Username)
        }

        // Moderador por header
        if r.Header.Get("X-Op") == "true" {
            sub.Op = true
        }

        // Guardar nick real
        if !strings.HasPrefix(sub.Username, "Invitado_") {
            GuardaNick(sub.Username, ip)
        }

        // Intentamos leer primer mensaje si el nick es automático
        if strings.HasPrefix(sub.Username, "Invitado_") {
            log.Debug("Nick automático, intentando recibir login...")

            _, msg, err := c.Read(ctx)
            if err != nil {
                log.Error("Error leyendo primer mensaje WebSocket: %s", err)
            } else {
                var loginMsg messages.Message
                if err := json.Unmarshal(msg, &loginMsg); err == nil && loginMsg.Action == messages.ActionLogin && loginMsg.Username != "" {
                    sub.Username = loginMsg.Username
                    log.Debug("Nick recibido del login: %s", sub.Username)
                } else {
                    log.Warn("No se pudo extraer el nick del primer mensaje, se mantiene el automático")
                }
            }
        }

        // Guardamos el nick definitivo (invitado o no)
        GuardaNick(sub.Username, ip)

        sub.authenticated = true
        sub.loginAt = time.Now()

        s.AddSubscriber(sub)

        s.Broadcast(messages.Message{
            Action:   messages.ActionPresence,
            Username: sub.Username,
            Message:  "entered",
        })
        s.SendWhoList()
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


// EstaBaneado verifica si la IP está en datos2.txt
func EstaBaneado(ip string) bool {
    exePath, err := os.Executable()
    if err != nil {
        log.Error("No se pudo obtener ruta del ejecutable: %s", err)
        return false
    }
    rutaArchivo := filepath.Join(filepath.Dir(exePath), "datos2.txt")

    data, err := os.ReadFile(rutaArchivo)
    if err != nil {
        log.Error("No se pudo leer datos2.txt: %s", err)
        return false
    }

    lineas := strings.Split(string(data), "")
    for _, linea := range lineas {
        if strings.TrimSpace(linea) == ip {
            return true
        }
    }
    return false
}