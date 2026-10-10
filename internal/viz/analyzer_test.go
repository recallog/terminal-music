package viz

import (
	"math"
	"testing"
)

func TestSpectrumPeaksAtToneFrequency(t *testing.T) {
	a := NewAnalyzer(nil)
	const freq = 1000.0
	for i := range a.samples {
		a.samples[i] = float32(0.5 * math.Sin(2*math.Pi*freq*float64(i)/SampleRate))
	}
	const n = 40
	bands := a.spectrum(n)
	best := 0
	for i, v := range bands {
		if v > bands[best] {
			best = i
		}
	}
	lo := 40 * math.Pow(16000.0/40, float64(best)/n)
	hi := 40 * math.Pow(16000.0/40, float64(best+1)/n)
	if freq < lo*0.9 || freq > hi*1.1 {
		t.Fatalf("loudest band %d covers %.0f-%.0f Hz, want %v Hz", best, lo, hi, freq)
	}
	if bands[best] < 0.8 {
		t.Fatalf("tone band level %.2f, want close to full", bands[best])
	}
	if bands[2] > 0.2 {
		t.Fatalf("low band level %.2f, want near zero", bands[2])
	}
}
