// Package ui is the Bubble Tea interface of the player.
package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/recallog/terminal-music/internal/config"
	"github.com/recallog/terminal-music/internal/mpv"
	"github.com/recallog/terminal-music/internal/playlist"
	"github.com/recallog/terminal-music/internal/theme"
	"github.com/recallog/terminal-music/internal/viz"
)

type vizMode int

const (
	vizSpectrum vizMode = iota
	vizScope
	vizOff
)

var vizNames = [...]string{"spectrum", "scope", "off"}

type pane int

const (
	panePlaylist pane = iota
	paneEQ
)

type overlay int

const (
	overlayNone overlay = iota
	overlayHelp
	overlayAdd
	overlaySearch
)

// zone is a clickable screen area registered while rendering.
type zone struct {
	id string
	r  image.Rectangle
}

type (
	mpvEventMsg  mpv.Event
	mpvClosedMsg struct{}
	tickMsg      time.Time
	probedMsg    []playlist.Meta
	addedMsg     struct {
		tracks []*playlist.Track
		errs   []error
	}
)

// Options are the dependencies of the UI.
type Options struct {
	Config   config.Config
	Themes   []theme.Theme
	MPV      *mpv.Client
	Playlist *playlist.Playlist
	Analyzer *viz.Analyzer
	Autoplay bool     // start playing the first track right away
	Warnings []string // shown in the status line at startup
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg      config.Config
	themes   []theme.Theme
	themeIdx int
	st       styles
	mpv      *mpv.Client
	pl       *playlist.Playlist
	an       *viz.Analyzer
	autoplay bool

	width, height int
	now           time.Time
	start         time.Time

	// Playback state, as reported by mpv.
	idle        bool
	paused      bool
	pos, dur    float64
	volume      float64
	muted       bool
	bitrate     float64
	samplerate  int
	channels    int
	playingPath string
	meta        map[string]string
	queued      int // playlist index appended to mpv's internal playlist

	// UI state.
	cursor, scroll int
	focus          pane
	eqSel          int // selected EQ slider, 0 is the preamp
	eqDirty        bool
	eqChangedAt    time.Time
	viz            vizMode
	bars, peaks    []float64
	wave           []float64
	probing        bool

	overlay overlay
	input   textinput.Model

	drag        zone // slider being dragged
	seekPreview float64
	lastClickAt time.Time
	lastClickID string

	status      string
	statusUntil time.Time

	// Filled by View, used to interpret mouse events and sizes.
	zones    []zone
	vizCols  int
	listRows int
}

// New creates the model.
func New(o Options) *Model {
	m := &Model{
		cfg:         o.Config,
		themes:      o.Themes,
		mpv:         o.MPV,
		pl:          o.Playlist,
		an:          o.Analyzer,
		autoplay:    o.Autoplay,
		idle:        true,
		volume:      o.Config.Volume,
		muted:       o.Config.Muted,
		queued:      -1,
		seekPreview: -1,
		start:       time.Now(),
		now:         time.Now(),
		meta:        map[string]string{},
	}
	for i, t := range m.themes {
		if strings.EqualFold(t.Name, o.Config.Theme) {
			m.themeIdx = i
		}
	}
	m.st = newStyles(m.themes[m.themeIdx])
	for i, n := range vizNames {
		if n == o.Config.Viz {
			m.viz = vizMode(i)
		}
	}
	m.pl.Repeat = playlist.ParseRepeat(o.Config.Repeat)
	if i := o.Config.LastTrack; i >= 0 && i < m.pl.Len() && !o.Autoplay {
		m.pl.SetCurrent(i)
		m.cursor = i
	}
	m.pl.SetShuffle(o.Config.Shuffle)

	m.input = textinput.New()
	m.input.Prompt = ""
	if len(o.Warnings) > 0 {
		m.setStatus(strings.Join(o.Warnings, " · "), 8*time.Second)
	}
	return m
}

// Config returns the settings to persist.
func (m *Model) Config() config.Config {
	c := m.cfg
	c.Theme = m.themes[m.themeIdx].Name
	c.Volume = m.volume
	c.Muted = m.muted
	c.Shuffle = m.pl.Shuffle()
	c.Repeat = m.pl.Repeat.String()
	c.Viz = vizNames[m.viz]
	c.LastTrack = m.pl.Current
	return c
}

