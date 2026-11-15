package main

import "os"

// colorAllowed decides whether ANSI styling is safe to emit. The env vars are
// the operator's explicit override and win over the console probe; the probe
// itself is per-OS because Windows has to opt into VT processing first.
func colorAllowed(console bool) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return console
}
