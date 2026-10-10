package ui

import (
	"image"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/recallog/terminal-music/internal/mpv"
)

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampScroll()
	case tickMsg:
		return m, m.onTick(time.Time(msg))
	case mpvEventMsg:
		m.onMPV(mpv.Event(msg))
		return m, m.waitMPV()
	case mpvClosedMsg:
		m.setStatus("mpv exited unexpectedly", time.Hour)
		return m, tea.Quit
	case probedMsg:
		return m, m.onProbed(msg)
	case addedMsg:
		return m, m.onAdded(msg)
	case tea.PasteMsg:
		if m.overlay == overlayAdd || m.overlay == overlaySearch {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		// Files dragged onto the terminal arrive as a paste.
		if paths := parsePasted(msg.Content); len(paths) > 0 {
			return m, m.addPaths(paths)
		}
	case tea.KeyPressMsg:
		return m, m.onKey(msg)
	case tea.MouseClickMsg:
		m.onClick(tea.Mouse(msg))
	case tea.MouseMotionMsg:
		m.onDrag(tea.Mouse(msg))
	case tea.MouseReleaseMsg:
		m.onRelease(tea.Mouse(msg))
	case tea.MouseWheelMsg:
		m.onWheel(tea.Mouse(msg))
	}
	return m, nil
}

// --- keyboard ------------------------------------------------------------

func (m *Model) onKey(k tea.KeyPressMsg) tea.Cmd {
	key := k.String()
	switch m.overlay {
	case overlayHelp:
		m.overlay = overlayNone
		return nil
	case overlayAdd, overlaySearch:
		return m.onInputKey(k)
	}

	// Keys shared by every pane.
	switch key {
	case "q", "ctrl+c":
		return tea.Quit
	case "?", "f1":
		m.overlay = overlayHelp
	case "z":
		m.prev()
	case "x":
		m.playButton()
	case "c", "space":
		m.playPause()
	case "v":
		m.stop()
	case "b":
		m.next()
	case "+", "=":
		m.setVolume(m.volume + 5)
	case "-", "_":
		m.setVolume(m.volume - 5)
	case "m":
		m.toggleMute()
	case "s":
		m.toggleShuffle()
	case "r":
		m.cycleRepeat()
	case "e":
		m.cfg.ShowEQ = !m.cfg.ShowEQ
		if !m.cfg.ShowEQ {
			m.focus = panePlaylist
		}
	case "p":
		m.cfg.ShowPlaylist = !m.cfg.ShowPlaylist
		if !m.cfg.ShowPlaylist && m.cfg.ShowEQ {
			m.focus = paneEQ
		}
	case "t":
		m.setTheme(m.themeIdx + 1)
	case "T":
		m.setTheme(m.themeIdx - 1)
	case "f":
		m.viz = (m.viz + 1) % 3
	case "ctrl+r":
		m.cfg.TimeRemaining = !m.cfg.TimeRemaining
	case "tab", "shift+tab":
		if m.cfg.ShowEQ && m.cfg.ShowPlaylist {
			m.focus = 1 - m.focus
		}
	case "a", "o":
		return m.openInput(overlayAdd, "")
	case "/":
		return m.openInput(overlaySearch, "")
	case "shift+left", "<":
		m.seekBy(-30)
	case "shift+right", ">":
		m.seekBy(30)
	default:
		if m.focus == paneEQ && m.cfg.ShowEQ {
			m.onEQKey(key)
		} else {
			m.onPlaylistKey(key)
		}
	}
	return nil
}

func (m *Model) onPlaylistKey(key string) {
	page := max(m.listRows-1, 1)
	switch key {
	case "left", "h":
		m.seekBy(-5)
	case "right", "l":
		m.seekBy(5)
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "pgup", "ctrl+b":
		m.moveCursor(-page)
	case "pgdown", "ctrl+f":
		m.moveCursor(page)
	case "home", "g":
		m.moveCursor(-m.pl.Len())
	case "end", "G":
		m.moveCursor(m.pl.Len())
	case "enter":
		m.play(m.cursor)
	case "K", "shift+up":
		m.moveTrack(-1)
	case "J", "shift+down":
		m.moveTrack(1)
	case "delete", "d":
		m.removeAt(m.cursor)
	case "D":
		m.clearPlaylist()
	case ".":
		if m.pl.Current >= 0 {
			m.cursor = m.pl.Current
			m.ensureVisible()
		}
	}
}

