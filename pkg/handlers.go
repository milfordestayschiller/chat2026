package barertc

import (
	"fmt"
	"strings"
	"time"

	"git.kirsle.net/apps/barertc/pkg/config"
	"git.kirsle.net/apps/barertc/pkg/jwt"
	"git.kirsle.net/apps/barertc/pkg/log"
	"git.kirsle.net/apps/barertc/pkg/util"
)

// OnLogin handles "login" actions from the client.
func (s *Server) OnLogin(sub *Subscriber, msg Message) {
	// Using a JWT token for authentication?
	var claims = &jwt.Claims{}
	if msg.JWTToken != "" || (config.Current.JWT.Enabled && config.Current.JWT.Strict) {
		parsed, ok, err := jwt.ParseAndValidate(msg.JWTToken)
		if err != nil {
			log.Error("Error parsing JWT token in WebSocket login: %s", err)
			sub.ChatServer("Your authentication has expired. Please go back and launch the chat room again.")
			return
		}

		// Sanity check the username.
		if msg.Username != parsed.Subject {
			log.Error("JWT login had a different username: %s vs %s", parsed.Subject, msg.Username)
			sub.ChatServer("Your authentication username did not match the expected username. Please go back and launch the chat room again.")
			return
		}

		// Strict enforcement?
		if config.Current.JWT.Strict && !ok {
			log.Error("JWT enforcement is strict and user did not pass JWT checks")
			sub.ChatServer("Server side authentication is required. Please go back and launch the chat room from your logged-in account.")
			return
		}

		claims = parsed
		msg.Username = claims.Subject
		sub.JWTClaims = claims
	}

	log.Info("JWT claims: %+v", claims)

	// Ensure the username is unique, or rename it.
	var duplicate bool
	for _, other := range s.IterSubscribers() {
		if other.ID != sub.ID && other.Username == msg.Username {
			duplicate = true
			break
		}
	}

	if duplicate {
		// Give them one that is unique.
		msg.Username = fmt.Sprintf("%s %d",
			msg.Username,
			time.Now().Nanosecond(),
		)
	}

	// Use their username.
	sub.Username = msg.Username
	sub.authenticated = true
	log.Debug("OnLogin: %s joins the room", sub.Username)

	// Tell everyone they joined.
	s.Broadcast(Message{
		Action:   ActionPresence,
		Username: msg.Username,
		Message:  "has joined the room!",
	})

	// Send the user back their settings.
	sub.SendMe()

	// Send the WhoList to everybody.
	s.SendWhoList()

	// Send the initial ChatServer messages to the public channels.
	for _, channel := range config.Current.PublicChannels {
		for _, msg := range channel.WelcomeMessages {
			sub.SendJSON(Message{
				Channel:  channel.ID,
				Action:   ActionError,
				Username: "ChatServer",
				Message:  RenderMarkdown(msg),
			})
		}
	}
}

// OnMessage handles a chat message posted by the user.
func (s *Server) OnMessage(sub *Subscriber, msg Message) {
	log.Info("[%s] %s", sub.Username, msg.Message)
	if sub.Username == "" {
		sub.ChatServer("You must log in first.")
		return
	}

	// Translate their message as Markdown syntax.
	markdown := RenderMarkdown(msg.Message)
	if markdown == "" {
		return
	}

	// Message to be echoed to the channel.
	var message = Message{
		Action:   ActionMessage,
		Channel:  msg.Channel,
		Username: sub.Username,
		Message:  markdown,
	}

	// Is this a DM?
	if strings.HasPrefix(msg.Channel, "@") {
		// Echo the message only to both parties.
		s.SendTo(sub.Username, message)
		message.Channel = "@" + sub.Username
		s.SendTo(msg.Channel, message)
		return
	}

	// Broadcast a chat message to the room.
	s.Broadcast(message)
}

// OnMe handles current user state updates.
func (s *Server) OnMe(sub *Subscriber, msg Message) {
	if msg.VideoActive {
		log.Debug("User %s turns on their video feed", sub.Username)
	}

	sub.VideoActive = msg.VideoActive

	// Sync the WhoList to everybody.
	s.SendWhoList()
}

// OnOpen is a client wanting to start WebRTC with another, e.g. to see their camera.
func (s *Server) OnOpen(sub *Subscriber, msg Message) {
	// Look up the other subscriber.
	other, err := s.GetSubscriber(msg.Username)
	if err != nil {
		sub.ChatServer(err.Error())
		return
	}

	// Make up a WebRTC shared secret and send it to both of them.
	secret := util.RandomString(16)
	log.Info("WebRTC: %s opens %s with secret %s", sub.Username, other.Username, secret)

	// Ring the target of this request and give them the secret.
	other.SendJSON(Message{
		Action:     ActionRing,
		Username:   sub.Username,
		OpenSecret: secret,
	})

	// To the caller, echo back the Open along with the secret.
	sub.SendJSON(Message{
		Action:     ActionOpen,
		Username:   other.Username,
		OpenSecret: secret,
	})
}

// OnCandidate handles WebRTC candidate signaling.
func (s *Server) OnCandidate(sub *Subscriber, msg Message) {
	// Look up the other subscriber.
	other, err := s.GetSubscriber(msg.Username)
	if err != nil {
		sub.ChatServer(err.Error())
		return
	}

	other.SendJSON(Message{
		Action:    ActionCandidate,
		Username:  sub.Username,
		Candidate: msg.Candidate,
	})
}

// OnSDP handles WebRTC sdp signaling.
func (s *Server) OnSDP(sub *Subscriber, msg Message) {
	// Look up the other subscriber.
	other, err := s.GetSubscriber(msg.Username)
	if err != nil {
		sub.ChatServer(err.Error())
		return
	}

	other.SendJSON(Message{
		Action:      ActionSDP,
		Username:    sub.Username,
		Description: msg.Description,
	})
}

// OnWatch communicates video watching status between users.
func (s *Server) OnWatch(sub *Subscriber, msg Message) {
	// Look up the other subscriber.
	other, err := s.GetSubscriber(msg.Username)
	if err != nil {
		sub.ChatServer(err.Error())
		return
	}

	other.SendJSON(Message{
		Action:   ActionWatch,
		Username: sub.Username,
	})
}

// OnUnwatch communicates video Unwatching status between users.
func (s *Server) OnUnwatch(sub *Subscriber, msg Message) {
	// Look up the other subscriber.
	other, err := s.GetSubscriber(msg.Username)
	if err != nil {
		sub.ChatServer(err.Error())
		return
	}

	other.SendJSON(Message{
		Action:   ActionUnwatch,
		Username: sub.Username,
	})
}
