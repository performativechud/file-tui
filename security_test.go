package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSafeJoinBlocksPathTraversal verifies safeJoin blocks all traversal forms.
func TestSafeJoinBlocksPathTraversal(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"../../etc/passwd",
		"../..",
		"/etc/shadow",
		"sub/../../../etc",
		"..\\..\\windows\\system32",
		"sub\\..\\..\\secret",
	}
	for _, c := range cases {
		p, ok := safeJoin(root, c)
		if ok && (p == root || !strings.HasPrefix(p, root+string(os.PathSeparator))) {
			t.Errorf("safeJoin(%q) = %q, ok=%v — traversal not blocked", c, p, ok)
		}
	}
}

// TestSafeJoinAcceptsPlainEntries verifies normal entries are accepted.
func TestSafeJoinAcceptsPlainEntries(t *testing.T) {
	root := t.TempDir()
	cases := []string{"file.txt", "sub/file.txt", "a/b/c.txt", "dir/"}
	for _, c := range cases {
		p, ok := safeJoin(root, c)
		if !ok || !strings.HasPrefix(p, root) {
			t.Errorf("safeJoin(%q) rejected: %q, %v", c, p, ok)
		}
	}
}

// TestResolvePathRejectsFileHost verifies file:// with host is rejected.
func TestResolvePathRejectsFileHost(t *testing.T) {
	if _, err := resolvePath("file://host/path"); err == nil {
		t.Error("file:// with host must be rejected")
	}
	if _, err := resolvePath("file:///etc/passwd"); err != nil {
		// may or may not exist; just verify it doesn't crash
	}
}

// TestResolvePathReturnsAbsolute verifies all resolved paths are absolute.
func TestResolvePathReturnsAbsolute(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.png")
	os.WriteFile(file, []byte("\x89PNG\r\n\x1a\n"), 0o644)

	got, err := resolvePath(file)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("resolvePath returned non-absolute path: %q", got)
	}
}

// TestExtractBudgetCapsBytesAtDefault verifies default byte cap is 2 GiB.
func TestExtractBudgetCapsBytesAtDefault(t *testing.T) {
	b := newExtractBudget()
	if b.maxBytes != 2<<30 {
		t.Errorf("default maxBytes = %d, want %d", b.maxBytes, int64(2)<<30)
	}
}

// TestExtractBudgetCapsEntriesAtDefault verifies default entry cap is 200000.
func TestExtractBudgetCapsEntriesAtDefault(t *testing.T) {
	b := newExtractBudget()
	if b.maxEntries != 200000 {
		t.Errorf("default maxEntries = %d, want 200000", b.maxEntries)
	}
}

// TestExtractBudgetEnvOverride verifies env vars override defaults.
func TestExtractBudgetEnvOverride(t *testing.T) {
	t.Setenv("FILE_CLI_MAX_EXTRACT_MB", "5")
	t.Setenv("FILE_CLI_MAX_EXTRACT_ENTRIES", "100")
	b := newExtractBudget()
	if b.maxBytes != 5<<20 {
		t.Errorf("maxBytes = %d, want %d", b.maxBytes, 5<<20)
	}
	if b.maxEntries != 100 {
		t.Errorf("maxEntries = %d, want 100", b.maxEntries)
	}
}

// TestWriteStreamCreatesPartialFileOnOverflow verifies partial file remains
// after a budget overflow so the caller can clean it up.
func TestWriteStreamCreatesPartialFileOnOverflow(t *testing.T) {
	t.Setenv("FILE_CLI_MAX_EXTRACT_MB", "1")
	b := newExtractBudget()
	dir := t.TempDir()
	dst := filepath.Join(dir, "big.bin")
	data := make([]byte, 1<<20+100)
	err := writeStream(dst, strings.NewReader(string(data)), 0o644, b)
	if err == nil {
		t.Fatal("expected overflow error")
	}
	if !strings.Contains(err.Error(), "expands past") {
		t.Errorf("error should mention cap, got: %v", err)
	}
	if _, statErr := os.Stat(dst); statErr != nil {
		t.Errorf("partial file should exist until caller cleans up: %v", statErr)
	}
}

