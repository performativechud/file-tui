package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type uiState int

const (
	stEmpty uiState = iota
	stReady
	stRunning
	stDone
	stError
)

type tickMsg struct{}
type infoMsg struct{ text string }
type progressMsg Progress
type suggMsg struct {
	id   int
	path string
	sugg []suggestion
}
type doneMsg struct {
	res Result
	err error
}

type hit struct {
	y int
	i int
}

type model struct {
	tools *Tools

	width, height int
	vOffset       int // vertical offset for centered content
	in            string

	src  string
	kind Kind
	info string
	size int64

	all        []Target
	targets    []Target
	sel        int
	showLocked bool

	st         uiState
	out        string
	base       string
	prog       Progress
	res        Result
	errText    string
	toastText  string
	toastUntil time.Time

	spin    int
	pulse   int
	send    func(tea.Msg)
	cancel  func()
	started time.Time

	// Autocomplete state (empty state only).
	suggestions []suggestion
	selSugg     int
	suggReqID   int // request ID for stale-result prevention

	hits []hit
}

func newModel(initial string, tools *Tools) *model {
	m := &model{tools: tools, width: 100, height: 32}
	if initial != "" {
		p, err := resolvePath(initial)
		if err != nil {
			m.toast(err.Error())
		} else {
			m.loadResolved(p)
		}
	}
	return m
}

func (m *model) loadResolved(p string) {
	k, err := detectFile(p)
	if err != nil {
		m.toast("cannot read: " + err.Error())
		return
	}
	fi, err := os.Stat(p)
	if err != nil {
		m.toast("cannot read: " + err.Error())
		return
	}
	if fi.IsDir() {
		m.toast("that is a directory, drop a file")
		return
	}
	m.src, m.kind = p, k
	m.size = fi.Size()
	m.all = targetsFor(k, m.tools)
	m.showLocked = false
	m.rebuild()
	m.st = stReady
	m.errText = ""
	m.in = p
	m.prog = Progress{Percent: -1}
}

// size is kept apart from Result so the card can show it before converting.
func (m *model) rebuild() {
	m.targets = nil
	for _, t := range m.all {
		if ok, _ := t.Available(m.tools); ok || m.showLocked {
			m.targets = append(m.targets, t)
		}
	}
	if len(m.targets) == 0 {
		m.sel = 0
		m.out = ""
		return
	}
	if m.sel >= len(m.targets) {
		m.sel = len(m.targets) - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
	for i, t := range m.targets {
		if ok, _ := t.Available(m.tools); ok {
			m.sel = i
			break
		}
	}
	m.refreshOut()
}

func (m *model) refreshOut() {
	if len(m.targets) == 0 {
		m.out, m.base = "", ""
		return
	}
	m.out, m.base = planFor(m.src, m.targets[m.sel])
}

func (m *model) move(d int) {
	if len(m.targets) == 0 {
		return
	}
	m.sel = (m.sel + d + len(m.targets)) % len(m.targets)
	m.refreshOut()
}

func (m *model) reset() {
	m.src, m.kind, m.info = "", Kind{}, ""
	m.all, m.targets, m.sel = nil, nil, 0
	m.showLocked = false
	m.st = stEmpty
	m.in = ""
	m.suggestions = nil
	m.selSugg = 0
	m.out, m.base = "", ""
	m.errText = ""
	m.prog = Progress{Percent: -1}
	m.res = Result{}
	m.toastText = ""
	m.toastUntil = time.Time{}
	m.size = 0
}

func (m *model) toast(s string) {
	m.toastText = s
	m.toastUntil = time.Now().Add(4 * time.Second)
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), m.infoCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(110*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *model) infoCmd() tea.Cmd {
	if m.src == "" {
		return nil
	}
	src, kind, tools := m.src, m.kind, m.tools
	return func() tea.Msg {
		return infoMsg{text: probeInfo(src, kind, tools)}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		m.spin++
		m.pulse++
		if m.toastText != "" && time.Now().After(m.toastUntil) {
			m.toastText = ""
		}
		return m, tickCmd()
	case infoMsg:
		m.info = msg.text
	case suggMsg:
		// Only apply results if the request ID matches (prevents stale results).
		if msg.id == m.suggReqID {
			m.suggestions = msg.sugg
			m.selSugg = 0
		}
	case progressMsg:
		m.prog = Progress(msg)
	case doneMsg:
		m.cancel = nil
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				m.st = stReady
				m.toast("cancelled")
			} else {
				m.st = stError
				m.errText = msg.err.Error()
			}
		} else {
			m.st = stDone
			m.res = msg.res
		}
	case tea.KeyMsg:
		return m, m.key(msg)
	case tea.MouseMsg:
		return m, m.mouse(msg)
	}
	return m, nil
}

