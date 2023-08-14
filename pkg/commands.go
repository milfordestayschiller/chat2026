package barertc

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"git.kirsle.net/apps/barertc/pkg/config"
	ourjwt "git.kirsle.net/apps/barertc/pkg/jwt"
	"git.kirsle.net/apps/barertc/pkg/log"
	"git.kirsle.net/apps/barertc/pkg/messages"
	"github.com/golang-jwt/jwt/v4"
	"github.com/mattn/go-shellwords"
)

// ProcessCommand parses a chat message for "/commands"
func (s *Server) ProcessCommand(sub *Subscriber, msg messages.Message) bool {
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
		case "/unban":
			s.UnbanCommand(words, sub)
			return true
		case "/bans":
			s.BansCommand(words, sub)
			return true
		case "/nsfw":
			s.NSFWCommand(words, sub)
			return true
		case "/help":
			sub.ChatServer(RenderMarkdown("Moderator commands are:\n\n" +
				"* `/kick <username>` to kick from chat\n" +
				"* `/ban <username> <duration>` to ban from chat (default duration is 24 (hours))\n" +
				"* `/unban <username>` to list the ban on a user\n" +
				"* `/bans` to list current banned users and their expiration date\n" +
				"* `/nsfw <username>` to mark their camera NSFW\n" +
				"* `/op <username>` to grant operator rights to a user\n" +
				"* `/deop <username>` to remove operator rights from a user\n" +
				"* `/shutdown` to gracefully shut down (reboot) the chat server\n" +
				"* `/kickall` to kick EVERYBODY off and force them to log back in\n" +
				"* `/reconfigure` to dynamically reload the chat server settings file\n" +
				"* `/help` to show this message\n\n" +
				"Note: shell-style quoting is supported, if a username has a space in it, quote the whole username, e.g.: `/kick \"username 2\"`",
			))
			return true
		case "/shutdown":
			s.Broadcast(messages.Message{
				Action:   messages.ActionError,
				Username: "ChatServer",
				Message:  "The chat server is going down for a reboot NOW!",
			})
			os.Exit(1)
			return true
		case "/kickall":
			s.KickAllCommand()
			return true
		case "/reconfigure":
			s.ReconfigureCommand(sub)
			return true
		case "/op":
			s.OpCommand(words, sub)
			return true
		case "/deop":
			s.DeopCommand(words, sub)
			return true
		}

	}

	// Not handled.
	return false
}

// NSFWCommand handles the `/nsfw` operator command.
func (s *Server) NSFWCommand(words []string, sub *Subscriber) {
	if len(words) == 1 {
		sub.ChatServer("Usage: `/nsfw username` to add the NSFW flag to their camera.")
	}
	username := words[1]
	other, err := s.GetSubscriber(username)
	if err != nil {
		sub.ChatServer("/nsfw: username not found: %s", username)
	} else {
		// The message to deliver to the target.
		var message = "Just a friendly reminder to mark your camera as 'Explicit' by using the button at the top " +
			"of the page if you are going to be sexual on webcam. "

		// If the admin who marked it was previously booted
		if other.Boots(sub.Username) {
			message += "Your camera was detected to depict 'Explicit' activity and has been marked for you."
		} else {
			message += fmt.Sprintf("Your camera has been marked as Explicit for you by @%s", sub.Username)
		}

		other.ChatServer(message)
		other.VideoStatus |= messages.VideoFlagNSFW
		other.SendMe()
		s.SendWhoList()
		sub.ChatServer("%s now has their camera marked as Explicit", username)
	}
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
		other.SendJSON(messages.Message{
			Action: messages.ActionKick,
		})
		s.DeleteSubscriber(other)
		sub.ChatServer("%s has been kicked from the room", username)

		// Broadcast it to everyone.
		s.Broadcast(messages.Message{
			Action:   messages.ActionPresence,
			Username: username,
			Message:  "has been kicked from the room!",
		})
	}
}

