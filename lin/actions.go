package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func OpenInFileManager(path string) error {
	if path == "" {
		return errors.New("path required")
	}
	target := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		target = filepath.Dir(path)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = configureCommand(exec.Command("open", target))
	case "windows":
		cmd = configureCommand(exec.Command("explorer", target))
	default:
		cmd = configureCommand(exec.Command("xdg-open", target))
	}
	return cmd.Start()
}
