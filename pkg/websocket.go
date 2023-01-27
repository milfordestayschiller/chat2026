package barertc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"git.kirsle.net/apps/barertc/pkg/log"
	"nhooyr.io/websocket"
)

// Subscriber represents a connected WebSocket session.
type Subscriber struct {
	// User properties
	ID          int // ID assigned by server
	Username    string
	VideoActive bool
	conn        *websocket.Conn
	ctx         context.Context
	messages    chan []byte
	closeSlow   func()
}

// ReadLoop spawns a goroutine that reads from the websocket connection.
func (sub *Subscriber) ReadLoop(s *Server) {
	go func() {
		for {
			msgType, data, err := sub.conn.Read(sub.ctx)
			if err != nil {
				log.Error("ReadLoop error: %+v", err)
				s.DeleteSubscriber(sub)
				s.Broadcast(Message{
					Action:   ActionPresence,
					Username: sub.Username,
					Message:  "has exited the room!",
				})
				s.SendWhoList()
				return
			}

			if msgType != websocket.MessageText {
				log.Error("Unexpected MessageType")
				continue
			}

			// Read the user's posted message.
			var msg Message
			log.Debug("Read(%s): %s", sub.Username, data)
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Error("Message error: %s", err)
				continue
			}

			// What action are they performing?
			switch msg.Action {
			case ActionLogin:
				s.OnLogin(sub, msg)
			case ActionMessage:
				s.OnMessage(sub, msg)
			case ActionMe:
				s.OnMe(sub, msg)
			case ActionOpen:
				s.OnOpen(sub, msg)
			case ActionCandidate:
				s.OnCandidate(sub, msg)
			case ActionSDP:
				s.OnSDP(sub, msg)
			default:
				sub.ChatServer("Unsupported message type.")
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
	log.Debug("SendJSON(%s): %s", sub.Username, data)
	return sub.conn.Write(sub.ctx, websocket.MessageText, data)
}

// SendMe sends the current user state to the client.
func (sub *Subscriber) SendMe() {
	sub.SendJSON(Message{
		Action:      ActionMe,
		Username:    sub.Username,
		VideoActive: sub.VideoActive,
	})
}

// ChatServer is a convenience function to deliver a ChatServer error to the client.
func (sub *Subscriber) ChatServer(message string, v ...interface{}) {
	sub.SendJSON(Message{
		Action:   ActionError,
		Username: "ChatServer",
		Message:  fmt.Sprintf(message, v...),
	})
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
		// defer s.DeleteSubscriber(sub)

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

// Auto incrementing Subscriber ID, assigned in AddSubscriber.
var SubscriberID int

// AddSubscriber adds a WebSocket subscriber to the server.
func (s *Server) AddSubscriber(sub *Subscriber) {
	// Assign a unique ID.
	SubscriberID++
	sub.ID = SubscriberID
	log.Debug("AddSubscriber: %s", sub.ID)

	s.subscribersMu.Lock()
	s.subscribers[sub] = struct{}{}
	s.subscribersMu.Unlock()
}

// GetSubscriber by username.
func (s *Server) GetSubscriber(username string) (*Subscriber, error) {
	s.subscribersMu.RLock()
	defer s.subscribersMu.RUnlock()
	for _, sub := range s.IterSubscribers(true) {
		if sub.Username == username {
			return sub, nil
		}
	}
	return nil, errors.New("not found")
}

// DeleteSubscriber removes a subscriber from the server.
func (s *Server) DeleteSubscriber(sub *Subscriber) {
	log.Error("DeleteSubscriber: %s", sub.Username)
	s.subscribersMu.Lock()
	delete(s.subscribers, sub)
	s.subscribersMu.Unlock()
}

// IterSubscribers loops over the subscriber list with a read lock. If the
// caller already holds a lock, pass the optional `true` parameter for isLocked.
func (s *Server) IterSubscribers(isLocked ...bool) []*Subscriber {
	log.Debug("IterSubscribers START..")

	var result = []*Subscriber{}

	// Has the caller already taken the read lock or do we get it?
	if locked := len(isLocked) > 0 && isLocked[0]; !locked {
		log.Debug("Taking the lock")
		s.subscribersMu.RLock()
		defer s.subscribersMu.RUnlock()
	}

	for sub := range s.subscribers {
		result = append(result, sub)
	}

	log.Debug("IterSubscribers STOP..")
	return result
}

// Broadcast a message to the chat room.
func (s *Server) Broadcast(msg Message) {
	log.Debug("Broadcast: %+v", msg)
	s.subscribersMu.RLock()
	defer s.subscribersMu.RUnlock()
	for _, sub := range s.IterSubscribers(true) {
		sub.SendJSON(Message{
			Action:   msg.Action,
			Username: msg.Username,
			Message:  msg.Message,
		})
	}
}

// SendWhoList broadcasts the connected members to everybody in the room.
func (s *Server) SendWhoList() {
	var (
		users       = []WhoList{}
		subscribers = s.IterSubscribers()
	)

	for _, sub := range subscribers {
		users = append(users, WhoList{
			Username:    sub.Username,
			VideoActive: sub.VideoActive,
		})
	}

	for _, sub := range subscribers {
		sub.SendJSON(Message{
			Action:  ActionWhoList,
			WhoList: users,
		})
	}
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Write(ctx, websocket.MessageText, msg)
}
