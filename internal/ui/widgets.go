package ui

import (
	"fmt"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// fit truncates or pads a styled string to exactly w cells.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if sw := ansi.StringWidth(s); sw > w {
		s = ansi.Truncate(s, w, "…")
	}
	if sw := ansi.StringWidth(s); sw < w {
		s += strings.Repeat(" ", w-sw)
	}
	return s
}

func width(s string) int { return ansi.StringWidth(s) }

func fmtTime(secs float64) string {
	if secs < 0 || math.IsNaN(secs) {
		secs = 0
	}
	t := int(secs)
	if t >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", t/3600, t/60%60, t%60)
	}
	return fmt.Sprintf("%d:%02d", t/60, t%60)
}

// fmtClock formats a time for the big display: always two-digit minutes.
func fmtClock(secs float64) string {
	if secs < 0 || math.IsNaN(secs) {
		secs = 0
	}
	t := int(secs)
	if t >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", t/3600, t/60%60, t%60)
	}
	return fmt.Sprintf("%02d:%02d", t/60, t%60)
}

// Big LCD-style digits, three cells wide and three rows tall.
var bigGlyphs = map[rune][3]string{
	'0': {"█▀█", "█ █", "▀▀▀"},
	'1': {" ▀█", "  █", "  ▀"},
	'2': {"▀▀█", "█▀▀", "▀▀▀"},
	'3': {"▀▀█", " ▀█", "▀▀▀"},
	'4': {"█ █", "▀▀█", "  ▀"},
	'5': {"█▀▀", "▀▀█", "▀▀▀"},
	'6': {"█▀▀", "█▀█", "▀▀▀"},
	'7': {"▀▀█", "  █", "  ▀"},
	'8': {"█▀█", "█▀█", "▀▀▀"},
	'9': {"█▀█", "▀▀█", "▀▀▀"},
	':': {" ", "▀", "▀"},
	'-': {"  ", "▀▀", "  "},
	' ': {"  ", "  ", "  "},
}

// bigText renders s with the big glyphs.
func bigText(s string) [3]string {
	var rows [3]string
	for i, r := range s {
		g, ok := bigGlyphs[r]
		if !ok {
			continue
		}
		for k := range rows {
			if i > 0 {
				rows[k] += " "
			}
			rows[k] += g[k]
		}
	}
	return rows
}

var eighths = []rune(" ▁▂▃▄▅▆▇█")

// spectrumView draws bars (values in [0, 1]) as w×h cells. Each bar is one
// cell wide with a one cell gap when there is room.
func (m *Model) spectrumView(bars, peaks []float64, w, h int) []string {
	rows := make([]string, h)
	grid := make([][]string, h)
	for r := range grid {
		grid[r] = make([]string, w)
		for c := range grid[r] {
			grid[r][c] = " "
		}
	}
	step := 2
	if w < 24 {
		step = 1
	}
	for i := range bars {
		x := i * step
		if x >= w {
			break
		}
		units := int(bars[i]*float64(h*8) + 0.5)
		peakRow := -1
		if i < len(peaks) && peaks[i] > 0.02 {
			peakRow = h - 1 - min(h-1, int(peaks[i]*float64(h)))
		}
		for r := 0; r < h; r++ {
			fromBottom := h - 1 - r
			fill := units - fromBottom*8
			style := m.st.bar(float64(fromBottom) / float64(max(h-1, 1)))
			switch {
			case fill >= 8:
				grid[r][x] = style.Render("█")
			case fill > 0:
				grid[r][x] = style.Render(string(eighths[fill]))
			case r == peakRow:
				grid[r][x] = m.st.peak.Render("▔")
			}
		}
	}
	for r := range rows {
		rows[r] = strings.Join(grid[r], "")
	}
	return rows
}

// scopeView draws the waveform with braille dots (2×4 dots per cell).
func (m *Model) scopeView(wave []float64, w, h int) []string {
	dotsW, dotsH := w*2, h*4
	cells := make([][]rune, h)
	for r := range cells {
		cells[r] = make([]rune, w)
		for c := range cells[r] {
			cells[r][c] = 0x2800
		}
	}
	bit := [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}
	set := func(x, y int) {
		if x < 0 || y < 0 || x >= dotsW || y >= dotsH {
			return
		}
		cells[y/4][x/2] |= bit[y%4][x%2]
	}
	yOf := func(v float64) int {
		return int((1 - v) / 2 * float64(dotsH-1))
	}
	prev := dotsH / 2
	for x := 0; x < dotsW; x++ {
		v := 0.0
		if x < len(wave) {
			v = wave[x]
		}
		y := yOf(v)
		if x == 0 {
			prev = y
		}
		lo, hi := min(prev, y), max(prev, y)
		for yy := lo; yy <= hi; yy++ {
			set(x, yy)
		}
		prev = y
	}
	rows := make([]string, h)
	for r := range rows {
		frac := 1 - math.Abs(float64(r)-float64(h-1)/2)/(float64(h)/2)
		rows[r] = m.st.bar(frac).Render(string(cells[r]))
	}
	return rows
}

// hslider draws a horizontal slider of w cells at position f in [0, 1].
func hslider(f float64, w int, done, knob, rest lipgloss.Style) string {
	if w <= 0 {
		return ""
	}
	f = max(0, min(1, f))
	k := int(f*float64(w-1) + 0.5)
	return done.Render(strings.Repeat("━", k)) +
		knob.Render("●") +
		rest.Render(strings.Repeat("─", w-k-1))
}

// marquee scrolls text horizontally within w cells, Winamp style.
func marquee(text string, w, offset int) string {
	if width(text) <= w {
		return text
	}
	runes := []rune(text + "  ***  ")
	n := len(runes)
	offset %= n
	var b strings.Builder
	cells := 0
	for i := 0; cells < w; i++ {
		r := runes[(offset+i)%n]
		rw := ansi.StringWidth(string(r))
		if cells+rw > w {
			break
		}
		b.WriteRune(r)
		cells += rw
	}
	return b.String()
}
