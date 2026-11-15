//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// prepareConsole turns on what this menu needs and reports what it had to
// change: VT processing for colour, and a UTF-8 output code page so box drawing
// is not rendered as mojibake. Both are console-global, so the returned restore
// puts the console back the way the shell found it.
//
// A Windows console keeps VT off until a process asks, and its default OEM code
// page has no box-drawing glyphs.
func prepareConsole(f *os.File) (bool, func()) {
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return false, nil
	}

	prevCP, cpErr := windows.GetConsoleOutputCP()
	changedCP := cpErr == nil && prevCP != 65001 && windows.SetConsoleOutputCP(65001) == nil

	restore := func() {
		if changedCP {
			_ = windows.SetConsoleOutputCP(prevCP)
		}
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true, restore
	}
	if err := windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return false, restore
	}
	return true, restore
}

// consoleGlyphs picks the frame set from the console output code page: box
// drawing only survives on a UTF-8 console, which is UTF-8 by the time this is
// asked thanks to prepareConsole, or already was in Windows Terminal.
func consoleGlyphs() glyphs {
	cp, err := windows.GetConsoleOutputCP()
	if err == nil && cp == 65001 {
		return boxGlyphs
	}
	return asciiGlyphs
}
