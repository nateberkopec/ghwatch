package app

import "github.com/gen2brain/beeep"

// notify sends a system notification with the given title and message.
// Uses Alert which includes a system sound.
// Errors are ignored to ensure notification failures don't crash the app.
func notify(title, message string) {
	_ = beeep.Alert(title, message, "")
}
