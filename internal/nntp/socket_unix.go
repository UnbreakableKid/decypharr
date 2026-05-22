//go:build !windows

package nntp

import "golang.org/x/sys/unix"

func setSocketBuffers(fd uintptr, rb, wb int) error {
	if rb > 0 {
		_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF, rb)
	}
	if wb > 0 {
		_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_SNDBUF, wb)
	}
	return nil
}
