// Package config loads and saves the persistent settings.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// EQ holds the equalizer settings. Gains are in dB, in [-12, 12].
type EQ struct {
	Enabled bool        `json:"enabled"`
	Preamp  float64     `json:"preamp"`
	Bands   [10]float64 `json:"bands"`
	Preset  string      `json:"preset"`
}

// Config is everything remembered between runs.
type Config struct {
	Theme         string  `json:"theme"`
	Volume        float64 `json:"volume"`
	Muted         bool    `json:"muted"`
	Shuffle       bool    `json:"shuffle"`
	Repeat        string  `json:"repeat"` // off, all, one
	ShowEQ        bool    `json:"showEQ"`
	ShowPlaylist  bool    `json:"showPlaylist"`
	Viz           string  `json:"viz"` // spectrum, scope, off
	TimeRemaining bool    `json:"timeRemaining"`
	LastTrack     int     `json:"lastTrack"`
	EQ            EQ      `json:"eq"`
}

// Default returns the settings used on first run.
func Default() Config {
	return Config{
		Theme:        "winamp",
		Volume:       70,
		Repeat:       "all",
		ShowEQ:       true,
		ShowPlaylist: true,
		Viz:          "spectrum",
		LastTrack:    -1,
		EQ:           EQ{Enabled: true, Preset: "Flat"},
	}
}

// Dir returns the configuration directory (~/.config/terminal-music).
func Dir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "terminal-music")
}

// PlaylistPath is where the playlist is saved between runs.
func PlaylistPath() string { return filepath.Join(Dir(), "playlist.m3u") }

// ThemesDir holds user themes.
func ThemesDir() string { return filepath.Join(Dir(), "themes") }

// Load reads the config file, returning defaults when it does not exist.
func Load() (Config, error) {
	c := Default()
	b, err := os.ReadFile(filepath.Join(Dir(), "config.json"))
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

// Save writes the config file.
func Save(c Config) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(Dir(), "config.json")
	if err := os.WriteFile(path+".tmp", append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
