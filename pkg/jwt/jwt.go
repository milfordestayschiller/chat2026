package jwt

import (
	"encoding/json"
	"errors"
	"html/template"

	"git.kirsle.net/apps/barertc/pkg/config"
	"github.com/golang-jwt/jwt/v4"
)

// Custom JWT Claims.
type Claims struct {
	// Custom claims.
	IsAdmin    bool   `json:"op"`
	Avatar     string `json:"img"`
	ProfileURL string `json:"url"`
	Nick       string `json:"nick"`

	// Standard claims. Notes:
	// subject = username
	jwt.RegisteredClaims
}

// ToJSON serializes the claims to JavaScript.
func (c Claims) ToJSON() template.JS {
	data, _ := json.Marshal(c)
	return template.JS(data)
}

// ParseAndValidate returns the Claims, a boolean authOK, and any errors.
func ParseAndValidate(tokenStr string) (*Claims, bool, error) {
	// Handle a JWT authentication token.
	var (
		claims = &Claims{}
		authOK bool
	)
	if tokenStr != "" {
		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(config.Current.JWT.SecretKey), nil
		})
		if err != nil {
			return nil, false, err
		}

		if parsed, ok := token.Claims.(*Claims); ok && token.Valid {
			claims = parsed
			authOK = true
		} else {
			return nil, false, errors.New("claims did not parse OK")
		}
	}

	return claims, authOK, nil
}
