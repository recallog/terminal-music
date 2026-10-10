package ui

import (
	"fmt"
	"image"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	mainHeight = 10 // main panel, borders included
	eqHeight   = 11 // equalizer with 7 slider rows
	eqCompact  = 9  // equalizer with 5 slider rows
	minList    = 5  // smallest useful playlist panel
	leftMin    = 50 // left column width in the side-by-side layout
	leftMax    = 72
)

// line builds one row of a panel while registering clickable zones at their
// absolute screen position.
type line struct {
	m    *Model
	x, y int
	b    strings.Builder
	w    int
}

func (m *Model) newLine(x, y int) *line { return &line{m: m, x: x, y: y} }

func (l *line) add(s string) *line {
	l.b.WriteString(s)
	l.w += width(s)
	return l
}

func (l *line) zone(id, s string) *line {
	l.m.addZone(id, l.x+l.w, l.y, width(s), 1)
	return l.add(s)
}

func (l *line) pad(w int) *line {
	if w > l.w {
		l.add(strings.Repeat(" ", w-l.w))
	}
	return l
}

func (l *line) String() string { return l.b.String() }

func (m *Model) addZone(id string, x, y, w, h int) {
	if w > 0 && h > 0 {
		m.zones = append(m.zones, zone{id, image.Rect(x, y, x+w, y+h)})
	}
}

// box draws a rounded panel of w×h around the given lines, with a title in
// the top border and an optional label in the bottom border.
func (m *Model) box(title, bottom string, lines []string, w, h int, focused bool) string {
	bs := m.st.border
	if focused {
		bs = m.st.borderFocus
	}
	iw := w - 2
	top := "─ " + m.st.title.Render(title) + bs.Render(" ")
	fill := iw - width(top)
	var b strings.Builder
	b.WriteString(bs.Render("╭") + bs.Render("─ ") + m.st.title.Render(title) + bs.Render(" "+strings.Repeat("─", max(fill, 0))+"╮"))
	for i := 0; i < h-2; i++ {
		s := ""
		if i < len(lines) {
			s = lines[i]
		}
		b.WriteString("\n" + bs.Render("│") + fit(s, iw) + bs.Render("│"))
	}
	bot := strings.Repeat("─", iw)
	if bottom != "" && width(bottom)+4 <= iw {
		bot = bs.Render(strings.Repeat("─", iw-width(bottom)-3)+" ") + m.st.muted.Render(bottom) + bs.Render(" ─")
	} else {
		bot = bs.Render(bot)
	}
	b.WriteString("\n" + bs.Render("╰") + bot + bs.Render("╯"))
	return b.String()
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	m.zones = m.zones[:0]
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.BackgroundColor = m.st.bg
	v.ForegroundColor = m.st.fg
	v.WindowTitle = m.windowTitle()

	if m.width < 30 || m.height < 6 {
		v.SetContent("terminal-music: window too small")
		return v
	}

	body := m.layout(m.height - 1)
	content := body + "\n" + m.footer(m.height-1)

	if m.overlay == overlayHelp {
		content = m.withHelp(content)
	}
	v.SetContent(content)
	return v
}

// layout renders the panels into h rows.
func (m *Model) layout(h int) string {
	w := m.width
	showPL := m.cfg.ShowPlaylist

	side := showPL && w >= 96 && (h < mainHeight+eqHeight+12 || w >= 130)
	if side {
		lw := max(leftMin, min(leftMax, w*45/100))
		left := []string{m.mainPanel(0, 0, lw)}
		used := mainHeight
		if eh := m.eqFit(h - used); m.cfg.ShowEQ && eh > 0 {
			left = append(left, m.eqPanel(0, used, lw, eh))
			used += eh
		}
		if h-used >= 5 {
			left = append(left, m.infoPanel(lw, h-used))
			used = h
		}
		col := lipgloss.JoinVertical(lipgloss.Left, left...)
		if used < h {
			col += strings.Repeat("\n"+strings.Repeat(" ", lw), h-used)
		}
		right := m.playlistPanel(lw, 0, w-lw, h)
		return lipgloss.JoinHorizontal(lipgloss.Top, col, right)
	}

	parts := []string{m.mainPanel(0, 0, w)}
	used := mainHeight
	if m.cfg.ShowEQ {
		avail := h - used
		if showPL {
			avail -= minList
		}
		if eh := m.eqFit(avail); eh > 0 {
			parts = append(parts, m.eqPanel(0, used, w, eh))
			used += eh
		}
	}
	if showPL && h-used >= 4 {
		parts = append(parts, m.playlistPanel(0, used, w, h-used))
		used = h
	}
	out := lipgloss.JoinVertical(lipgloss.Left, parts...)
	lines := strings.Split(out, "\n")
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines[:h], "\n")
}

