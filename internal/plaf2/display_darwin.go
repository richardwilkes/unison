package plaf2

import (
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/mac"
)

// PrimaryDisplay returns the primary display. This is usually the display where elements like the Windows task bar or
// the macOS menu bar is located.
func PrimaryDisplay() *Display {
	return convertDarwinDisplay(mac.MainDisplayID())
}

// ActiveDisplays returns all currently active displays.
func ActiveDisplays() []*Display {
	displayIDs := mac.ActiveDisplayList()
	result := make([]*Display, 0, len(displayIDs))
	for _, id := range displayIDs {
		if display := convertDarwinDisplay(id); display != nil {
			result = append(result, display)
		}
	}
	return result
}

func convertDarwinDisplay(id mac.DisplayID) *Display {
	if mac.DisplayIsAsleep(id) {
		return nil
	}
	screen := mac.ScreenForDisplayID(id)
	if screen == 0 {
		return nil
	}
	mainDisplayID := mac.MainDisplayID()
	height := mac.DisplayBounds(mainDisplayID).Height
	var display Display
	display.Frame = screen.Frame()
	pixels := screen.ConvertRectToBacking(display.Frame)
	display.Frame.Y = height - display.Frame.Bottom()
	display.Usable = screen.VisibleFrame()
	display.Usable.Y = height - display.Usable.Bottom()
	display.Scale = geom.NewPoint(pixels.Width/display.Frame.Width, pixels.Height/display.Frame.Height)
	sizeMM := mac.DisplayScreenSize(id)
	display.PPI = (int)(pixels.Width / (sizeMM.Width / 25.4))
	display.Primary = id == mainDisplayID
	return &display
}

func transformCocoaY(y float32) float32 {
	return mac.DisplayBounds(mac.MainDisplayID()).Height - y
}