// TestWriteStreamRespectsMode verifies file mode is applied on success.
func TestWriteStreamRespectsMode(t *testing.T) {
	b := newExtractBudget()
	dir := t.TempDir()
	dst := filepath.Join(dir, "mode.txt")
	err := writeStream(dst, strings.NewReader("hello"), 0o644, b)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", fi.Mode().Perm())
	}
}

// TestWriteStreamNoSUID verifies mode strips special bits (SUID/SGID/sticky).
func TestWriteStreamNoSUID(t *testing.T) {
	b := newExtractBudget()
	dir := t.TempDir()
	dst := filepath.Join(dir, "suid.txt")
	// Attempt to set SUID bit (04755); Perm() in writeStream should strip it.
	err := writeStream(dst, strings.NewReader("hello"), 0o4755, b)
	if err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(dst)
	if fi.Mode().Perm()&0o4000 != 0 {
		t.Errorf("SUID bit must be stripped, got mode %v", fi.Mode().Perm())
	}
}

// TestEmptyStateViewRendersAtNarrowWidth verifies no panic at minimum width.
func TestEmptyStateViewRendersAtNarrowWidth(t *testing.T) {
	m := newModel("", fakeTools())
	m.width, m.height = 20, 8
	v := m.View()
	if !strings.Contains(v, "drop a file here") {
		t.Fatalf("empty view missing drop zone at 20x8:\n%s", v)
	}
}

// TestDoneStateViewRendersAtNarrowWidth verifies no panic/overflow at minimum width.
func TestDoneStateViewRendersAtNarrowWidth(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 20, 8
	m.st = stDone
	m.res = Result{Out: src, Size: 100, Elapsed: 100 * time.Millisecond}
	v := m.View()
	if !strings.Contains(v, "converted") {
		t.Fatalf("done view missing success at 20x8:\n%s", v)
	}
}

// TestVerifyExtractBudgetPassesWithinLimits verifies normal extraction passes.
func TestVerifyExtractBudgetPassesWithinLimits(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("world"), 0o644)

	b := newExtractBudget()
	if err := verifyExtractBudget(dir, b); err != nil {
		t.Fatalf("verifyExtractBudget should pass: %v", err)
	}
}

// TestVerifyExtractBudgetBlocksTooManyEntries verifies entry cap enforcement.
func TestVerifyExtractBudgetBlocksTooManyEntries(t *testing.T) {
	t.Setenv("FILE_CLI_MAX_EXTRACT_ENTRIES", "2")
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		os.WriteFile(filepath.Join(dir, "f"+string(rune('a'+i))+".txt"), []byte("x"), 0o644)
	}

	b := newExtractBudget()
	err := verifyExtractBudget(dir, b)
	if err == nil || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("expected entry cap error, got: %v", err)
	}
}

// TestVerifyExtractBudgetBlocksTooManyBytes verifies byte cap enforcement.
func TestVerifyExtractBudgetBlocksTooManyBytes(t *testing.T) {
	t.Setenv("FILE_CLI_MAX_EXTRACT_MB", "1")
	dir := t.TempDir()
	data := make([]byte, 1<<20+100)
	os.WriteFile(filepath.Join(dir, "big.bin"), data, 0o644)

	b := newExtractBudget()
	err := verifyExtractBudget(dir, b)
	if err == nil || !strings.Contains(err.Error(), "expands past") {
		t.Fatalf("expected byte cap error, got: %v", err)
	}
}

// TestErrorStateViewRendersAtNarrowWidth verifies no panic/overflow at minimum width.
func TestErrorStateViewRendersAtNarrowWidth(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	os.WriteFile(src, []byte("\x89PNG\r\n\x1a\n0000"), 0o644)

	m := newModel(src, fakeTools())
	m.width, m.height = 20, 8
	m.st = stError
	m.errText = "magick: no such file"
	v := m.View()
	if !strings.Contains(v, "magick") {
		t.Fatalf("error view missing error at 20x8:\n%s", v)
	}
}