// eqFit returns the equalizer height that fits in avail rows, or 0.
func (m *Model) eqFit(avail int) int {
	switch {
	case avail >= eqHeight:
		return eqHeight
	case avail >= eqCompact:
		return eqCompact
	}
	return 0
}

func (m *Model) windowTitle() string {
	if m.idle || m.pl.Current < 0 || m.pl.Current >= m.pl.Len() {
		return "terminal-music"
	}
	icon := "▶"
	if m.paused {
		icon = "⏸"
	}
	return icon + " " + m.pl.Tracks[m.pl.Current].Display() + " — terminal-music"
}

// --- main panel ----------------------------------------------------------

func (m *Model) mainPanel(x, y, w int) string {
	iw := w - 2
	ix, iy := x+1, y+1
	m.addZone("main", x, y, w, mainHeight)
	lines := make([]string, 0, mainHeight-2)

	// Row 0: scrolling song title.
	title := "terminal-music — press a to add music, ? for help"
	if i := m.pl.Current; i >= 0 && i < m.pl.Len() {
		t := m.pl.Tracks[i]
		title = fmt.Sprintf("%d. %s", i+1, t.Display())
		if t.Duration > 0 {
			title += " (" + fmtTime(t.Duration) + ")"
		}
	}
	offset := int(m.now.Sub(m.start).Milliseconds() / 250)
	lines = append(lines, " "+m.st.display.Render(marquee(title, iw-2, offset)))

	// Rows 1-4: state + big time on the left, visualizer on the right.
	t := m.pos
	if m.seekPreview >= 0 {
		t = m.seekPreview * m.dur
	}
	timeStr := fmtClock(t)
	if m.cfg.TimeRemaining && m.dur > 0 {
		timeStr = "-" + fmtClock(m.dur-t)
	}
	if m.idle {
		timeStr = "  :  "
	}
	digits := bigText(timeStr)
	icon := "■"
	switch {
	case m.idle:
	case m.paused:
		icon = "▮▮"
	default:
		icon = "▶"
	}
	leftW := max(24, width(digits[0])+4)
	info := m.audioInfo()
	vizW := iw - leftW - 1
	if vizW < 8 {
		vizW, leftW = 0, iw
	}
	m.vizCols = 0
	var vizRows []string
	if vizW > 0 {
		vizRows = m.vizView(vizW, 4)
		m.addZone("viz", ix+leftW+1, iy+1, vizW, 4)
	}
	for r := 0; r < 4; r++ {
		var left string
		switch {
		case r < 3:
			ic := "  "
			if r == 0 {
				ic = fit(icon, 2)
			}
			left = " " + m.st.accent.Render(ic) + " " + m.st.display.Render(digits[r])
		default:
			left = " " + m.st.muted.Render(info)
		}
		row := fit(left, leftW)
		if vizW > 0 {
			row += " " + vizRows[r]
		}
		lines = append(lines, row)
	}
	m.addZone("time", ix, iy+1, leftW, 3)

	// Row 5: seek bar.
	total := "--:--"
	if m.dur > 0 {
		total = fmtTime(m.dur)
	}
	barW := iw - 3 - width(total)
	f := 0.0
	if m.dur > 0 {
		f = t / m.dur
	}
	seek := m.newLine(ix, iy+5).add(" ")
	seek.zone("seek", hslider(f, barW, m.st.display, m.st.accent, m.st.muted))
	seek.add(" " + m.st.muted.Render(total))
	lines = append(lines, seek.String())

	// Row 6: transport buttons and volume.
	tr := m.newLine(ix, iy+6).add(" ")
	btn := func(id, label string, on bool) {
		st := m.st.button
		if on {
			st = m.st.buttonOn
		}
		tr.zone(id, st.Render("["+label+"]"))
		tr.add(" ")
	}
	btn("btn:prev", "|◀", false)
	btn("btn:play", "▶", !m.idle && !m.paused)
	btn("btn:pause", "▮▮", !m.idle && m.paused)
	btn("btn:stop", "■", m.idle)
	btn("btn:next", "▶|", false)
	btn("btn:eject", "▲", false)
	volLabel := fmt.Sprintf("%3.0f%%", m.volume)
	volIcon := "VOL"
	if m.muted {
		volIcon = "MUTE"
	}
	volW := min(16, iw-tr.w-width(volLabel)-width(volIcon)-3)
	if volW >= 4 {
		tr.pad(iw - volW - width(volLabel) - width(volIcon) - 3)
		tr.zone("btn:mute", m.st.muted.Render(volIcon))
		tr.add(" ")
		tr.zone("vol", hslider(m.volume/100, volW, m.st.accent, m.st.accent, m.st.muted))
		tr.add(" " + m.st.text.Render(volLabel))
	} else if iw-tr.w-width(volLabel)-1 >= 0 {
		// No room for the slider: the label alone still takes the wheel.
		tr.pad(iw - width(volLabel) - 1)
		tr.zone("vol", m.st.text.Render(volLabel))
	}
	lines = append(lines, tr.String())

	// Row 7: toggles and theme selector.
	tg := m.newLine(ix, iy+7).add(" ")
	toggle := func(id, label string, on bool) {
		st := m.st.muted
		if on {
			st = m.st.buttonOn
		}
		tg.zone(id, st.Render(label))
		tg.add(" ")
	}
	toggle("tog:shuffle", "SHUF", m.pl.Shuffle())
	toggle("tog:repeat", "REP:"+strings.ToUpper(m.pl.Repeat.String()), m.pl.Repeat != 0)
	toggle("tog:eq", "EQ", m.cfg.ShowEQ)
	toggle("tog:pl", "PL", m.cfg.ShowPlaylist)
	name := m.themes[m.themeIdx].Name
	if room := iw - tg.w - width(name) - 6; room >= 0 {
		tg.pad(iw - width(name) - 5)
		tg.zone("theme:prev", m.st.accent.Render("◂"))
		tg.add(" " + m.st.muted.Render(name) + " ")
		tg.zone("theme:next", m.st.accent.Render("▸"))
	}
	lines = append(lines, tg.String())

	return m.box("TERMINAL-MUSIC", "", lines, w, mainHeight, false)
}

