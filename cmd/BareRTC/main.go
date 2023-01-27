package main

import (
	"flag"
	"math/rand"
	"time"

	barertc "git.kirsle.net/apps/barertc/pkg"
	"git.kirsle.net/apps/barertc/pkg/log"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	// Command line flags.
	var (
		debug   bool
		address string
	)
	flag.BoolVar(&debug, "debug", false, "Enable debug-level logging in the app.")
	flag.StringVar(&address, "address", ":9000", "Address to listen on, like localhost:5000 or :8080")
	flag.Parse()

	if debug {
		log.SetDebug(true)
	}

	app := barertc.NewServer()
	app.Setup()
	panic(app.ListenAndServe(address))
}
