# BareRTC Web API

BareRTC provides some web API endpoints over HTTP to support better integration with your website.

Authentication to the API endpoints is gated by the `AdminAPIKey` value in your settings.toml file.

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

## POST /api/authentication

This endpoint can provide JWT authentication token signing on behalf of your website. The [Chatbot](Chatbot.md) program calls this endpoint for authentication.

Post your desired JWT claims to the endpoint to customize your user and it will return a signed token for the WebSocket protocol.

```json
{
	"APIKey": "from settings.toml",
	"Claims": {
		"sub": "username",
		"nick": "Display Name",
		"op": false,
		"img": "/static/photos/avatar.png",
		"url": "/users/username",
		"emoji": "🤖",
		"gender": "m"
	}
}
```

The return schema looks like:

```json
{
	"OK": true,
	"Error": "error string, omitted if none",
	"JWT": "jwt token string"
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
    "APIKey": "from your settings.toml",
    "Username": "soandso",
    "Blocklist": [ "usernames", "that", "they", "block" ],
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
