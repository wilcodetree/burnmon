//go:build !windows

package scan

import "time"

// wslSources returns nil on every platform except Windows: there is no WSL
// distribution to detect here, and no Lxss registry key to read.
func wslSources(deadline time.Duration) []string {
	return nil
}
