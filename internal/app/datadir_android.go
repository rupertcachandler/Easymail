//go:build android

package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// dataDir returns the base directory for BOI's config/database on Android:
// the app's private internal files dir (getFilesDir()), which needs no
// permissions and is wiped with the app. $HOME/$XDG_CONFIG_HOME do not exist
// in an Android app process, so os.UserConfigDir() is not usable here.
func dataDir() (string, error) {
	base := application.Android.StoragePath()
	if base == "" {
		return "", fmt.Errorf("android storage path unavailable (native bridge not ready)")
	}
	dir := filepath.Join(base, "config")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("cannot create config dir %s: %w", dir, err)
	}
	return dir, nil
}
