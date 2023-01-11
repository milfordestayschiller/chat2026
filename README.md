# BareRTC

BareRTC is a simple WebRTC-based chat room application. It is especially
designed to be plugged into any existing website, with or without a pre-existing
base of users.

# Features

Planned features:

* One common group chat area where all participants can broadcast text messages.
* Direct (one-on-one) text conversations between any two users.
* Simple integration with your pre-existing userbase via signed JWT tokens.

# Configuration

TBD

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

# License

GPLv3.
