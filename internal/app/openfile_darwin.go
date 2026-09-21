//go:build darwin

package app

import (
	"fmt"
	"os/exec"
)

// openDefaultApp opens the given file with the platform's default
// application. macOS uses `open`. It returns an error if the open command
// could not be launched (not if the app itself fails afterwards).
func openDefaultApp(path string) error {
	cmd := exec.Command("open", path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open failed: %w", err)
	}
	// Detach so the app doesn't wait on the opened program.
	go cmd.Wait()
	return nil
}
