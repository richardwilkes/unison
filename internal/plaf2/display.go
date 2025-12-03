package plaf2

import "github.com/richardwilkes/toolbox/v2/geom"

// Display represents a single display.
type Display struct {
	Frame  geom.Rect  // The position of the display in the global screen coordinate system
	Usable geom.Rect  // The usable area, i.e. the Frame minus the area used by global menu bars or task bars
	Scale  geom.Point // The scale of the content
	// The pixels-per-inch for the display. This may not be accurate, either because the display's EDID data is
	// incorrect, or because the driver does not report it accurately.
	PPI     int
	Primary bool
}