func (m *Model) audioInfo() string {
	if m.idle {
		return "stopped"
	}
	var parts []string
	if m.bitrate > 0 {
		parts = append(parts, fmt.Sprintf("%.0fkbps", m.bitrate/1000))
	}
	if m.samplerate > 0 {
		parts = append(parts, fmt.Sprintf("%gkHz", float64(m.samplerate/100)/10))
	}
	switch m.channels {
	case 0:
	case 1:
		parts = append(parts, "mono")
	case 2:
		parts = append(parts, "stereo")
	default:
		parts = append(parts, fmt.Sprintf("%dch", m.channels))
	}
	return strings.Join(parts, " ")
}

func (m *Model) vizView(w, h int) []string {
	blank := make([]string, h)
	switch m.viz {
	case vizSpectrum:
		n := (w + 1) / 2
		if w < 24 {
			n = w
		}
		m.vizCols = n
		if !m.an.Available() && m.playing() {
			blank[h/2] = m.st.muted.Render(fit("no audio capture (parec)", w))
			return blank
		}
		return m.spectrumView(m.bars, m.peaks, w, h)
	case vizScope:
		m.vizCols = w
		return m.scopeView(m.wave, w, h)
	}
	blank[h/2] = m.st.muted.Render(fit("  visualizer off · click", w))
	return blank
}

