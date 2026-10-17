package app

import "fmt"

// notifyNewMail raises the platform "mail arrived" signal after a background
// poll delivered new messages: an Android system notification (with sound), a
// macOS dock bounce, or nothing on other platforms (the frontend's beep on
// the new-mail event covers those). The frontend chime is driven separately
// by the new-mail event emitted alongside this call.
func (a *App) notifyNewMail(count int) {
	if count == 1 {
		platformNotify("BOI", "1 new message")
		return
	}
	platformNotify("BOI", fmt.Sprintf("%d new messages", count))
}
