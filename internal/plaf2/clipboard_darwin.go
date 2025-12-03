package plaf2

import "github.com/richardwilkes/unison/internal/mac"

// GetClipboardString returns the contents of the system clipboard, if it contains or is convertible to a UTF-8 encoded
// string.
func GetClipboardString() string {
	return mac.PasteboardString()
}

// SetClipboardString sets the system clipboard to the specified UTF-8 encoded string.
func SetClipboardString(str string) {
	mac.SetPasteboardString(str)
}
