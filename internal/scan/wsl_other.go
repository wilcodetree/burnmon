//go:build !windows

package scan

import "time"

// wslSources returns nil on every platform except Windows: there is no WSL
// distribution to detect here, and no Lxss registry key to read.
func wslSources(deadline time.Duration) []string {
	return nil
}

// WSLHomeSources returns nil on every platform except Windows, same reason
// as wslSources. See wsl.go for the real implementation.
func WSLHomeSources(deadline time.Duration, relPath, envVar string) []string {
	return nil
}

// WSLDistroNames returns nil on every platform except Windows, same reason
// as wslSources: there is nothing here for the "reading Linux files from
// Windows is slow" WSL cadence in app.go's stampAreaHTML/appRebuildNotice to
// ever name.
func WSLDistroNames() []string {
	return nil
}
