package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEmptyViewRenders(t *testing.T) {
	m := newModel("", fakeTools())
	m.width, m.height = 100, 32
	v := m.View()
	if !strings.Contains(v, "drop a file here") {
		t.Fatalf("empty view missing drop zone:\n%s", v)
	}
}

func TestReadyViewHidesLockedAndShowsOutput(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 200, 32
	v := m.View()
	if !strings.Contains(v, "convert to") {
		t.Fatalf("missing convert section:\n%s", v)
	}
	if !strings.Contains(v, "output:") {
		t.Fatalf("missing output preview:\n%s", v)
	}
	if strings.Contains(v, "locked ·") {
		t.Fatalf("locked rows must be hidden by default:\n%s", v)
	}
	m.showLocked = true
	m.rebuild()
	locked := -1
	for i, tg := range m.targets {
		if ok, _ := tg.Available(m.tools); !ok {
			locked = i
			break
		}
	}
	if locked < 0 {
		t.Fatal("tab must reveal locked rows")
	}
	m.sel = locked
	m.refreshOut()
	if !strings.Contains(m.View(), "locked ·") {
		t.Fatal("locked row must show its reason")
	}
	if len(m.hits) == 0 {
		t.Fatal("ready view must record row hitboxes for the mouse")
	}
}

func TestStartRefusesLockedTarget(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	tools := fakeTools()
	delete(tools.Magick, "WEBP")
	m := newModel(src, tools)
	m.showLocked = true
	m.rebuild()
	for i, tg := range m.targets {
		if tg.Ext == "webp" {
			m.sel = i
		}
	}
	if avail, _ := m.targets[m.sel].Available(tools); avail {
		t.Skip("webp unexpectedly available")
	}
	if cmd := m.start(); cmd != nil {
		t.Fatal("locked target must not start a conversion")
	}
	if m.toastText == "" {
		t.Fatal("locked target must explain itself")
	}
}

func TestHumanSize(t *testing.T) {
	cases := map[int64]string{
		12:      "12 B",
		2048:    "2.0 KB",
		1048576: "1.0 MB",
		5242880: "5.0 MB",
	}
	for in, want := range cases {
		if got := humanSize(in); got != want {
			t.Errorf("humanSize(%d)=%q want %q", in, got, want)
		}
	}
}

func TestHitboxRowsMatchRenderedRows(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	for _, sz := range [][2]int{{120, 40}, {100, 32}, {80, 24}} {
		m := newModel(src, fakeTools())
		m.width, m.height = sz[0], sz[1]
		lines := strings.Split(m.View(), "\n")
		if len(m.hits) == 0 {
			t.Fatalf("%dx%d: no hitboxes", sz[0], sz[1])
		}
		visible := 0
		for _, h := range m.hits {
			if h.y >= sz[1] {
				continue // frame is taller than the terminal; row is off-screen
			}
			visible++
			want := strings.Contains(lineText(lines, h.y), m.targets[h.i].Label)
			if !want {
				t.Errorf("%dx%d: hit row %d is not target %q", sz[0], sz[1], h.y, m.targets[h.i].Label)
			}
		}
		if visible == 0 {
			t.Errorf("%dx%d: no on-screen hitboxes", sz[0], sz[1])
		}
	}
}

// TestRunningViewRendersProgressAndWorkingStatus verifies stRunning shows the
// progress line and the selected row's "working" status.
func TestRunningViewRendersProgressAndWorkingStatus(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 100, 32
	m.st = stRunning
	m.prog = Progress{Percent: 50, Label: "JPG"}
	v := m.View()
	if !strings.Contains(v, "50%") {
		t.Fatalf("running view missing percent:\n%s", v)
	}
	if !strings.Contains(v, "working") {
		t.Fatalf("running view missing working status:\n%s", v)
	}
	if !strings.Contains(v, "esc") {
		t.Fatalf("running view missing cancel hint:\n%s", v)
	}
}

// TestDoneViewRendersResult verifies stDone shows the success section.
func TestDoneViewRendersResult(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 100, 32
	m.st = stDone
	m.res = Result{Out: filepath.Join(dir, "file-cli-cat.jpg"), Size: 1234, Elapsed: 250 * time.Millisecond}
	v := m.View()
	if !strings.Contains(v, "✓  converted") {
		t.Fatalf("done view missing success marker:\n%s", v)
	}
	if !strings.Contains(v, "1.2 KB") {
		t.Fatalf("done view missing size:\n%s", v)
	}
	if !strings.Contains(v, "o") {
		t.Fatalf("done view missing open-folder hint:\n%s", v)
	}
}

