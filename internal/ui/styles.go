package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/recallog/terminal-music/internal/theme"
)

// styles are the lipgloss styles derived from a theme.
type styles struct {
	bg, fg color.Color

	text, muted, title, display, accent, playing lipgloss.Style
	border, borderFocus                          lipgloss.Style
	selection                                    lipgloss.Style
	button, buttonOn, buttonHot                  lipgloss.Style
	barLow, barMid, barHigh, peak                lipgloss.Style
}

func col(s string) color.Color {
	if s == "" {
		return nil
	}
	return lipgloss.Color(s)
}

func fgStyle(s string) lipgloss.Style {
	st := lipgloss.NewStyle()
	if c := col(s); c != nil {
		st = st.Foreground(c)
	}
	return st
}

func newStyles(t theme.Theme) styles {
	s := styles{
		bg:          col(t.Background),
		fg:          col(t.Foreground),
		text:        fgStyle(t.Foreground),
		muted:       fgStyle(t.Muted),
		title:       fgStyle(t.Title).Bold(true),
		display:     fgStyle(t.Display),
		accent:      fgStyle(t.Accent),
		playing:     fgStyle(t.Playing).Bold(true),
		border:      fgStyle(t.Border),
		borderFocus: fgStyle(t.BorderFocus),
		button:      fgStyle(t.Foreground),
		buttonOn:    fgStyle(t.Accent).Bold(true),
		barLow:      fgStyle(t.BarLow),
		barMid:      fgStyle(t.BarMid),
		barHigh:     fgStyle(t.BarHigh),
		peak:        fgStyle(t.Peak),
	}
	s.selection = fgStyle(t.SelectionFg)
	if c := col(t.SelectionBg); c != nil {
		s.selection = s.selection.Background(c)
	}
	s.buttonHot = s.selection.Bold(true)
	return s
}

// bar returns the gradient style for a cell at height fraction f in [0, 1].
func (s styles) bar(f float64) lipgloss.Style {
	switch {
	case f < 0.45:
		return s.barLow
	case f < 0.8:
		return s.barMid
	}
	return s.barHigh
}
