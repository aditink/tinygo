//go:build linux

package main

import "syscall"

func syscallSocketpair() ([2]int, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return fds, err
	}
	// Make both ends non-blocking so os.NewFile registers them with the
	// runtime netpoller (pollable = nonBlocking for kindNewFile).
	for _, fd := range fds {
		if err := syscall.SetNonblock(fd, true); err != nil {
			return fds, err
		}
	}
	return fds, nil
}
