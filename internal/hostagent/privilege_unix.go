//go:build !windows

package hostagent

import "os"

func isElevated() bool { return os.Geteuid() == 0 }
