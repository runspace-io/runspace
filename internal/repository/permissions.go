package repository

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	agentUID = 1000
	agentGID = 1000
)

// prepareAgentCheckout transfers a cloned checkout to the fixed non-root
// identity used by terminal and runtime containers. Docker Desktop exposes the
// named volume through the Linux gateway, where Chown is authoritative.
func prepareAgentCheckout(root string) error {
	return filepath.Walk(root, func(path string, _ os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if runtime.GOOS == "windows" {
			return nil
		}
		return os.Lchown(path, agentUID, agentGID)
	})
}
