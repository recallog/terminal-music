package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/recallog/terminal-music/internal/config"
)

func TestEQFilter(t *testing.T) {
	eq := config.EQ{Enabled: true}
	if got := eqFilter(eq); got != "" {
		t.Fatalf("flat EQ = %q, want empty", got)
	}
	eq.Bands[5] = 3
	eq.Preamp = -2
	want := "@eq:lavfi=[volume=volume=-2.0dB,equalizer=f=1000:t=o:w=1:g=3.0]"
	if got := eqFilter(eq); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	eq.Enabled = false
	if got := eqFilter(eq); got != "" {
		t.Fatalf("disabled EQ = %q, want empty", got)
	}
}

func TestParsePasted(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "my song.mp3")
	b := filepath.Join(dir, "it's.flac")
	for _, p := range []string{a, b} {
		_ = os.WriteFile(p, nil, 0o644)
	}
	cases := map[string][]string{
		"'" + a + "' " + `"` + b + `"`:                  {a, b},
		filepath.Join(dir, `my\ song.mp3`):              {a},
		"file://" + filepath.Join(dir, "my%20song.mp3"): {a},
		"http://radio/stream":                           {"http://radio/stream"},
		"not a path":                                    nil,
	}
	for in, want := range cases {
		if got := parsePasted(in); !slices.Equal(got, want) {
			t.Errorf("parsePasted(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestBigTextWidth(t *testing.T) {
	rows := bigText("12:34")
	for _, r := range rows {
		if width(r) != width(rows[0]) {
			t.Fatalf("rows have different widths: %q", rows)
		}
	}
	if width(rows[0]) != 17 {
		t.Fatalf("width = %d, want 17", width(rows[0]))
	}
}
