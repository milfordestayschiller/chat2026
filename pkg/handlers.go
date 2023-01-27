package barertc

import (
	"fmt"
	"time"

	"git.kirsle.net/apps/barertc/pkg/log"
)

// OnLogin handles "login" actions from the client.
func (s *Server) OnLogin(sub *Subscriber, msg Message) {
	// Ensure the username is unique, or rename it.
	var duplicate bool
	for other := range s.IterSubscribers() {
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
}

// OnMessage handles a chat message posted by the user.
func (s *Server) OnMessage(sub *Subscriber, msg Message) {
	log.Info("[%s] %s", sub.Username, msg.Message)
	if sub.Username == "" {
		sub.SendJSON(Message{
			Action:   ActionMessage,
			Username: "ChatServer",
			Message:  "You must log in first.",
		})
		return
	}

	// Broadcast a chat message to the room.
	s.Broadcast(Message{
		Action:   ActionMessage,
		Username: sub.Username,
		Message:  msg.Message,
	})
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