// infoPanel shows details about the current track.
func (m *Model) infoPanel(w, h int) string {
	i := m.pl.Current
	if i < 0 || i >= m.pl.Len() {
		i = m.cursor
	}
	var lines []string
	if i >= 0 && i < m.pl.Len() {
		t := m.pl.Tracks[i]
		row := func(k, v string) {
			if v != "" {
				lines = append(lines, " "+m.st.muted.Render(fit(k, 8))+m.st.text.Render(v))
			}
		}
		row("Title", t.Title)
		row("Artist", t.Artist)
		row("Album", t.Album)
		if t.Duration > 0 {
			row("Length", fmtTime(t.Duration))
		}
		if i == m.pl.Current && !m.idle {
			row("Format", m.audioInfo())
		}
		row("Track", fmt.Sprintf("%d of %d", i+1, m.pl.Len()))
		row("File", t.Path)
	}
	return m.box("INFO", "", lines, w, h, false)
}

// --- equalizer -----------------------------------------------------------

func (m *Model) eqPanel(x, y, w, h int) string {
	iw := w - 2
	ix, iy := x+1, y+1
	eq := &m.cfg.EQ
	focused := m.focus == paneEQ
	sliderRows := h - 4
	lines := make([]string, 0, h-2)

	// Row 0: on/off, reset, presets, selected value.
	hd := m.newLine(ix, iy).add(" ")
	on := m.st.muted.Render("[OFF]")
	if eq.Enabled {
		on = m.st.buttonOn.Render("[ON]")
	}
	hd.zone("eq:on", on).add(" ")
	hd.zone("eq:reset", m.st.button.Render("[RESET]")).add("  ")
	preset := eq.Preset
	if preset == "" {
		preset = "Custom"
	}
	hd.zone("eq:preset-", m.st.accent.Render("◂"))
	hd.add(" " + m.st.text.Render(preset) + " ")
	hd.zone("eq:preset+", m.st.accent.Render("▸"))
	sel := fmt.Sprintf("%s %+.1f dB", eqLabels[m.eqSel], eqGain(eq, m.eqSel))
	if m.eqSel > 0 {
		sel = eqLabels[m.eqSel] + "Hz" + sel[len(eqLabels[m.eqSel]):]
	}
	if iw-hd.w-width(sel)-1 > 0 {
		hd.pad(iw - width(sel) - 1)
		hd.add(m.st.accent.Render(sel))
	}
	lines = append(lines, hd.String())

	// Sliders: 11 columns (preamp + 10 bands).
	colW := max(3, min(6, iw/12))
	scaleW := 0
	if iw-11*colW >= 6 {
		scaleW = 4
	}
	margin := max(0, (iw-11*colW-scaleW)/2)
	barW := max(1, colW-2)
	center := (sliderRows - 1) / 2
	halfCells := center * 2 // half cells available above (and below) center

	grid := make([]*line, sliderRows+1)
	for r := range grid {
		grid[r] = m.newLine(ix, iy+1+r).add(strings.Repeat(" ", margin))
		if scaleW > 0 {
			label := ""
			switch r {
			case 0:
				label = "+12"
			case center:
				label = "  0"
			case sliderRows - 1:
				label = "-12"
			}
			grid[r].add(m.st.muted.Render(fit(label, scaleW)))
		}
	}
	for i := 0; i < 11; i++ {
		g := eqGain(eq, i)
		units := int(g/eqMax*float64(halfCells) + sign(g)*0.5)
		st := m.st.display
		switch {
		case !eq.Enabled:
			st = m.st.muted
		case focused && i == m.eqSel:
			st = m.st.accent
		}
		pad := (colW - barW) / 2
		m.addZone(fmt.Sprintf("eq:band:%d", i), ix+grid[0].w, iy+1, colW, sliderRows)
		for r := 0; r < sliderRows; r++ {
			cell := m.st.muted.Render(fit(strings.Repeat(" ", barW/2)+"│", barW))
			dist := center - r // rows above center (negative below)
			switch {
			case r == center:
				cell = st.Render(strings.Repeat("━", barW))
			case dist > 0 && units > 0:
				need := units - (dist-1)*2
				if need >= 2 {
					cell = st.Render(strings.Repeat("█", barW))
				} else if need == 1 {
					cell = st.Render(strings.Repeat("▄", barW))
				}
			case dist < 0 && units < 0:
				need := -units - (-dist-1)*2
				if need >= 2 {
					cell = st.Render(strings.Repeat("█", barW))
				} else if need == 1 {
					cell = st.Render(strings.Repeat("▀", barW))
				}
			}
			grid[r].add(strings.Repeat(" ", pad) + cell + strings.Repeat(" ", colW-pad-barW))
		}
		label := eqLabels[i]
		if colW < 4 {
			label = eqShortLabels[i]
		}
		lst := m.st.muted
		if i == m.eqSel && focused {
			lst = m.st.accent
		}
		lp := (colW - width(label)) / 2
		grid[sliderRows].add(fit(strings.Repeat(" ", max(lp, 0))+lst.Render(label), colW))
	}
	for _, l := range grid {
		lines = append(lines, l.String())
	}
	return m.box("EQUALIZER", "", lines, w, h, focused)
}

