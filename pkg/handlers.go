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
	"git.kirsle.net/apps/barertc/pkg/messages"
	"git.kirsle.net/apps/barertc/pkg/util"
)

// OnLogin handles "login" actions from the client.
func (s *Server) OnLogin(sub *Subscriber, msg messages.Message) {
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
	username, err := s.UniqueUsername(msg.Username)
	if err != nil {
		// If JWT authentication was used: disconnect the original (conflicting) username.
		if claims.Subject == msg.Username {
			if other, err := s.GetSubscriber(msg.Username); err == nil {
				other.ChatServer("You have been signed out of chat because you logged in from another location.")
				other.SendJSON(messages.Message{
					Action: messages.ActionKick,
				})
				s.DeleteSubscriber(other)
			}

			// They will take over their original username.
			username = msg.Username
		}

		// If JWT auth was not used: UniqueUsername already gave them a uniquely spelled name.
	}
	msg.Username = username

	// Is the username currently banned?
	if IsBanned(msg.Username) {
		sub.ChatServer(
			"You are currently banned from entering the chat room. Chat room bans are temporarily and usually last for " +
				"24 hours. Please try coming back later.",
		)
		sub.SendJSON(messages.Message{
			Action: messages.ActionKick,
		})
		s.DeleteSubscriber(sub)
		return
	}

	// Use their username.
	sub.Username = msg.Username
	sub.authenticated = true
	sub.loginAt = time.Now()
	log.Debug("OnLogin: %s joins the room", sub.Username)

	// Tell everyone they joined.
	s.Broadcast(messages.Message{
		Action:   messages.ActionPresence,
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
			sub.SendJSON(messages.Message{
				Channel:  channel.ID,
				Action:   messages.ActionError,
				Username: "ChatServer",
				Message:  RenderMarkdown(msg),
			})
		}
	}
}

