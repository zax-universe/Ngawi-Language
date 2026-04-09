package main

import (
	"os/exec"
)

func newShellCmd(cmd string) *exec.Cmd {
	return exec.Command("sh", "-c", cmd)
}
