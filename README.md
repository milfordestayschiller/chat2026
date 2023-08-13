# BareRTC

BareRTC is a simple WebRTC-based chat room application. It is especially designed to be plugged into any existing website, with or without a pre-existing base of users.

**Live demo:** [BareRTC Demo Chat](https://chat.kirsle.net/)

![Screenshot of BareRTC](screenshot.png)

It is very much in the style of the old-school Flash based webcam chat rooms of the early 2000's: a multi-user chat room with DMs and _some_ users may broadcast video and others may watch multiple video feeds in an asynchronous manner. I thought that this should be such an obvious free and open source app that should exist, but it did not and so I had to write it myself.

* [Features](#features)
* [Configuration](#configuration)
* [Authentication](#authentication)
    * [JWT Strict Mode](#jwt-strict-mode)
    * [Running Without Authentication](#running-without-authentication)
    * [Known Bugs Running Without Authentication](#known-bugs-running-without-authentication)
* [Moderator Commands](#moderator-commands)
* [JSON APIs](#json-apis)
* [Tour of the Codebase](#tour-of-the-codebase)
* [Deploying This App](#deploying-this-app)
* [License](#license)

# Features

* Specify multiple Public Channels that all users have access to.
* Users can open direct message (one-on-one) conversations with each other.
* No long-term server side state: messages are pushed out as they come in.
* Users may share pictures and GIFs from their computer, which are pushed out as `data:` URLs (images scaled and metadata stripped by server) directly to connected chatters with no storage required.
* Users may broadcast their webcam which shows a camera icon by their name in the Who List. Users may click on those icons to open multiple camera feeds of other users they are interested in.
    * Mutual webcam options: users may opt that anyone who views their cam must also be sharing their own camera first.
    * Users may mark their own cameras as explicit/NSFW which marks the icon in red so other users can get a warning before clicking in (if NSFW is enabled in the settings.toml)
    * Users may boot people off their camera, and to the booted person it appears the same as if the broadcaster had turned their camera off completely - the chat server lies about the camera status so the booted user can't easily tell they'd been booted.
* Mobile friendly: works best on iPads and above but adapts to smaller screens well.
* WebRTC means peer-to-peer video streaming so cheap on hosting costs!
* Simple integration with your existing userbase via signed JWT tokens.
* User configurable sound effects to be notified of DMs or users entering/exiting the room.
* Operator commands
    * [x] /kick users
    * [x] /ban users (and /unban, /bans to list)
    * [x] /nsfw to tag a user's camera as explicit
    * [x] /shutdown to gracefully reboot the server
    * [x] /kickall to kick EVERYBODY off the server (e.g., for mandatory front-end reload for new features)
    * [x] /op and /deop users (give temporary mod control)
    * [x] /help to get in-chat help for moderator commands

# Configuration

On first run it will create the default settings.toml file for you which you may then customize to your liking:

```toml
Version = 2
Title = "BareRTC"
Branding = "BareRTC"
WebsiteURL = "https://www.example.com"
CORSHosts = ["https://www.example.com"]
PermitNSFW = true
UseXForwardedFor = true
WebSocketReadLimit = 41943040
MaxImageWidth = 1280
PreviewImageWidth = 360

[JWT]
  Enabled = false
  Strict = true
  SecretKey = ""

[[PublicChannels]]
  ID = "lobby"
  Name = "Lobby"
  Icon = "fa fa-gavel"
  WelcomeMessages = ["Welcome to the chat server!", "Please follow the basic rules:\n\n1. Have fun\n2. Be kind"]

[[PublicChannels]]
  ID = "offtopic"
  Name = "Off Topic"
  WelcomeMessages = ["Welcome to the Off Topic channel!"]
```

A description of the config directives includes:

* Website settings:
    * **Title** goes in the title bar of the chat page.
    * **Branding** is the title shown in the corner of the page. HTML is permitted here! You may write an `<img>` tag to embed an image or use custom markup to color and prettify your logo.
    * **WebsiteURL** is the base URL of your actual website which is used in a couple of places:
        * The About page will link to your website.
        * If using [JWT authentication](#authentication), avatar and profile URLs may be relative (beginning with a "/") and will append to your website URL to safe space on the JWT token size!
    * **UseXForwardedFor**: set it to true and (for logging) the user's remote IP will use the X-Real-IP header or the first address in X-Forwarded-For. Set this if you run the app behind a proxy like nginx if you want IPs not to be all localhost.
    * **CORSHosts**: your website's domain names that will be allowed to access [JSON APIs](#JSON APIs), like `/api/statistics`.
    * **PermitNSFW**: for user webcam streams, expressly permit "NSFW" content if the user opts in to mark their feed as such. Setting this will enable pop-up modals regarding NSFW video and give broadcasters an opt-in button, which will warn other users before they click in to watch.
    * **WebSocketReadLimit**: sets a size limit for WebSocket messages - it essentially also caps the max upload size for shared images (add a buffer as images will be base64 encoded on upload).
    * **MaxImageWidth**: for pictures shared in chat the server will resize them down to no larger than this width for the full size view.
    * **PreviewImageWidth**: to not flood the chat, the image in chat is this wide and users can click it to see the MaxImageWidth in a lightbox modal.
* **JWT**: settings for JWT [Authentication](#authentication).
    * Enabled (bool): activate the JWT token authentication feature.
    * Strict (bool): if true, **only** valid signed JWT tokens may log in. If false, users with no/invalid token can enter their own username without authentication.
    * SecretKey (string): the JWT signing secret shared with your back-end app.
* **PublicChannels**: list the public channels and their configuration. The default channel will be the first one listed.
    * ID (string): an arbitrary 'username' for the chat channel, like "lobby".
    * Name (string): the user friendly name for the channel, like "Off Topic"
    * Icon (string, optional): CSS class names for FontAwesome icon for the channel, like "fa fa-message"
    * WelcomeMessages ([]string, optional): messages that are delivered by ChatServer to the user when they connect to the server. Useful to give an introduction to each channel, list its rules, etc.

# Authentication

BareRTC supports custom (user-defined) authentication with your app in the form of JSON Web Tokens (JWTs). JWTs will allow your existing app to handle authentication for users by signing a token that vouches for them, and the BareRTC app will trust your signed token.

The workflow is as follows:

1. Your existing app already has the user logged-in and you trust who they are. To get them into the chat room, your server signs a JWT token using a secret key that both it and BareRTC knows.
2. Your server redirects the user to your BareRTC website sending the JWT token as a `jwt` parameter, either in the query string (GET) or POST request.
    * e.g. you send them to `https://chat.example.com/?jwt=TOKEN`
    * If the JWT token is too long to fit in a query string, you may create a `<form>` with `method="POST"` that posts the `jwt` as a form field.
3. The BareRTC server will parse and validate the token using the shared Secret Key that only it and your back-end website knows.

There are JWT libraries available for most programming languages.

Configure a shared secret key (random text string) in both the BareRTC settings and in your app, and your app will sign a JWT including claims that look like the following (using signing method HS264):

```javascript
// JSON Web Token "claims" expected by BareRTC
{
    // Custom claims
    "sub": "username", // Username for chat (standard JWT claim)
    "op": true,        // User will have admin/operator permissions.
    "nick": "Display name",               // Friendly name
    "img": "/static/photos/username.jpg", // user picture URL
    "url": "/u/username",                 // user profile URL
    "gender": "m",                        // gender (m, f, o)
    "emoji": "🤖",                        // emoji icon

    // Standard JWT claims that we support:
    "iss": "my own app", // Issuer name
    "exp": 1675645084,   // Expires at (time): 5 minutes out is plenty!
    "nbf": 1675644784,   // Not Before (time)
    "iat": 1675644784,   // Issued At (time)
}
```

**Notice:** your picture and profile URL may be relative URIs beginning with a forward slash as seen above; BareRTC will append them to the end of your WebsiteURL and you can save space on your JWT token size this way. Full URLs beginning with `https?://` will also be accepted and used as-is.

See [Custom JWT Claims](#custom-jwt-claims) for more information on the
custom claims and how they work.

An example how to sign your JWT tokens in Go (using [golang-jwt](https://github.com/golang-jwt/jwt)):

```golang
import "github.com/golang-jwt/jwt/v4"

// JWT signing key - keep it a secret on your back-end shared between
// your app and BareRTC, do not use it in front-end javascript code or
// where a user can find it.
const SECRET = "change me"

// Your custom JWT claims.
type CustomClaims struct {
    // Custom claims used by BareRTC.
    Avatar     string `json:"img"`  // URI to user profile picture
    ProfileURL string `json:"url"`  // URI to user's profile page
    IsAdmin    bool   `json:"op"`   // give operator permission

    // Standard JWT claims
    jwt.RegisteredClaims
}

// Assuming your internal User struct looks anything at all like:
type User struct {
    Username       string
    IsAdmin        bool
    ProfilePicture string  // like "/static/photos/username.jpg"
}

// Create a JWT token for this user.
func SignForUser(user User) string {
    claims := CustomClaims{
        // Custom claims
        ProfileURL: "/users/" + user.Username,
        Avatar:     user.ProfilePicture,
        IsAdmin:    user.IsAdmin,

        // Standard claims
        Subject:   user.Username, // their chat username!
        ExpiresAt: time.Now().Add(5 * time.Minute),
        IssuedAt:  time.Now(),
        NotBefore: time.Now(),
        Issuer:    "my own app",
        ID:        user.ID,
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenStr, err := token.SignedString(SECRET)
    if err != nil {
        panic(err)
    }
    return tokenstr
}
```

## Custom JWT Claims

With JWT authentication your website can pass a lot of fun variables to decorate your Who Is Online list for your users.

Here is in-depth documentation on what custom claims are supported by BareRTC and what effects they have in chat:

* **Subject** (`sub`): this is a standard JWT claim and BareRTC will collect your username from it. The username is shown in the Who's Online list and below the user's nickname on their chat messages (in "@username" format). Do not prefix your subject with the @ symbol yourself.
* **Operator** (`op`): this boolean will mark your user to have operator (admin) status in chat. In the Who List they will have a gavel icon after their username, and they will be allowed to run operator commands (e.g. to kick other users from chat).
* **Nickname** (`nick`): you may send your users in with a custom Display Name that will appear on their chat messages. If they don't have a nickname, their username will be used in its place.
* **Image** (`img`): a profile picture or avatar for your users. It should be a square image and will appear in the Who List and alongside their chat messages. If they don't have an image, a default blue silhouette avatar is used. The image URL may be a relative URI beginning with `/` and it will be appended onto your configured WebsiteURL.
* **Profile URL** (`url`): a link to a user's profile page. If provided, clicking on their picture in chat or the Who List will open this URL in a new tab. They will also get a profile button added next to their name on the Who List. Relative URLs beginning with `/` are supported, and will be appended to your WebsiteURL automatically.
* **Gender** (`gender`): a single-character gender code for your user. If they also have a Profile URL, their profile button on the Who List can be color-coded by gender. Supported values include:
    * **m** (male) to set their profile button blue.
    * **f** (female) to set their profile button pink.
    * Other value (canonically, **o**) to set their profile button purple.
    * Missing/no value won't set a color and it will be the default text color.
* **Emoji** (`emoji`): you may associate users with an emoji character that will appear on the Who List next to their name. Some example ideas and use cases include:
    * Country flag emojis, to indicate where your users are connecting from.
    * Robot emojis, to indicate bot users.
    * Any emoji you want! Mark your special guests or VIP users, etc.

## JWT Strict Mode

You can enable JWT authentication in a mixed mode: users presenting a valid token will get a profile picture and operator status (if applicable) and users who don't have a JWT token are asked to pick their own username and don't get any special flair.

In strict mode (default/recommended), only a valid JWT token can sign a user into the chat room. Set `[JWT]/Strict=false` in your settings.toml to disable strict JWT verification and allow "guest users" to log in. Note that this can have the same caveats as [running without authentication](#running-without-authentication) and is not a recommended use case.

## Running Without Authentication

The default app doesn't need any authentication at all: users are asked to pick their own username when joining the chat. The server may re-assign them a new name if they enter one that's already taken.

It is not recommended to run in this mode as admin controls to moderate the server are disabled.

### Known Bugs Running Without Authentication

This app is not designed to run without JWT authentication for users enabled. In the app's default state, users can pick their own username when they connect and the server will adjust their name to resolve duplicates. Direct message threads are based on the username so if a user logs off, somebody else could log in with the same username and "resume" direct message threads that others were involved in.

Note that they would not get past history of those DMs as this server only pushes _new_ messages to users after they connect.

# Moderator Commands

If you authenticate an Op user via JWT they can enter IRC-style chat commands to moderate the server. Current commands include:

* `/kick <username>` to disconnect a user's chat session.
* `/nsfw <username>` to tag a user's video feed as NSFW (if your settings.toml has PermitNSFW enabled).

# JSON APIs

For better integration with your website, the chat server exposes some data via JSON APIs ready for cross-origin ajax requests. In your settings.toml set the `CORSHosts` to your list of website domains, such as "https://www.example.com", "http://localhost:8080" or so on.

Current API endpoints include:

## GET /api/statistics

Returns basic info about the count and usernames of connected chatters:

```json
{
    "UserCount": 1,
    "Usernames": ["admin"]
}
```

## POST /api/blocklist

Your server may pre-cache the user's blocklist for them **before** they
enter the chat room. Your site will use the `AdminAPIKey` parameter that
matches the setting in BareRTC's settings.toml (by default, a random UUID
is generated the first time).

The request payload coming from your site will be an application/json
post body like:

```json
{
    APIKey: "from your settings.toml",
    Username: "soandso",
    Blocklist: [ "usernames", "that", "they", "block" ],
}
```

The server holds onto these in memory and when that user enters the chat
room (**JWT authentication only**) the front-end page will embed their
cached blocklist. When they connect to the WebSocket server, they send a
`blocklist` message to push their blocklist to the server -- it is
basically a bulk `mute` action that mutes all these users pre-emptively:
the user will not see their chat messages and the muted users can not see
the user's webcam when they broadcast later, the same as a regular `mute`
action.

The JSON response to this endpoint may look like:

```json
{
    "OK": true,
    "Error": "if error, or this key is omitted if OK"
}
```

# Webhook URLs

BareRTC supports setting up webhook URLs so the chat server can call out to _your_ website in response to certain events, such as allowing users to send you reports about messages they receive on chat.

Webhooks are configured in your settings.toml file and look like so:

```toml
[[WebhookURLs]]
  Name = "report"
  Enabled = true
  URL = "http://localhost:8080/v1/barertc/report"
```

All Webhooks will be called as **POST** requests and will contain a JSON payload that will always have the following two keys:

* `Action` will be the name of the webhook (e.g. "report")
* `APIKey` will be your AdminAPIKey as configure in the settings.toml (shared secret so your web app can authenticate BareRTC's webhooks).

The JSON payload may also contain a relevant object per the Action -- see the specific examples below.

## Report Webhook

Enabling this webhook will cause BareRTC to display a red "Report" flag button underneath user messages on chat so that they can report problematic messages to your website.

The webhook name for your settings.toml is "report"

Example JSON payload posted to the webhook:

```javascript
{
    "Action": "report",
    "APIKey": "shared secret from settings.toml#AdminAPIKey",
    "Report": {
        "FromUsername": "sender",
        "AboutUsername": "user being reported on",
        "Channel": "lobby",  // or "@username" for DM threads
        "Timestamp": "(stringified timestamp of chat message)",
        "Reason": "It's spam",
        "Comment": "custom user note about the report",
        "Message": "the actual message that was being reported on",
    }
}
```

BareRTC expects your webhook URL to return a 200 OK status code or it will surface an error in chat to the reporter.

# Tour of the Codebase

This app uses WebSockets and WebRTC at the very simplest levels, without using a framework like `Socket.io`. Here is a tour of the codebase with the more interesting modules listed first.

## Backend files

* `cmd/BareRTC/main.go`: the entry point for the Go back-end application (parses command-line flags and starts the web server)
* `pkg/` contains the Go source code for the server side (the application).
    * `config/` handles the settings.toml config file for the app.
    * `jwt/` handles the JWT authentication logic
    * `log/` is an internal logger library - not very interesting
    * `util/` houses some miscellaneous utility functions, such as generating random strings or getting the user's IP address (w/ X-Forwarded-For support, etc.)
* `pkg/server.go` sets up the Go HTTP server and all the endpoint routes (e.g.: the /about page, static files, the WebSockets endpoint)
* `pkg/websocket.go` handles the WebSockets endpoint which drives 99% of the chat app (all the login, text chat, who list portions - not webcams). Some related files to this include:
    * `pkg/messages.go` is where I define the JSON message schema for the WebSockets protocol. Client and server messages marshal into the Message struct.
    * `pkg/handlers.go` is where I write "high level" chat event handlers (OnLogin, OnMessage, etc.) - the WebSocket read loop parses their message and then nicely calls my event handler based on action.
    * `pkg/commands.go` handles commands like /kick from moderators.
* `pkg/api.go` handles the JSON API endpoints from the web server.
* `pkg/pages.go` handles the index (w/ jwt parsing) and about pages.

## Frontend files

The `web/` folder holds front-end files and templates used by the Go app.

* `web/templates` holds Go html/template sources that are rendered server-side.
    * `chat.html` is the template for the main chat page (index route, `/`).
    * `about.html` is the template for the `/about` page.
* `web/static` holds the static files (scripts, stylesheets, images) for the site.
    * `js/BareRTC.js` does the whole front-end Vue.js app for BareRTC. The portions of the code that handle the WebSockets and WebRTC features are marked off with comment banners so you can scroll until you find them.
    * `js/sounds.js` handles the sound effects for the chat room.
    * `css/chat.css` has custom CSS for the chat room UI (mainly a lot of CSS Grid stuff for the full-screen layout).

Other front-end files are all vendored libraries or frameworks used by this app:

* [Bulma](https://bulma.io) CSS framework
* [Font Awesome](https://fontawesome.com) for icons.

# Deploying This App

It is recommended to use a reverse proxy such as nginx in front of this app. You will need to configure nginx to forward WebSocket related headers:

```nginx
server {
    server_name chat.example.com;
    listen 443 ssl http2;
    listen [::]:443 ssl http2;

    ssl_certificate /etc/letsencrypt/live/chat.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/chat.example.com/privkey.pem;

    # Proxy pass to BareRTC.
    location / {
        proxy_pass http://127.0.0.1:9000;

        # WebSocket headers to forward along.
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
        proxy_set_header Host $host;
    }
}
```

You can run the BareRTC app itself using any service supervisor you like. I use [Supervisor](http://supervisord.org/introduction.html) and you can configure BareRTC like so:

```ini
# /etc/supervisor/conf.d/barertc.conf
[program:barertc]
command = /home/user/git/BareRTC/BareRTC -address 127.0.0.1:9000
directory = /home/user/git/BareRTC
user = user
```

Then `sudo supervisorctl reread && sudo supervisorctl add barertc` to start the app.

# License

GPLv3.
