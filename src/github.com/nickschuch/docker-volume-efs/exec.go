package main

import (
	"os"
	"os/exec"
)

// Helper function to execute a command.
func Exec(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	if *cliVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