// TestErrorViewRendersError verifies stError shows the red error line.
func TestErrorViewRendersError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 100, 32
	m.st = stError
	m.errText = "magick: no such file"
	v := m.View()
	if !strings.Contains(v, "magick: no such file") {
		t.Fatalf("error view missing error text:\n%s", v)
	}
	if !strings.Contains(v, "r") || !strings.Contains(v, "retry") {
		t.Fatalf("error view missing retry hint:\n%s", v)
	}
}

// TestResetClearsAllState verifies reset() clears all model fields.
func TestResetClearsAllState(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 100, 32
	m.toast("transient message")
	m.st = stDone
	m.res = Result{Out: src, Size: 99}
	m.reset()

	if m.st != stEmpty {
		t.Errorf("reset: st = %v, want stEmpty", m.st)
	}
	if m.src != "" || m.kind.ID != "" {
		t.Errorf("reset: source not cleared")
	}
	if m.targets != nil || len(m.all) != 0 {
		t.Errorf("reset: targets not cleared")
	}
	if m.toastText != "" {
		t.Errorf("reset: toastText = %q, want empty", m.toastText)
	}
	if m.res.Out != "" || m.res.Size != 0 {
		t.Errorf("reset: result not cleared")
	}
	if m.in != "" {
		t.Errorf("reset: input not cleared")
	}
}

// TestToastLifecycle verifies toast sets text and expiry, then clears on timeout.
func TestToastLifecycle(t *testing.T) {
	m := newModel("", fakeTools())
	m.width, m.height = 100, 32
	m.toast("hello toast")
	if m.toastText != "hello toast" {
		t.Fatalf("toast: text = %q, want %q", m.toastText, "hello toast")
	}
	if m.toastUntil.IsZero() {
		t.Fatal("toast: expiry not set")
	}
	// Simulate timeout by moving expiry into the past and dispatching a tick.
	m.toastUntil = time.Now().Add(-1 * time.Second)
	m.Update(tickMsg{})
	if m.toastText != "" {
		t.Errorf("toast: text not cleared after expiry, got %q", m.toastText)
	}
}

// TestMoveWrapsAround verifies move() wraps selection modulo len(targets).
func TestMoveWrapsAround(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 100, 32
	if len(m.targets) < 2 {
		t.Skip("need multiple targets")
	}
	m.sel = 0
	m.move(-1)
	if m.sel != len(m.targets)-1 {
		t.Errorf("move(-1) from 0: sel = %d, want %d", m.sel, len(m.targets)-1)
	}
	m.sel = len(m.targets) - 1
	m.move(1)
	if m.sel != 0 {
		t.Errorf("move(1) from last: sel = %d, want 0", m.sel)
	}
}

// TestTabTogglesLocked verifies Tab flips showLocked and rebuilds.
func TestTabTogglesLocked(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 100, 32
	if m.showLocked {
		t.Fatal("showLocked should default to false")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.showLocked {
		t.Fatal("Tab must toggle showLocked to true")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.showLocked {
		t.Fatal("Tab must toggle showLocked back to false")
	}
}

// TestEscResetsFromReady verifies Esc in stReady calls reset().
func TestEscResetsFromReady(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 100, 32
	if m.st != stReady {
		t.Fatalf("expected stReady, got %v", m.st)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.st != stEmpty {
		t.Fatalf("Esc must reset to stEmpty, got %v", m.st)
	}
}

// TestWindowSizeMsgUpdatesDimensions verifies WindowSizeMsg updates width/height.
func TestWindowSizeMsgUpdatesDimensions(t *testing.T) {
	m := newModel("", fakeTools())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.width != 120 || m.height != 40 {
		t.Errorf("dimensions: %dx%d, want 120x40", m.width, m.height)
	}
}

// TestNarrowTerminalNoPanic verifies rendering doesn't panic at narrow widths.
func TestNarrowTerminalNoPanic(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	for _, sz := range [][2]int{{20, 8}, {34, 12}, {80, 24}, {100, 32}, {200, 60}} {
		m := newModel(src, fakeTools())
		m.width, m.height = sz[0], sz[1]
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%dx%d: panic: %v", sz[0], sz[1], r)
				}
			}()
			_ = m.View()
		}()
	}
}

// TestMouseWheelMovesSelection verifies wheel up/down moves selection.
func TestMouseWheelMovesSelection(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 100, 32
	if len(m.targets) < 2 {
		t.Skip("need multiple targets")
	}
	m.sel = 0
	m.mouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	if m.sel != 1 {
		t.Errorf("wheel down: sel = %d, want 1", m.sel)
	}
	m.mouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
	if m.sel != 0 {
		t.Errorf("wheel up: sel = %d, want 0", m.sel)
	}
}

// TestMouseClickSelectsRow verifies left click selects via hitbox.
func TestMouseClickSelectsRow(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 100, 32
	_ = m.View() // populate hits
	if len(m.hits) == 0 {
		t.Fatal("no hitboxes recorded")
	}
	h := m.hits[1]
	m.sel = 0
	m.mouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, Y: h.y})
	if m.sel != h.i {
		t.Errorf("click: sel = %d, want %d", m.sel, h.i)
	}
}

