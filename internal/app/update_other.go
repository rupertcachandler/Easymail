//go:build !android

package app

// Desktop builds don't self-update: packages come from the deb (apt) or the
// Mac build process. InstallUpdate exists as a binding so the frontend code
// is platform-agnostic; it's a no-op here.
func startUpdateWatcher(a *App) {}

func (a *App) InstallUpdate() {}
