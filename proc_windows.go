//go:build windows

package main

import (
	"fmt"
	"os/exec"
)

func configureCmd(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Kill the process tree on Windows.
		return exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
	}
}
