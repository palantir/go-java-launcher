//go:build unix

package cli

import (
	"os/exec"
)

func setAttrToRunInBackground(cmd *exec.Cmd) {
	// Not necessary for unix
}
