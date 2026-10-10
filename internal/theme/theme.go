// Package theme defines the colour themes, built-in and user supplied.
package theme

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Theme is a colour scheme. Colours are "#rrggbb" hex strings or ANSI colour
// numbers ("0"-"255"). An empty Background keeps the terminal's own.
type Theme struct {
	Name        string `json:"name"`
	Background  string `json:"background"`
	Foreground  string `json:"foreground"`
	Muted       string `json:"muted"`       // secondary text, inactive buttons
	Border      string `json:"border"`      // panel borders
	BorderFocus string `json:"borderFocus"` // border of the focused panel
	Title       string `json:"title"`       // panel titles
	Display     string `json:"display"`     // time digits, song title (the "LCD")
	Accent      string `json:"accent"`      // active buttons, sliders
	Playing     string `json:"playing"`     // current track in the playlist
	SelectionBg string `json:"selectionBg"` // playlist cursor
	SelectionFg string `json:"selectionFg"`
	BarLow      string `json:"barLow"`  // visualizer gradient, bottom
	BarMid      string `json:"barMid"`  // visualizer gradient, middle
	BarHigh     string `json:"barHigh"` // visualizer gradient, top
	Peak        string `json:"peak"`    // visualizer peaks
}

// Builtins returns the built-in themes.
func Builtins() []Theme {
	return []Theme{
		{
			Name: "winamp", Background: "#0f0f18", Foreground: "#00e000",
			Muted: "#4d6b4d", Border: "#5a5a78", BorderFocus: "#c6c6e0",
			Title: "#d8d8ec", Display: "#00ff00", Accent: "#f0d000",
			Playing: "#ffffff", SelectionBg: "#0000c6", SelectionFg: "#ffffff",
			BarLow: "#18b818", BarMid: "#d8d800", BarHigh: "#e01818", Peak: "#b8b8b8",
		},
		{
			Name: "terminal", Foreground: "7", Muted: "8", Border: "8",
			BorderFocus: "4", Title: "12", Display: "10", Accent: "11",
			Playing: "15", SelectionBg: "4", SelectionFg: "15",
			BarLow: "2", BarMid: "3", BarHigh: "1", Peak: "7",
		},
		{
			Name: "dracula", Background: "#282a36", Foreground: "#f8f8f2",
			Muted: "#6272a4", Border: "#44475a", BorderFocus: "#bd93f9",
			Title: "#ff79c6", Display: "#50fa7b", Accent: "#ffb86c",
			Playing: "#8be9fd", SelectionBg: "#44475a", SelectionFg: "#f8f8f2",
			BarLow: "#bd93f9", BarMid: "#ff79c6", BarHigh: "#ff5555", Peak: "#f1fa8c",
		},
		{
			Name: "nord", Background: "#2e3440", Foreground: "#d8dee9",
			Muted: "#616e88", Border: "#4c566a", BorderFocus: "#88c0d0",
			Title: "#88c0d0", Display: "#a3be8c", Accent: "#ebcb8b",
			Playing: "#8fbcbb", SelectionBg: "#434c5e", SelectionFg: "#eceff4",
			BarLow: "#5e81ac", BarMid: "#81a1c1", BarHigh: "#88c0d0", Peak: "#eceff4",
		},
		{
			Name: "gruvbox", Background: "#282828", Foreground: "#ebdbb2",
			Muted: "#928374", Border: "#504945", BorderFocus: "#d79921",
			Title: "#fabd2f", Display: "#b8bb26", Accent: "#fe8019",
			Playing: "#83a598", SelectionBg: "#504945", SelectionFg: "#fbf1c7",
			BarLow: "#98971a", BarMid: "#d79921", BarHigh: "#cc241d", Peak: "#ebdbb2",
		},
		{
			Name: "catppuccin", Background: "#1e1e2e", Foreground: "#cdd6f4",
			Muted: "#6c7086", Border: "#45475a", BorderFocus: "#cba6f7",
			Title: "#cba6f7", Display: "#a6e3a1", Accent: "#f9e2af",
			Playing: "#89dceb", SelectionBg: "#313244", SelectionFg: "#f5e0dc",
			BarLow: "#89b4fa", BarMid: "#cba6f7", BarHigh: "#f38ba8", Peak: "#f5e0dc",
		},
		{
			Name: "tokyo-night", Background: "#1a1b26", Foreground: "#c0caf5",
			Muted: "#565f89", Border: "#3b4261", BorderFocus: "#7aa2f7",
			Title: "#7aa2f7", Display: "#9ece6a", Accent: "#e0af68",
			Playing: "#7dcfff", SelectionBg: "#283457", SelectionFg: "#c0caf5",
			BarLow: "#2ac3de", BarMid: "#bb9af7", BarHigh: "#f7768e", Peak: "#c0caf5",
		},
		{
			Name: "synthwave", Background: "#1b1029", Foreground: "#f0e4fc",
			Muted: "#7a5c99", Border: "#4a2f6b", BorderFocus: "#ff4fd8",
			Title: "#ff4fd8", Display: "#36f9f6", Accent: "#fede5d",
			Playing: "#36f9f6", SelectionBg: "#3d2263", SelectionFg: "#ffffff",
			BarLow: "#36f9f6", BarMid: "#ff4fd8", BarHigh: "#fede5d", Peak: "#ffffff",
		},
		{
			Name: "amber", Background: "#140d00", Foreground: "#ffb000",
			Muted: "#805800", Border: "#5c3f00", BorderFocus: "#ffcc00",
			Title: "#ffcc00", Display: "#ffb000", Accent: "#ffe080",
			Playing: "#fff0c0", SelectionBg: "#5c3f00", SelectionFg: "#ffe080",
			BarLow: "#a06e00", BarMid: "#ffb000", BarHigh: "#ffe080", Peak: "#fff0c0",
		},
		{
			Name: "solarized-light", Background: "#fdf6e3", Foreground: "#586e75",
			Muted: "#93a1a1", Border: "#cfc8b4", BorderFocus: "#268bd2",
			Title: "#268bd2", Display: "#859900", Accent: "#cb4b16",
			Playing: "#d33682", SelectionBg: "#eee8d5", SelectionFg: "#073642",
			BarLow: "#2aa198", BarMid: "#b58900", BarHigh: "#dc322f", Peak: "#073642",
		},
	}
}

// LoadDir reads user themes (*.json) from dir. A user theme only needs the
// colours it changes: missing ones are taken from the "winamp" theme, or
// from the theme named in its "base" field.
func LoadDir(dir string, builtins []Theme) ([]Theme, []error) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	slices.Sort(files)
	var themes []Theme
	var errs []error
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		var hdr struct {
			Base string `json:"base"`
		}
		_ = json.Unmarshal(b, &hdr)
		t := builtins[0]
		for _, bt := range builtins {
			if strings.EqualFold(bt.Name, hdr.Base) {
				t = bt
			}
		}
		t.Name = strings.TrimSuffix(filepath.Base(f), ".json")
		if err := json.Unmarshal(b, &t); err != nil {
			errs = append(errs, err)
			continue
		}
		themes = append(themes, t)
	}
	return themes, errs
}
