//go:build !windows

package dashboard

// platformConnRefused is the POSIX half of the check: syscall.ECONNREFUSED and
// ECONNRESET already match there, so the numeric list is empty.
func platformConnRefused(uintptr) bool { return false }
