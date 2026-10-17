//go:build linux && !android

package app

import "github.com/godbus/dbus/v5"

// badgeSet emits the Unity LauncherEntry update on the session bus. Count
// badges are rendered by Dash-to-Dock, Ubuntu Dock and Plank; vanilla GNOME
// Shell ignores them (SetBadge's window-title suffix is the fallback there).
// godbus caches the session bus connection; errors are deliberately silent —
// no bus (or no dock that cares) just means no badge.
func (a *App) badgeSet(count int) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return
	}
	_ = conn.Emit("/com/canonical/unity/launcherentry/boi",
		"com.canonical.Unity.LauncherEntry.Update",
		"application://boi.desktop",
		map[string]dbus.Variant{
			"count":         dbus.MakeVariant(int64(count)),
			"count_visible": dbus.MakeVariant(count > 0),
		},
	)
}
