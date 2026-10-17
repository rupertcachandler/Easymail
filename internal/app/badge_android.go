//go:build android

package app

// badgeSet mirrors the unread count onto the always-present foreground-service
// notification (silent, MIN importance) via JNI. Samsung-style launchers badge
// the icon with the notification's number, so the launcher icon shows the same
// unread count as the desktop title bar, decrementing as mail is read.
func (a *App) badgeSet(count int) { platformUpdateBadge(count) }