func sign(f float64) float64 {
	switch {
	case f > 0:
		return 1
	case f < 0:
		return -1
	}
	return 0
}

// --- playlist ------------------------------------------------------------

func (m *Model) playlistPanel(x, y, w, h int) string {
	iw := w - 2
	ix, iy := x+1, y+1
	focused := m.focus == panePlaylist || !m.cfg.ShowEQ
	rows := max(h-3, 1)
	m.listRows = rows
	m.clampScroll()
	n := m.pl.Len()
	lines := make([]string, 0, h-2)

	scrollbar := n > rows
	listW := iw
	if scrollbar {
		listW--
	}
	numW := len(fmt.Sprint(max(n, 1)))

	if n == 0 {
		hints := []string{
			"",
			"The playlist is empty.",
			"",
			"Press a to add a file or a folder,",
			"drop files on the window,",
			"or start with: terminal-music ~/Music",
		}
		for _, s := range hints {
			lines = append(lines, m.st.muted.Render(fit("  "+s, listW)))
		}
	}
	// Registered first so that the row zones, added later, win the hit test.
	m.addZone("pl:list", ix, iy, listW, rows)
	for r := 0; r < rows && n > 0; r++ {
		i := m.scroll + r
		if i >= n {
			lines = append(lines, strings.Repeat(" ", listW))
			continue
		}
		t := m.pl.Tracks[i]
		dur := ""
		if t.Duration > 0 {
			dur = fmtTime(t.Duration)
		}
		mark := " "
		current := i == m.pl.Current
		if current && !m.idle {
			mark = "▶"
			if m.paused {
				mark = "‖"
			}
		}
		num := fmt.Sprintf("%*d.", numW, i+1)
		titleW := listW - width(num) - width(dur) - 4
		text := fmt.Sprintf("%s%s %s %s ", mark, num, fit(t.Display(), max(titleW, 1)), dur)
		text = fit(text, listW)
		st := m.st.text
		if current {
			st = m.st.playing
		}
		if i == m.cursor {
			st = st.Inherit(m.st.selection)
			if !focused {
				st = m.st.muted.Inherit(m.st.selection)
			}
			if bg := m.st.selection.GetBackground(); bg != nil {
				st = st.Background(bg)
			}
		}
		m.addZone(fmt.Sprintf("pl:row:%d", i), ix, iy+r, listW, 1)
		lines = append(lines, st.Render(text))
	}
	if scrollbar {
		thumbH := max(1, rows*rows/n)
		thumbY := 0
		if n > rows {
			thumbY = (rows - thumbH) * m.scroll / (n - rows)
		}
		for r := 0; r < rows && r < len(lines); r++ {
			c := m.st.muted.Render("│")
			if r >= thumbY && r < thumbY+thumbH {
				c = m.st.accent.Render("┃")
			}
			lines[r] = fit(lines[r], listW) + c
		}
		m.addZone("pl:scroll", ix+listW, iy, 1, rows)
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}

	// Last row: playlist buttons and totals.
	ft := m.newLine(ix, iy+rows).add(" ")
	for _, b := range []struct{ id, label string }{
		{"pl:add", "+ADD"}, {"pl:del", "-DEL"}, {"pl:clear", "CLEAR"}, {"pl:find", "FIND"},
	} {
		ft.zone(b.id, m.st.button.Render("["+b.label+"]")).add(" ")
	}
	total, partial := m.pl.TotalDuration()
	summary := fmt.Sprintf("%d tracks", n)
	if total > 0 {
		summary += " · " + fmtTime(total)
		if partial {
			summary += "+"
		}
	}
	if iw-ft.w-width(summary)-1 > 0 {
		ft.pad(iw - width(summary) - 1)
		ft.add(m.st.muted.Render(summary))
	}
	lines = append(lines, ft.String())

	return m.box("PLAYLIST", "", lines, w, h, focused && m.cfg.ShowEQ)
}

