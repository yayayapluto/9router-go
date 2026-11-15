//go:build windows

package dashboard

import (
	"golang.org/x/sys/windows"
)

// platformConnRefused reports the Winsock codes a dial actually returns.
//
// Windows needs its own list because Go's syscall.ECONNREFUSED on that target is
// Errno(0x20000016) — the APPLICATION_ERROR flag plus 22 — while a refused dial
// returns WSAECONNREFUSED (10061). errors.Is therefore never matches, which is
// why a refused upstream used to surface as a raw `dial tcp … connectex` string
// instead of the "connection refused" message.
func platformConnRefused(errno uintptr) bool {
	switch errno {
	case uintptr(windows.WSAECONNREFUSED), uintptr(windows.WSAECONNRESET):
		return true
	}
	return false
}
