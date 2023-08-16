package client

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"git.kirsle.net/apps/barertc/pkg/log"
	"git.kirsle.net/apps/barertc/pkg/messages"
	"github.com/aichaos/rivescript-go"
)

const (
	// Number of recent chat messages to hold onto.
	ScrollbackBuffer = 500

	// How long for the lobby room to be quiet before you'll greet the
	// next person who joins the room.
	LobbyDeadThreshold = 30 * time.Minute

	// Default (lobby) channel.
	LobbyChannel = "lobby"
)

// BotHandlers holds onto a set of handler functions for the BareBot.
type BotHandlers struct {
	rs     *rivescript.RiveScript
	client *Client

	// Cache for the Who's Online list.
	whoList []messages.WhoList
	whoMu   sync.RWMutex

	// Auto-greeter cooldowns
	autoGreet         map[string]time.Time
	autoGreetCooldown time.Time // global cooldown between auto-greets
	autoGreetMu       sync.RWMutex

	// MessageID history. Keep a buffer of recent messages sent in
	// case the robot needs to report one (which should generally
	// happen immediately, if it does).
	messageBuf   []messages.Message
	messageBufMu sync.RWMutex

	// Main (lobby) channel quiet detector. Record the time of the last
	// message seen: if the lobby has been quiet for a long time, and
	// someone new joins the room, greet them - overriding the global
	// autoGreet cooldown or ignoring the number of chatters in the room.
	lobbyChannelLastUpdated time.Time

	// Store the reactions we have previously sent by messageID,
	// so we don't accidentally take back our own reactions.
	reactions   map[int]map[string]interface{}
	reactionsMu sync.Mutex
}

// SetupChatbot configures a sensible set of default handlers for the BareBot application.
//
// This function is very opinionated and is designed for the BareBot program. It will
// initialize a RiveScript bot using the brain found at the "./brain" folder, and register
// handlers for the various WebSocket messages on chat.
func (c *Client) SetupChatbot() error {
	var handler = &BotHandlers{
		client: c,
		rs: rivescript.New(&rivescript.Config{
			UTF8: true,
		}),
		autoGreet:  map[string]time.Time{},
		messageBuf: []messages.Message{},
		reactions:  map[int]map[string]interface{}{},
	}

	log.Info("Initializing RiveScript brain")
	if err := handler.rs.LoadDirectory("./brain"); err != nil {
		return fmt.Errorf("RiveScript LoadDirectory: %s", err)
	}
	if err := handler.rs.SortReplies(); err != nil {
		return fmt.Errorf("RiveScript SortReplies: %s", err)
	}

	// Attach RiveScript object macros.
	handler.setObjectMacros()

	// Set all the handler funcs.
	c.OnWho = handler.OnWho
	c.OnMe = handler.OnMe
	c.OnMessage = handler.OnMessage
	c.OnReact = handler.OnReact
	c.OnPresence = handler.OnPresence
	c.OnRing = handler.OnRing
	c.OnOpen = handler.OnOpen
	c.OnWatch = handler.OnWatch
	c.OnUnwatch = handler.OnUnwatch
	c.OnError = handler.OnError
	c.OnDisconnect = handler.OnDisconnect
	c.OnPing = handler.OnPing

	return nil
}

// OnWho handles Who List updates in chat.
func (h *BotHandlers) OnWho(msg messages.Message) {
	log.Info("OnWho: %d people online", len(msg.WhoList))
	h.whoMu.Lock()
	defer h.whoMu.Unlock()
	h.whoList = msg.WhoList
}

// OnMe handles Who List updates in chat.
func (h *BotHandlers) OnMe(msg messages.Message) {
	// Has the server changed our name?
	if h.client.Username() != msg.Username {
		log.Error("OnMe: the server has renamed us to '%s'", msg.Username)
		h.client.claims.Subject = msg.Username
	}
}