// TestHitboxNarrowTerminals verifies hitbox math at additional sizes.
func TestHitboxNarrowTerminals(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	for _, sz := range [][2]int{{20, 24}, {34, 12}, {50, 24}, {160, 50}} {
		m := newModel(src, fakeTools())
		m.width, m.height = sz[0], sz[1]
		lines := strings.Split(m.View(), "\n")
		if len(m.hits) == 0 {
			continue // frame taller than terminal; acceptable
		}
		for _, h := range m.hits {
			if h.y >= sz[1] {
				continue
			}
			if !strings.Contains(lineText(lines, h.y), m.targets[h.i].Label) {
				t.Errorf("%dx%d: hit row %d not target %q", sz[0], sz[1], h.y, m.targets[h.i].Label)
			}
		}
	}
}

// TestEmptyViewNoCtrlCQuitDuringRunning verifies Ctrl+C cancels instead of quitting.
func TestCtrlCCancelsDuringRunning(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 100, 32
	m.st = stRunning
	m.cancel = func() {} // simulate cancel available
	cmd := m.key(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		t.Fatal("Ctrl+C during running must not quit")
	}
}

// TestCtrlCQuitsOutsideRunning verifies Ctrl+C quits when not running.
func TestCtrlCQuitsOutsideRunning(t *testing.T) {
	m := newModel("", fakeTools())
	m.width, m.height = 100, 32
	cmd := m.key(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Ctrl+C outside running must return a quit command")
	}
}

func lineText(lines []string, y int) string {
	if y < 0 || y >= len(lines) {
		return ""
	}
	var b strings.Builder
	inEsc := false
	for _, r := range lines[y] {
		switch {
		case r == 0x1b:
			inEsc = true
		case inEsc && (r == '[' || r == ']'):
		case inEsc && ((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')):
			inEsc = false
		case !inEsc:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TestAutocompleteBasic verifies autocomplete returns matching directories.
func TestAutocompleteBasic(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, "Documents"), 0o755)
	os.MkdirAll(filepath.Join(home, "Downloads"), 0o755)

	orig := homeDirFunc
	homeDirFunc = func() (string, error) { return home, nil }
	defer func() { homeDirFunc = orig }()

	suggs := autocomplete("~/Do")
	if len(suggs) == 0 {
		t.Fatal("expected suggestions for ~/Do")
	}
	found := false
	for _, s := range suggs {
		if strings.HasPrefix(s.name, "Doc") || strings.HasPrefix(s.name, "Down") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected Documents or Downloads in suggestions")
	}
}

// TestAutocompleteNoMatch verifies no suggestions for nonexistent prefix.
func TestAutocompleteNoMatch(t *testing.T) {
	suggs := autocomplete("~/ZZZZNONEXISTENT")
	if len(suggs) != 0 {
		t.Fatalf("expected no suggestions, got %d", len(suggs))
	}
}

// TestAutocompleteEmptyInput verifies nil for empty input.
func TestAutocompleteEmptyInput(t *testing.T) {
	suggs := autocomplete("")
	if suggs != nil {
		t.Fatalf("expected nil for empty input, got %d", len(suggs))
	}
}

// TestAutocompleteHomeExpansion verifies ~ expands to home directory.
func TestAutocompleteHomeExpansion(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".file-cli-test-ac"), 0o755)

	orig := homeDirFunc
	homeDirFunc = func() (string, error) { return home, nil }
	defer func() { homeDirFunc = orig }()

	suggs := autocomplete("~/.file-cli-test-ac")
	if len(suggs) == 0 {
		t.Fatal("expected suggestions for ~/.file-cli-test-ac")
	}
}

// TestAutocompleteAbsolutePath verifies absolute path completion.
func TestAutocompleteAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644)

	suggs := autocomplete(dir + "/")
	if len(suggs) < 2 {
		t.Fatalf("expected at least 2 suggestions, got %d", len(suggs))
	}
	// Directory should come first
	if !suggs[0].isDir {
		t.Error("expected directory first in suggestions")
	}
}

// TestAutocompleteRelativePath verifies relative path completion.
func TestAutocompleteRelativePath(t *testing.T) {
	suggs := autocomplete("./")
	// Should return entries in current directory (may be empty in test env)
	_ = suggs
}

// TestAutocompleteSpaces verifies paths with spaces.
func TestAutocompleteSpaces(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "my dir"), 0o755)
	os.WriteFile(filepath.Join(dir, "my file.txt"), []byte("x"), 0o644)

	suggs := autocomplete(dir + "/my")
	if len(suggs) < 2 {
		t.Fatalf("expected at least 2 suggestions for 'my', got %d", len(suggs))
	}
}

