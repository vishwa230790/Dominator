// +build linux

package main

import (
	"os"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

func openPty() (pty, tty *os.File, err error) {
	p, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	// In case of error after this point, make sure we close the ptmx fd.
	defer func() {
		if err != nil {
			p.Close() // Best effort.
		}
	}()
	sname, err := ptsname(p)
	if err != nil {
		return nil, nil, err
	}
	if err := unlockpt(p); err != nil {
		return nil, nil, err
	}
	t, err := os.OpenFile(sname, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return p, t, nil
}

func ptsname(f *os.File) (string, error) {
	var n uint32
	err := wsyscall.Ioctl(int(f.Fd()), syscall.TIOCGPTN,
		uintptr(unsafe.Pointer(&n)))
	if err != nil {
		return "", err
	}
	return "/dev/pts/" + strconv.Itoa(int(n)), nil
}

func unlockpt(f *os.File) error {
	var u int32
	// Use TIOCSPTLCK with a zero valued arg to clear the slave pty lock.
	return wsyscall.Ioctl(int(f.Fd()), syscall.TIOCSPTLCK,
		uintptr(unsafe.Pointer(&u)))
}
