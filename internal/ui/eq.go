package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/recallog/terminal-music/internal/config"
)

const eqMax = 12.0 // dB, the sliders go from -eqMax to +eqMax

var (
	eqFreqs  = [10]float64{31, 62, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}
	eqLabels = [11]string{"PRE", "31", "62", "125", "250", "500", "1k", "2k", "4k", "8k", "16k"}
	// Two-cell labels for narrow windows; the header shows the full name.
	eqShortLabels = [11]string{"PR", "31", "62", "12", "25", "50", "1k", "2k", "4k", "8k", "16"}
)

type eqPreset struct {
	name  string
	bands [10]float64
}

var eqPresets = []eqPreset{
	{"Flat", [10]float64{}},
	{"Rock", [10]float64{5, 4, 3, 1, -1, -1, 1, 3, 4, 5}},
	{"Pop", [10]float64{-1, 1, 3, 4, 3, 0, -1, -1, -1, -1}},
	{"Jazz", [10]float64{3, 2, 1, 2, -1, -1, 0, 1, 2, 3}},
	{"Classical", [10]float64{0, 0, 0, 0, 0, 0, -4, -4, -4, -6}},
	{"Dance", [10]float64{6, 5, 2, 0, 0, -3, -4, -4, 0, 0}},
	{"Bass boost", [10]float64{7, 6, 5, 3, 1, 0, 0, 0, 0, 0}},
	{"Treble boost", [10]float64{0, 0, 0, 0, 0, 1, 3, 5, 6, 7}},
	{"Vocal", [10]float64{-2, -3, -3, 1, 4, 4, 3, 1, 0, -2}},
	{"Loudness", [10]float64{5, 3, 0, 0, -1, 0, 0, 2, 4, 5}},
}

// eqGain returns the gain of slider i (0 is the preamp).
func eqGain(eq *config.EQ, i int) float64 {
	if i == 0 {
		return eq.Preamp
	}
	return eq.Bands[i-1]
}

// setEQGain sets slider i, clamped and rounded to 0.5 dB.
func setEQGain(eq *config.EQ, i int, g float64) {
	g = math.Round(math.Max(-eqMax, math.Min(eqMax, g))*2) / 2
	if i == 0 {
		eq.Preamp = g
	} else {
		eq.Bands[i-1] = g
	}
	eq.Preset = ""
}

func applyPreset(eq *config.EQ, p eqPreset) {
	eq.Bands = p.bands
	eq.Preset = p.name
}

func presetIndex(name string) int {
	for i, p := range eqPresets {
		if p.name == name {
			return i
		}
	}
	return -1
}

// eqFilter builds the mpv "af" property value for the equalizer.
func eqFilter(eq config.EQ) string {
	if !eq.Enabled {
		return ""
	}
	var parts []string
	if eq.Preamp != 0 {
		parts = append(parts, fmt.Sprintf("volume=volume=%.1fdB", eq.Preamp))
	}
	for i, g := range eq.Bands {
		if g != 0 {
			parts = append(parts, fmt.Sprintf("equalizer=f=%g:t=o:w=1:g=%.1f", eqFreqs[i], g))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "@eq:lavfi=[" + strings.Join(parts, ",") + "]"
}
