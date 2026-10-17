//go:build !android && !darwin

package app

// platformNotify is a no-op on platforms without a wired-up native
// attention API (Linux desktop): the frontend's chime on the new-mail event
// is the signal there.
func platformNotify(title, text string) {}