func (m *model) key(msg tea.KeyMsg) tea.Cmd {
	if msg.Type == tea.KeyCtrlC {
		// During a running conversion, Ctrl+C cancels instead of quitting so
		// the footer's "ctrl+c · cancel" hint matches behavior. Outside of
		// stRunning it remains the standard quit signal.
		if m.st == stRunning && m.cancel != nil {
			m.cancel()
			return nil
		}
		return tea.Quit
	}
	switch m.st {
	case stEmpty:
		// Handle suggestion navigation when suggestions are visible.
		if len(m.suggestions) > 0 {
			switch msg.Type {
			case tea.KeyUp:
				m.selSugg = (m.selSugg - 1 + len(m.suggestions)) % len(m.suggestions)
				return nil
			case tea.KeyDown:
				m.selSugg = (m.selSugg + 1) % len(m.suggestions)
				return nil
			case tea.KeyTab:
				if m.selSugg < len(m.suggestions) {
					m.in = m.suggestions[m.selSugg].path
					m.suggReqID++
					return fetchSuggestions(m.suggReqID, m.in)
				}
				return nil
			case tea.KeyEsc:
				m.suggestions = nil
				m.selSugg = 0
				return nil
			case tea.KeyEnter:
				if m.selSugg < len(m.suggestions) {
					s := m.suggestions[m.selSugg]
					if s.isDir {
						m.in = s.path
						m.suggReqID++
						return fetchSuggestions(m.suggReqID, m.in)
					}
					p, err := resolvePath(s.path)
					if err != nil {
						m.toast(err.Error())
						return nil
					}
					m.loadResolved(p)
					m.suggestions = nil
					return m.infoCmd()
				}
				p, err := resolvePath(m.in)
				if err != nil {
					m.toast(err.Error())
					return nil
				}
				m.loadResolved(p)
				m.suggestions = nil
				return m.infoCmd()
			}
		}
		switch msg.Type {
		case tea.KeyEnter:
			p, err := resolvePath(m.in)
			if err != nil {
				m.toast(err.Error())
				return nil
			}
			m.loadResolved(p)
			m.suggestions = nil
			return m.infoCmd()
		case tea.KeyBackspace:
			r := []rune(m.in)
			if len(r) > 0 {
				m.in = string(r[:len(r)-1])
			}
			m.suggReqID++
			return fetchSuggestions(m.suggReqID, m.in)
		case tea.KeyEsc:
			if len(m.suggestions) > 0 {
				m.suggestions = nil
				m.selSugg = 0
				return nil
			}
			return tea.Quit
		case tea.KeyRunes:
			m.in += strings.ReplaceAll(string(msg.Runes), "\n", " ")
			m.suggReqID++
			return fetchSuggestions(m.suggReqID, m.in)
		case tea.KeySpace:
			m.in += " "
			m.suggReqID++
			return fetchSuggestions(m.suggReqID, m.in)
		}

	case stReady:
		switch {
		case msg.Type == tea.KeyUp:
			m.move(-1)
		case msg.Type == tea.KeyDown:
			m.move(1)
		case msg.Type == tea.KeyEnter:
			return m.start()
		case msg.Type == tea.KeyTab:
			m.showLocked = !m.showLocked
			m.rebuild()
			if m.showLocked {
				m.toast("showing locked targets · tab to hide")
			}
		case msg.Type == tea.KeyEsc:
			m.reset()
		case msg.Type == tea.KeyRunes && msg.String() == "q":
			return tea.Quit
		}

	case stRunning:
		if msg.Type == tea.KeyEsc {
			if m.cancel != nil {
				m.cancel()
			}
			m.toast("cancelling…")
		}

	case stDone:
		switch {
		case msg.Type == tea.KeyEsc:
			m.reset()
		case msg.Type == tea.KeyRunes && msg.String() == "o":
			openFolder(filepath.Dir(m.src))
		case msg.Type == tea.KeyRunes && msg.String() == "q":
			return tea.Quit
		}

	case stError:
		switch {
		case msg.Type == tea.KeyRunes && msg.String() == "r":
			return m.start()
		case msg.Type == tea.KeyEsc:
			m.reset()
		case msg.Type == tea.KeyRunes && msg.String() == "q":
			return tea.Quit
		}
	}
	return nil
}