// Buffer a message seen on chat for a while.
func (h *BotHandlers) cacheMessage(msg messages.Message) {
	h.messageBufMu.Lock()
	defer h.messageBufMu.Unlock()

	h.messageBuf = append(h.messageBuf, msg)

	if len(h.messageBuf) > ScrollbackBuffer {
		h.messageBuf = h.messageBuf[len(h.messageBuf)-ScrollbackBuffer:]
	}
}

// Get a message by ID from the recent message buffer.
func (h *BotHandlers) getMessageByID(msgID int) (messages.Message, bool) {
	h.messageBufMu.RLock()
	defer h.messageBufMu.RUnlock()
	for _, msg := range h.messageBuf {
		if msg.MessageID == msgID {
			return msg, true
		}
	}

	return messages.Message{}, false
}

// OnMessage handles Who List updates in chat.
func (h *BotHandlers) OnMessage(msg messages.Message) {
	// Strip HTML.
	msg.Message = StripHTML(msg.Message)

	// Ignore echoed message from ourself.
	if msg.Username == h.client.Username() {
		return
	}

	// Cache it in our message buffer.
	h.cacheMessage(msg)

	// Record the last seen if this is the lobby channel.
	if msg.Channel == LobbyChannel {
		h.lobbyChannelLastUpdated = time.Now()
	}

	// Do we send a reply to this?
	var (
		sendReply   bool
		replyPrefix string

		// original topic the user was in, in case of PublicChannel match
		// so we can put the user back in their original topic after.
		userTopic string
	)
	if strings.HasPrefix(msg.Channel, "@") {
		// Direct message: always reply.
		sendReply = true

		// Log message to console.
		log.Info("DM [%s] %s", msg.Username, msg.Message)
	} else {
		// Log message to console.
		log.Info("[%s to #%s] %s", msg.Username, msg.Channel, msg.Message)

		// Public channel message. See if they at-mention the robot.
		if ok, message := AtMentioned(h.client, msg.Message); ok {
			msg.Message = message
			sendReply = true
			replyPrefix = fmt.Sprintf("**@%s:** ", msg.Username)
		} else {
			// We were not at mentioned: can reply anyway but put us
			// into the PublicChannel topic.
			log.Error("trying for PublicChannel")
			if topic, err := h.rs.GetUservar(msg.Username, "topic"); err == nil {
				userTopic = topic
			} else {
				log.Error("Couldn't get topic for %s: %s", msg.Username, err)
				userTopic = "random"
			}

			h.rs.SetUservar(msg.Username, "topic", "PublicChannel")
			sendReply = true

			// Restore the user's original topic?
			defer func() {
				if userTopic != "" {
					log.Error("Set user topic back to: %s", userTopic)
					h.rs.SetUservar(msg.Username, "topic", userTopic)
				}
			}()
		}
	}

	// Do we reply?
	if sendReply {
		// Set their user variables.
		h.SetUserVariables(msg)
		reply, err := h.rs.Reply(msg.Username, msg.Message)
		log.Error("REPLY: %s", reply)
		if NoReply(reply) {
			return
		}

		// Delay a moment before responding.
		time.Sleep(500 * time.Millisecond)

		if err != nil {
			h.client.Send(messages.Message{
				Action:   messages.ActionMessage,
				Channel:  msg.Channel,
				Username: msg.Username,
				Message:  fmt.Sprintf("[RiveScript Error] %s", err),
			})
		} else {
			h.client.Send(messages.Message{
				Action:   messages.ActionMessage,
				Channel:  msg.Channel,
				Username: msg.Username,
				Message:  replyPrefix + reply,
			})
		}
	}
}

// OnTakeback handles Who List updates in chat.
func (h *BotHandlers) OnTakeback(msg messages.Message) {
	log.Info("Takeback: user %s takes back msgID %d", msg.Username, msg.MessageID)
}

