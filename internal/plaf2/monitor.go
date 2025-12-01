package plaf2

import (
	"log/slog"
)

var monitorList []*Monitor

type Monitor struct {
	originalGammaRamp *GammaRamp
	id                platformMonitorID
}

func (m *Monitor) GammaRamp() *GammaRamp { // formerly plafGetGammaRamp
	return m.gammaRamp()
}

func (m *Monitor) SetGammaRamp(ramp *GammaRamp) { // formerly plafSetGammaRamp
	if !ramp.Valid() {
		slog.Warn("Monitor.SetGammaRamp: ignoring invalid ramp")
		return
	}
	saved := m.originalGammaRamp
	if m.originalGammaRamp == nil {
		m.originalGammaRamp = m.GammaRamp()
	}
	if len(m.originalGammaRamp.Red) != len(ramp.Red) {
		m.originalGammaRamp = saved
		slog.Warn("Monitor.SetGammaRamp: ignoring invalid ramp - must have same number of entries as original")
		return
	}
	m.setGammaRamp(ramp)
}

type GammaRamp struct {
	Red   []uint16
	Green []uint16
	Blue  []uint16
}

func (g *GammaRamp) Valid() bool {
	return g != nil && len(g.Red) != 0 && len(g.Red) == len(g.Green) && len(g.Red) == len(g.Blue)
}
