# BareRTC

BareRTC is a simple WebRTC-based chat room application. It is especially designed to be plugged into any existing website, with or without a pre-existing base of users.

It is very much in the style of the old-school Flash based webcam chat rooms of the early 2000's: a multi-user chat room with DMs and _some_ users may broadcast video and others may watch multiple video feeds in an asynchronous manner. I thought that this should be such an obvious free and open source app that should exist, but it did not and so I had to write it myself.

This is still a **work in progress** and see the features it still needs, below.

# Features

* Specify multiple Public Channels that all users have access to.
* Users can open direct message (one-on-one) conversations with each other.
* No long-term server side state: messages are pushed out as they come in.
* Users may broadcast their webcam which shows a camera icon by their name in the Who List. Users may click on those icons to open multiple camera feeds of other users they are interested in.
* Mobile friendly: works best on iPads and above but adapts to smaller screens well.
* WebRTC means peer-to-peer video streaming so cheap on hosting costs!
* Simple integration with your existing userbase via signed JWT tokens.

Some important features it still needs:

* JWT authentication, and admin user permissions (kick/ban/etc.)
    * Support for profile URLs, custom avatar image URLs, custom profile fields to show in-app
* Lots of UI cleanup.

# Configuration

Work in progress. On first run it will create the settings.toml file for you:

```toml
[JWT]
  Enabled = false
  SecretKey = ""

[[PublicChannels]]
  ID = "lobby"
  Name = "Lobby"
  Icon = "fa fa-gavel"

[[PublicChannels]]
  ID = "offtopic"
  Name = "Off Topic"
```

# Authentication

BareRTC supports custom (user-defined) authentication with your app in the form
of JSON Web Tokens (JWTs). Configure a shared Secret Key in the ChatRTC settings
and have your app create a signed JWT with the same key and the following custom
claims:

```json
{
    "username": "Soandso",
    "icon": "https://path/to/square/icon.png",
    "admin": false,
}
```

This feature is not hooked up yet. JWT authenticated users sent by your app is the primary supported userbase and will bring many features such as:

* Admin user permissions: you tell us who the admin is and they can moderate the chat room.
* User profile URLs that can be opened from the Who List.
* Custom avatar image URLs for your users.
* Extra profile fields/icons that you can customize the display with.

## Running Without Authentication

The default app doesn't need any authentication at all: users are asked to pick their own username when joining the chat. The server may re-assign them a new name if they enter one that's already taken.

It is not recommended to run in this mode as admin controls to moderate the server are disabled.

### Known Bugs Running Without Authentication

This app is not designed to run without JWT authentication for users enabled. In the app's default state, users can pick their own username when they connect and the server will adjust their name to resolve duplicates. Direct message threads are based on the username so if a user logs off, somebody else could log in with the same username and "resume" direct message threads that others were involved in.

Note that they would not get past history of those DMs as this server only pushes _new_ messages to users after they connect.

# License

GPLv3.
