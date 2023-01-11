package barertc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"git.kirsle.net/apps/barertc/pkg/log"
	"nhooyr.io/websocket"
)

// Subscriber represents a connected WebSocket session.
type Subscriber struct {
	Username  string
	conn      *websocket.Conn
	ctx       context.Context
	messages  chan []byte
	closeSlow func()
}

// ReadLoop spawns a goroutine that reads from the websocket connection.
func (sub *Subscriber) ReadLoop(s *Server) {
	go func() {
		for {
			msgType, data, err := sub.conn.Read(sub.ctx)
			if err != nil {
				log.Error("ReadLoop error: %+v", err)
				return
			}

			if msgType != websocket.MessageText {
				log.Error("Unexpected MessageType")
				continue
			}

			// Read the user's posted message.
			var msg Message
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Error("Message error: %s", err)
				continue
			}

			// What action are they performing?
			switch msg.Action {
			case ActionLogin:
				// TODO: ensure unique?
				sub.Username = msg.Username
				s.Broadcast(Message{
					Username: msg.Username,
					Message:  "has joined the room!",
				})
			case ActionMessage:
				if sub.Username == "" {
					sub.SendJSON(Message{
						Username: "ChatServer",
						Message:  "You must log in first.",
					})
					continue
				}

				// Broadcast a chat message to the room.
				s.Broadcast(Message{
					Username: sub.Username,
					Message:  msg.Message,
				})
			default:
				sub.SendJSON(Message{
					Username: "ChatServer",
					Message:  "Unsupported message type.",
				})
			}
		}
	}()
}

// SendJSON sends a JSON message to the websocket client.
func (sub *Subscriber) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return sub.conn.Write(sub.ctx, websocket.MessageText, data)
}

// WebSocket handles the /ws websocket connection.
func (s *Server) WebSocket() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Websocket error: %s", err)
			return
		}
		defer c.Close(websocket.StatusInternalError, "the sky is falling")

		log.Debug("WebSocket: %s has connected", r.RemoteAddr)

		// CloseRead starts a goroutine that will read from the connection
		// until it is closed.
		// ctx := c.CloseRead(r.Context())
		ctx, _ := context.WithCancel(r.Context())

		sub := &Subscriber{
			conn:     c,
			ctx:      ctx,
			messages: make(chan []byte, s.subscriberMessageBuffer),
			closeSlow: func() {
				c.Close(websocket.StatusPolicyViolation, "connection too slow to keep up with messages")
			},
		}

		s.AddSubscriber(sub)
		defer s.DeleteSubscriber(sub)

		go sub.ReadLoop(s)
		for {
			select {
			case msg := <-sub.messages:
				err = writeTimeout(ctx, time.Second*5, c, msg)
				if err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}

	})
}

// AddSubscriber adds a WebSocket subscriber to the server.
func (s *Server) AddSubscriber(sub *Subscriber) {
	s.subscribersMu.Lock()
	s.subscribers[sub] = struct{}{}
	s.subscribersMu.Unlock()
}

// DeleteSubscriber removes a subscriber from the server.
func (s *Server) DeleteSubscriber(sub *Subscriber) {
	s.subscribersMu.Lock()
	delete(s.subscribers, sub)
	s.subscribersMu.Unlock()
}

// Broadcast a message to the chat room.
func (s *Server) Broadcast(msg Message) {
	s.subscribersMu.RLock()
	defer s.subscribersMu.RUnlock()
	for sub := range s.subscribers {
		sub.SendJSON(Message{
			Username: msg.Username,
			Message:  msg.Message,
		})
	}
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Write(ctx, websocket.MessageText, msg)
}
