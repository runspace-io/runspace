//go:build windows

package hostagent

import (
	"os/exec"
	"strings"
)

func isElevated() bool {
	output, err := exec.Command("whoami", "/groups").CombinedOutput()
	if err != nil {
		return false
	}
	groups := string(output)
	return strings.Contains(groups, "S-1-16-12288") ||
		strings.Contains(groups, "S-1-16-16384")
}
