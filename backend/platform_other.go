//go:build !windows

package backend

import (
	"os/exec"
	"path/filepath"
)

var Xdelta = resolveXdelta()

func resolveXdelta() string {
	local := filepath.Join("Tools", "xdelta")
	if FilepathExists(local) {
		return local
	}
	if path, err := exec.LookPath("xdelta3"); err == nil {
		return path
	}
	return local
}

func HideWindow(cmd *exec.Cmd) {}
