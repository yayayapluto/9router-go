package main

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// testStyle is a monochrome ASCII terminal: the renderer's lowest common
// denominator, so layout assertions do not depend on the machine's console.
func testStyle() style { return style{pal: palette{}, g: asciiGlyphs} }

func TestParseChoice(t *testing.T) {
	cases := map[string]string{
		"":        "1", // Enter takes the default
		"\n":      "1",
		"1\n":     "1",
		"3":       "3",
		"6\n":     "6",
		" 5 \n":   "5",
		"0\n":     "0",
		"q\n":     "0",
		"bogus\n": "", // not an option: re-prompt, do not quit
		"2x\n":    "",
	}
	for in, want := range cases {
		if got := parseChoice(in); got != want {
			t.Errorf("parseChoice(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestReadChoiceEOFIsNotABlankLine: the caller tells "no operator at all" from
// "the operator pressed Ctrl+Z" by whether anything arrived, so an empty read
// has to be distinguishable from a blank line.
func TestReadChoiceEOFIsNotABlankLine(t *testing.T) {
	if line, ok := readChoice(bufio.NewReader(strings.NewReader(""))); ok {
		t.Errorf("readChoice on an empty stream = (%q, true), want ok=false", line)
	}
	if line, ok := readChoice(bufio.NewReader(strings.NewReader("\n"))); !ok || line != "\n" {
		t.Errorf("readChoice(blank line) = (%q, %v), want (\\n, true)", line, ok)
	}
	// Terminal key sequences arrive ahead of the typed character; they must not
	// turn a valid choice into "not an option".
	if got := parseChoice("\x1bOR2\n"); got != "2" {
		t.Errorf("parseChoice(function-key noise + 2) = %q, want 2", got)
	}
	// A line with no trailing newline still carries the operator's input.
	if line, ok := readChoice(bufio.NewReader(strings.NewReader("3"))); !ok || line != "3" {
		t.Errorf("readChoice(\"3\" at EOF) = (%q, %v), want (3, true)", line, ok)
	}
}

func TestTrayBinaryName(t *testing.T) {
	name := trayBinaryName()
	if !strings.HasSuffix(name, "-tray") {
		t.Errorf("expected tray binary name to end with -tray, got %q", name)
	}
}

// TestRenderMenuFitsTheWidth: the menu is read in a narrow console, so every
// line has to fit the budget with no wrapping. Both glyph sets are checked
// because the ASCII fallback is what a legacy Windows console actually sees.
func TestRenderMenuFitsTheWidth(t *testing.T) {
	for name, g := range map[string]glyphs{"ascii": asciiGlyphs, "box": boxGlyphs} {
		for _, pid := range []int{0, 4242} {
			out := renderMenu(style{pal: palette{}, g: g}, pid, "http://127.0.0.1:20130")
			for _, line := range strings.Split(out, "\n") {
				if n := visibleLen(line); n > menuWidth {
					t.Errorf("%s pid=%d: line is %d columns, budget %d: %q", name, pid, n, menuWidth, line)
				}
			}
		}
	}
}

// TestRenderMenuNamesEveryAction: the numbered keys are the menu's contract
// with the launcher switch. A key that disappears from the render is an action
// the operator can no longer discover.
func TestRenderMenuNamesEveryAction(t *testing.T) {
	out := renderMenu(testStyle(), 0, "http://127.0.0.1:20130")
	for _, want := range []string{
		"1) Open the dashboard",
		"2) Run in the background",
		"3) Run in this terminal",
		"4) Stop the background server",
		"5) Show status and log paths",
		"6) Background with tray icon",
		"Enter opens the dashboard",
		"q leaves",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("menu is missing %q", want)
		}
	}
}

// TestRenderMenuOnboardingOnlyWhenStopped: the first-run pointer is only true
// while the server is down. Showing it next to a running pid would be a lie.
func TestRenderMenuOnboardingOnlyWhenStopped(t *testing.T) {
	stopped := renderMenu(testStyle(), 0, "http://127.0.0.1:20130")
	if !strings.Contains(stopped, "First run") || !strings.Contains(stopped, "stopped") {
		t.Errorf("stopped menu should offer the first-run pointer:\n%s", stopped)
	}
	running := renderMenu(testStyle(), 4242, "http://127.0.0.1:20130")
	if strings.Contains(running, "First run") {
		t.Errorf("running menu must not show the first-run pointer:\n%s", running)
	}
	if !strings.Contains(running, "running") || !strings.Contains(running, "pid 4242") {
		t.Errorf("running menu should name the pid:\n%s", running)
	}
}

// TestRenderMenuIsPlainWhenColourIsOff: a redirected or dumb terminal must not
// receive escape codes, and the same renderer has to stay legible without them.
func TestRenderMenuIsPlainWhenColourIsOff(t *testing.T) {
	out := renderMenu(style{pal: palette{on: false}, g: asciiGlyphs}, 4242, "http://127.0.0.1:20130")
	if strings.Contains(out, "\x1b[") {
		t.Errorf("colourless menu contains ANSI escapes:\n%q", out)
	}
	if !strings.Contains(out, "running") {
		t.Errorf("colourless menu lost the status line:\n%s", out)
	}
}

// TestVisibleLenSkipsSGR: padding is computed from printable width, so a
// coloured cell must measure the same as its plain text.
func TestVisibleLenSkipsSGR(t *testing.T) {
	if got := visibleLen("\x1b[1mbold\x1b[0m"); got != 4 {
		t.Errorf("visibleLen(bold) = %d, want 4", got)
	}
	if got := visibleLen("● stopped"); got != 9 {
		t.Errorf("visibleLen with a multi-byte mark = %d, want 9", got)
	}
}

// TestTrayHintTracksTheBinary: the menu may only promise a tray icon when the
// companion binary is actually there.
func TestTrayHintTracksTheBinary(t *testing.T) {
	hint := trayHint()
	if trayBinaryPath() == "" {
		return
	}
	if _, err := os.Stat(trayBinaryPath()); err != nil && !strings.Contains(hint, "not found") {
		t.Errorf("absent tray binary should be reported, got %q", hint)
	}
}