func (m *model) mouse(msg tea.MouseMsg) tea.Cmd {
	if msg.Action != tea.MouseActionPress {
		return nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.st == stReady {
			m.move(-1)
		}
	case tea.MouseButtonWheelDown:
		if m.st == stReady {
			m.move(1)
		}
	case tea.MouseButtonLeft:
		if m.st != stReady {
			return nil
		}
		for _, h := range m.hits {
			if msg.Y == h.y {
				if m.sel == h.i {
					return m.start()
				}
				m.sel = h.i
				m.refreshOut()
				return nil
			}
		}
	}
	return nil
}

func (m *model) start() tea.Cmd {
	if len(m.targets) == 0 {
		m.toast("no targets available")
		return nil
	}
	t := m.targets[m.sel]
	if ok, reason := t.Available(m.tools); !ok {
		m.toast(reason)
		return nil
	}
	out, base := planFor(m.src, t)
	m.out, m.base = out, base
	m.st = stRunning
	m.prog = Progress{Percent: -1, Label: toolName(t)}
	m.started = time.Now()
	m.errText = ""
	send := m.send
	src, kind := m.src, m.kind

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	return func() tea.Msg {
		res, err := convert(ctx, src, kind, t, out, base, func(p Progress) {
			if send != nil {
				send(progressMsg(p))
			}
		})
		return doneMsg{res: res, err: err}
	}
}

func (m *model) View() string {
	m.hits = nil
	w := m.contentWidth()
	var lines []string
	lines = append(lines, m.header(w))

	switch m.st {
	case stEmpty:
		lines = append(lines, "", m.emptyZone(w), "", m.inputLine(w))
		if len(m.suggestions) > 0 {
			lines = append(lines, "")
			// Window the suggestion list so it never exceeds terminal height.
			visible := m.height - len(lines) - 4
			if visible < 3 {
				visible = 3
			}
			if visible > len(m.suggestions) {
				visible = len(m.suggestions)
			}
			start := 0
			if m.selSugg >= visible {
				start = m.selSugg - visible + 1
			}
			end := start + visible
			if end > len(m.suggestions) {
				end = len(m.suggestions)
			}
			for i := start; i < end; i++ {
				s := m.suggestions[i]
				mark := "  "
				if i == m.selSugg {
					mark = "▸ "
				}
				style := styleMuted
				if i == m.selSugg {
					style = styleAccent
				}
				lines = append(lines, style.Render(mark+s.name))
			}
			if len(m.suggestions) > visible {
				lines = append(lines, styleMuted.Render(fmt.Sprintf("  %d of %d", m.selSugg+1, len(m.suggestions))))
			}
		}
	case stDone:
		lines = append(lines, "")
		lines = append(lines, m.sourceSection(w)...)
		lines = append(lines, "", "", m.resultSection(w))
	case stError:
		lines = append(lines, "")
		lines = append(lines, m.sourceSection(w)...)
		lines = append(lines, "", "")
		firstRowY := lipgloss.Height(strings.Join(lines, "\n"))
		lines = append(lines, m.targetsSection(w, firstRowY))
		lines = append(lines, "", styleError.Render("✗  "+trunc(m.errText, w)))
	default:
		lines = append(lines, "")
		lines = append(lines, m.sourceSection(w)...)
		lines = append(lines, "", "")
		firstRowY := lipgloss.Height(strings.Join(lines, "\n"))
		panel := m.targetsSection(w, firstRowY)
		lines = append(lines, panel)
		if m.st == stRunning {
			lines = append(lines, "", m.progressLine(w))
		} else {
			lines = append(lines, "", m.outputLine(w))
		}
	}

	if m.toastText != "" {
		lines = append(lines, "", styleWarning.Render("! "+trunc(m.toastText, w)))
	}
	lines = append(lines, "", "", m.footer(w))

	frame := strings.Join(lines, "\n")
	placed := m.place(frame) // computes and stores vOffset
	// Adjust hitbox y-coordinates by the vertical centering offset so
	// mouse clicks land on the correct row after vertical centering.
	for i := range m.hits {
		m.hits[i].y += m.vOffset
	}
	return placed
}

func (m *model) place(frame string) string {
	fh := lipgloss.Height(frame)
	h := m.height
	if h < fh {
		h = fh
	}
	if m.width <= 0 {
		return frame
	}
	// Center the content block both horizontally and vertically within the
	// available terminal area. lipgloss.Place with Center vertical alignment
	// handles the vertical offset; we also store it so hitboxes can be adjusted.
	m.vOffset = (h - fh) / 2
	if m.vOffset < 0 {
		m.vOffset = 0
	}
	return lipgloss.Place(m.width, h, lipgloss.Center, lipgloss.Center, frame)
}

