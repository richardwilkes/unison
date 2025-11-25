package plaf

//#include "platform.h"
import "C"

import (
	"log/slog"
	"math"
	"runtime"
	"unsafe"
)

// Monitor represents a monitor.
type Monitor struct {
	data *C.plafMonitor
}

// GammaRamp describes the gamma ramp for a monitor.
type GammaRamp struct {
	Red   []uint16
	Green []uint16
	Blue  []uint16
}

// VidMode describes a single video mode.
type VidMode struct {
	Width       int // The width, in screen coordinates, of the video mode.
	Height      int // The height, in screen coordinates, of the video mode.
	RedBits     int // The bit depth of the red channel of the video mode.
	GreenBits   int // The bit depth of the green channel of the video mode.
	BlueBits    int // The bit depth of the blue channel of the video mode.
	RefreshRate int // The refresh rate, in Hz, of the video mode.
}

// MonitorCallback is called when a monitor has been connected or disconnected.
var MonitorCallback func(monitor *Monitor, connected bool)

// GetMonitors returns a slice of handles for all currently connected monitors.
func GetMonitors() []*Monitor {
	count := int(C._plaf.monitorCount)
	if count == 0 {
		return nil
	}
	m := make([]*Monitor, count)
	list := unsafe.Slice(C._plaf.monitors, count)
	for i := range count {
		m[i] = &Monitor{data: list[i]}
	}
	return m
}

// GetPrimaryMonitor returns the primary monitor. This is usually the monitor where elements like the Windows task bar
// or the OS X menu bar is located.
func GetPrimaryMonitor() *Monitor {
	if C._plaf.monitorCount == 0 {
		return nil
	}
	return &Monitor{data: *C._plaf.monitors}
}

// GetPos returns the position, in screen coordinates, of the upper-left corner of the monitor.
func (m *Monitor) GetPos() (x, y int) {
	var cx, cy C.int
	C.plafGetMonitorPos(m.data, &cx, &cy)
	return int(cx), int(cy)
}

// GetWorkarea returns the position, in screen coordinates, of the upper-left corner of the work area of the specified
// monitor along with the work area size in screen coordinates. The work area is defined as the area of the monitor not
// occluded by the operating system task bar where present. If no task bar exists then the work area is the monitor
// resolution in screen coordinates.
func (m *Monitor) GetWorkarea() (x, y, width, height int) {
	var cX, cY, cWidth, cHeight C.int
	C.plafGetMonitorWorkarea(m.data, &cX, &cY, &cWidth, &cHeight)
	return int(cX), int(cY), int(cWidth), int(cHeight)
}

// GetContentScale function retrieves the content scale for the specified monitor. The content scale is the ratio
// between the current DPI and the platform's default DPI. If you scale all pixel dimensions by this scale then your
// content should appear at an appropriate size. This is especially important for text and any UI elements.
func (m *Monitor) GetContentScale() (x, y float32) {
	var cX, cY C.float
	C.plafGetMonitorContentScale(m.data, &cX, &cY)
	return float32(cX), float32(cY)
}

// GetPhysicalSize returns the size, in millimeters, of the display area of the monitor.
//
// Note: Some operating systems do not provide accurate information, either because the monitor's EDID data is
// incorrect, or because the driver does not report it accurately.
func (m *Monitor) GetPhysicalSize() (width, height int) {
	return int(m.data.widthMM), int(m.data.heightMM)
}

// GetName returns a human-readable name of the monitor, encoded as UTF-8.
func (m *Monitor) GetName() string {
	if m.data.name[0] == 0 {
		return ""
	}
	return C.GoString(&m.data.name[0])
}

// GetVideoModes returns an array of all video modes supported by the monitor. The returned array is sorted in ascending
// order, first by color bit depth (the sum of all channel depths) and then by resolution area (the product of width and
// height).
func (m *Monitor) GetVideoModes() []*VidMode {
	if !C.plafRefreshVideoModes(m.data) || m.data.modes == nil {
		return nil
	}
	count := int(m.data.modeCount)
	result := make([]*VidMode, count)
	list := unsafe.Slice(m.data.modes, count)
	for i := range count {
		result[i] = &VidMode{
			Width:       int(list[i].width),
			Height:      int(list[i].height),
			RedBits:     int(list[i].redBits),
			GreenBits:   int(list[i].greenBits),
			BlueBits:    int(list[i].blueBits),
			RefreshRate: int(list[i].refreshRate),
		}
	}
	return result
}

// GetVideoMode returns the current video mode of the monitor. If you are using a full screen window, the return value
// will therefore depend on whether it is focused.
func (m *Monitor) GetVideoMode() *VidMode {
	t := C.plafGetVideoMode(m.data)
	if t == nil {
		return nil
	}
	return &VidMode{int(t.width), int(t.height), int(t.redBits), int(t.greenBits), int(t.blueBits), int(t.refreshRate)}
}

// SetGamma generates a gamma ramp from the specified exponent and then calls SetGamma with it.
func (m *Monitor) SetGamma(gamma float64) {
	if gamma != gamma || gamma <= 0 || gamma > math.MaxFloat64 {
		slog.Warn("SetGamma: ignoring invalid gamma value", "gamma", gamma)
		return
	}
	ramp := m.GetGammaRamp()
	if ramp == nil {
		slog.Warn("SetGamma: unable to get existing gamma ramp")
		return
	}
	channel := make([]uint16, len(ramp.Red))
	for i := range channel {
		channel[i] = uint16(min(math.Pow(float64(i)/float64(len(channel)-1), 1/gamma), 65535))
	}
	ramp.Red = channel
	ramp.Green = channel
	ramp.Blue = channel
	m.SetGammaRamp(ramp)
}

// GetGammaRamp retrieves the current gamma ramp of the monitor.
func (m *Monitor) GetGammaRamp() *GammaRamp {
	rampC := C.plafGetGammaRamp(m.data)
	if rampC == nil {
		return nil
	}
	length := int(rampC.size)
	var ramp GammaRamp
	ramp.Red = make([]uint16, length)
	ramp.Green = make([]uint16, length)
	ramp.Blue = make([]uint16, length)
	copy(ramp.Red, unsafe.Slice((*uint16)(rampC.red), length))
	copy(ramp.Green, unsafe.Slice((*uint16)(rampC.green), length))
	copy(ramp.Blue, unsafe.Slice((*uint16)(rampC.blue), length))
	return &ramp
}

// SetGammaRamp sets the current gamma ramp for the monitor.
func (m *Monitor) SetGammaRamp(ramp *GammaRamp) {
	length := len(ramp.Red)
	if length == 0 || length != len(ramp.Green) || length != len(ramp.Blue) {
		slog.Warn("SetGammaRamp: ignoring invalid ramp")
		return
	}
	cRamp := &C.plafGammaRamp{
		red:   (*C.ushort)(&ramp.Red[0]),
		green: (*C.ushort)(&ramp.Green[0]),
		blue:  (*C.ushort)(&ramp.Blue[0]),
		size:  C.uint(length),
	}
	C.plafSetGammaRamp(m.data, cRamp)
	runtime.KeepAlive(cRamp)
}
