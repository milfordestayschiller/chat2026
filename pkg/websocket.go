package barertc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"git.kirsle.net/apps/barertc/pkg/config"
	"git.kirsle.net/apps/barertc/pkg/jwt"
	"git.kirsle.net/apps/barertc/pkg/log"
	"git.kirsle.net/apps/barertc/pkg/util"
	"nhooyr.io/websocket"
)

// Subscriber represents a connected WebSocket session.
type Subscriber struct {
	// User properties
	ID              int // ID assigned by server
	Username        string
	VideoActive     bool
	VideoMutual     bool
	VideoMutualOpen bool
	VideoNSFW       bool
	ChatStatus      string
	JWTClaims       *jwt.Claims
	authenticated   bool // has passed the login step
	conn            *websocket.Conn
	ctx             context.Context
	cancel          context.CancelFunc
	messages        chan []byte
	closeSlow       func()

	muteMu sync.RWMutex
	booted map[string]struct{} // usernames booted off your camera
	muted  map[string]struct{} // usernames you muted

	// Record which message IDs belong to this user.
	midMu      sync.Mutex
	messageIDs map[int]struct{}
}

// ReadLoop spawns a goroutine that reads from the websocket connection.
func (sub *Subscriber) ReadLoop(s *Server) {
	go func() {
		for {
			msgType, data, err := sub.conn.Read(sub.ctx)
			if err != nil {
				log.Error("ReadLoop error(%d=%s): %+v", sub.ID, sub.Username, err)
				s.DeleteSubscriber(sub)

				// Notify if this user was auth'd and not hidden
				if sub.authenticated && sub.ChatStatus != "hidden" {
					s.Broadcast(Message{
						Action:   ActionPresence,
						Username: sub.Username,
						Message:  "has exited the room!",
					})
					s.SendWhoList()
				}
				return
			}

			if msgType != websocket.MessageText {
				log.Error("Unexpected MessageType")
				continue
			}

			// Read the user's posted message.
			var msg Message
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Error("Read(%d=%s) Message error: %s", sub.ID, sub.Username, err)
				continue
			}

			if msg.Action != ActionFile {
				log.Debug("Read(%d=%s): %s", sub.ID, sub.Username, data)
			}

			// What action are they performing?
			switch msg.Action {
			case ActionLogin:
				s.OnLogin(sub, msg)
			case ActionMessage:
				s.OnMessage(sub, msg)
			case ActionFile:
				s.OnFile(sub, msg)
			case ActionMe:
				s.OnMe(sub, msg)
			case ActionOpen:
				s.OnOpen(sub, msg)
			case ActionBoot:
				s.OnBoot(sub, msg)
			case ActionMute, ActionUnmute:
				s.OnMute(sub, msg, msg.Action == ActionMute)
			case ActionCandidate:
				s.OnCandidate(sub, msg)
			case ActionSDP:
				s.OnSDP(sub, msg)
			case ActionWatch:
				s.OnWatch(sub, msg)
			case ActionUnwatch:
				s.OnUnwatch(sub, msg)
			case ActionTakeback:
				s.OnTakeback(sub, msg)
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
	log.Debug("SendJSON(%d=%s): %s", sub.ID, sub.Username, data)
	return sub.conn.Write(sub.ctx, websocket.MessageText, data)
}

// SendMe sends the current user state to the client.
func (sub *Subscriber) SendMe() {
	sub.SendJSON(Message{
		Action:      ActionMe,
		Username:    sub.Username,
		VideoActive: sub.VideoActive,
		NSFW:        sub.VideoNSFW,
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

// WebSocket handles the /ws websocket connection endpoint.
func (s *Server) WebSocket() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := util.IPAddress(r)
		log.Info("WebSocket connection from %s - %s", ip, r.Header.Get("User-Agent"))
		log.Debug("Headers: %+v", r.Header)
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			CompressionMode: websocket.CompressionDisabled,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Could not accept websocket connection: %s", err)
			return
		}
		defer c.Close(websocket.StatusInternalError, "the sky is falling")

		log.Debug("WebSocket: %s has connected", ip)
		c.SetReadLimit(config.Current.WebSocketReadLimit)

		// CloseRead starts a goroutine that will read from the connection
		// until it is closed.
		// ctx := c.CloseRead(r.Context())
		ctx, cancel := context.WithCancel(r.Context())

		sub := &Subscriber{
			conn:     c,
			ctx:      ctx,
			cancel:   cancel,
			messages: make(chan []byte, s.subscriberMessageBuffer),
			closeSlow: func() {
				c.Close(websocket.StatusPolicyViolation, "connection too slow to keep up with messages")
			},
			booted:     make(map[string]struct{}),
			muted:      make(map[string]struct{}),
			messageIDs: make(map[int]struct{}),
			ChatStatus: "online",
		}

		s.AddSubscriber(sub)
		defer s.DeleteSubscriber(sub)

		go sub.ReadLoop(s)
		pinger := time.NewTicker(PingInterval)
		for {
			select {
			case msg := <-sub.messages:
				err = writeTimeout(ctx, time.Second*5, c, msg)
				if err != nil {
					return
				}
			case <-pinger.C:
				// Send a ping, and a refreshed JWT token if the user sent one.
				var token string
				if sub.JWTClaims != nil {
					if jwt, err := sub.JWTClaims.ReSign(); err != nil {
						log.Error("ReSign JWT token for %s: %s", sub.Username, err)
					} else {
						token = jwt
					}
				}

				sub.SendJSON(Message{
					Action:   ActionPing,
					JWTToken: token,
				})
			case <-ctx.Done():
				pinger.Stop()
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
	log.Debug("AddSubscriber: ID #%d", sub.ID)

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

	// Cancel its context to clean up the for-loop goroutine.
	if sub.cancel != nil {
		sub.cancel()
	}

	s.subscribersMu.Lock()
	delete(s.subscribers, sub)
	s.subscribersMu.Unlock()
}

// IterSubscribers loops over the subscriber list with a read lock. If the
// caller already holds a lock, pass the optional `true` parameter for isLocked.
func (s *Server) IterSubscribers(isLocked ...bool) []*Subscriber {
	var result = []*Subscriber{}

	// Has the caller already taken the read lock or do we get it?
	if locked := len(isLocked) > 0 && isLocked[0]; !locked {
		s.subscribersMu.RLock()
		defer s.subscribersMu.RUnlock()
	}

	for sub := range s.subscribers {
		result = append(result, sub)
	}

	return result
}

// UniqueUsername ensures a username will be unique or renames it.
func (s *Server) UniqueUsername(username string) string {
	var (
		subs         = s.IterSubscribers()
		usernames    = map[string]interface{}{}
		origUsername = username
		counter      = 2
	)
	for _, sub := range subs {
		usernames[sub.Username] = nil
	}

	// Check until unique.
	for {
		if _, ok := usernames[username]; ok {
			username = fmt.Sprintf("%s %d", origUsername, counter)
			counter++
		} else {
			break
		}
	}

	return username
}

// Broadcast a message to the chat room.
func (s *Server) Broadcast(msg Message) {
	if len(msg.Message) < 1024 {
		log.Debug("Broadcast: %+v", msg)
	}

	s.subscribersMu.RLock()
	defer s.subscribersMu.RUnlock()
	for _, sub := range s.IterSubscribers(true) {
		if !sub.authenticated {
			continue
		}

		// Don't deliver it if the receiver has muted us.
		if sub.Mutes(msg.Username) {
			log.Debug("Do not broadcast message to %s: they have muted or booted %s", sub.Username, msg.Username)
			continue
		}

		sub.SendJSON(msg)
	}
}

// SendTo sends a message to a given username.
func (s *Server) SendTo(username string, msg Message) error {
	log.Debug("SendTo(%s): %+v", username, msg)
	username = strings.TrimPrefix(username, "@")
	s.subscribersMu.RLock()
	defer s.subscribersMu.RUnlock()

	var found bool
	for _, sub := range s.IterSubscribers(true) {
		if sub.Username == username {
			found = true
			sub.SendJSON(Message{
				Action:   msg.Action,
				Channel:  msg.Channel,
				Username: msg.Username,
				Message:  msg.Message,
			})
		}
	}

	if !found {
		return fmt.Errorf("%s is not online", username)
	}
	return nil
}

// SendWhoList broadcasts the connected members to everybody in the room.
func (s *Server) SendWhoList() {
	var (
		subscribers = s.IterSubscribers()
		usernames   = []string{} // distinct and sorted usernames
		userSub     = map[string]*Subscriber{}
	)

	for _, sub := range subscribers {
		if !sub.authenticated {
			continue
		}
		usernames = append(usernames, sub.Username)
		userSub[sub.Username] = sub
	}
	sort.Strings(usernames)

	// Build the WhoList for each subscriber.
	// TODO: it's the only way to fake videoActive for booted user views.
	for _, sub := range subscribers {
		if !sub.authenticated {
			continue
		}

		var users = []WhoList{}
		for _, un := range usernames {
			user := userSub[un]
			if user.ChatStatus == "hidden" {
				continue
			}

			who := WhoList{
				Username:        user.Username,
				Status:          user.ChatStatus,
				VideoActive:     user.VideoActive,
				VideoMutual:     user.VideoMutual,
				VideoMutualOpen: user.VideoMutualOpen,
				NSFW:            user.VideoNSFW,
			}

			// If this person had booted us, force their camera to "off"
			if user.Boots(sub.Username) || user.Mutes(sub.Username) {
				who.VideoActive = false
				who.NSFW = false
			}

			if user.JWTClaims != nil {
				who.Operator = user.JWTClaims.IsAdmin
				who.Avatar = user.JWTClaims.Avatar
				who.ProfileURL = user.JWTClaims.ProfileURL
				who.Nickname = user.JWTClaims.Nick
			}
			users = append(users, who)
		}

		sub.SendJSON(Message{
			Action:  ActionWhoList,
			WhoList: users,
		})
	}
}

// Boots checks whether the subscriber has blocked username from their camera.
func (s *Subscriber) Boots(username string) bool {
	s.muteMu.RLock()
	defer s.muteMu.RUnlock()
	_, ok := s.booted[username]
	return ok
}

// Mutes checks whether the subscriber has muted username.
func (s *Subscriber) Mutes(username string) bool {
	s.muteMu.RLock()
	defer s.muteMu.RUnlock()
	_, ok := s.muted[username]
	return ok
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Write(ctx, websocket.MessageText, msg)
}
