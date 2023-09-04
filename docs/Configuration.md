# Configuration

On first run it will create the default settings.toml file for you which you may then customize to your liking:

```toml
Version = 7
Title = "BareRTC"
Branding = "BareRTC"
WebsiteURL = "http://www.example.com"
CORSHosts = ["http://www.example.com"]
AdminAPIKey = "e635e463-7987-4788-94f3-671a5c2a589f"
PermitNSFW = true
UseXForwardedFor = false
WebSocketReadLimit = 40971520
MaxImageWidth = 1280
PreviewImageWidth = 360

[JWT]
  Enabled = true
  Strict = true
  SecretKey = "05c45344-1c52-430b-beb9-c3f64ff7ed12"
  LandingPageURL = "https://www.example.com/enter-chat"

[TURN]
  URLs = ["stun:stun.l.google.com:19302"]
  Username = ""
  Credential = ""

[[PublicChannels]]
  ID = "lobby"
  Name = "Lobby"
  Icon = "fa fa-gavel"
  WelcomeMessages = ["Welcome to the chat server!", "Please follow the basic rules:\n\n1. Have fun\n2. Be kind"]

[[PublicChannels]]
  ID = "offtopic"
  Name = "Off Topic"
  WelcomeMessages = ["Welcome to the Off Topic channel!"]

[VIP]
  Name = "VIP"
  Branding = "<em>VIP Members</em>"
  Icon = "fa fa-circle"
  MutuallySecret = false
```

A description of the config directives includes:

## Website Settings

* **Version** number for the settings file itself. When new features are added, the Version will increment and your settings.toml will be written to disk with sensible defaults filled in for the new options.
* **Title** goes in the title bar of the chat page.
* **Branding** is the title shown in the corner of the page. HTML is permitted here! You may write an `<img>` tag to embed an image or use custom markup to color and prettify your logo.
* **WebsiteURL** is the base URL of your actual website which is used in a couple of places:
    * The About page will link to your website.
    * If using [JWT authentication](#authentication), avatar and profile URLs may be relative (beginning with a "/") and will append to your website URL to safe space on the JWT token size!
* **CORSHosts** names HTTP hosts for Cross Origin Resource Sharing. Usually, this will be the same as your WebsiteURL. This feature is used with the [Web API](API.md) if your front-end page needs to call e.g. the /api/statistics endpoint on BareRTC.
* **AdminAPIKey** is a shared secret authentication key for the admin API endpoints.
* **PermitNSFW**: for user webcam streams, expressly permit "NSFW" content if the user opts in to mark their feed as such. Setting this will enable pop-up modals regarding NSFW video and give broadcasters an opt-in button, which will warn other users before they click in to watch.
* **UseXForwardedFor**: set it to true and (for logging) the user's remote IP will use the X-Real-IP header or the first address in X-Forwarded-For. Set this if you run the app behind a proxy like nginx if you want IPs not to be all localhost.
* **WebSocketReadLimit**: sets a size limit for WebSocket messages - it essentially also caps the max upload size for shared images (add a buffer as images will be base64 encoded on upload).
* **MaxImageWidth**: for pictures shared in chat the server will resize them down to no larger than this width for the full size view.
* **PreviewImageWidth**: to not flood the chat, the image in chat is this wide and users can click it to see the MaxImageWidth in a lightbox modal.

## JWT Authentication

Settings for JWT [Authentication](#authentication):

* **Enabled** (bool): activate the JWT token authentication feature.
* **Strict** (bool): if true, **only** valid signed JWT tokens may log in. If false, users with no/invalid token can enter their own username without authentication.
* **SecretKey** (string): the JWT signing secret shared with your back-end app.

## Public Channels

Settings for the default public text channels of your room.

* **ID** (string): an arbitrary 'username' for the chat channel, like "lobby".
* **Name** (string): the user friendly name for the channel, like "Off Topic"
* **Icon** (string, optional): CSS class names for FontAwesome icon for the channel, like "fa fa-message"
* **WelcomeMessages** ([]string, optional): messages that are delivered by ChatServer to the user when they connect to the server. Useful to give an introduction to each channel, list its rules, etc.

## VIP Status

If using JWT authentication, your website can mark some users as VIPs when sending them over to the chat. The `[VIP]` section of settings.toml lets you customize the branding and behavior in BareRTC:

* **Name** (string): what you call your VIP users, used in mouse-over tooltips.
* **Branding** (string): HTML supported, this will appear in webcam sharing modals to "make my cam only visible to fellow VIP users"
* **Icon** (string): icon CSS name from Font Awesome.
* **MutuallySecret** (bool): if true, the VIP features are hidden and only visible to people who are, themselves, VIP. For example, the icon on the Who List will only show to VIP users but non-VIP will not see the icon.