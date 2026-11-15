package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode/utf8"

	"9router/proxy/internal/daemon"
	"9router/proxy/internal/updater"
)

// menuWidth is the column budget every menu line is laid out inside. A stock
// console wraps before 80 columns once its scrollbar is visible, and the
// launcher is read in narrow windows more often than not.
const menuWidth = 66

// labelWidth is the label column: long enough for the longest action, short
// enough to leave a hint column that still fits the budget.
const labelWidth = 26

// glyphs is the frame vocabulary. Two sets exist because a Windows console
// still running in a non-UTF-8 code page renders box drawing as mojibake;
// there the menu degrades to ASCII instead of printing garbage.
type glyphs struct {
	topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical, mark string
}

var (
	boxGlyphs   = glyphs{"╭", "╮", "╰", "╯", "─", "│", "●"}
	asciiGlyphs = glyphs{"+", "+", "+", "+", "-", "|", "*"}
)

// palette wraps text in SGR codes only when the terminal said it understands
// them. Every style helper is a no-op otherwise, so the same renderer serves a
// colour terminal and a redirected pipe.
type palette struct{ on bool }

func (p palette) paint(code, s string) string {
	if !p.on {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (p palette) bold(s string) string  { return p.paint("1", s) }
func (p palette) dim(s string) string   { return p.paint("2", s) }
func (p palette) green(s string) string { return p.paint("32", s) }

// style is everything the renderer needs to know about the terminal it draws
// into. restore undoes any console state newStyle had to change; the menu defers
// it so the shell gets its console back.
type style struct {
	pal     palette
	g       glyphs
	restore func()
}

func newStyle() style {
	color, restore := prepareConsole(os.Stdout)
	return style{pal: palette{colorAllowed(color)}, g: consoleGlyphs(), restore: restore}
}

func (s style) paint(code, text string) string { return s.pal.paint(code, text) }
func (s style) bold(text string) string        { return s.pal.bold(text) }
func (s style) dim(text string) string         { return s.pal.dim(text) }
func (s style) green(text string) string       { return s.pal.green(text) }

// menuText renders the menu for the live daemon state.
func menuText(s style) string {
	return renderMenu(s, daemon.RunningPID(), daemonURL())
}

// item is one menu row: the key to type, what it does, and when to reach for
// it. The hint column is what stops "run in the background" and "run in this
// terminal" from reading as the same option.
type item struct {
	key, label, hint string
}

// renderMenu draws the status panel, the grouped actions, and the prompt.
//
// The pid is a parameter rather than a call so a test can pin the running
// state without a daemon on the machine.
func renderMenu(s style, pid int, url string) string {
	var b strings.Builder
	b.WriteString("\n")
	writePanel(&b, s, pid, url)
	b.WriteString("\n")

	// Keys stay the numbers the CLI has always used; only their grouping and
	// wording change. Option 1 starts the server as a side effect, so the
	// status decides whether that row promises a start or a jump.
	dashboardHint := "starts the server if stopped"
	if pid != 0 {
		dashboardHint = "already running"
	}
	writeSection(&b, s, "START", []item{
		{"1", "Open the dashboard", dashboardHint},
		{"2", "Run in the background", "survives this window closing"},
		{"3", "Run in this terminal", "logs stay here; blocks menu"},
	})
	writeSection(&b, s, "MANAGE", []item{
		{"4", "Stop the background server", stopHint(pid)},
		{"5", "Show status and log paths", ""},
		{"6", "Background with tray icon", trayHint()},
	})

	b.WriteString("\n")
	if pid == 0 {
		// Onboarding, not decoration: this is the one state where the next
		// move is not obvious, and the dashboard is where a provider is added.
		b.WriteString("  " + s.dim("First run: press 1, then add a provider in the dashboard.") + "\n")
	} else {
		b.WriteString("  " + s.dim("Leaving keeps the server up; 4 stops it.") + "\n")
	}
	b.WriteString("  " + s.dim("Enter opens the dashboard. q leaves.") + "\n\n")
	b.WriteString("  " + s.bold(">") + " ")
	return b.String()
}

// writePanel draws identity and live state. The status mark is the menu's only
// colour: it answers "is it up?" before any action is read.
func writePanel(b *strings.Builder, s style, pid int, url string) {
	title := fmt.Sprintf("%s v%s", binaryName(), updater.CurrentVersion)
	head := " " + s.bold(title) + " "
	// 2 indent + corner + horizontal + head + filler + corner.
	pad := menuWidth - 5 - visibleLen(head)
	if pad < 0 {
		pad = 0
	}
	b.WriteString("  " + s.dim(s.g.topLeft+s.g.horizontal) + head +
		s.dim(strings.Repeat(s.g.horizontal, pad)+s.g.topRight) + "\n")

	b.WriteString(panelRow(s, runtime.GOOS+"/"+runtime.GOARCH) + "\n")
	b.WriteString(panelRow(s, statusLine(s, pid)) + "\n")
	b.WriteString(panelRow(s, url) + "\n")

	b.WriteString("  " + s.dim(s.g.bottomLeft+strings.Repeat(s.g.horizontal, menuWidth-4)+s.g.bottomRight) + "\n")
}

// statusLine is the state the whole menu hangs off: running with a pid, or
// stopped with nothing invented in its place.
func statusLine(s style, pid int) string {
	if pid != 0 {
		return s.green(s.g.mark) + " running" + s.dim(fmt.Sprintf("   pid %d", pid))
	}
	return s.dim(s.g.mark) + " stopped"
}

// panelRow is one framed content row: 2 indent + corner + space + content +
// filler + space + corner.
func panelRow(s style, content string) string {
	pad := menuWidth - 6 - visibleLen(content)
	if pad < 0 {
		pad = 0
	}
	return "  " + s.dim(s.g.vertical) + " " + content + strings.Repeat(" ", pad) + " " + s.dim(s.g.vertical)
}

// writeSection emits one labelled group of actions with its hint column.
func writeSection(b *strings.Builder, s style, title string, items []item) {
	fmt.Fprintf(b, "  %s\n", s.dim(title))
	for _, it := range items {
		b.WriteString("  " + s.bold(it.key+")") + " " + it.label)
		if it.hint != "" {
			// 2 indent + "N) " + label column + gap + hint.
			pad := labelWidth - visibleLen(it.label)
			if pad < 1 {
				pad = 1
			}
			b.WriteString(strings.Repeat(" ", pad) + s.dim(it.hint))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func stopHint(pid int) string {
	if pid == 0 {
		return "nothing is running"
	}
	return fmt.Sprintf("stops pid %d", pid)
}

// trayHint reports the tray binary's presence, because promising an icon that
// is not installed is exactly the kind of surprise this menu is meant to end.
func trayHint() string {
	path := trayBinaryPath()
	if path == "" {
		return "tray binary not found"
	}
	if _, err := os.Stat(path); err != nil {
		return "tray binary not found"
	}
	return "icon in the notification area"
}

// visibleLen counts printable runes, skipping ANSI SGR sequences, so padding
// survives coloured cells.
func visibleLen(s string) int {
	n := 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		n++
		i += size
	}
	return n
}
