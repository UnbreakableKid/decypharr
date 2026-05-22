//go:build windows

package nntp

import "syscall"

func setSocketBuffers(fd uintptr, rb, wb int) error {
	if rb > 0 {
		_ = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, rb)
	}
	if wb > 0 {
		_ = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, wb)
	}
	return nil
}
