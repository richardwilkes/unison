package plaf2

import "github.com/richardwilkes/unison/internal/mac"

func PollEvents() {
	mac.PollEvents()
}

func WaitEvents() {
	mac.WaitEvents()
}

func WaitEventsTimeout(timeoutSeconds float64) {
	mac.WaitEventsTimeout(timeoutSeconds)
}

func PostEmptyEvent() {
	mac.PostEmptyEvent()
}