func (m *Model) onEQKey(key string) {
	eq := &m.cfg.EQ
	switch key {
	case "left", "h":
		m.eqSel = max(0, m.eqSel-1)
	case "right", "l":
		m.eqSel = min(10, m.eqSel+1)
	case "up", "k":
		setEQGain(eq, m.eqSel, eqGain(eq, m.eqSel)+1)
		m.eqChanged()
	case "down", "j":
		setEQGain(eq, m.eqSel, eqGain(eq, m.eqSel)-1)
		m.eqChanged()
	case "0":
		setEQGain(eq, m.eqSel, 0)
		m.eqChanged()
	case "enter":
		m.toggleEQ()
	case "[":
		m.cyclePreset(-1)
	case "]":
		m.cyclePreset(1)
	case "R":
		m.resetEQ()
	}
}

func (m *Model) toggleEQ() {
	m.cfg.EQ.Enabled = !m.cfg.EQ.Enabled
	m.applyEQ()
	m.setStatus("Equalizer "+onOff(m.cfg.EQ.Enabled), 2*time.Second)
}

func (m *Model) resetEQ() {
	m.cfg.EQ.Preamp = 0
	applyPreset(&m.cfg.EQ, eqPresets[0])
	m.applyEQ()
}

func (m *Model) cyclePreset(d int) {
	n := len(eqPresets)
	i := presetIndex(m.cfg.EQ.Preset)
	if i < 0 && d < 0 {
		i = 0
	}
	i = ((i+d)%n + n) % n
	applyPreset(&m.cfg.EQ, eqPresets[i])
	m.applyEQ()
	m.setStatus("EQ preset: "+eqPresets[i].name, 2*time.Second)
}

// --- text input (add path, search) --------------------------------------

func (m *Model) openInput(kind overlay, value string) tea.Cmd {
	m.overlay = kind
	m.input.SetValue(value)
	m.input.CursorEnd()
	switch kind {
	case overlayAdd:
		m.input.Placeholder = "file, folder, playlist or URL (tab completes)"
		if value == "" {
			if home, err := os.UserHomeDir(); err == nil {
				m.input.SetValue(home + "/")
				m.input.CursorEnd()
			}
		}
	case overlaySearch:
		m.input.Placeholder = "type to find a track"
	}
	return m.input.Focus()
}

func (m *Model) closeInput() {
	m.overlay = overlayNone
	m.input.Blur()
}

func (m *Model) onInputKey(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "esc", "ctrl+c":
		m.closeInput()
		return nil
	case "enter":
		val := strings.TrimSpace(m.input.Value())
		kind := m.overlay
		m.closeInput()
		if kind == overlayAdd && val != "" {
			return m.addPaths([]string{val})
		}
		if kind == overlaySearch && val != "" {
			m.play(m.cursor)
		}
		return nil
	case "tab":
		if m.overlay == overlayAdd {
			m.completePath()
			return nil
		}
	case "down", "ctrl+n":
		if m.overlay == overlaySearch {
			m.search(m.cursor + 1)
			return nil
		}
	case "up", "ctrl+p":
		if m.overlay == overlaySearch {
			m.searchBack(m.cursor - 1)
			return nil
		}
	}
	before := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(k)
	if m.overlay == overlaySearch && m.input.Value() != before {
		m.search(0)
	}
	return cmd
}

func (m *Model) matches(i int, q string) bool {
	t := m.pl.Tracks[i]
	return strings.Contains(strings.ToLower(t.Display()+" "+t.Album+" "+t.Path), q)
}

