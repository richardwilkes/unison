package plaf2

import "github.com/richardwilkes/unison/internal/mac"

type platformMonitorID = mac.DisplayID

func (m *Monitor) gammaRamp() *GammaRamp {
	r, g, b := mac.GetDisplayGammaRamp(m.id)
	return &GammaRamp{
		Red:   rampToUint16(r),
		Green: rampToUint16(g),
		Blue:  rampToUint16(b),
	}
}

func rampToUint16(in []float32) []uint16 {
	out := make([]uint16, len(in))
	for i, value := range in {
		out[i] = uint16(value * 65535)
	}
	return out
}

func (m *Monitor) setGammaRamp(ramp *GammaRamp) {
	mac.SetDisplayGammaRamp(m.id, rampToFloat32(ramp.Red), rampToFloat32(ramp.Green), rampToFloat32(ramp.Blue))
}

func rampToFloat32(in []uint16) []float32 {
	out := make([]float32, len(in))
	for i, value := range in {
		out[i] = float32(value) / 65535
	}
	return out
}

func pollMonitors() {
	// TODO: Implement
}
