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
