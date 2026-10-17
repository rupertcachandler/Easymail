package app

import (
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SetBadge shows the unread-mail count: Linux emits the Unity LauncherEntry
// D-Bus signal (rendered by Dash-to-Dock/Ubuntu Dock/Plank) and updates the
// window title — "BOI (5 unread)" — which is the universal fallback that
// even vanilla GNOME Shell shows in the top bar and alt-tab; macOS sets the
// classic dock-icon badge; Android is a no-op (its notifications already
// carry the count; launchers only support dots, not numbers).
func (a *App) SetBadge(count int) {
	if count < 0 {
		count = 0
	}
	a.badgeSet(count)
	if count > 0 {
		runtime.WindowSetTitle(a.ctx, fmt.Sprintf("BOI (%d unread)", count))
	} else {
		runtime.WindowSetTitle(a.ctx, "BOI")
	}
}
