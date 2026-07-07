//go:build !windows
// +build !windows

package launchlib

import (
	"os/exec"
)

const javaExecutablePath = "bin/java.exe"

func setSysProcAttr(cmd *exec.Cmd) {}
