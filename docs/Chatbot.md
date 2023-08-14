# BareBot (Chatbot Program)

The source repo for BareRTC also includes a chatbot program that you can use for fun & games and for auto-moderating your room.

The entrypoint for the program is at `cmd/BareBot/main.go` and compiles to a `BareBot` command line program.

This feature is currently still brand new and will be fleshed out over time.

# Quick Start

Initialize a new chatbot:

```bash
$ BareBot init /path/to/chatbot

# Example
$ BareBot init ./chatbot
```

Point it to a new or empty directory and it will populate it with a `chatbot.toml` settings file and a default RiveScript brain. The default brain will come with useful triggers and topics set up for integration with your BareRTC server.

Edit your `chatbot.toml` to configure it and then run it:

```bash
$ BareBot run /path/to/chatbot

# Example
$ BareBot run ./chatbot
```

You may also simply call `BareBot run` from inside your bot's folder (with the ./chatbot.toml file at the current working directory).

# Settings

An example of the chatbot.toml file:

```toml
Version = 1
WebSocketReadLimit = 41943040

[BareRTC]
  AdminAPIKey = "c0ffd6b5-37ce-4184-a3df-a28b698ecb48"
  URL = "http://localhost:9000"

[Profile]
  Username = "shybot"
  Nickname = "BareBOT"
  ProfileURL = "/u/shybot"
  AvatarURL = "http://localhost:9000/static/img/server.png"
  Emoji = "🤖"
  Gender = "f"
  IsAdmin = true
```

Settings you'll want to configure include:

* BareRTC/URL: the base website URL to your BareRTC server.
* BareRTC/AdminAPIKey: this should match the AdminAPIKey in your BareRTC settings.toml -- used for authentication.
* Profile: these are the JWT claims for user authentication: how you want your chatbot to look in chat.

# Features

## RiveScript

The reply engine for BareBot is [RiveScript](https://www.rivescript.com), a chatbot scripting language that makes it easy to set up custom "canned response" trigger and response pairs to match your user's messages.

## Direct Message Conversations

Users who send a DM to the chatbot may get a response to every message they send.

## At-mentions in Public Channels

Users in a public channel can invoke the bot also by at-mentioning its username in chat (starting or ending their message with the bot's username). The bot will fetch a reply as normal, and send it to the public channel while at-mentioning the user's name back.

## Public Channel Keywords

The default RiveScript file `public_keywords.rive` sets up some default triggers to be matched on public channel messages (where the bot was _not_ at mentioned).

How it works is that on public channel messages (not at-mentioned) the user is placed into a special RiveScript topic named "PublicChannel" that constrains the set of triggers that will be tried for a match.

Avoid spamming public channels too much: the default topic in `public_keywords.rive` sets a catch-all `*` trigger that says `<noreply>` so that the bot will not send a message to the channel unless another trigger has it do so.

Examples what you can do with this includes:

* If a user says hello to the chat, react to their message with a wave emoji.
* If a user shares a picture on chat, randomly decide to react to it.
* If users say certain keywords, you can send messages to the chat in response or take other actions (such as kick the user from the room).

## Auto Greeter

The default chatbot brain has a `/greet` command in `commands.rive` which is called during presence updates when a user joins the room.

The chatbot will say hello to a new user (in the default "lobby" chat room ID - TODO: make configurable), no more than one time per hour. So if a user is popping in and out the bot won't spam and greet them too often.

The bot waits a few seconds before greeting, and if the user logs off before, then the bot doesn't send the message.

The bot also will not greet users when there are more than 10 people in the room by default - you can tune this in `commands.rive` if you like.

# RiveScript Variables

For user messages, the following variables are set on the RiveScript instance for the current user:

* `<get name>` will be the user's display name (nickname) or username if not set.
* `<get isAdmin>` will be "true" if the user has admin (operator) status or "false" if not.
* `<get messageID>` will be the BareRTC MessageID of the user's message you are responding to (integer value, useful for the `react` object macro).

Global variables available in your RiveScript replies include:

* `<env numUsersOnline>` will be an integer number of chatters currently on the room, in case you want to know how many.

The source of truth for these is in `client/chatbot.go` in case the documentation is out of date.

# RiveScript Object Macros

The following object macros are available to your RiveScript bot.

The source of truth for these is in the `client/rivescript_macros.go` source file, in case this documentation gets out of date.

You can invoke these by using the `<call>` tag in your RiveScript responses -- see the examples.

## Reload

This command can reload the chatbot's RiveScript sources from disk, making it easy to iterate on your robot without rebooting the whole program.

Example:

```rivescript
+ /reload
* <get isAdmin> != true => You do not have permission for that command.
- <call>reload</call>
```

It returns a message like "The RiveScript brain has been reloaded!"

## React

You can send an emoji reaction to a message ID. The current message ID is available in the `<get messageID>` tag of a RiveScript reply.

Example:

```rivescript
// Auto react to hello messages with a wave
+ [*] (hello|hi|howdy|yo|sup) [*]
- <call>react <get messageID> 👋</call>
^ <noreply>
```

Note: the `react` command returns no text (except on error). Couple it with a `<noreply>` if you want to ensure the bot does not send a reply to the message in chat, but simply applies the reaction emoji.

The reaction is delayed about 2.5 seconds.
