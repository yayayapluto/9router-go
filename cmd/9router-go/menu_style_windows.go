//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableColor turns on VT processing for the console behind f. Windows only
// learned ANSI through that mode, and it stays off until a process asks, so a
// SetConsoleMode failure is the honest "no colour here" answer.
func enableColor(f *os.File) bool {
	var mode uint32
	h := windows.Handle(f.Fd())
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}
	return windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}

// consoleGlyphs picks the frame set from the console output code page: box
// drawing only survives when the console is already UTF-8 (code page 65001),
// which is the default in Windows Terminal and modern conhost but not in a
// legacy shell.
func consoleGlyphs() glyphs {
	cp, err := windows.GetConsoleOutputCP()
	if err == nil && cp == 65001 {
		return boxGlyphs
	}
	return asciiGlyphs
}