// KickAllCommand kicks everybody out of the room.
func (s *Server) KickAllCommand() {

	// If we have JWT enabled and a landing page, link users to it.
	if config.Current.JWT.Enabled && config.Current.JWT.LandingPageURL != "" {
		s.Broadcast(messages.Message{
			Action:   messages.ActionError,
			Username: "ChatServer",
			Message: fmt.Sprintf(
				"<strong>Notice:</strong> The chat operator has requested that you log back in to the chat room. "+
					"Probably, this is because a new feature was launched that needs you to reload the page. "+
					"You may refresh the tab or <a href=\"%s\">click here</a> to re-enter the room.",
				config.Current.JWT.LandingPageURL,
			),
		})
	} else {
		s.Broadcast(messages.Message{
			Action:   messages.ActionError,
			Username: "ChatServer",
			Message: "<strong>Notice:</strong> The chat operator has kicked everybody from the room. Usually, this " +
				"may mean a new feature of the chat has been launched and you need to reload the page for it " +
				"to function correctly.",
		})
	}

	// Kick everyone off.
	s.Broadcast(messages.Message{
		Action: messages.ActionKick,
	})

	// Disconnect everybody.
	for _, sub := range s.IterSubscribers() {
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
				"Set another duration (in hours) like: `/ban username 2` for a 2-hour ban.",
		))
		return
	}

	// Parse the command.
	var (
		username = words[1]
		duration = 24 * time.Hour
	)
	if len(words) >= 3 {
		if dur, err := strconv.Atoi(words[2]); err == nil {
			duration = time.Duration(dur) * time.Hour
		}
	}

	log.Info("Operator %s bans %s for %d hours", sub.Username, username, duration/time.Hour)

	other, err := s.GetSubscriber(username)
	if err != nil {
		sub.ChatServer("/ban: username not found: %s", username)
	} else {
		// Ban them.
		BanUser(username, duration)

		// Broadcast it to everyone.
		s.Broadcast(messages.Message{
			Action:   messages.ActionPresence,
			Username: username,
			Message:  "has been banned!",
		})

		other.ChatServer("You have been banned from the chat room by %s. You may come back after %d hours.", sub.Username, duration/time.Hour)
		other.SendJSON(messages.Message{
			Action: messages.ActionKick,
		})
		s.DeleteSubscriber(other)
		sub.ChatServer("%s has been banned from the room for %d hours.", username, duration/time.Hour)
	}
}

// UnbanCommand handles the `/unban` operator command.
func (s *Server) UnbanCommand(words []string, sub *Subscriber) {
	if len(words) == 1 {
		sub.ChatServer(RenderMarkdown(
			"Usage: `/unban username` to lift the ban on a user and allow them back into the chat room.",
		))
		return
	}

	// Parse the command.
	var username = words[1]

	if UnbanUser(username) {
		sub.ChatServer("The ban on %s has been lifted.", username)
	} else {
		sub.ChatServer("/unban: user %s was not found to be banned. Try `/bans` to see current banned users.", username)
	}
}

// BansCommand handles the `/bans` operator command.
func (s *Server) BansCommand(words []string, sub *Subscriber) {
	result := StringifyBannedUsers()
	sub.ChatServer(
		RenderMarkdown("The listing of banned users currently includes:\n\n" + result),
	)
}

// ReconfigureCommand handles the `/reconfigure` operator command.
func (s *Server) ReconfigureCommand(sub *Subscriber) {
	// Reload the settings.
	if err := config.LoadSettings(); err != nil {
		sub.ChatServer("Error reloading the server config: %s", err)
		return
	}

	sub.ChatServer("The server config file has been reloaded successfully!")
}

// OpCommand handles the `/op` operator command.
func (s *Server) OpCommand(words []string, sub *Subscriber) {
	if len(words) == 1 {
		sub.ChatServer(RenderMarkdown(
			"Usage: `/op username` to grant temporary operator rights to a user.",
		))
		return
	}

	// Parse the command.
	var username = words[1]
	if other, err := s.GetSubscriber(username); err != nil {
		sub.ChatServer("/op: user %s was not found.", username)
	} else {
		if other.JWTClaims == nil {
			other.JWTClaims = &ourjwt.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: username,
				},
			}
		}
		other.JWTClaims.IsAdmin = true

		// Send everyone the Who List.
		s.SendWhoList()

		sub.ChatServer("Operator rights have been granted to %s", username)
	}
}

// DeopCommand handles the `/deop` operator command.
func (s *Server) DeopCommand(words []string, sub *Subscriber) {
	if len(words) == 1 {
		sub.ChatServer(RenderMarkdown(
			"Usage: `/deop username` to remove operator rights from a user.",
		))
		return
	}

	// Parse the command.
	var username = words[1]
	if other, err := s.GetSubscriber(username); err != nil {
		sub.ChatServer("/deop: user %s was not found.", username)
	} else {
		if other.JWTClaims == nil {
			other.JWTClaims = &ourjwt.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: username,
				},
			}
		}
		other.JWTClaims.IsAdmin = false

		// Send everyone the Who List.
		s.SendWhoList()

		sub.ChatServer("Operator rights have been taken from %s", username)
	}
}