func (m *model) contentWidth() int {
	w := m.width
	if w > 100 {
		w = 100
	}
	if w < 20 {
		w = 20
	}
	return w
}

func (m *model) header(w int) string {
	left := styleAccent.Render("file-tui") + styleDim.Render("  ·  ") + styleMuted.Render("local converter")
	right := styleMuted.Render(m.stateLabel())
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	sep := styleDim.Render(strings.Repeat("─", w))
	return line + "\n" + sep
}

func (m *model) stateLabel() string {
	switch m.st {
	case stEmpty:
		return "waiting"
	case stReady:
		return "ready"
	case stRunning:
		return "working"
	case stDone:
		return "done"
	case stError:
		return "error"
	}
	return ""
}

func (m *model) emptyZone(w int) string {
	inside := lipgloss.JoinVertical(lipgloss.Center,
		stylePrimary.Render("drop a file here"),
		styleMuted.Render("or paste a path below"),
	)
	return lipgloss.PlaceHorizontal(w, lipgloss.Center, inside)
}

func (m *model) inputLine(w int) string {
	text := m.in
	if text == "" {
		text = styleMuted.Render("path/to/file")
	}
	cursor := "█"
	if (m.spin/5)%2 == 1 {
		cursor = " "
	}
	return styleAccent.Render("›  ") + trunc(text, w-6) + styleAccent.Render(cursor)
}

func (m *model) sourceSection(w int) []string {
	meta := m.kind.Label
	if m.size > 0 {
		meta += "  ·  " + humanSize(m.size)
	}
	if m.info != "" {
		meta += "  ·  " + m.info
	}

	return []string{
		styleLabel.Render("source"),
		stylePrimary.Render(trunc(m.src, w-2)),
		styleMuted.Render("  " + trunc(meta, w-4)),
	}
}

func (m *model) targetsSection(w int, firstRowY int) string {
	var lines []string
	lines = append(lines, styleLabel.Render("convert to"))

	if len(m.targets) == 0 {
		lines = append(lines, "", styleMuted.Render("no logical targets for this file"))
		return strings.Join(lines, "\n")
	}

	maxRows := m.height - 10
	if maxRows < 3 {
		maxRows = 3
	}
	if maxRows > 12 {
		maxRows = 12
	}

	start := 0
	if m.sel >= maxRows {
		start = m.sel - maxRows + 1
	}
	end := start + maxRows
	if end > len(m.targets) {
		end = len(m.targets)
	}

	for i := start; i < end; i++ {
		lines = append(lines, m.renderRow(i, w))
		if m.st == stReady {
			// The label line sits at firstRowY, target rows start one line below.
			m.hits = append(m.hits, hit{y: firstRowY + 1 + (i - start), i: i})
		}
	}

	if len(m.targets) > maxRows {
		lines = append(lines, styleMuted.Render(fmt.Sprintf("  %d of %d — scroll", m.sel+1, len(m.targets))))
	}

	return strings.Join(lines, "\n")
}

func (m *model) renderRow(i, w int) string {
	t := m.targets[i]
	avail, reason := t.Available(m.tools)

	mark := "  "
	if i == m.sel {
		mark = "▸ "
	}
	status := "ready"
	if !avail {
		status = "locked · " + reason
	} else if m.st == stRunning && i == m.sel {
		status = "working"
	}

	left := fmt.Sprintf("%s%-7s %s", mark, t.Label, t.Desc)
	if t.Warn != "" {
		left += "  (" + t.Warn + ")"
	}

	statusW := lipgloss.Width(status)
	if statusW > w-12 {
		status = trunc(status, w-12)
		statusW = lipgloss.Width(status)
	}
	leftW := w - statusW - 2
	if leftW < 8 {
		leftW = 8
	}
	left = padRight(trunc(left, leftW), leftW)
	plain := left + "  " + status

	if i == m.sel {
		fg := colFg
		if !avail {
			fg = colDim
		}
		return styleSelected.Foreground(fg).Render(plain)
	}
	leftPart := stylePrimary.Render(left)
	statusPart := styleSuccess.Render(status)
	if !avail {
		leftPart = styleMuted.Render(left)
		statusPart = styleMuted.Render(status)
	}
	return leftPart + "  " + statusPart
}

func (m *model) outputLine(w int) string {
	if m.out == "" {
		return styleMuted.Render("output: —")
	}
	return styleLabel.Render("output:") + "  " + stylePrimary.Render(trunc(m.out, w-12))
}