// search moves the cursor to the first match at or after index from.
func (m *Model) search(from int) {
	q := strings.ToLower(strings.TrimSpace(m.input.Value()))
	n := m.pl.Len()
	if q == "" || n == 0 {
		return
	}
	for k := 0; k < n; k++ {
		i := (from + k) % n
		if m.matches(i, q) {
			m.cursor = i
			m.ensureVisible()
			return
		}
	}
}

func (m *Model) searchBack(from int) {
	q := strings.ToLower(strings.TrimSpace(m.input.Value()))
	n := m.pl.Len()
	if q == "" || n == 0 {
		return
	}
	for k := 0; k < n; k++ {
		i := ((from-k)%n + n) % n
		if m.matches(i, q) {
			m.cursor = i
			m.ensureVisible()
			return
		}
	}
}

// completePath completes the path being typed, shell style.
func (m *Model) completePath() {
	val := m.input.Value()
	p := val
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = home + p[1:]
		}
	}
	matches, _ := filepath.Glob(globEscape(p) + "*")
	if len(matches) == 0 {
		return
	}
	common := matches[0]
	for _, mt := range matches[1:] {
		for !strings.HasPrefix(mt, common) {
			common = common[:len(common)-1]
		}
	}
	if len(matches) == 1 {
		if info, err := os.Stat(common); err == nil && info.IsDir() {
			common += "/"
		}
	} else {
		names := make([]string, 0, len(matches))
		for _, mt := range matches {
			names = append(names, filepath.Base(mt))
		}
		m.setStatus(strings.Join(names, "  "), 4*time.Second)
	}
	if len(common) >= len(p) {
		m.input.SetValue(common)
		m.input.CursorEnd()
	}
}

func globEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`)
	return r.Replace(s)
}

// parsePasted extracts file paths from a paste (terminals paste dropped
// files as quoted or backslash-escaped paths, or file:// URIs).
func parsePasted(s string) []string {
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "file://") {
			if u, err := url.Parse(p); err == nil {
				p = u.Path
			}
		}
		if p == "" {
			return
		}
		if strings.Contains(p, "://") {
			out = append(out, p)
			return
		}
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	var cur strings.Builder
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\n' || r == '\r' || r == '\t':
			add(cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	add(cur.String())
	return out
}

// --- mouse ---------------------------------------------------------------

func (m *Model) hit(x, y int) zone {
	pt := image.Pt(x, y)
	for i := len(m.zones) - 1; i >= 0; i-- {
		if pt.In(m.zones[i].r) {
			return m.zones[i]
		}
	}
	return zone{}
}

// frac returns the horizontal position of x inside r, in [0, 1].
func frac(r image.Rectangle, x int) float64 {
	if r.Dx() <= 1 {
		return 0
	}
	return max(0, min(1, float64(x-r.Min.X)/float64(r.Dx()-1)))
}

func (m *Model) onClick(ms tea.Mouse) {
	if m.overlay == overlayHelp {
		m.overlay = overlayNone
		return
	}
	if ms.Button != tea.MouseLeft {
		return
	}
	z := m.hit(ms.X, ms.Y)
	double := z.id != "" && z.id == m.lastClickID && time.Since(m.lastClickAt) < 400*time.Millisecond
	m.lastClickAt, m.lastClickID = time.Now(), z.id

	if strings.HasPrefix(z.id, "pl") {
		m.focus = panePlaylist
	}
	if strings.HasPrefix(z.id, "eq") {
		m.focus = paneEQ
	}

	switch {
	case z.id == "seek", z.id == "vol", z.id == "pl:scroll", strings.HasPrefix(z.id, "eq:band:"):
		m.drag = z
		m.onDrag(ms)
	case strings.HasPrefix(z.id, "pl:row:"):
		i, _ := strconv.Atoi(strings.TrimPrefix(z.id, "pl:row:"))
		m.cursor = i
		if double {
			m.play(i)
			m.lastClickID = ""
		}
	case z.id == "time":
		m.cfg.TimeRemaining = !m.cfg.TimeRemaining
	case z.id == "viz":
		m.viz = (m.viz + 1) % 3
	case z.id == "btn:prev":
		m.prev()
	case z.id == "btn:play":
		m.playButton()
	case z.id == "btn:pause":
		if !m.idle {
			_ = m.mpv.Set("pause", !m.paused)
		}
	case z.id == "btn:stop":
		m.stop()
	case z.id == "btn:next":
		m.next()
	case z.id == "btn:eject", z.id == "pl:add":
		m.openInput(overlayAdd, "")
	case z.id == "btn:mute":
		m.toggleMute()
	case z.id == "tog:shuffle":
		m.toggleShuffle()
	case z.id == "tog:repeat":
		m.cycleRepeat()
	case z.id == "tog:eq":
		m.cfg.ShowEQ = !m.cfg.ShowEQ
	case z.id == "tog:pl":
		m.cfg.ShowPlaylist = !m.cfg.ShowPlaylist
	case z.id == "theme:prev":
		m.setTheme(m.themeIdx - 1)
	case z.id == "theme:next":
		m.setTheme(m.themeIdx + 1)
	case z.id == "eq:on":
		m.toggleEQ()
	case z.id == "eq:reset":
		m.resetEQ()
	case z.id == "eq:preset-":
		m.cyclePreset(-1)
	case z.id == "eq:preset+":
		m.cyclePreset(1)
	case z.id == "pl:del":
		m.removeAt(m.cursor)
	case z.id == "pl:clear":
		m.clearPlaylist()
	case z.id == "pl:find":
		m.openInput(overlaySearch, "")
	case z.id == "help":
		m.overlay = overlayHelp
	}
}

func (m *Model) onDrag(ms tea.Mouse) {
	z := m.drag
	switch {
	case z.id == "":
	case z.id == "seek":
		m.seekPreview = frac(z.r, ms.X)
	case z.id == "vol":
		m.setVolume(100 * frac(z.r, ms.X))
	case z.id == "pl:scroll":
		h := max(z.r.Dy()-1, 1)
		f := max(0, min(1, float64(ms.Y-z.r.Min.Y)/float64(h)))
		m.scroll = int(f*float64(m.pl.Len()-m.listRows) + 0.5)
		m.clampScroll()
	case strings.HasPrefix(z.id, "eq:band:"):
		i, _ := strconv.Atoi(strings.TrimPrefix(z.id, "eq:band:"))
		m.eqSel = i
		h := max(z.r.Dy()-1, 1)
		f := max(0, min(1, float64(ms.Y-z.r.Min.Y)/float64(h)))
		setEQGain(&m.cfg.EQ, i, eqMax-f*2*eqMax)
		m.eqChanged()
	}
}

func (m *Model) onRelease(ms tea.Mouse) {
	if m.drag.id == "seek" && m.seekPreview >= 0 && m.dur > 0 {
		m.seekTo(m.seekPreview * m.dur)
	}
	m.drag = zone{}
	m.seekPreview = -1
}

func (m *Model) onWheel(ms tea.Mouse) {
	up := ms.Button == tea.MouseWheelUp
	if ms.Button != tea.MouseWheelUp && ms.Button != tea.MouseWheelDown {
		return
	}
	d := 1.0
	if !up {
		d = -1
	}
	z := m.hit(ms.X, ms.Y)
	switch {
	case z.id == "seek":
		m.seekBy(5 * d)
	case strings.HasPrefix(z.id, "pl:row:"), z.id == "pl:scroll", z.id == "pl:list":
		m.scroll -= 3 * int(d)
		m.clampScroll()
	case strings.HasPrefix(z.id, "eq:band:"):
		i, _ := strconv.Atoi(strings.TrimPrefix(z.id, "eq:band:"))
		m.eqSel = i
		setEQGain(&m.cfg.EQ, i, eqGain(&m.cfg.EQ, i)+d)
		m.eqChanged()
	case strings.HasPrefix(z.id, "main"), z.id == "vol", z.id == "time", z.id == "viz":
		m.setVolume(m.volume + 2*d)
	}
}
