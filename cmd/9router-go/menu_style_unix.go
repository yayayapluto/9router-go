//go:build !windows

package main

import (
	"os"
	"strings"
)

// prepareConsole reports whether f is a terminal. A character device is the
// same test the stdin probe uses; nothing more is needed off Windows, where
// ANSI is the default and no console state has to be restored.
func prepareConsole(f *os.File) (bool, func()) {
	fi, err := f.Stat()
	if err != nil {
		return false, nil
	}
	return fi.Mode()&os.ModeCharDevice != 0, nil
}

// consoleGlyphs checks the locale for UTF-8 before trusting box drawing.
func consoleGlyphs() glyphs {
	locale := strings.ToUpper(os.Getenv("LC_ALL") + os.Getenv("LC_CTYPE") + os.Getenv("LANG"))
	if strings.Contains(locale, "UTF-8") || strings.Contains(locale, "UTF8") {
		return boxGlyphs
	}
	return asciiGlyphs
}
