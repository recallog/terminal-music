package ui

import (
	"os/exec"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/recallog/terminal-music/internal/config"
	"github.com/recallog/terminal-music/internal/mpv"
	"github.com/recallog/terminal-music/internal/playlist"
	"github.com/recallog/terminal-music/internal/theme"
	"github.com/recallog/terminal-music/internal/viz"
)

func testModel(t *testing.T, w, h int) *Model {
	t.Helper()
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv not installed")
	}
	c, err := mpv.Start(mpv.Options{ExtraArgs: []string{"--ao=null"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	pl := playlist.New()
	for _, n := range []string{"a.mp3", "b.mp3", "c.mp3"} {
		pl.Add(&playlist.Track{Path: filepath.Join("/nonexistent", n), Probed: true})
	}
	m := New(Options{
		Config:   config.Default(),
		Themes:   theme.Builtins(),
		MPV:      c,
		Playlist: pl,
		Analyzer: viz.NewAnalyzer(nil),
	})
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.View()
	return m
}

func (m *Model) zoneByID(t *testing.T, id string) zone {
	t.Helper()
	for _, z := range m.zones {
		if z.id == id {
			return z
		}
	}
	t.Fatalf("zone %q not rendered", id)
	return zone{}
}

func click(m *Model, x, y int) {
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	m.View()
}

func TestMouseControls(t *testing.T) {
	m := testModel(t, 80, 40)

	// Toggle buttons.
	z := m.zoneByID(t, "tog:shuffle")
	click(m, z.r.Min.X, z.r.Min.Y)
	if !m.pl.Shuffle() {
		t.Fatal("shuffle toggle did not turn shuffle on")
	}
	z = m.zoneByID(t, "tog:repeat")
	click(m, z.r.Min.X, z.r.Min.Y)
	if m.pl.Repeat != playlist.RepeatOne {
		t.Fatalf("repeat = %v, want one", m.pl.Repeat)
	}

	// Volume slider: click at the right end.
	z = m.zoneByID(t, "vol")
	click(m, z.r.Max.X-1, z.r.Min.Y)
	if m.volume != 100 {
		t.Fatalf("volume = %v, want 100", m.volume)
	}

	// EQ slider: drag the 1 kHz band to the top.
	z = m.zoneByID(t, "eq:band:6")
	m.Update(tea.MouseClickMsg{X: z.r.Min.X + 1, Y: z.r.Max.Y - 1, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: z.r.Min.X + 1, Y: z.r.Min.Y - 3, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: z.r.Min.X + 1, Y: z.r.Min.Y - 3, Button: tea.MouseLeft})
	if g := m.cfg.EQ.Bands[5]; g != eqMax {
		t.Fatalf("1k gain = %v, want %v", g, eqMax)
	}

	// Playlist: single click selects, double click plays.
	z = m.zoneByID(t, "pl:row:2")
	click(m, z.r.Min.X+3, z.r.Min.Y)
	if m.cursor != 2 || m.pl.Current == 2 {
		t.Fatalf("single click: cursor=%d current=%d", m.cursor, m.pl.Current)
	}
	click(m, z.r.Min.X+3, z.r.Min.Y)
	if m.pl.Current != 2 {
		t.Fatalf("double click did not play the track (current=%d)", m.pl.Current)
	}

	// Theme arrows.
	before := m.themeIdx
	z = m.zoneByID(t, "theme:next")
	click(m, z.r.Min.X, z.r.Min.Y)
	if m.themeIdx != before+1 {
		t.Fatalf("theme index = %d, want %d", m.themeIdx, before+1)
	}

	// Wheel over the main panel changes the volume.
	z = m.zoneByID(t, "time")
	m.Update(tea.MouseWheelMsg{X: z.r.Min.X, Y: z.r.Min.Y, Button: tea.MouseWheelDown})
	if m.volume != 98 {
		t.Fatalf("volume after wheel = %v, want 98", m.volume)
	}
}

func TestLayoutsFit(t *testing.T) {
	for _, size := range [][2]int{{30, 6}, {42, 26}, {80, 40}, {120, 12}, {140, 32}, {200, 60}} {
		m := testModel(t, size[0], size[1])
		for _, eq := range []bool{true, false} {
			for _, pl := range []bool{true, false} {
				m.cfg.ShowEQ, m.cfg.ShowPlaylist = eq, pl
				v := m.View()
				lines := splitLines(v.Content)
				if len(lines) > size[1] {
					t.Errorf("%v eq=%v pl=%v: %d lines, want <= %d", size, eq, pl, len(lines), size[1])
				}
				for i, l := range lines {
					if w := width(l); w > size[0] {
						t.Errorf("%v eq=%v pl=%v: line %d is %d wide", size, eq, pl, i, w)
					}
				}
			}
		}
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