// OnMessage handles a chat message posted by the user.
func (s *Server) OnMessage(sub *Subscriber, msg messages.Message) {
	if !strings.HasPrefix(msg.Channel, "@") {
		log.Info("[%s to #%s] %s", sub.Username, msg.Channel, msg.Message)
	}

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

	// Assign a message ID and own it to the sender.
	messages.MessageID++
	var mid = messages.MessageID
	sub.midMu.Lock()
	sub.messageIDs[mid] = struct{}{}
	sub.midMu.Unlock()

	// Message to be echoed to the channel.
	var message = messages.Message{
		Action:    messages.ActionMessage,
		Channel:   msg.Channel,
		Username:  sub.Username,
		Message:   markdown,
		MessageID: mid,
	}

	// Is this a DM?
	if strings.HasPrefix(msg.Channel, "@") {
		// Echo the message only to both parties.
		s.SendTo(sub.Username, message)
		message.Channel = "@" + sub.Username

		// Don't deliver it if the receiver has muted us. Note: admin users, even if muted,
		// can still deliver a DM to the one who muted them.
		rcpt, err := s.GetSubscriber(strings.TrimPrefix(msg.Channel, "@"))
		if err == nil && rcpt.Mutes(sub.Username) && !sub.IsAdmin() {
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

// OnTakeback handles takebacks (delete your message for everybody)
func (s *Server) OnTakeback(sub *Subscriber, msg messages.Message) {
	// Permission check.
	if sub.JWTClaims == nil || !sub.JWTClaims.IsAdmin {
		sub.midMu.Lock()
		defer sub.midMu.Unlock()
		if _, ok := sub.messageIDs[msg.MessageID]; !ok {
			sub.ChatServer("That is not your message to take back.")
			return
		}
	}

	// Broadcast to everybody to remove this message.
	s.Broadcast(messages.Message{
		Action:    messages.ActionTakeback,
		MessageID: msg.MessageID,
	})
}

// OnReact handles emoji reactions for chat messages.
func (s *Server) OnReact(sub *Subscriber, msg messages.Message) {
	// Forward the reaction to everybody.
	s.Broadcast(messages.Message{
		Action:    messages.ActionReact,
		Username:  sub.Username,
		Message:   msg.Message,
		MessageID: msg.MessageID,
	})
}

// OnFile handles a picture shared in chat with a channel.
func (s *Server) OnFile(sub *Subscriber, msg messages.Message) {
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

	// Assign a message ID and own it to the sender.
	messages.MessageID++
	var mid = messages.MessageID
	sub.midMu.Lock()
	sub.messageIDs[mid] = struct{}{}
	sub.midMu.Unlock()

	// Message to be echoed to the channel.
	var message = messages.Message{
		Action:    messages.ActionMessage,
		Channel:   msg.Channel,
		Username:  sub.Username,
		MessageID: mid,

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
func (s *Server) OnMe(sub *Subscriber, msg messages.Message) {
	if msg.VideoStatus&messages.VideoFlagActive == messages.VideoFlagActive {
		log.Debug("User %s turns on their video feed", sub.Username)
	}

	// Hidden status: for operators only, + fake a join/exit chat message.
	if sub.JWTClaims != nil && sub.JWTClaims.IsAdmin {
		if sub.ChatStatus != "hidden" && msg.ChatStatus == "hidden" {
			// Going hidden - fake leave message
			s.Broadcast(messages.Message{
				Action:   messages.ActionPresence,
				Username: sub.Username,
				Message:  "has exited the room!",
			})
		} else if sub.ChatStatus == "hidden" && msg.ChatStatus != "hidden" {
			// Leaving hidden - fake join message
			s.Broadcast(messages.Message{
				Action:   messages.ActionPresence,
				Username: sub.Username,
				Message:  "has joined the room!",
			})
		}
	} else if msg.ChatStatus == "hidden" {
		// normal users can not set this status
		msg.ChatStatus = "away"
	}

	sub.VideoStatus = msg.VideoStatus
	sub.ChatStatus = msg.ChatStatus

	// Sync the WhoList to everybody.
	s.SendWhoList()
}

// OnOpen is a client wanting to start WebRTC with another, e.g. to see their camera.
func (s *Server) OnOpen(sub *Subscriber, msg messages.Message) {
	// Look up the other subscriber.
	other, err := s.GetSubscriber(msg.Username)
	if err != nil {
		log.Error(err.Error())
		return
	}

	// Make up a WebRTC shared secret and send it to both of them.
	secret := util.RandomString(16)
	log.Info("WebRTC: %s opens %s with secret %s", sub.Username, other.Username, secret)

	// If the current user is an admin and was booted or muted, inform them.
	if sub.IsAdmin() {
		if other.Boots(sub.Username) {
			sub.ChatServer("Note: %s had booted you off their camera before, and won't be notified of your watch.", other.Username)
		} else if other.Mutes(sub.Username) {
			sub.ChatServer("Note: %s had muted you before, and won't be notified of your watch.", other.Username)
		}
	}

	// Ring the target of this request and give them the secret.
	other.SendJSON(messages.Message{
		Action:     messages.ActionRing,
		Username:   sub.Username,
		OpenSecret: secret,
	})

	// To the caller, echo back the Open along with the secret.
	sub.SendJSON(messages.Message{
		Action:     messages.ActionOpen,
		Username:   other.Username,
		OpenSecret: secret,
	})
}

// OnBoot is a user kicking you off their video stream.
func (s *Server) OnBoot(sub *Subscriber, msg messages.Message) {
	log.Info("%s boots %s off their camera", sub.Username, msg.Username)

	sub.muteMu.Lock()
	sub.booted[msg.Username] = struct{}{}
	sub.muteMu.Unlock()

	// If the subject of the boot is an admin, inform them they have been booted.
	if other, err := s.GetSubscriber(msg.Username); err == nil && other.IsAdmin() {
		other.ChatServer(
			"%s has booted you off of their camera!",
			sub.Username,
		)
	}

	s.SendWhoList()
}

// OnMute is a user kicking setting the mute flag for another user.
func (s *Server) OnMute(sub *Subscriber, msg messages.Message, mute bool) {
	log.Info("%s mutes or unmutes %s: %v", sub.Username, msg.Username, mute)

	sub.muteMu.Lock()

	if mute {
		sub.muted[msg.Username] = struct{}{}
	} else {
		delete(sub.muted, msg.Username)
	}

	sub.muteMu.Unlock()

	// If the subject of the mute is an admin, inform them they have been booted.
	if other, err := s.GetSubscriber(msg.Username); err == nil && other.IsAdmin() {
		other.ChatServer(
			"%s has muted you! Your new mute status is: %v",
			sub.Username, mute,
		)
	}

	// Send the Who List in case our cam will show as disabled to the muted party.
	s.SendWhoList()
}

// OnBlocklist is a bulk user mute from the CachedBlocklist sent by the website.
func (s *Server) OnBlocklist(sub *Subscriber, msg messages.Message) {
	log.Info("%s syncs their blocklist: %s", sub.Username, msg.Usernames)

	sub.muteMu.Lock()
	for _, username := range msg.Usernames {
		sub.muted[username] = struct{}{}
	}

	sub.muteMu.Unlock()

	// Send the Who List in case our cam will show as disabled to the muted party.
	s.SendWhoList()
}

// OnReport handles a user's report of a message.
func (s *Server) OnReport(sub *Subscriber, msg messages.Message) {
	if !WebhookEnabled(WebhookReport) {
		sub.ChatServer("Unfortunately, the report webhook is not enabled so your report could not be received!")
		return
	}

	// Post to the report webhook.
	if err := PostWebhook(WebhookReport, WebhookRequest{
		Action: WebhookReport,
		APIKey: config.Current.AdminAPIKey,
		Report: WebhookRequestReport{
			FromUsername:  sub.Username,
			AboutUsername: msg.Username,
			Channel:       msg.Channel,
			Timestamp:     msg.Timestamp,
			Reason:        msg.Reason,
			Message:       msg.Message,
			Comment:       msg.Comment,
		},
	}); err != nil {
		sub.ChatServer("Error sending the report to the website: %s", err)
	} else {
		sub.ChatServer("Your report has been delivered successfully.")
	}
}

// OnCandidate handles WebRTC candidate signaling.
func (s *Server) OnCandidate(sub *Subscriber, msg messages.Message) {
	// Look up the other subscriber.
	other, err := s.GetSubscriber(msg.Username)
	if err != nil {
		log.Error(err.Error())
		return
	}

	other.SendJSON(messages.Message{
		Action:    messages.ActionCandidate,
		Username:  sub.Username,
		Candidate: msg.Candidate,
	})
}

// OnSDP handles WebRTC sdp signaling.
func (s *Server) OnSDP(sub *Subscriber, msg messages.Message) {
	// Look up the other subscriber.
	other, err := s.GetSubscriber(msg.Username)
	if err != nil {
		log.Error(err.Error())
		return
	}

	other.SendJSON(messages.Message{
		Action:      messages.ActionSDP,
		Username:    sub.Username,
		Description: msg.Description,
	})
}

// OnWatch communicates video watching status between users.
func (s *Server) OnWatch(sub *Subscriber, msg messages.Message) {
	// Look up the other subscriber.
	other, err := s.GetSubscriber(msg.Username)
	if err != nil {
		log.Error(err.Error())
		return
	}

	other.SendJSON(messages.Message{
		Action:   messages.ActionWatch,
		Username: sub.Username,
	})
}

// OnUnwatch communicates video Unwatching status between users.
func (s *Server) OnUnwatch(sub *Subscriber, msg messages.Message) {
	// Look up the other subscriber.
	other, err := s.GetSubscriber(msg.Username)
	if err != nil {
		log.Error(err.Error())
		return
	}

	other.SendJSON(messages.Message{
		Action:   messages.ActionUnwatch,
		Username: sub.Username,
	})
}