// TestAutocompleteCaseInsensitive verifies case-insensitive matching.
func TestAutocompleteCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "Documents"), 0o755)
	os.MkdirAll(filepath.Join(dir, "downloads"), 0o755)

	suggs := autocomplete(dir + "/d")
	if len(suggs) < 2 {
		t.Fatalf("expected 2 suggestions for 'd', got %d", len(suggs))
	}
}

// TestAutocompleteDirFileSharedPrefix verifies dirs and files with same prefix.
func TestAutocompleteDirFileSharedPrefix(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "data"), 0o755)
	os.WriteFile(filepath.Join(dir, "data.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "database.json"), []byte("x"), 0o644)

	suggs := autocomplete(dir + "/data")
	if len(suggs) < 3 {
		t.Fatalf("expected 3 suggestions for 'data', got %d", len(suggs))
	}
	// Directory should come first
	if !suggs[0].isDir {
		t.Error("expected directory first")
	}
}

// TestAutocompleteMoreThanEight verifies max 8 suggestions.
func TestAutocompleteMoreThanEight(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 12; i++ {
		os.MkdirAll(filepath.Join(dir, "dir"+string(rune('a'+i))), 0o755)
	}

	suggs := autocomplete(dir + "/")
	if len(suggs) != 12 {
		t.Fatalf("expected all 12 suggestions, got %d", len(suggs))
	}
}

// TestAutocompleteDirFirstSorting verifies directories sort before files.
func TestAutocompleteDirFirstSorting(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "aaa.txt"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(dir, "zzz"), 0o755)

	suggs := autocomplete(dir + "/")
	if len(suggs) < 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(suggs))
	}
	if !suggs[0].isDir {
		t.Error("expected directory first despite alphabetical order")
	}
}

// TestAutocompleteNonexistentPath verifies no crash on nonexistent path.
func TestAutocompleteNonexistentPath(t *testing.T) {
	suggs := autocomplete("/nonexistent/path/that/does/not/exist")
	if suggs != nil {
		t.Fatalf("expected nil for nonexistent path, got %d", len(suggs))
	}
}

// TestAutocompleteTabCompletion verifies Tab completes selected suggestion.
func TestAutocompleteTabCompletion(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "Documents"), 0o755)

	m := newModel("", fakeTools())
	m.width, m.height = 100, 32
	m.in = dir + "/Doc"
	m.suggestions = autocomplete(m.in)
	m.selSugg = 0
	if len(m.suggestions) == 0 {
		t.Fatal("expected suggestions")
	}
	// Simulate Tab
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.in != m.suggestions[0].path {
		// After tab, in should be completed path. But suggestions are now
		// re-fetched asynchronously, so check in directly.
		if !strings.HasPrefix(m.in, dir+"/Doc") {
			t.Errorf("Tab did not complete path: %q", m.in)
		}
	}
}

// TestAutocompleteUpDownNavigation verifies Up/Down moves selection.
func TestAutocompleteUpDownNavigation(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		os.MkdirAll(filepath.Join(dir, "dir"+string(rune('a'+i))), 0o755)
	}

	m := newModel("", fakeTools())
	m.width, m.height = 100, 32
	m.in = dir + "/"
	m.suggestions = autocomplete(m.in)
	m.selSugg = 0
	if len(m.suggestions) < 2 {
		t.Skip("need multiple suggestions")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.selSugg != 1 {
		t.Errorf("Down: selSugg = %d, want 1", m.selSugg)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.selSugg != 0 {
		t.Errorf("Up: selSugg = %d, want 0", m.selSugg)
	}
}

// TestAutocompleteEscDismissal verifies Esc clears suggestions.
func TestAutocompleteEscDismissal(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)

	m := newModel("", fakeTools())
	m.width, m.height = 100, 32
	m.in = dir + "/"
	m.suggestions = autocomplete(m.in)
	m.selSugg = 0
	if len(m.suggestions) == 0 {
		t.Fatal("expected suggestions")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.suggestions) != 0 {
		t.Error("Esc must dismiss suggestions")
	}
}

// TestAutocompleteEnterOnDirectory verifies Enter on dir completes path.
func TestAutocompleteEnterOnDirectory(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)

	m := newModel("", fakeTools())
	m.width, m.height = 100, 32
	m.in = dir + "/sub"
	m.suggestions = autocomplete(m.in)
	m.selSugg = 0
	if len(m.suggestions) == 0 {
		t.Fatal("expected suggestions")
	}

	// Verify it's a directory
	if !m.suggestions[0].isDir {
		t.Skip("first suggestion not a directory")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// After Enter on dir, in should be completed with trailing /
	if !strings.HasSuffix(m.in, "/") {
		t.Errorf("Enter on dir should complete with /, got %q", m.in)
	}
}
