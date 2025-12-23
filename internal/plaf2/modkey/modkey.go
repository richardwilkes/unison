package modkey

// State holds the state of modifier keys.
type State byte

// Modifier keys.
const (
	Shift State = 1 << iota
	Control
	Alt
	Super
	CapsLock
	NumLock
)