// OnReact handles Who List updates in chat.
func (h *BotHandlers) OnReact(msg messages.Message) {
	log.Info("React: user %s reacts with %s on msgID %d", msg.Username, msg.Message, msg.MessageID)

	// Ignore echoed message from ourself.
	if msg.Username == h.client.Username() {
		return
	}

	// Sanity check that we can actually see the message being reacted to: so we don't
	// upvote reactions posted to messageIDs in other peoples' DM threads.
	if _, ok := h.getMessageByID(msg.MessageID); !ok {
		return
	}

	// If we have already reacted to it, don't react again.
	h.reactionsMu.Lock()
	defer h.reactionsMu.Unlock()
	if _, ok := h.reactions[msg.MessageID]; !ok {
		h.reactions[msg.MessageID] = map[string]interface{}{}
	}
	if _, ok := h.reactions[msg.MessageID][msg.Message]; ok {
		log.Info("I already reacted %s on message %d", msg.Message, msg.MessageID)
		return // already upvoted it
	} else {
		h.reactions[msg.MessageID][msg.Message] = nil
	}

	// Half the time, agree with the reaction.
	if rand.Intn(100) > 50 {
		go func() {
			time.Sleep(2500 * time.Millisecond)
			h.client.Send(messages.Message{
				Action:    messages.ActionReact,
				MessageID: msg.MessageID,
				Message:   msg.Message,
			})
		}()
	}
}

// OnPresence handles Who List updates in chat.
func (h *BotHandlers) OnPresence(msg messages.Message) {
	log.Info("Presence: [%s] %s", msg.Username, msg.Message)

	// Ignore echoed message from ourself.
	if msg.Username == h.client.Username() {
		return
	}

	// A join message?
	if strings.Contains(msg.Message, "has joined the room") {
		// Do we force a greeting? (if lobby channel has been quiet)
		var forceGreeting = time.Now().Sub(h.lobbyChannelLastUpdated) > LobbyDeadThreshold

		// Global auto-greet cooldown.
		if time.Now().Before(h.autoGreetCooldown) {
			return
		}
		h.autoGreetCooldown = time.Now().Add(15 * time.Minute)

		// Don't greet the same user too often in case of bouncing.
		h.autoGreetMu.Lock()
		if timeout, ok := h.autoGreet[msg.Username]; ok {
			if time.Now().Before(timeout) && !forceGreeting {
				// Do not greet again.
				log.Info("Do not auto-greet again: too soon")
				h.autoGreetMu.Unlock()
				return
			}
		}
		h.autoGreet[msg.Username] = time.Now().Add(time.Hour)
		h.autoGreetMu.Unlock()

		// Send a message to the lobby. TODO: configurable channel name.
		time.Sleep(5 * time.Second)

		// Ensure they are still online.
		if _, ok := h.GetUser(msg.Username); !ok {
			log.Error("Wanted to auto-greet [%s] but they left the room!", msg.Username)
			return
		}

		// Set their user variables.
		h.SetUserVariables(msg)
		if forceGreeting {
			h.rs.SetGlobal("numUsersOnline", "0")
		}
		reply, err := h.rs.Reply(msg.Username, "/greet")
		if err == nil && !NoReply(reply) {
			h.client.Send(messages.Message{
				Action:   messages.ActionMessage,
				Channel:  LobbyChannel,
				Username: msg.Username,
				Message:  reply,
			})
		}
	}
}

// OnRing handles Who List updates in chat.
func (h *BotHandlers) OnRing(msg messages.Message) {

}

// OnOpen handles Who List updates in chat.
func (h *BotHandlers) OnOpen(msg messages.Message) {

}

// OnWatch handles Who List updates in chat.
func (h *BotHandlers) OnWatch(msg messages.Message) {

}

// OnUnwatch handles Who List updates in chat.
func (h *BotHandlers) OnUnwatch(msg messages.Message) {

}

// OnError handles Who List updates in chat.
func (h *BotHandlers) OnError(msg messages.Message) {
	log.Error("[%s] %s", msg.Username, msg.Message)
}

// OnDisconnect handles Who List updates in chat.
func (h *BotHandlers) OnDisconnect(msg messages.Message) {

}

// OnPing handles Who List updates in chat.
func (h *BotHandlers) OnPing(msg messages.Message) {

}
