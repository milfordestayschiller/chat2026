package barertc

type Message struct {
	Action   string `json:"action,omitempty"`
	Username string `json:"username"`
	Message  string `json:"message"`

	// WhoList for `who` actions
	WhoList []WhoList `json:"whoList"`

	// Sent on `me` actions along with Username
	VideoActive bool `json:"videoActive"` // user tells us their cam status
}

const (
	// Actions sent by the client side only
	ActionLogin = "login" // post the username to backend

	// Actions sent by server or client
	ActionMessage = "message" // post a message to the room
	ActionMe      = "me"      // user self-info sent by FE or BE

	// Actions sent by server only
	ActionWhoList  = "who"      // server pushes the Who List
	ActionPresence = "presence" // a user joined or left the room
)

// WhoList is a member entry in the chat room.
type WhoList struct {
	Username    string `json:"username"`
	VideoActive bool   `json:"videoActive"`
}
