package barertc

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"git.kirsle.net/apps/barertc/pkg/log"
	"git.kirsle.net/apps/barertc/pkg/messages"
	"git.kirsle.net/apps/barertc/pkg/util"
	"github.com/google/uuid"
)

const PollingUserTimeout = time.Minute

type PollMessage struct {
	Username  string            `json:"username,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	Message   messages.Message  `json:"msg"`
}

type PollResponse struct {
	Username  string             `json:"username,omitempty"`
	SessionID string             `json:"session_id,omitempty"`
	Messages  []messages.Message `json:"messages"`
}

func PollResponseError(message string) PollResponse {
	return PollResponse{
		Messages: []messages.Message{
			{
				Action:   messages.ActionError,
				Username: "ChatServer",
				Message:  message,
			},
		},
	}
}

func (s *Server) KickIdlePollUsers() {
	log.Debug("KickIdlePollUsers goroutine running")
	for {
		time.Sleep(10 * time.Second)
		for _, sub := range s.IterSubscribers() {
			if sub.usePolling && time.Since(sub.lastPollAt) > PollingUserTimeout {
				if sub.authenticated && sub.ChatStatus != "hidden" {
					log.Error("KickIdlePollUsers: %s timed out after %s", sub.Username, time.Since(sub.lastPollAt))

					sub.authenticated = false
					s.Broadcast(messages.Message{
						Action:   messages.ActionPresence,
						Username: sub.Username,
						Message:  messages.PresenceTimedOut,
					})
					s.SendWhoList()
				}
				s.DeleteSubscriber(sub)
			}
		}
	}
}

func (sub *Subscriber) FlushPollResponse() PollResponse {
	var msgs []messages.Message

	for len(sub.messages) > 0 {
		raw := <-sub.messages
		var msg messages.Message
		if err := json.Unmarshal(raw, &msg); err == nil {
			msgs = append(msgs, msg)
		}
	}

	return PollResponse{
		Username:  sub.Username,
		SessionID: sub.sessionID,
		Messages:  msgs,
	}
}

func (s *Server) PollingAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := util.IPAddress(r)

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			enc.Encode(PollResponseError("Only POST methods allowed"))
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			enc.Encode(PollResponseError("Only application/json content-types allowed"))
			return
		}

		defer r.Body.Close()
		var params PollMessage
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			enc.Encode(PollResponseError("Invalid JSON: "+err.Error()))
			return
		}

		log.Debug("Polling from %s [%s]", ip, r.Header.Get("User-Agent"))

		var sub *Subscriber
		if params.Username != "" && params.SessionID != "" {
			var err error
			sub, err = s.GetSubscriber(params.Username)
			if err == nil && sub.sessionID == params.SessionID {
				sub.lastPollAt = time.Now()
			} else {
				if sub != nil {
					s.DeleteSubscriber(sub)
				}
				w.WriteHeader(http.StatusUnauthorized)
				enc.Encode(PollResponse{
					Messages: []messages.Message{
						{
							Action:   messages.ActionError,
							Username: "ChatServer",
							Message:  "Your authentication has expired, please log back into the chat again.",
						},
						{Action: messages.ActionKick},
					},
				})
				return
			}
		}

		if sub != nil && sub.authenticated {
			s.OnClientMessage(sub, params.Message)

			if time.Since(sub.lastPollJWT) > PingInterval {
				sub.lastPollJWT = time.Now()

				if sub.JWTClaims != nil {
					if jwt, err := sub.JWTClaims.ReSign(); err == nil {
						sub.SendJSON(messages.Message{
							Action:   messages.ActionPing,
							JWTToken: jwt,
						})
					}
				}
			}

			enc.Encode(sub.FlushPollResponse())
			return
		}

		if params.Message.Action != messages.ActionLogin {
			w.WriteHeader(http.StatusUnauthorized)
			enc.Encode(PollResponseError("Not logged in."))
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		sub = s.NewPollingSubscriber(ctx, cancel)
		s.AddSubscriber(sub)

		s.OnLogin(sub, params.Message)

		if sub.authenticated {
			sub.sessionID = uuid.New().String()
			log.Debug("Polling login: %s [%s]", sub.Username, sub.sessionID)
		} else {
			s.DeleteSubscriber(sub)
		}

		enc.Encode(sub.FlushPollResponse())
	}
}

