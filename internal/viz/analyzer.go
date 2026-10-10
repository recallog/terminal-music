package viz

import (
	"math"
	"math/cmplx"
)

const fftSize = 2048

// Analyzer turns captured samples into smoothed spectrum bars with falling
// peaks, Winamp style. It is not safe for concurrent use.
type Analyzer struct {
	cap     *Capture
	samples []float32
	window  []float64
	buf     []complex128

	bars  []float64
	peaks []float64
	hold  []int
}

// NewAnalyzer creates an analyzer reading from c (which may be nil).
func NewAnalyzer(c *Capture) *Analyzer {
	a := &Analyzer{
		cap:     c,
		samples: make([]float32, fftSize),
		window:  make([]float64, fftSize),
		buf:     make([]complex128, fftSize),
	}
	for i := range a.window {
		a.window[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(fftSize-1))
	}
	return a
}

// Available reports whether audio is being captured.
func (a *Analyzer) Available() bool { return a.cap != nil && a.cap.Err() == nil }

// Update computes n bars. When active is false (paused, stopped) the bars
// just fall back to zero. Values are in [0, 1].
func (a *Analyzer) Update(n int, active bool) (bars, peaks []float64) {
	if n <= 0 {
		return nil, nil
	}
	if len(a.bars) != n {
		a.bars = make([]float64, n)
		a.peaks = make([]float64, n)
		a.hold = make([]int, n)
	}
	var target []float64
	if active && a.Available() {
		a.cap.Latest(a.samples)
		target = a.spectrum(n)
	}
	for i := range a.bars {
		v := 0.0
		if target != nil {
			v = target[i]
		}
		if v > a.bars[i] {
			a.bars[i] = 0.4*a.bars[i] + 0.6*v
		} else {
			a.bars[i] = max(v, a.bars[i]-0.035)
		}
		if a.bars[i] >= a.peaks[i] {
			a.peaks[i] = a.bars[i]
			a.hold[i] = 12
		} else if a.hold[i] > 0 {
			a.hold[i]--
		} else {
			a.peaks[i] = max(0, a.peaks[i]-0.02)
		}
	}
	return a.bars, a.peaks
}

// Wave returns n samples in [-1, 1] spread over the last ~23 ms of audio.
func (a *Analyzer) Wave(n int) []float64 {
	out := make([]float64, n)
	if n == 0 || !a.Available() {
		return out
	}
	const span = 1024
	a.cap.Latest(a.samples[:span])
	for i := range out {
		v := float64(a.samples[i*span/n]) * 1.5
		out[i] = math.Max(-1, math.Min(1, v))
	}
	return out
}

// spectrum returns n log-spaced bands between 40 Hz and 16 kHz.
func (a *Analyzer) spectrum(n int) []float64 {
	for i, s := range a.samples {
		a.buf[i] = complex(float64(s)*a.window[i], 0)
	}
	fft(a.buf)

	const fmin, fmax = 40.0, 16000.0
	binHz := float64(SampleRate) / fftSize
	// A full-scale sine through a Hann window peaks at N/4.
	norm := float64(fftSize) / 4
	out := make([]float64, n)
	for i := range out {
		lo := fmin * math.Pow(fmax/fmin, float64(i)/float64(n))
		hi := fmin * math.Pow(fmax/fmin, float64(i+1)/float64(n))
		b0 := int(lo / binHz)
		b1 := max(int(hi/binHz), b0+1)
		mag := 0.0
		for b := b0; b < b1 && b < fftSize/2; b++ {
			mag = math.Max(mag, cmplx.Abs(a.buf[b]))
		}
		db := 20 * math.Log10(mag/norm+1e-9)
		// Music energy falls with frequency: tilt by +3 dB/octave around 1 kHz.
		db += 3 * math.Log2(math.Sqrt(lo*hi)/1000)
		out[i] = math.Max(0, math.Min(1, (db+58)/52))
	}
	return out
}

// fft is an in-place iterative radix-2 FFT. len(x) must be a power of two.
func fft(x []complex128) {
	n := len(x)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		w := cmplx.Exp(complex(0, -2*math.Pi/float64(size)))
		for start := 0; start < n; start += size {
			wk := complex(1, 0)
			for k := 0; k < size/2; k++ {
				u := x[start+k]
				v := x[start+k+size/2] * wk
				x[start+k] = u + v
				x[start+k+size/2] = u - v
				wk *= w
			}
		}
	}
}
