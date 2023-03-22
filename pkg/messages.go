package barertc

/*
Message is the basic carrier of WebSocket chat protocol actions.

Every message (client or server) has an Action and the rest of the
fields may vary depending on the action. Many messages target (or carry)
a Username, chat Channel and carry an arbitrary Message.
*/
type Message struct {
	Action   string `json:"action,omitempty"`
	Channel  string `json:"channel,omitempty"`
	Username string `json:"username,omitempty"`
	Message  string `json:"message,omitempty"`

	// JWT token for `login` actions.
	JWTToken string `json:"jwt,omitempty"`

	// WhoList for `who` actions
	WhoList []WhoList `json:"whoList,omitempty"`

	// Sent on `me` actions along with Username
	VideoActive bool `json:"videoActive,omitempty"` // user tells us their cam status
	NSFW        bool `json:"nsfw,omitempty"`        // user tags their video NSFW

	// Sent on `open` actions along with the (other) Username.
	OpenSecret string `json:"openSecret,omitempty"`

	// Send on `file` actions, passing e.g. image data.
	Bytes []byte `json:"bytes,omitempty"`

	// WebRTC negotiation messages: proxy their signaling messages
	// between the two users to negotiate peer connection.
	Candidate   string `json:"candidate,omitempty"`   // candidate
	Description string `json:"description,omitempty"` // sdp
}

const (
	// Actions sent by the client side only
	ActionLogin = "login" // post the username to backend

	// Actions sent by server or client
	ActionMessage = "message" // post a message to the room
	ActionMe      = "me"      // user self-info sent by FE or BE
	ActionOpen    = "open"    // user wants to view a webcam (open WebRTC)
	ActionRing    = "ring"    // receiver of a WebRTC open request
	ActionWatch   = "watch"   // user has received video and is watching you
	ActionUnwatch = "unwatch" // user has closed your video
	ActionFile    = "file"    // image sharing in chat

	// Actions sent by server only
	ActionPing     = "ping"
	ActionWhoList  = "who"        // server pushes the Who List
	ActionPresence = "presence"   // a user joined or left the room
	ActionError    = "error"      // ChatServer errors
	ActionKick     = "disconnect" // client should disconnect (e.g. have been kicked).

	// WebRTC signaling messages.
	ActionCandidate = "candidate"
	ActionSDP       = "sdp"
)

// WhoList is a member entry in the chat room.
type WhoList struct {
	Username    string `json:"username"`
	VideoActive bool   `json:"videoActive,omitempty"`
	NSFW        bool   `json:"nsfw,omitempty"`

	// JWT auth extra settings.
	Operator   bool   `json:"op"`
	Avatar     string `json:"avatar,omitempty"`
	ProfileURL string `json:"profileURL,omitempty"`
}
