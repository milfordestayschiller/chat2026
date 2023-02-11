# BareRTC

BareRTC is a simple WebRTC-based chat room application. It is especially designed to be plugged into any existing website, with or without a pre-existing base of users.

**Live demo:** [BareRTC Demo Chat](https://chat.kirsle.net/)

![Screenshot of BareRTC](screenshot.png)

It is very much in the style of the old-school Flash based webcam chat rooms of the early 2000's: a multi-user chat room with DMs and _some_ users may broadcast video and others may watch multiple video feeds in an asynchronous manner. I thought that this should be such an obvious free and open source app that should exist, but it did not and so I had to write it myself.

# Features

* Specify multiple Public Channels that all users have access to.
* Users can open direct message (one-on-one) conversations with each other.
* No long-term server side state: messages are pushed out as they come in.
* Users may broadcast their webcam which shows a camera icon by their name in the Who List. Users may click on those icons to open multiple camera feeds of other users they are interested in.
* Mobile friendly: works best on iPads and above but adapts to smaller screens well.
* WebRTC means peer-to-peer video streaming so cheap on hosting costs!
* Simple integration with your existing userbase via signed JWT tokens.
* User configurable sound effects to be notified of DMs or users entering/exiting the room.

Some important features still lacking:

* Operator controls (kick/ban users)

# Configuration

On first run it will create the default settings.toml file for you which you may then customize to your liking:

```toml
Title = "BareRTC"
Branding = "BareRTC"
WebsiteURL = "https://www.example.com"
UseXForwardedFor = true
CORSHosts = ["https://www.example.com"]
PermitNSFW = true

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
    "op": true,  // User will have admin/operator permissions.
    "img": "/static/photos/username.jpg", // user picture URL
    "url": "/u/username",                 // user profile URL

    // Standard JWT claims that we support:
    "iss": "my own app", // Issuer name
    "exp": 1675645084,   // Expires at (time): 5 minutes out is plenty!
    "nbf": 1675644784,   // Not Before (time)
    "iat": 1675644784,   // Issued At (time)
}
```

**Notice:** your picture and profile URL may be relative URIs beginning with a forward slash as seen above; BareRTC will append them to the end of your WebsiteURL and you can save space on your JWT token size this way. Full URLs beginning with `https?://` will also be accepted and used as-is.

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

* `GET /api/statistics`

Returns basic info about the count and usernames of connected chatters:

```json
{
    "UserCount": 1,
    "Usernames": ["admin"]
}
```

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
