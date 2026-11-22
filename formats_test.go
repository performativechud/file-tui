package main

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func fakeTools() *Tools {
	return &Tools{
		Bins: map[string]bool{
			"magick": true, "ffmpeg": true, "pdftoppm": true, "pdftotext": true,
			"xz": true, "zstd": true, "bzip2": true,
		},
		Magick: map[string]bool{
			"PNG": true, "JPEG": true, "WEBP": true, "AVIF": true, "GIF": true,
			"BMP": true, "TIFF": true, "ICO": true, "HEIC": true, "JXL": true,
			"PSD": true, "TGA": true, "SVG": true, "PDF": true,
		},
		Encoders: map[string]bool{
			"libx264": true, "aac": true, "libvpx-vp9": true, "libopus": true,
			"libmp3lame": true, "libvorbis": true, "libtheora": true, "mpeg4": true,
			"png": true, "mjpeg": true, "gif": true, "flac": true, "pcm_s16le": true,
		},
		Langs: map[string]bool{},
	}
}

func targetExts(ts []Target) map[string]Target {
	m := map[string]Target{}
	for _, t := range ts {
		m[t.Ext] = t
	}
	return m
}

func TestImageHasNoArchiveTargets(t *testing.T) {
	got := targetExts(targetsFor(kindOf("jpg"), fakeTools()))
	for _, bad := range []string{"zip", "tar", "mp4", "mp3", "docx"} {
		if _, ok := got[bad]; ok {
			t.Errorf("jpg must not offer .%s", bad)
		}
	}
	for _, want := range []string{"png", "webp", "pdf"} {
		if _, ok := got[want]; !ok {
			t.Errorf("jpg should offer .%s", want)
		}
	}
	if _, ok := got["jpg"]; ok {
		t.Error("jpg -> jpg must be filtered out")
	}
}

func TestAudioHasNoImageTargets(t *testing.T) {
	got := targetExts(targetsFor(kindOf("mp3"), fakeTools()))
	for _, bad := range []string{"png", "jpg", "pdf", "zip", "mp4"} {
		if _, ok := got[bad]; ok {
			t.Errorf("mp3 must not offer .%s", bad)
		}
	}
	if _, ok := got["wav"]; !ok {
		t.Error("mp3 should offer wav")
	}
}

func TestArchiveTargets(t *testing.T) {
	got := targetExts(targetsFor(kindOf("zip"), fakeTools()))
	for _, bad := range []string{"png", "mp3", "pdf"} {
		if _, ok := got[bad]; ok {
			t.Errorf("zip must not offer .%s", bad)
		}
	}
	for _, want := range []string{"tar.gz", "tar.xz", "tar.zst"} {
		if _, ok := got[want]; !ok {
			t.Errorf("zip should offer .%s", want)
		}
	}
	if _, ok := got["zip"]; ok {
		t.Error("zip -> zip must be filtered out")
	}
}

func TestLockedTargetsStayVisibleButUnavailable(t *testing.T) {
	tools := fakeTools()
	got := targetExts(targetsFor(kindOf("jpg"), tools))
	ocr, ok := got["txt"]
	if !ok {
		t.Fatal("ocr row should be listed even without tesseract")
	}
	if avail, reason := ocr.Available(tools); avail || reason == "" {
		t.Fatalf("ocr row must be locked with a reason, got avail=%v reason=%q", avail, reason)
	}
	if avail, _ := got["png"].Available(tools); !avail {
		t.Fatal("magick png must be available")
	}
}

func TestNoToolsMeansNoAvailableTargets(t *testing.T) {
	tools := &Tools{Bins: map[string]bool{}, Magick: map[string]bool{}, Encoders: map[string]bool{}}
	for _, tg := range targetsFor(kindOf("png"), tools) {
		if avail, _ := tg.Available(tools); avail {
			t.Errorf(".%s should be locked without tools", tg.Ext)
		}
	}
}

func TestOCRNeedsLanguagePack(t *testing.T) {
	tools := fakeTools()
	tools.Bins["tesseract"] = true
	tools.Langs = map[string]bool{"osd": true}
	ocr := targetExts(targetsFor(kindOf("png"), tools))["txt"]
	ok, reason := ocr.Available(tools)
	if ok || !strings.Contains(reason, "tesseract-data-eng") {
		t.Fatalf("ocr without eng: avail=%v reason=%q", ok, reason)
	}
	tools.Langs["eng"] = true
	if ok, reason := ocr.Available(tools); !ok {
		t.Fatalf("ocr with eng must be available: %s", reason)
	}
}

