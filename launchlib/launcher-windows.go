//go:build windows
// +build windows

package launchlib

import (
	"os/exec"

	"golang.org/x/sys/windows"
)

const javaExecutablePath = "bin/java.exe"

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
}
