package config

import (
	"bytes"
	"encoding/json"
	"html/template"
	"os"

	"git.kirsle.net/apps/barertc/pkg/log"
	"github.com/BurntSushi/toml"
)

// Config for your BareRTC app.
type Config struct {
	JWT struct {
		Enabled   bool
		Strict    bool
		SecretKey string
	}

	Title      string
	Branding   string
	WebsiteURL string

	PublicChannels []Channel
}

// GetChannels returns a JavaScript safe array of the default PublicChannels.
func (c Config) GetChannels() template.JS {
	data, _ := json.Marshal(c.PublicChannels)
	return template.JS(data)
}

// Channel config for a default public room.
type Channel struct {
	ID   string // Like "lobby"
	Name string // Like "Main Chat Room"
	Icon string `toml:",omitempty"` // CSS class names for room icon (optional)

	// ChatServer messages to send to the user immediately upon connecting.
	WelcomeMessages []string
}

// Current loaded configuration.
var Current = DefaultConfig()

// DefaultConfig returns sensible defaults and will write the initial
// settings.toml file to disk.
func DefaultConfig() Config {
	var c = Config{
		Title:      "BareRTC",
		WebsiteURL: "https://www.example.com",
		PublicChannels: []Channel{
			{
				ID:   "lobby",
				Name: "Lobby",
				Icon: "fa fa-gavel",
				WelcomeMessages: []string{
					"Welcome to the chat server!",
					"Please follow the basic rules:\n\n1. Have fun\n2. Be kind",
				},
			},
			{
				ID:   "offtopic",
				Name: "Off Topic",
				WelcomeMessages: []string{
					"Welcome to the Off Topic channel!",
				},
			},
		},
	}
	c.JWT.Strict = true
	return c
}

// LoadSettings reads a settings.toml from disk if available.
func LoadSettings() error {
	data, err := os.ReadFile("./settings.toml")
	if err != nil {
		// Settings file didn't exist, create the default one.
		if os.IsNotExist(err) {
			WriteSettings()
			return nil
		}

		return err
	}

	_, err = toml.Decode(string(data), &Current)
	return err
}

// WriteSettings will commit the settings.toml to disk.
func WriteSettings() error {
	log.Error("Note: initial settings.toml was written to disk.")
	var buf = new(bytes.Buffer)
	err := toml.NewEncoder(buf).Encode(Current)
	if err != nil {
		return err
	}
	return os.WriteFile("./settings.toml", buf.Bytes(), 0644)
}
