package barertc

type Message struct {
	Action   string `json:"action,omitempty"`
	Username string `json:"username"`
	Message  string `json:"message"`
}

const (
	ActionLogin   = "login"   // post the username to backend
	ActionMessage = "message" // post a message to the room
)
