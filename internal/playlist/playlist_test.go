package playlist

import (
	"os"
	"path/filepath"
	"testing"
)

func list(n int) *Playlist {
	p := New()
	for i := 0; i < n; i++ {
		p.Add(&Track{Path: string(rune('a' + i))})
	}
	return p
}

func TestNextPrev(t *testing.T) {
	p := list(3)
	if got := p.Next(false); got != 0 {
		t.Fatalf("next with no current = %d, want 0", got)
	}
	p.SetCurrent(2)
	if got := p.Next(false); got != -1 {
		t.Fatalf("next at end, repeat off = %d, want -1", got)
	}
	p.Repeat = RepeatAll
	if got := p.Next(false); got != 0 {
		t.Fatalf("next at end, repeat all = %d, want 0", got)
	}
	p.Repeat = RepeatOne
	if got := p.Next(false); got != 2 {
		t.Fatalf("auto next, repeat one = %d, want 2", got)
	}
	if got := p.Next(true); got != 0 {
		t.Fatalf("manual next, repeat one = %d, want 0", got)
	}
	p.SetCurrent(0)
	if got := p.Prev(); got != 2 {
		t.Fatalf("prev at start, repeat = %d, want 2", got)
	}
}

func TestShuffleVisitsAll(t *testing.T) {
	p := list(10)
	p.SetCurrent(4)
	p.SetShuffle(true)
	seen := map[int]bool{4: true}
	for i := 0; i < 9; i++ {
		n := p.Next(false)
		if n < 0 || seen[n] {
			t.Fatalf("step %d: got %d (seen %v)", i, n, seen)
		}
		seen[n] = true
		p.SetCurrent(n)
	}
	if n := p.Next(false); n != -1 {
		t.Fatalf("after all tracks, repeat off: got %d, want -1", n)
	}
}

func TestRemoveCurrent(t *testing.T) {
	p := list(4)
	p.SetCurrent(1)
	p.Remove(1)
	if p.Current != -1 {
		t.Fatalf("current = %d, want -1", p.Current)
	}
	if got := p.Next(false); got != 1 || p.Tracks[got].Path != "c" {
		t.Fatalf("next after removing current = %d, want the following track", got)
	}
	p.SetCurrent(2)
	p.Remove(0)
	if p.Current != 1 {
		t.Fatalf("current after removing before it = %d, want 1", p.Current)
	}
}

func TestMove(t *testing.T) {
	p := list(3)
	p.SetCurrent(0)
	if j := p.Move(0, 1); j != 1 || p.Current != 1 || p.Tracks[0].Path != "b" {
		t.Fatalf("move: j=%d current=%d first=%s", j, p.Current, p.Tracks[0].Path)
	}
}

func TestM3URoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "list.m3u")
	in := []*Track{
		{Path: filepath.Join(dir, "x.mp3"), Artist: "Art", Title: "Song", Duration: 61},
		{Path: "http://radio.example/stream"},
	}
	if err := WriteM3U(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := ReadPlaylist(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].Path != in[0].Path || out[0].Artist != "Art" ||
		out[0].Title != "Song" || out[0].Duration != 61 || out[1].Path != in[1].Path {
		t.Fatalf("round trip mismatch: %+v %+v", *out[0], *out[1])
	}
}

func TestExpandDirAndPLS(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"b.flac", "a.mp3", "cover.jpg", ".hidden/c.mp3"} {
		p := filepath.Join(dir, n)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, nil, 0o644)
	}
	pls := filepath.Join(dir, "radio.pls")
	_ = os.WriteFile(pls, []byte("[playlist]\nFile1=http://x/stream\nTitle1=Radio X\nNumberOfEntries=1\n"), 0o644)

	tracks, errs := Expand([]string{dir})
	if len(errs) > 0 || len(tracks) != 2 || filepath.Base(tracks[0].Path) != "a.mp3" {
		t.Fatalf("dir expand: %v %v", tracks, errs)
	}
	tracks, _ = Expand([]string{pls})
	if len(tracks) != 1 || tracks[0].Title != "Radio X" || tracks[0].Path != "http://x/stream" {
		t.Fatalf("pls: %+v", tracks)
	}
}
