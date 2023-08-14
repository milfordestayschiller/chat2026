package client

import (
	"fmt"
	"strconv"
	"time"

	"git.kirsle.net/apps/barertc/pkg/messages"
	"github.com/aichaos/rivescript-go"
)

// Set up object macros for RiveScript.
func (h *BotHandlers) setObjectMacros() {
	// Reload the bot's RiveScript brain.
	h.rs.SetSubroutine("reload", func(rs *rivescript.RiveScript, args []string) string {
		var bot = rivescript.New(&rivescript.Config{
			UTF8:  true,
			Debug: rs.Debug,
		})
		if err := bot.LoadDirectory("brain"); err != nil {
			return fmt.Sprintf("Error on LoadDirectory: %s", err)
		}
		if err := bot.SortReplies(); err != nil {
			return fmt.Sprintf("Error on SortReplies: %s", err)
		}

		// Install the new bot and set object macros on it.
		h.rs = bot
		h.setObjectMacros()

		return "The RiveScript brain has been reloaded!"
	})

	// React to a message.
	h.rs.SetSubroutine("react", func(rs *rivescript.RiveScript, args []string) string {
		if len(args) >= 2 {
			if msgID, err := strconv.Atoi(args[0]); err == nil {
				// With a small delay.
				go func() {
					time.Sleep(2500 * time.Millisecond)
					h.client.Send(messages.Message{
						Action:    messages.ActionReact,
						MessageID: msgID,
						Message:   args[1],
					})
				}()
			} else {
				return fmt.Sprintf("[react: %s]", err)
			}
		} else {
			return "[react: invalid number of parameters]"
		}
		return ""
	})
}
