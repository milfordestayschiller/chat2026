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
	"git.kirsle.net/apps/barertc/pkg/messages"
	"git.kirsle.net/apps/barertc/pkg/util"
	"nhooyr.io/websocket"
)

// Subscriber represents a connected WebSocket session.
type Subscriber struct {
	// User properties
	ID            int // ID assigned by server
	Username      string
	ChatStatus    string
	VideoStatus   int
	DND           bool // Do Not Disturb status (DMs are closed)
	JWTClaims     *jwt.Claims
	authenticated bool // has passed the login step
	loginAt       time.Time
	conn          *websocket.Conn
	ctx           context.Context
	cancel        context.CancelFunc
	messages      chan []byte
	closeSlow     func()

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
					s.Broadcast(messages.Message{
						Action:   messages.ActionPresence,
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
			var msg messages.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Error("Read(%d=%s) Message error: %s", sub.ID, sub.Username, err)
				continue
			}

			if msg.Action != messages.ActionFile {
				log.Debug("Read(%d=%s): %s", sub.ID, sub.Username, data)
			}

			// What action are they performing?
			switch msg.Action {
			case messages.ActionLogin:
				s.OnLogin(sub, msg)
			case messages.ActionMessage:
				s.OnMessage(sub, msg)
			case messages.ActionFile:
				s.OnFile(sub, msg)
			case messages.ActionMe:
				s.OnMe(sub, msg)
			case messages.ActionOpen:
				s.OnOpen(sub, msg)
			case messages.ActionBoot:
				s.OnBoot(sub, msg)
			case messages.ActionMute, messages.ActionUnmute:
				s.OnMute(sub, msg, msg.Action == messages.ActionMute)
			case messages.ActionBlocklist:
				s.OnBlocklist(sub, msg)
			case messages.ActionCandidate:
				s.OnCandidate(sub, msg)
			case messages.ActionSDP:
				s.OnSDP(sub, msg)
			case messages.ActionWatch:
				s.OnWatch(sub, msg)
			case messages.ActionUnwatch:
				s.OnUnwatch(sub, msg)
			case messages.ActionTakeback:
				s.OnTakeback(sub, msg)
			case messages.ActionReact:
				s.OnReact(sub, msg)
			case messages.ActionReport:
				s.OnReport(sub, msg)
			default:
				sub.ChatServer("Unsupported message type.")
			}
		}
	}()
}

// IsAdmin safely checks if the subscriber is an admin.
func (sub *Subscriber) IsAdmin() bool {
	return sub.JWTClaims != nil && sub.JWTClaims.IsAdmin
}

// IsVIP safely checks if the subscriber has VIP status.
func (sub *Subscriber) IsVIP() bool {
	return sub.JWTClaims != nil && sub.JWTClaims.VIP
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
	sub.SendJSON(messages.Message{
		Action:      messages.ActionMe,
		Username:    sub.Username,
		VideoStatus: sub.VideoStatus,
	})
}

// ChatServer is a convenience function to deliver a ChatServer error to the client.
func (sub *Subscriber) ChatServer(message string, v ...interface{}) {
	sub.SendJSON(messages.Message{
		Action:   messages.ActionError,
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

				sub.SendJSON(messages.Message{
					Action:   messages.ActionPing,
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
	for _, sub := range s.IterSubscribers() {
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

// IterSubscribers loops over the subscriber list with a read lock.
func (s *Server) IterSubscribers() []*Subscriber {
	var result = []*Subscriber{}

	// Lock for reads.
	s.subscribersMu.RLock()
	for sub := range s.subscribers {
		result = append(result, sub)
	}
	s.subscribersMu.RUnlock()

	return result
}

// UniqueUsername ensures a username will be unique or renames it. If the name is already unique, the error result is nil.
func (s *Server) UniqueUsername(username string) (string, error) {
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

	if username != origUsername {
		return username, errors.New("username was not unique and a unique name has been returned")
	}

	return username, nil
}

// Broadcast a message to the chat room.
func (s *Server) Broadcast(msg messages.Message) {
	if len(msg.Message) < 1024 {
		log.Debug("Broadcast: %+v", msg)
	}

	// Get the list of users who are online NOW, so we don't hold the mutex lock too long.
	// Example: sending a fat GIF to a large audience could hang up the server for a long
	// time until every copy of the GIF has been sent.
	var subs = s.IterSubscribers()
	for _, sub := range subs {
		if !sub.authenticated {
			continue
		}

		// Don't deliver it if the receiver has muted us.
		if sub.Mutes(msg.Username) {
			log.Debug("Do not broadcast message to %s: they have muted or booted %s", sub.Username, msg.Username)
			continue
		}

		// VIP channels: only deliver to subscribed VIP users.
		if ch, ok := config.Current.GetChannel(msg.Channel); ok && ch.VIP && !sub.IsVIP() {
			log.Debug("Do not broadcast message to %s: VIP channel and they are not VIP", sub.Username)
			continue
		}

		sub.SendJSON(msg)
	}
}

// SendTo sends a message to a given username.
func (s *Server) SendTo(username string, msg messages.Message) error {
	log.Debug("SendTo(%s): %+v", username, msg)
	username = strings.TrimPrefix(username, "@")

	var found bool
	var subs = s.IterSubscribers()
	for _, sub := range subs {
		if sub.Username == username {
			found = true
			sub.SendJSON(messages.Message{
				Action:    msg.Action,
				Channel:   msg.Channel,
				Username:  msg.Username,
				Message:   msg.Message,
				MessageID: msg.MessageID,
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

		var users = []messages.WhoList{}
		for _, un := range usernames {
			user := userSub[un]
			if user.ChatStatus == "hidden" {
				continue
			}

			who := messages.WhoList{
				Username: user.Username,
				Status:   user.ChatStatus,
				Video:    user.VideoStatus,
				DND:      user.DND,
				LoginAt:  user.loginAt.Unix(),
			}

			// Hide video flags of other users (never for the current user).
			if user.Username != sub.Username {

				// If this person had booted us, force their camera to "off"
				if (user.Boots(sub.Username) || user.Mutes(sub.Username)) && !sub.IsAdmin() {
					who.Video = 0
				}

				// If this person's VideoFlag is set to VIP Only, force their camera to "off"
				// except when the person looking has the VIP status.
				if (user.VideoStatus&messages.VideoFlagOnlyVIP == messages.VideoFlagOnlyVIP) && (!sub.IsVIP() && !sub.IsAdmin()) {
					who.Video = 0
				}
			}

			if user.JWTClaims != nil {
				who.Operator = user.JWTClaims.IsAdmin
				who.Avatar = user.JWTClaims.Avatar
				who.ProfileURL = user.JWTClaims.ProfileURL
				who.Nickname = user.JWTClaims.Nick
				who.Emoji = user.JWTClaims.Emoji
				who.Gender = user.JWTClaims.Gender

				// VIP flags: if we are in MutuallySecret mode, only VIPs can see
				// other VIP flags on the Who List.
				if config.Current.VIP.MutuallySecret {
					if sub.IsVIP() || sub.IsAdmin() {
						who.VIP = user.JWTClaims.VIP
					}
				} else {
					who.VIP = user.JWTClaims.VIP
				}
			}
			users = append(users, who)
		}

		sub.SendJSON(messages.Message{
			Action:  messages.ActionWhoList,
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