var observed = []string{
	"time-pos", "duration", "pause", "volume", "mute", "idle-active", "path",
	"metadata", "audio-bitrate", "audio-params",
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	for i, p := range observed {
		_ = m.mpv.Observe(i+1, p)
	}
	_ = m.mpv.Set("mute", m.muted)
	m.applyRepeat()
	m.applyEQ()
	cmds := []tea.Cmd{m.waitMPV(), m.tick(), m.probeNext()}
	if m.autoplay && m.pl.Len() > 0 {
		first := 0
		if m.pl.Shuffle() {
			first = m.pl.Next(true)
		}
		m.play(first)
	}
	return tea.Batch(cmds...)
}

func (m *Model) waitMPV() tea.Cmd {
	ch := m.mpv.Events
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return mpvClosedMsg{}
		}
		return mpvEventMsg(ev)
	}
}

func (m *Model) playing() bool { return !m.idle && !m.paused }

func (m *Model) tick() tea.Cmd {
	d := 250 * time.Millisecond
	if m.viz != vizOff && (m.playing() || m.barsActive()) {
		d = 33 * time.Millisecond
	}
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) barsActive() bool {
	for _, p := range m.peaks {
		if p > 0 {
			return true
		}
	}
	return false
}

func (m *Model) onTick(t time.Time) tea.Cmd {
	m.now = t
	switch m.viz {
	case vizSpectrum:
		m.bars, m.peaks = m.an.Update(m.vizCols, m.playing())
	case vizScope:
		if m.playing() {
			m.wave = m.an.Wave(m.vizCols * 2)
		} else {
			m.wave = nil
		}
	}
	if m.eqDirty && t.Sub(m.eqChangedAt) > 150*time.Millisecond {
		m.applyEQ()
	}
	if m.status != "" && t.After(m.statusUntil) {
		m.status = ""
	}
	return m.tick()
}

func (m *Model) setStatus(s string, d time.Duration) {
	m.status = s
	m.statusUntil = time.Now().Add(d)
}

// --- mpv ---------------------------------------------------------------

func (m *Model) onMPV(ev mpv.Event) {
	switch ev.Name {
	case "property-change":
		m.onProperty(ev.Property, ev.Data)
	case "end-file":
		if ev.Reason == "error" {
			name := m.playingPath
			if i := m.pl.IndexOf(name); i >= 0 {
				name = m.pl.Tracks[i].Display()
			}
			m.setStatus(fmt.Sprintf("Cannot play %s: %s", name, ev.Error), 6*time.Second)
		}
	case "command-error":
		m.setStatus("mpv: "+ev.Error, 4*time.Second)
	}
}

func (m *Model) onProperty(name string, data json.RawMessage) {
	var f float64
	var b bool
	var s string
	switch name {
	case "time-pos":
		if json.Unmarshal(data, &f) == nil {
			m.pos = f
		}
	case "duration":
		m.dur = 0
		if json.Unmarshal(data, &f) == nil {
			m.dur = f
			if i := m.pl.Current; i >= 0 && i < m.pl.Len() && f > 0 && !m.pl.Tracks[i].IsURL() {
				m.pl.Tracks[i].Duration = f
			}
		}
	case "pause":
		if json.Unmarshal(data, &b) == nil {
			m.paused = b
		}
	case "volume":
		if json.Unmarshal(data, &f) == nil {
			m.volume = f
		}
	case "mute":
		if json.Unmarshal(data, &b) == nil {
			m.muted = b
		}
	case "idle-active":
		if json.Unmarshal(data, &b) == nil {
			m.idle = b
			if b {
				m.pos, m.dur, m.bitrate, m.samplerate, m.channels = 0, 0, 0, 0, 0
				m.playingPath = ""
				m.queued = -1
			}
		}
	case "path":
		if json.Unmarshal(data, &s) == nil && s != "" {
			m.onTrackStart(s)
		}
	case "metadata":
		raw := map[string]string{}
		_ = json.Unmarshal(data, &raw)
		m.meta = map[string]string{}
		for k, v := range raw {
			m.meta[strings.ToLower(k)] = v
		}
		m.applyStreamMeta()
	case "audio-bitrate":
		m.bitrate = 0
		_ = json.Unmarshal(data, &m.bitrate)
	case "audio-params":
		var p struct {
			Samplerate int `json:"samplerate"`
			Channels   int `json:"channel-count"`
		}
		if json.Unmarshal(data, &p) == nil {
			m.samplerate, m.channels = p.Samplerate, p.Channels
		}
	}
}

