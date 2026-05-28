//go:build windows

package main

import (
	"errors"
	"os"
)

func pauseProcess(proc *os.Process) error {
	if proc == nil {
		return errors.New("process not available")
	}
	return errors.New("pause not supported on Windows")
}

func resumeProcess(proc *os.Process) error {
	if proc == nil {
		return errors.New("process not available")
	}
	return errors.New("resume not supported on Windows")
}
