package app

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// SetWindow gives the App its main window handle (called from main.go after
// window creation) so the badge can update the title bar.
func (a *App) SetWindow(w *application.WebviewWindow) {
	a.win = w
}

// SetBadge shows the unread-mail count: Linux emits the Unity LauncherEntry
// D-Bus signal (rendered by Dash-to-Dock/Ubuntu Dock/Plank) and updates the
// window title — "BOI (5 unread)" — which is the universal fallback that
// even vanilla GNOME Shell shows in the top bar and alt-tab; macOS sets the
// classic dock-icon badge; Android mirrors the count onto the foreground-
// service notification so the launcher icon badges with the same number.
func (a *App) SetBadge(count int) {
	if count < 0 {
		count = 0
	}
	a.badgeSet(count)
	if a.win != nil {
		if count > 0 {
			a.win.SetTitle(fmt.Sprintf("BOI (%d unread)", count))
		} else {
			a.win.SetTitle("BOI")
		}
	}
}