// onTrackStart follows mpv when it moves to another file on its own
// (gapless advance to the queued track).
func (m *Model) onTrackStart(path string) {
	m.playingPath = path
	cur := m.pl.Current
	switch {
	case m.queued >= 0 && m.queued < m.pl.Len() && m.pl.Tracks[m.queued].Path == path:
		follow := m.cursor == cur
		m.pl.SetCurrent(m.queued)
		if follow {
			m.cursor = m.pl.Current
			m.ensureVisible()
		}
		m.queueNext()
	case cur >= 0 && cur < m.pl.Len() && m.pl.Tracks[cur].Path == path:
	default:
		if i := m.pl.IndexOf(path); i >= 0 {
			m.pl.SetCurrent(i)
			m.queueNext()
		}
	}
}

// applyStreamMeta uses the live metadata for tracks without tags (streams).
func (m *Model) applyStreamMeta() {
	i := m.pl.Current
	if i < 0 || i >= m.pl.Len() {
		return
	}
	t := m.pl.Tracks[i]
	if title := m.meta["icy-title"]; title != "" {
		if a, b, ok := strings.Cut(title, " - "); ok {
			t.Artist, t.Title = a, b
		} else {
			t.Artist, t.Title = "", title
		}
		return
	}
	if t.Title == "" && m.meta["title"] != "" {
		t.Title = m.meta["title"]
		t.Artist = m.meta["artist"]
	}
}

// --- playback actions ----------------------------------------------------

func (m *Model) play(i int) {
	if i < 0 || i >= m.pl.Len() {
		return
	}
	m.pl.SetCurrent(i)
	t := m.pl.Tracks[i]
	_ = m.mpv.Command("loadfile", t.Path, "replace")
	_ = m.mpv.Set("pause", false)
	m.playingPath = t.Path
	m.idle, m.paused = false, false
	m.pos, m.dur = 0, t.Duration
	m.meta = map[string]string{}
	m.queueNext()
}

// queueNext gives mpv the following track in advance so that it can play
// it gaplessly.
func (m *Model) queueNext() {
	if m.idle && m.playingPath == "" {
		return
	}
	_ = m.mpv.Command("playlist-clear")
	m.queued = -1
	if m.pl.Repeat == playlist.RepeatOne {
		return // handled by mpv's loop-file
	}
	if nx := m.pl.Next(false); nx >= 0 {
		_ = m.mpv.Command("loadfile", m.pl.Tracks[nx].Path, "append")
		m.queued = nx
	}
}

func (m *Model) applyRepeat() {
	loop := "no"
	if m.pl.Repeat == playlist.RepeatOne {
		loop = "inf"
	}
	_ = m.mpv.Set("loop-file", loop)
}

func (m *Model) applyEQ() {
	m.eqDirty = false
	_ = m.mpv.Set("af", eqFilter(m.cfg.EQ))
}

func (m *Model) eqChanged() {
	m.eqDirty = true
	m.eqChangedAt = time.Now()
}

func (m *Model) playPause() {
	switch {
	case m.idle:
		i := m.cursor
		if m.pl.Current >= 0 && m.cursor == m.pl.Current {
			i = m.pl.Current
		}
		m.play(i)
	default:
		_ = m.mpv.Set("pause", !m.paused)
	}
}

func (m *Model) playButton() {
	switch {
	case m.idle:
		m.playPause()
	case m.paused:
		_ = m.mpv.Set("pause", false)
	default:
		m.seekTo(0)
	}
}

func (m *Model) stop() {
	_ = m.mpv.Command("stop")
	m.queued = -1
}

func (m *Model) next() {
	if nx := m.pl.Next(true); nx >= 0 {
		m.followPlay(nx)
	}
}

func (m *Model) prev() {
	if pv := m.pl.Prev(); pv >= 0 {
		m.followPlay(pv)
	}
}

// followPlay plays track i, moving the cursor along if it was on the
// current track.
func (m *Model) followPlay(i int) {
	if m.cursor == m.pl.Current || m.pl.Current < 0 {
		m.cursor = i
		m.ensureVisible()
	}
	m.play(i)
}

func (m *Model) seekBy(d float64) {
	if !m.idle {
		_ = m.mpv.Command("seek", d, "relative")
	}
}