func (m *model) progressLine(w int) string {
	label := m.prog.Label
	if label == "" {
		label = "converting"
	}
	sp := spinFrames[m.spin%len(spinFrames)]
	elapsed := time.Since(m.started).Round(time.Second).String()
	if m.prog.Percent >= 0 {
		bar := progressBar(m.prog.Percent, 24)
		return styleAccent.Render(sp+" ") + padRight(trunc(label, 22), 22) + "  " +
			bar + "  " + stylePrimary.Render(fmt.Sprintf("%3d%%", m.prog.Percent)) + "  " + styleMuted.Render(elapsed)
	}
	bar := sliderBar(m.spin, 24)
	return styleAccent.Render(sp+" ") + padRight(trunc(label, 22), 22) + "  " +
		bar + "  " + styleMuted.Render(elapsed)
}

func (m *model) resultSection(w int) string {
	name := filepath.Base(strings.ReplaceAll(m.res.Out, "-<page>", ""))
	meta := humanSize(m.res.Size) + "  ·  " + m.res.Elapsed.Round(100*time.Millisecond).String()
	return strings.Join([]string{
		styleSuccess.Render("✓  converted"),
		"",
		stylePrimary.Render(trunc(name, w-2)),
		styleMuted.Render("  " + trunc(m.res.Out, w-4)),
		styleMuted.Render("  " + meta),
	}, "\n")
}

func (m *model) footer(w int) string {
	var items []string
	switch m.st {
	case stEmpty:
		items = []string{styleDim.Render("enter"), styleMuted.Render("load"), styleDim.Render("ctrl+c"), styleMuted.Render("quit")}
	case stReady:
		items = []string{styleDim.Render("↑↓"), styleMuted.Render("select"), styleDim.Render("enter"), styleMuted.Render("start"), styleDim.Render("⇥"), styleMuted.Render("locked"), styleDim.Render("esc"), styleMuted.Render("change"), styleDim.Render("q"), styleMuted.Render("quit")}
	case stRunning:
		items = []string{styleDim.Render("esc"), styleDim.Render("ctrl+c"), styleMuted.Render("cancel")}
	case stDone:
		items = []string{styleDim.Render("o"), styleMuted.Render("open folder"), styleDim.Render("esc"), styleMuted.Render("new file"), styleDim.Render("q"), styleMuted.Render("quit")}
	case stError:
		items = []string{styleDim.Render("r"), styleMuted.Render("retry"), styleDim.Render("esc"), styleMuted.Render("back"), styleDim.Render("q"), styleMuted.Render("quit")}
	}
	return strings.Join(items, "  ")
}

var (
	colFg     = lipgloss.Color("#d4d4d4") // soft white
	colMuted  = lipgloss.Color("#858585") // muted gray
	colDim    = lipgloss.Color("#5c5c5c") // darker subdued gray
	colAccent = lipgloss.Color("#f5871f") // warm orange / terracotta
	colGreen  = lipgloss.Color("#4caf50") // muted green
	colYellow = lipgloss.Color("#e5c07b") // soft yellow
	colRed    = lipgloss.Color("#e06c75") // muted red

	stylePrimary  = lipgloss.NewStyle().Foreground(colFg)
	styleMuted    = lipgloss.NewStyle().Foreground(colMuted)
	styleDim      = lipgloss.NewStyle().Foreground(colDim)
	styleAccent   = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleSuccess  = lipgloss.NewStyle().Foreground(colGreen)
	styleWarning  = lipgloss.NewStyle().Foreground(colYellow)
	styleError    = lipgloss.NewStyle().Foreground(colRed)
	styleLabel    = lipgloss.NewStyle().Foreground(colMuted).Bold(true)
	styleSelected = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

func trunc(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func padRight(s string, w int) string {
	gap := w - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

func progressBar(pct, w int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	fill := pct * w / 100
	full := strings.Repeat("▓", fill)
	rest := strings.Repeat("░", w-fill)
	return styleSuccess.Render(full) + styleMuted.Render(rest)
}

func sliderBar(step, w int) string {
	pos := (step / 2) % w
	cells := make([]rune, w)
	for i := range cells {
		cells[i] = '░'
	}
	for i := 0; i < 6 && pos+i < w; i++ {
		cells[pos+i] = '▓'
	}
	return styleAccent.Render(string(cells))
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 4 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func folderOpenerBinary(goos string) string {
	switch goos {
	case "darwin":
		return "open"
	case "windows":
		return "explorer"
	default:
		return "xdg-open"
	}
}

func openFolder(dir string) {
	opener := folderOpenerBinary(runtime.GOOS)
	if _, err := exec.LookPath(opener); err != nil {
		return
	}
	cmd := exec.Command(opener, dir)
	_ = cmd.Start()
}
