//go:build windows

package backend

import (
	"os/exec"
	"path/filepath"
	"syscall"
)

var Xdelta = filepath.Join("Tools", "xdelta.exe")

func HideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
