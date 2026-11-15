//go:build windows

package dashboard

import (
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
)

// TestClassifyRefusedOnWindows: a refused dial surfaces as WSAECONNREFUSED
// (10061), which syscall.ECONNREFUSED does not match on this platform. The
// classifier has to name it, because the raw `connectex` text is what the
// dashboard showed instead of "connection refused".
func TestClassifyRefusedOnWindows(t *testing.T) {
	dialErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: &os.SyscallError{Syscall: "connectex", Err: syscall.Errno(10061)},
	}
	status, code, msg := ClassifyUpstreamNetworkError(dialErr)
	if status != 502 || code != "UPSTREAM_UNREACHABLE" {
		t.Fatalf("classified as %d/%q, want 502/UPSTREAM_UNREACHABLE", status, code)
	}
	if !strings.Contains(msg, "Connection refused") {
		t.Errorf("message %q does not name the refusal", msg)
	}
}
