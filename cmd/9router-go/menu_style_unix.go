//go:build !windows

package main

import (
	"os"
	"strings"
)

// enableColor reports whether f is a terminal. A character device is the same
// test the stdin probe uses; nothing more is needed off Windows, where ANSI is
// the default and TERM/NO_COLOR already had their say.
func enableColor(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// consoleGlyphs checks the locale for UTF-8 before trusting box drawing.
func consoleGlyphs() glyphs {
	locale := strings.ToUpper(os.Getenv("LC_ALL") + os.Getenv("LC_CTYPE") + os.Getenv("LANG"))
	if strings.Contains(locale, "UTF-8") || strings.Contains(locale, "UTF8") {
		return boxGlyphs
	}
	return asciiGlyphs
}