// --- footer and overlays ------------------------------------------------

func (m *Model) footer(y int) string {
	w := m.width
	switch m.overlay {
	case overlayAdd, overlaySearch:
		label := " Add: "
		if m.overlay == overlaySearch {
			label = " Find: "
		}
		m.input.SetWidth(max(w-width(label)-2, 10))
		return fit(m.st.accent.Render(label)+m.input.View(), w)
	}
	l := m.newLine(0, y)
	if m.status != "" {
		l.add(" " + m.st.accent.Render(m.status))
	} else {
		l.add(" ")
		l.zone("help", m.st.accent.Render("? help"))
		l.add(m.st.muted.Render(" · q quit · space play/pause · ←→ seek · +- volume · a add · / find"))
	}
	return fit(l.String(), w)
}

var helpText = [][2]string{
	{"space / c", "play / pause"},
	{"x", "play (restart)"},
	{"v", "stop"},
	{"z / b", "previous / next track"},
	{"← → / h l", "seek 5s (shift: 30s)"},
	{"+ / -", "volume up / down"},
	{"m", "mute"},
	{"s / r", "shuffle / repeat (off, all, one)"},
	{"↑ ↓ / k j", "move in playlist"},
	{"enter", "play selected track"},
	{"K / J", "move track up / down"},
	{"d / D", "remove track / clear playlist"},
	{"a", "add file, folder, playlist or URL"},
	{"/", "find a track (↑ ↓ for more)"},
	{".", "go to playing track"},
	{"tab", "switch focus playlist / equalizer"},
	{"EQ: ← → ↑ ↓", "select band / change gain"},
	{"EQ: [ ] 0 R", "presets / zero band / reset"},
	{"EQ: enter", "turn equalizer on / off"},
	{"e / p", "show equalizer / playlist"},
	{"f", "visualizer: spectrum, scope, off"},
	{"t / T", "next / previous theme"},
	{"ctrl+r", "elapsed / remaining time"},
	{"q", "quit"},
}

func (m *Model) withHelp(base string) string {
	keyW := 0
	for _, h := range helpText {
		keyW = max(keyW, width(h[0]))
	}
	lines := []string{""}
	for _, h := range helpText {
		lines = append(lines, "  "+m.st.accent.Render(fit(h[0], keyW))+"  "+m.st.text.Render(h[1]))
	}
	lines = append(lines, "", "  "+m.st.muted.Render("Mouse: click buttons, drag sliders, wheel"), "  "+m.st.muted.Render("to scroll or change values, double-click"), "  "+m.st.muted.Render("a track to play it. Any key closes."))
	w := min(m.width, 56)
	h := min(m.height, len(lines)+3)
	boxStr := m.box("HELP", "", lines, w, h, true)
	// Paint the panel background so the help box is opaque.
	x := (m.width - w) / 2
	y := max(0, (m.height-h)/2)
	comp := lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(boxStr).X(x).Y(y).Z(1),
	)
	return comp.Render()
}
