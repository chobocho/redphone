//go:build windows

package discovery

import "syscall"

// enableBroadcast turns on SO_BROADCAST for the raw socket fd.
func enableBroadcast(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
}
