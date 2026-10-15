//go:build !android

package app

import "os"

// dataDir returns the base directory for BOI's config/database on desktop
// platforms: the user's config dir (~/.config on Linux,
// ~/Library/Application Support on macOS).
func dataDir() (string, error) {
	return os.UserConfigDir()
}
