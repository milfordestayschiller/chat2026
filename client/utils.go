package client

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reHTML = regexp.MustCompile(`<(.|\n)+?>`)
	reIMG  = regexp.MustCompile(`<img .+?>`)
)

// StripHTML removes HTML content from a message.
func StripHTML(s string) string {
	s = reIMG.ReplaceAllString(s, "inline embedded image")
	return strings.TrimSpace(reHTML.ReplaceAllString(s, ""))
}

// AtMentioned checks if somebody has "at mentioned" your username (having your
// name at the beginning or end of their message). Returns whether the at mention
// was detected, along with the modified message without the at mention name on the
// end of it.
func AtMentioned(c *Client, message string) (bool, string) {
	// Patterns to look for.
	var (
		reAtMention = regexp.MustCompile(
			fmt.Sprintf(`^@?%s|@?%s$`, c.Username(), c.Username()),
		)
	)
	m := reAtMention.FindStringSubmatch(message)
	if m != nil {
		// Found a match! Sub off the at mentioned part and return.
		message = strings.TrimSpace(reAtMention.ReplaceAllString(message, ""))
		if len(message) > 0 {
			return true, message
		}
	}
	return false, message
}
