//go:build !linux && !darwin

package app

// badgeSet is a no-op on platforms without a wired-up count badge (e.g.
// Windows): the window-title suffix from SetBadge is the signal there.
func (a *App) badgeSet(count int) {}
