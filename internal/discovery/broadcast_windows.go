//go:build windows

package discovery

import "syscall"

// enableBroadcast turns on SO_BROADCAST for the raw socket fd.
func enableBroadcast(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
}

// controlReuse sets SO_REUSEADDR (before bind) so multiple instances on one
// host can share the discovery port and all receive broadcasts.
func controlReuse(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}
