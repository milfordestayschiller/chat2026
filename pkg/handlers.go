package barertc

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
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

	if claims.Subject != "" {
		log.Debug("JWT claims: %+v", claims)
	}

	// Somehow no username?
	if msg.Username == "" {
		msg.Username = "anonymous"
	}

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

	// Process commands.
	if handled := s.ProcessCommand(sub, msg); handled {
		return
	}

	// Translate their message as Markdown syntax.
	markdown := RenderMarkdown(msg.Message)
	if markdown == "" {
		return
	}

	// Detect and expand media such as YouTube videos.
	markdown = s.ExpandMedia(markdown)

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

		// Don't deliver it if the receiver has muted us.
		rcpt, err := s.GetSubscriber(strings.TrimPrefix(msg.Channel, "@"))
		if err == nil && rcpt.Mutes(sub.Username) {
			log.Debug("Do not send message to %s: they have muted or booted %s", rcpt.Username, sub.Username)
			return
		}

		// If the sender already mutes the recipient, reply back with the error.
		if err == nil && sub.Mutes(rcpt.Username) {
			sub.ChatServer("You have muted %s and so your message has not been sent.", rcpt.Username)
			return
		}

		if err := s.SendTo(msg.Channel, message); err != nil {
			sub.ChatServer("Your message could not be delivered: %s", err)
		}
		return
	}

	// Broadcast a chat message to the room.
	s.Broadcast(message)
}

// OnFile handles a picture shared in chat with a channel.
func (s *Server) OnFile(sub *Subscriber, msg Message) {
	if sub.Username == "" {
		sub.ChatServer("You must log in first.")
		return
	}

	// Detect image type and convert it into an <img src="data:"> tag.
	var (
		filename = msg.Message
		ext      = filepath.Ext(filename)
		filetype string
	)
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		filetype = "image/jpeg"
	case ".gif":
		filetype = "image/gif"
	case ".png":
		filetype = "image/png"
	default:
		sub.ChatServer("Unsupported image type, should be a jpeg, GIF or png.")
		return
	}

	// Process the image: scale it down, strip metadata, etc.
	img, pvWidth, pvHeight := ProcessImage(filetype, msg.Bytes)
	var dataURL = fmt.Sprintf("data:%s;base64,%s", filetype, base64.StdEncoding.EncodeToString(img))

	// Message to be echoed to the channel.
	var message = Message{
		Action:   ActionMessage,
		Channel:  msg.Channel,
		Username: sub.Username,

		// Their image embedded via a data: URI - no server storage needed!
		Message: fmt.Sprintf(
			`<img src="%s" width="%d" height="%d" onclick="setModalImage(this.src)" style="cursor: pointer">`,
			dataURL,
			pvWidth, pvHeight,
		),
	}

	// Is this a DM?
	if strings.HasPrefix(msg.Channel, "@") {
		// Echo the message only to both parties.
		s.SendTo(sub.Username, message)
		message.Channel = "@" + sub.Username

		// Don't deliver it if the receiver has muted us.
		rcpt, err := s.GetSubscriber(strings.TrimPrefix(msg.Channel, "@"))
		if err == nil && rcpt.Mutes(sub.Username) {
			log.Debug("Do not send message to %s: they have muted or booted %s", rcpt.Username, sub.Username)
			return
		}

		// If the sender already mutes the recipient, reply back with the error.
		if sub.Mutes(rcpt.Username) {
			sub.ChatServer("You have muted %s and so your message has not been sent.", rcpt.Username)
			return
		}

		if err := s.SendTo(msg.Channel, message); err != nil {
			sub.ChatServer("Your message could not be delivered: %s", err)
		}
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

	// Hidden status: for operators only, + fake a join/exit chat message.
	if sub.JWTClaims != nil && sub.JWTClaims.IsAdmin {
		if sub.ChatStatus != "hidden" && msg.ChatStatus == "hidden" {
			// Going hidden - fake leave message
			s.Broadcast(Message{
				Action:   ActionPresence,
				Username: sub.Username,
				Message:  "has exited the room!",
			})
		} else if sub.ChatStatus == "hidden" && msg.ChatStatus != "hidden" {
			// Leaving hidden - fake join message
			s.Broadcast(Message{
				Action:   ActionPresence,
				Username: sub.Username,
				Message:  "has joined the room!",
			})
		}
	} else if msg.ChatStatus == "hidden" {
		// normal users can not set this status
		msg.ChatStatus = "away"
	}

	sub.VideoActive = msg.VideoActive
	sub.VideoMutual = msg.VideoMutual
	sub.VideoMutualOpen = msg.VideoMutualOpen
	sub.VideoNSFW = msg.NSFW
	sub.ChatStatus = msg.ChatStatus

	// Sync the WhoList to everybody.
	s.SendWhoList()
}

// OnOpen is a client wanting to start WebRTC with another, e.g. to see their camera.
func (s *Server) OnOpen(sub *Subscriber, msg Message) {
	// Look up the other subscriber.
	other, err := s.GetSubscriber(msg.Username)
	if err != nil {
		log.Error(err.Error())
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

// OnBoot is a user kicking you off their video stream.
func (s *Server) OnBoot(sub *Subscriber, msg Message) {
	log.Info("%s boots %s off their camera", sub.Username, msg.Username)

	sub.muteMu.Lock()
	sub.booted[msg.Username] = struct{}{}
	sub.muteMu.Unlock()

	s.SendWhoList()
}

// OnMute is a user kicking setting the mute flag for another user.
func (s *Server) OnMute(sub *Subscriber, msg Message, mute bool) {
	log.Info("%s mutes or unmutes %s: %v", sub.Username, msg.Username, mute)

	sub.muteMu.Lock()

	if mute {
		sub.muted[msg.Username] = struct{}{}
	} else {
		delete(sub.muted, msg.Username)
	}

	sub.muteMu.Unlock()

	// Send the Who List in case our cam will show as disabled to the muted party.
	s.SendWhoList()
}

// OnCandidate handles WebRTC candidate signaling.
func (s *Server) OnCandidate(sub *Subscriber, msg Message) {
	// Look up the other subscriber.
	other, err := s.GetSubscriber(msg.Username)
	if err != nil {
		log.Error(err.Error())
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
		log.Error(err.Error())
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
		log.Error(err.Error())
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
		log.Error(err.Error())
		return
	}

	other.SendJSON(Message{
		Action:   ActionUnwatch,
		Username: sub.Username,
	})
}
