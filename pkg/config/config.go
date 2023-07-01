package config

import (
	"bytes"
	"encoding/json"
	"html/template"
	"os"

	"git.kirsle.net/apps/barertc/pkg/log"
	"github.com/BurntSushi/toml"
)

// Version of the config format - when new fields are added, it will attempt
// to write the settings.toml to disk so new defaults populate.
var currentVersion = 4

// Config for your BareRTC app.
type Config struct {
	Version int // will re-save your settings.toml on migrations

	JWT struct {
		Enabled        bool
		Strict         bool
		SecretKey      string
		LandingPageURL string
	}

	Title      string
	Branding   string
	WebsiteURL string

	CORSHosts  []string
	PermitNSFW bool

	UseXForwardedFor bool

	WebSocketReadLimit int64
	MaxImageWidth      int
	PreviewImageWidth  int

	TURN TurnConfig

	PublicChannels []Channel
}

type TurnConfig struct {
	URLs       []string
	Username   string
	Credential string
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
		Branding:   "BareRTC",
		WebsiteURL: "https://www.example.com",
		CORSHosts: []string{
			"https://www.example.com",
		},
		WebSocketReadLimit: 1024 * 1024 * 40, // 40 MB.
		MaxImageWidth:      1280,
		PreviewImageWidth:  360,
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
		TURN: TurnConfig{
			URLs: []string{
				"stun:stun.l.google.com:19302",
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

	// Have we added new config fields? Save the settings.toml.
	if Current.Version != currentVersion {
		log.Warn("New options are available for your settings.toml file. Your settings will be re-saved now.")
		Current.Version = currentVersion
		if err := WriteSettings(); err != nil {
			log.Error("Couldn't write your settings.toml file: %s", err)
		}
	}

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
