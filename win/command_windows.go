//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func configureCommand(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}
