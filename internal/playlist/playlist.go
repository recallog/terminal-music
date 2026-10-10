// Package playlist holds the track list, play order and playlist file I/O.
package playlist

import (
	"math/rand/v2"
	"path/filepath"
	"strings"
)

// Track is one entry of the playlist.
type Track struct {
	Path     string
	Title    string
	Artist   string
	Album    string
	Duration float64 // seconds, 0 when unknown
	Probed   bool    // metadata lookup already attempted
}

// IsURL reports whether the track is a network stream.
func (t *Track) IsURL() bool { return strings.Contains(t.Path, "://") }

// Display returns "Artist - Title", falling back to the file name.
func (t *Track) Display() string {
	switch {
	case t.Title != "" && t.Artist != "":
		return t.Artist + " - " + t.Title
	case t.Title != "":
		return t.Title
	case t.IsURL():
		return t.Path
	}
	base := filepath.Base(t.Path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// Repeat is the repeat mode.
type Repeat int

const (
	RepeatOff Repeat = iota
	RepeatAll
	RepeatOne
)

func (r Repeat) String() string {
	return [...]string{"off", "all", "one"}[r]
}

// ParseRepeat is the inverse of Repeat.String.
func ParseRepeat(s string) Repeat {
	switch s {
	case "all":
		return RepeatAll
	case "one":
		return RepeatOne
	}
	return RepeatOff
}

// Playlist is an ordered list of tracks with a current position.
type Playlist struct {
	Tracks  []*Track
	Current int // index of the current track, -1 when none
	Repeat  Repeat
	shuffle bool
	order   []int // shuffled play order, valid when shuffle is on
	// resume is where playback continues when the current track was removed.
	resume int
}

// New returns an empty playlist.
func New() *Playlist { return &Playlist{Current: -1, resume: -1} }

// Len returns the number of tracks.
func (p *Playlist) Len() int { return len(p.Tracks) }

// Shuffle reports whether shuffle is on.
func (p *Playlist) Shuffle() bool { return p.shuffle }

// SetShuffle turns shuffle on or off.
func (p *Playlist) SetShuffle(on bool) {
	p.shuffle = on
	p.reshuffle()
}

// reshuffle builds a new random order that starts with the current track.
func (p *Playlist) reshuffle() {
	if !p.shuffle {
		p.order = nil
		return
	}
	p.order = rand.Perm(len(p.Tracks))
	if p.Current >= 0 {
		for i, v := range p.order {
			if v == p.Current {
				p.order[0], p.order[i] = p.order[i], p.order[0]
				break
			}
		}
	}
}

// Add appends tracks.
func (p *Playlist) Add(tracks ...*Track) {
	p.Tracks = append(p.Tracks, tracks...)
	if p.shuffle {
		// Insert the new tracks at random places after the current position.
		start := 0
		if pos := p.orderPos(p.Current); pos >= 0 {
			start = pos + 1
		}
		for i := len(p.Tracks) - len(tracks); i < len(p.Tracks); i++ {
			at := start + rand.IntN(len(p.order)-start+1)
			p.order = append(p.order[:at], append([]int{i}, p.order[at:]...)...)
		}
	}
}

// Remove deletes the track at index i.
func (p *Playlist) Remove(i int) {
	if i < 0 || i >= len(p.Tracks) {
		return
	}
	p.Tracks = append(p.Tracks[:i], p.Tracks[i+1:]...)
	switch {
	case i == p.Current:
		p.Current = -1
		p.resume = i
	case i < p.Current:
		p.Current--
	}
	if p.shuffle {
		order := p.order[:0]
		for _, v := range p.order {
			switch {
			case v == i:
				continue
			case v > i:
				v--
			}
			order = append(order, v)
		}
		p.order = order
	}
}

// Move swaps track i with its neighbour at i+delta (delta is -1 or 1).
// It returns the new index of the moved track.
func (p *Playlist) Move(i, delta int) int {
	j := i + delta
	if i < 0 || j < 0 || i >= len(p.Tracks) || j >= len(p.Tracks) {
		return i
	}
	p.Tracks[i], p.Tracks[j] = p.Tracks[j], p.Tracks[i]
	switch p.Current {
	case i:
		p.Current = j
	case j:
		p.Current = i
	}
	for k, v := range p.order {
		switch v {
		case i:
			p.order[k] = j
		case j:
			p.order[k] = i
		}
	}
	return j
}

// Clear removes every track.
func (p *Playlist) Clear() {
	p.Tracks = nil
	p.Current = -1
	p.resume = -1
	p.order = nil
}

// SetCurrent marks track i as the current one.
func (p *Playlist) SetCurrent(i int) {
	p.Current = i
	p.resume = -1
}

func (p *Playlist) orderPos(idx int) int {
	for k, v := range p.order {
		if v == idx {
			return k
		}
	}
	return -1
}

// Next returns the track that follows the current one, or -1. When manual is
// true (the user pressed "next"), repeat-one behaves like repeat-all.
func (p *Playlist) Next(manual bool) int {
	n := len(p.Tracks)
	if n == 0 {
		return -1
	}
	if p.Current < 0 {
		if p.resume >= 0 && p.resume < n {
			return p.resume
		}
		if p.shuffle && len(p.order) > 0 {
			return p.order[0]
		}
		return 0
	}
	if p.Repeat == RepeatOne && !manual {
		return p.Current
	}
	wrap := p.Repeat != RepeatOff
	if p.shuffle {
		pos := p.orderPos(p.Current)
		if pos+1 < len(p.order) {
			return p.order[pos+1]
		}
		if wrap {
			p.reshuffle()
			if len(p.order) > 1 {
				return p.order[1]
			}
			return p.order[0]
		}
		return -1
	}
	if p.Current+1 < n {
		return p.Current + 1
	}
	if wrap {
		return 0
	}
	return -1
}

// Prev returns the track before the current one, or -1.
func (p *Playlist) Prev() int {
	n := len(p.Tracks)
	if n == 0 {
		return -1
	}
	if p.Current < 0 {
		return 0
	}
	if p.shuffle {
		if pos := p.orderPos(p.Current); pos > 0 {
			return p.order[pos-1]
		}
		return p.Current
	}
	if p.Current > 0 {
		return p.Current - 1
	}
	if p.Repeat != RepeatOff {
		return n - 1
	}
	return 0
}

// IndexOf returns the index of the first track with the given path, or -1.
func (p *Playlist) IndexOf(path string) int {
	for i, t := range p.Tracks {
		if t.Path == path {
			return i
		}
	}
	return -1
}

// TotalDuration returns the sum of the known durations and whether some
// durations are still unknown.
func (p *Playlist) TotalDuration() (total float64, partial bool) {
	for _, t := range p.Tracks {
		if t.Duration > 0 {
			total += t.Duration
		} else {
			partial = true
		}
	}
	return total, partial
}