func TestNaming(t *testing.T) {
	if got := srcBase("/x/photo.JPG"); got != "photo" {
		t.Errorf("srcBase: %q", got)
	}
	if got := srcBase("/x/backup.tar.gz"); got != "backup" {
		t.Errorf("compound srcBase: %q", got)
	}
	if got := srcBase("/x/noext"); got != "noext" {
		t.Errorf("noext srcBase: %q", got)
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "cat.jpg")
	_ = os.WriteFile(src, []byte("x"), 0o644)

	out := planSingle(src, "png")
	if filepath.Base(out) != "file-tui-cat.png" {
		t.Fatalf("output name: %s", out)
	}
	_ = os.WriteFile(out, []byte("x"), 0o644)
	out2 := planSingle(src, "png")
	if filepath.Base(out2) != "file-tui-cat-2.png" {
		t.Fatalf("collision name: %s", out2)
	}

	base := planMultiBase(src, "png")
	if filepath.Base(base) != "file-tui-cat" {
		t.Fatalf("multi base: %s", base)
	}
	_ = os.WriteFile(base+"-1.png", []byte("x"), 0o644)
	base2 := planMultiBase(src, "png")
	if filepath.Base(base2) != "file-tui-cat-2" {
		t.Fatalf("multi collision base: %s", base2)
	}
}

func TestResolvePathForms(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "my file.png")
	_ = os.WriteFile(file, []byte("\x89PNG\r\n\x1a\n"), 0o644)

	forms := []string{
		file,
		"'" + file + "'",
		`"` + file + `"`,
		dir + "/my\\ file.png",
		"file://" + file,
	}
	for _, f := range forms {
		got, err := resolvePath(f)
		if err != nil {
			t.Errorf("%q: %v", f, err)
			continue
		}
		if got != file {
			t.Errorf("%q resolved to %q", f, got)
		}
	}

	if _, err := resolvePath(filepath.Join(dir, "missing")); err == nil {
		t.Error("missing file must error")
	}
	if _, err := resolvePath(dir); err == nil {
		t.Error("directory must error")
	}
}

func TestResolvePathEdgeCases(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "my file.png")
	_ = os.WriteFile(file, []byte("\x89PNG\r\n\x1a\n"), 0o644)

	// ~user expands to the current user's home directory.
	cur, curErr := user.Current()
	if curErr == nil {
		file2 := filepath.Join(cur.HomeDir, "my file.png")
		_ = os.WriteFile(file2, []byte("\x89PNG\r\n\x1a\n"), 0o644)
		got3, err3 := resolvePath("~" + cur.Username + "/my file.png")
		if err3 != nil {
			t.Errorf("~user expansion: %v", err3)
		} else if got3 != file2 {
			t.Errorf("~user resolved to %q, want %q", got3, file2)
		}
	}

	// A nonexistent ~user falls back to the literal path and fails to stat.
	if _, err := resolvePath("~alice/file"); err == nil {
		t.Error("nonexistent ~user must not resolve")
	}

	// file:// with a host is rejected.
	if _, err := resolvePath("file://host/path"); err == nil {
		t.Error("file:// with host must be rejected")
	}

	// Multi-line paste: first line is garbage, second line is the real path.
	// resolvePath picks the first candidate that stats successfully.
	bad := "/nonexistent/garbage\n" + file
	got, err := resolvePath(bad)
	if err != nil {
		t.Errorf("multi-line with valid second line: %v", err)
	}
	if got != file {
		t.Errorf("multi-line resolved to %q, want %q", got, file)
	}

	// file:// with host inside multi-line is skipped, falls through to valid line.
	bad2 := "file://host/path\n" + file
	got2, err := resolvePath(bad2)
	if err != nil {
		t.Errorf("multi-line with file://host then valid: %v", err)
	}
	if got2 != file {
		t.Errorf("multi-line resolved to %q, want %q", got2, file)
	}
}

func TestPDFTargetNeedsAWorkingPrinter(t *testing.T) {
	tools := fakeTools()
	tools.Bins["pandoc"] = true
	pdf := targetExts(targetsFor(kindOf("md"), tools))["pdf"]
	if ok, reason := pdf.Available(tools); ok || !strings.Contains(reason, "libreoffice or texlive") {
		t.Fatalf("pdf without printer: avail=%v reason=%q", ok, reason)
	}
	tools.Bins["soffice"] = true
	if ok, reason := pdf.Available(tools); !ok {
		t.Fatalf("pdf with libreoffice must be available: %s", reason)
	}
}

// TestHasAndSemantics tests the & (AND) operator in has() needs.
func TestHasAndSemantics(t *testing.T) {
	tools := &Tools{
		Bins:     map[string]bool{"7z": true, "xz": false},
		Magick:   map[string]bool{},
		Encoders: map[string]bool{},
		Langs:    map[string]bool{},
	}

	// Both 7z and xz required: "(7z OR unar) AND xz" -> since 7z is true but xz is false, should fail
	ok, reason := tools.has("bin:7z|unar&bin:xz")
	if ok {
		t.Fatalf("expected false when xz missing, got true")
	}
	if !strings.Contains(reason, "xz") {
		t.Fatalf("reason should mention xz, got %q", reason)
	}

	// Now provide xz, should succeed
	tools.Bins["xz"] = true
	ok, reason = tools.has("bin:7z|unar&bin:xz")
	if !ok {
		t.Fatalf("expected true when both 7z and xz present, got false: %s", reason)
	}

	// Test with unar instead of 7z
	tools2 := &Tools{
		Bins:     map[string]bool{"unar": true, "xz": true},
		Magick:   map[string]bool{},
		Encoders: map[string]bool{},
		Langs:    map[string]bool{},
	}
	ok, reason = tools2.has("bin:7z|unar&bin:xz")
	if !ok {
		t.Fatalf("expected true with unar and xz, got false: %s", reason)
	}

	// Test single clause (no &)
	ok, _ = tools.has("bin:7z")
	if !ok {
		t.Fatal("single clause bin:7z should succeed")
	}

	// Test empty need
	ok, _ = tools.has("")
	if !ok {
		t.Fatal("empty need should succeed")
	}
}

