//go:build !linux

package main

import (
	"errors"
	"os"
	"os/exec"
)

// startPty: the terminal panel needs a pseudo-terminal; only Linux is done
// (macOS / Windows: later — the editor says so).
func startPty(cmd *exec.Cmd, cols, rows int) (*os.File, error) {
	return nil, errors.New("the terminal panel works on Linux for now")
}

func resizePty(ptm *os.File, cols, rows int) {}

const ptySupported = false
