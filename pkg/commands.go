package barertc

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"git.kirsle.net/apps/barertc/pkg/config"
	"git.kirsle.net/apps/barertc/pkg/log"
	"github.com/mattn/go-shellwords"
)

// ProcessCommand parses a chat message for "/commands"
func (s *Server) ProcessCommand(sub *Subscriber, msg Message) bool {
	if len(msg.Message) == 0 || msg.Message[0] != '/' {
		return false
	}

	// Line begins with a slash, parse it apart.
	words, err := shellwords.Parse(msg.Message)
	if err != nil {
		log.Error("ProcessCommands: parsing shell words: %s", err)
		return false
	} else if len(words) == 0 {
		return false
	}

	// Moderator commands.
	if sub.JWTClaims != nil && sub.JWTClaims.IsAdmin {
		switch words[0] {
		case "/kick":
			s.KickCommand(words, sub)
			return true
		case "/ban":
			s.BanCommand(words, sub)
			return true
		case "/nsfw":
			if len(words) == 1 {
				sub.ChatServer("Usage: `/nsfw username` to add the NSFW flag to their camera.")
			}
			username := words[1]
			other, err := s.GetSubscriber(username)
			if err != nil {
				sub.ChatServer("/nsfw: username not found: %s", username)
			} else {
				other.ChatServer("Your camera has been marked as NSFW by %s", sub.Username)
				other.VideoStatus |= VideoFlagNSFW
				other.SendMe()
				s.SendWhoList()
				sub.ChatServer("%s has their camera marked as NSFW", username)
			}
			return true
		case "/help":
			sub.ChatServer(RenderMarkdown("Moderator commands are:\n\n" +
				"* `/kick <username>` to kick from chat\n" +
				"* `/nsfw <username>` to mark their camera NSFW\n" +
				"* `/shutdown` to gracefully shut down (reboot) the chat server\n" +
				"* `/kickall` to kick EVERYBODY off and force them to log back in\n" +
				"* `/help` to show this message\n\n" +
				"Note: shell-style quoting is supported, if a username has a space in it, quote the whole username, e.g.: `/kick \"username 2\"`",
			))
			return true
		case "/shutdown":
			s.Broadcast(Message{
				Action:   ActionError,
				Username: "ChatServer",
				Message:  "The chat server is going down for a reboot NOW!",
			})
			os.Exit(1)
		case "/kickall":
			s.KickAllCommand()
		}

	}

	// Not handled.
	return false
}

// KickCommand handles the `/kick` operator command.
func (s *Server) KickCommand(words []string, sub *Subscriber) {
	if len(words) == 1 {
		sub.ChatServer(RenderMarkdown(
			"Usage: `/kick username` to remove the user from the chat room.\n\nNote: if the username has spaces in it, quote the name (shell style), `/kick \"username 2\"`",
		))
		return
	}
	username := words[1]
	other, err := s.GetSubscriber(username)
	if err != nil {
		sub.ChatServer("/kick: username not found: %s", username)
	} else if other.Username == sub.Username {
		sub.ChatServer("/kick: did you really mean to kick yourself?")
	} else {
		other.ChatServer("You have been kicked from the chat room by %s", sub.Username)
		other.SendJSON(Message{
			Action: ActionKick,
		})
		s.DeleteSubscriber(other)
		sub.ChatServer("%s has been kicked from the room", username)
	}
}

// KickAllCommand kicks everybody out of the room.
func (s *Server) KickAllCommand() {

	// If we have JWT enabled and a landing page, link users to it.
	if config.Current.JWT.Enabled && config.Current.JWT.LandingPageURL != "" {
		s.Broadcast(Message{
			Action:   ActionError,
			Username: "ChatServer",
			Message: fmt.Sprintf(
				"<strong>Notice:</strong> The chat operator has requested that you log back in to the chat room. "+
					"Probably, this is because a new feature was launched that needs you to reload the page. "+
					"You may refresh the tab or <a href=\"%s\">click here</a> to re-enter the room.",
				config.Current.JWT.LandingPageURL,
			),
		})
	} else {
		s.Broadcast(Message{
			Action:   ActionError,
			Username: "ChatServer",
			Message: "<strong>Notice:</strong> The chat operator has kicked everybody from the room. Usually, this " +
				"may mean a new feature of the chat has been launched and you need to reload the page for it " +
				"to function correctly.",
		})
	}

	// Kick everyone off.
	s.Broadcast(Message{
		Action: ActionKick,
	})

	// Disconnect everybody.
	s.subscribersMu.RLock()
	defer s.subscribersMu.RUnlock()
	for _, sub := range s.IterSubscribers(true) {
		if !sub.authenticated {
			continue
		}

		s.DeleteSubscriber(sub)
	}
}

// BanCommand handles the `/ban` operator command.
func (s *Server) BanCommand(words []string, sub *Subscriber) {
	if len(words) == 1 {
		sub.ChatServer(RenderMarkdown(
			"Usage: `/ban username` to remove the user from the chat room for 24 hours (default).\n\n" +
				"Set another duration (in hours, fractions supported) like: `/ban username 0.5` for a 30-minute ban.",
		))
		return
	}

	// Parse the command.
	var (
		username = words[1]
		duration = 24 * time.Hour
	)
	if len(words) >= 3 {
		if dur, err := strconv.ParseFloat(words[2], 64); err == nil {
			if dur < 1 {
				duration = time.Duration(dur*60) * time.Second
			} else {
				duration = time.Duration(dur) * time.Hour
			}
		}
	}

	// TODO: banning, for now it just kicks.
	_ = duration

	other, err := s.GetSubscriber(username)
	if err != nil {
		sub.ChatServer("/ban: username not found: %s", username)
	} else {
		other.ChatServer("You have been kicked from the chat room by %s", sub.Username)
		other.SendJSON(Message{
			Action: ActionKick,
		})
		s.DeleteSubscriber(other)
		sub.ChatServer("%s has been kicked from the room", username)
	}
}