// TestTargetsFor7zSource verifies targets for 7z source include extraction requirement.
func TestTargetsFor7zSource(t *testing.T) {
	tools := fakeTools()
	tools.Bins["7z"] = true
	tools.Bins["xz"] = true
	tools.Bins["zstd"] = true

	targets := targetExts(targetsFor(kindOf("7z"), tools))

	// tar.xz should have Need = "bin:7z|unar&bin:xz"
	tarXz := targets["tar.xz"]
	if tarXz.Need != "bin:7z|unar&bin:xz" {
		t.Errorf("tar.xz Need = %q, want \"bin:7z|unar&bin:xz\"", tarXz.Need)
	}

	// tar.zst should have Need = "bin:7z|unar&bin:zstd"
	tarZst := targets["tar.zst"]
	if tarZst.Need != "bin:7z|unar&bin:zstd" {
		t.Errorf("tar.zst Need = %q, want \"bin:7z|unar&bin:zstd\"", tarZst.Need)
	}

	// tar.gz should have Need = "bin:7z|unar" (only extraction requirement, no compression tool needed)
	tarGz := targets["tar.gz"]
	if tarGz.Need != "bin:7z|unar" {
		t.Errorf("tar.gz Need = %q, want \"bin:7z|unar\"", tarGz.Need)
	}

	// zip should have Need = "bin:7z|unar" (just extraction)
	zipT := targets["zip"]
	if zipT.Need != "bin:7z|unar" {
		t.Errorf("zip Need = %q, want \"bin:7z|unar\"", zipT.Need)
	}

	// tar should have Need = "bin:7z|unar"
	tar := targets["tar"]
	if tar.Need != "bin:7z|unar" {
		t.Errorf("tar Need = %q, want \"bin:7z|unar\"", tar.Need)
	}
}

// TestTargetsForRarSource verifies targets for rar source include extraction requirement.
func TestTargetsForRarSource(t *testing.T) {
	tools := fakeTools()
	tools.Bins["7z"] = true
	tools.Bins["xz"] = true
	tools.Bins["zstd"] = true

	targets := targetExts(targetsFor(kindOf("rar"), tools))

	// All archive targets should have the 7z|unar requirement AND their original requirement
	tarXz := targets["tar.xz"]
	if tarXz.Need != "bin:7z|unar&bin:xz" {
		t.Errorf("rar->tar.xz Need = %q, want \"bin:7z|unar&bin:xz\"", tarXz.Need)
	}

	tarZst := targets["tar.zst"]
	if tarZst.Need != "bin:7z|unar&bin:zstd" {
		t.Errorf("rar->tar.zst Need = %q, want \"bin:7z|unar&bin:zstd\"", tarZst.Need)
	}

	tarGz := targets["tar.gz"]
	if tarGz.Need != "bin:7z|unar" {
		t.Errorf("rar->tar.gz Need = %q, want \"bin:7z|unar\"", tarGz.Need)
	}

	zipT := targets["zip"]
	if zipT.Need != "bin:7z|unar" {
		t.Errorf("rar->zip Need = %q, want \"bin:7z|unar\"", zipT.Need)
	}
}

// TestTargetsForZipSource verifies targets for zip source are NOT modified (no 7z/unar requirement).
func TestTargetsForZipSource(t *testing.T) {
	tools := fakeTools()
	tools.Bins["xz"] = true
	tools.Bins["zstd"] = true

	targets := targetExts(targetsFor(kindOf("zip"), tools))

	// zip source should NOT add 7z|unar requirement - it uses Go's archive/zip
	tarXz := targets["tar.xz"]
	if tarXz.Need != "bin:xz" {
		t.Errorf("zip->tar.xz Need = %q, want \"bin:xz\" (no 7z/unrar)", tarXz.Need)
	}

	tarZst := targets["tar.zst"]
	if tarZst.Need != "bin:zstd" {
		t.Errorf("zip->tar.zst Need = %q, want \"bin:zstd\" (no 7z/unrar)", tarZst.Need)
	}

	tarGz := targets["tar.gz"]
	if tarGz.Need != "go" {
		t.Errorf("zip->tar.gz Need = %q, want \"go\" (no 7z/unrar)", tarGz.Need)
	}

	// zip->zip is filtered out as no-op, so it won't be in targets
	if _, ok := targets["zip"]; ok {
		t.Errorf("zip->zip should be filtered out as no-op")
	}
}
