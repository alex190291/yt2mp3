//go:build !windows

package main

import (
	"os"
	"syscall"
)

func pauseProcess(proc *os.Process) error {
	return proc.Signal(syscall.SIGSTOP)
}

func resumeProcess(proc *os.Process) error {
	return proc.Signal(syscall.SIGCONT)
}
