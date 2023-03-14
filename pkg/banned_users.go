package barertc

import "time"

/* Functions to handle banned users */

/*
BanList holds (in memory) knowledge of currently banned users.

All bans are reset if the chat server is rebooted. Otherwise each ban
comes with a duration - default is 24 hours by the operator can specify
a duration with a ban. If the server is not rebooted, bans will be lifted
after they expire.

Bans are against usernames and will also block a JWT token from
authenticating if they are currently banned.
*/
type BanList struct {
	Active []Ban
}

// Ban is an entry on the ban list.
type Ban struct {
	Username  string
	ExpiresAt time.Time
}

//
