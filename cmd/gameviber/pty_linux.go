//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// startPty runs cmd on a new pseudo-terminal (Linux /dev/ptmx, standard
// library only): the master side, for reading output and writing input.
func startPty(cmd *exec.Cmd, cols, rows int) (*os.File, error) {
	ptm, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	fd := ptm.Fd()
	var unlock int32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); e != 0 {
		ptm.Close()
		return nil, e
	}
	var n uint32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n))); e != 0 {
		ptm.Close()
		return nil, e
	}
	pts, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		ptm.Close()
		return nil, err
	}
	defer pts.Close()
	resizePty(ptm, cols, rows)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = pts, pts, pts
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		ptm.Close()
		return nil, err
	}
	return ptm, nil
}

// resizePty tells the terminal its size (TIOCSWINSZ).
func resizePty(ptm *os.File, cols, rows int) {
	if cols <= 0 || rows <= 0 {
		return
	}
	ws := struct{ Row, Col, X, Y uint16 }{uint16(rows), uint16(cols), 0, 0}
	syscall.Syscall(syscall.SYS_IOCTL, ptm.Fd(), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(&ws)))
}

const ptySupported = true