func (m *Model) seekTo(secs float64) {
	if !m.idle {
		_ = m.mpv.Command("seek", secs, "absolute")
		m.pos = secs
	}
}

func (m *Model) setVolume(v float64) {
	m.volume = max(0, min(100, v))
	_ = m.mpv.Set("volume", m.volume)
}

func (m *Model) toggleMute() {
	m.muted = !m.muted
	_ = m.mpv.Set("mute", m.muted)
}

func (m *Model) toggleShuffle() {
	m.pl.SetShuffle(!m.pl.Shuffle())
	m.queueNext()
	m.setStatus("Shuffle "+onOff(m.pl.Shuffle()), 2*time.Second)
}

func (m *Model) cycleRepeat() {
	m.pl.Repeat = (m.pl.Repeat + 1) % 3
	m.applyRepeat()
	m.queueNext()
	m.setStatus("Repeat "+m.pl.Repeat.String(), 2*time.Second)
}

func (m *Model) setTheme(i int) {
	n := len(m.themes)
	m.themeIdx = ((i % n) + n) % n
	m.st = newStyles(m.themes[m.themeIdx])
	m.setStatus("Theme: "+m.themes[m.themeIdx].Name, 2*time.Second)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// --- playlist editing ----------------------------------------------------

func (m *Model) removeAt(i int) {
	if i < 0 || i >= m.pl.Len() {
		return
	}
	m.pl.Remove(i)
	m.cursor = min(m.cursor, m.pl.Len()-1)
	m.cursor = max(m.cursor, 0)
	m.queueNext()
}

func (m *Model) clearPlaylist() {
	m.pl.Clear()
	m.cursor, m.scroll = 0, 0
	m.queueNext()
	m.setStatus("Playlist cleared", 2*time.Second)
}

func (m *Model) moveTrack(delta int) {
	m.cursor = m.pl.Move(m.cursor, delta)
	m.ensureVisible()
	m.queueNext()
}

func (m *Model) addPaths(paths []string) tea.Cmd {
	return func() tea.Msg {
		tracks, errs := playlist.Expand(paths)
		return addedMsg{tracks, errs}
	}
}

func (m *Model) onAdded(msg addedMsg) tea.Cmd {
	wasEmpty := m.pl.Len() == 0
	m.pl.Add(msg.tracks...)
	text := fmt.Sprintf("Added %d track(s)", len(msg.tracks))
	if len(msg.errs) > 0 {
		text += fmt.Sprintf(" · %v", msg.errs[0])
	}
	m.setStatus(text, 4*time.Second)
	if wasEmpty && len(msg.tracks) > 0 && m.idle {
		m.play(0)
	} else {
		m.queueNext()
	}
	return m.probeNext()
}

// probeNext reads tags and durations of a batch of tracks in the background.
func (m *Model) probeNext() tea.Cmd {
	if m.probing {
		return nil
	}
	var paths []string
	for _, t := range m.pl.Tracks {
		if !t.Probed && !t.IsURL() {
			t.Probed = true
			paths = append(paths, t.Path)
			if len(paths) == 16 {
				break
			}
		}
	}
	if len(paths) == 0 {
		return nil
	}
	m.probing = true
	return func() tea.Msg {
		res := make([]playlist.Meta, len(paths))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 4)
		for i, p := range paths {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				res[i] = playlist.Probe(p)
				<-sem
			}()
		}
		wg.Wait()
		return probedMsg(res)
	}
}

func (m *Model) onProbed(res probedMsg) tea.Cmd {
	byPath := make(map[string]playlist.Meta, len(res))
	for _, r := range res {
		byPath[r.Path] = r
	}
	for _, t := range m.pl.Tracks {
		if r, ok := byPath[t.Path]; ok {
			r.Apply(t)
		}
	}
	m.probing = false
	return m.probeNext()
}

func (m *Model) ensureVisible() {
	rows := max(m.listRows, 1)
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+rows {
		m.scroll = m.cursor - rows + 1
	}
	m.clampScroll()
}

func (m *Model) clampScroll() {
	m.scroll = min(m.scroll, m.pl.Len()-max(m.listRows, 1))
	m.scroll = max(m.scroll, 0)
}

func (m *Model) moveCursor(d int) {
	if m.pl.Len() == 0 {
		return
	}
	m.cursor = max(0, min(m.pl.Len()-1, m.cursor+d))
	m.ensureVisible()
}
