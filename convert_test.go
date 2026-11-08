package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func mustCmd(t *testing.T, name string, args ...string) {
	t.Helper()
	if err := exec.Command(name, args...).Run(); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func TestConvertImageIntegration(t *testing.T) {
	if _, err := exec.LookPath("magick"); err != nil {
		t.Skip("imagemagick missing")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "cat.png")
	mustCmd(t, "magick", "-size", "16x16", "xc:red", src)

	out := planSingle(src, "webp")
	res, err := convert(context.Background(), src, kindOf("png"),
		Target{Ext: "webp", Engine: "magick"}, out, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(res.Out)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("no output: %v", err)
	}
	if filepath.Base(res.Out) != "file-cli-cat.webp" {
		t.Fatalf("name: %s", res.Out)
	}
}

func TestConvertAudioIntegration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg missing")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "tone.wav")
	mustCmd(t, "ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1", src)

	out := planSingle(src, "mp3")
	res, err := convert(context.Background(), src, kindOf("wav"),
		Target{Ext: "mp3", Engine: "ffaudio", Arg: "mp3"}, out, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(res.Out)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("no output: %v", err)
	}
}

func TestArchiveRepackZipToTarGz(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.zip")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"a.txt", "sub/b.txt"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte("hello " + name))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	out := planSingle(src, "tar.gz")
	k, err := detectFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := convert(context.Background(), src, k,
		Target{Ext: "tar.gz", Engine: "archive"}, out, "", nil); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		names = append(names, hdr.Name)
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "a.txt") || !strings.Contains(joined, "sub/b.txt") {
		t.Fatalf("missing entries: %s", joined)
	}
}

func TestSafeJoinBlocksTraversal(t *testing.T) {
	root := t.TempDir()
	p, ok := safeJoin(root, "../../etc/passwd")
	if ok && !strings.HasPrefix(p, root+string(os.PathSeparator)) {
		t.Fatalf("entry escaped root: %s", p)
	}
	if _, ok := safeJoin(root, "sub/file"); !ok {
		t.Fatal("plain entry must be accepted")
	}
}

func TestDataCSVToJSONToCSV(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "t.csv")
	os.WriteFile(csvPath, []byte("a,b\n1,2\n3,4\n"), 0o644)

	jsonPath := filepath.Join(dir, "file-cli-t.json")
	if err := convertData(csvPath, Target{Ext: "json"}, jsonPath); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"a": "1"`) {
		t.Fatalf("json body: %s", b)
	}

	backPath := filepath.Join(dir, "file-cli-t.tsv")
	if err := convertData(jsonPath, Target{Ext: "tsv"}, backPath); err != nil {
		t.Fatal(err)
	}
	back, err := os.ReadFile(backPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(back), "a\tb") {
		t.Fatalf("tsv body: %q", back)
	}
}

func makeDocx(t *testing.T, dir string) string {
	t.Helper()
	md := filepath.Join(dir, "note.md")
	if err := os.WriteFile(md, []byte("# Title\n\nHello world.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	docx := filepath.Join(dir, "note.docx")
	mustCmd(t, "pandoc", md, "-o", docx)
	return docx
}

func TestConvertPandocDocxToEpub(t *testing.T) {
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc missing")
	}
	dir := t.TempDir()
	src := makeDocx(t, dir)
	k, err := detectFile(src)
	if err != nil || k.ID != "docx" {
		t.Fatalf("detect docx: %v %q", err, k.ID)
	}
	out := planSingle(src, "epub")
	if _, err := convert(context.Background(), src, k,
		Target{Ext: "epub", Engine: "pandoc"}, out, "", nil); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(out); err != nil || fi.Size() == 0 {
		t.Fatalf("no epub output: %v", err)
	}
}

func TestConvertLibreOfficeDocxToPDF(t *testing.T) {
	if _, err := exec.LookPath("soffice"); err != nil {
		t.Skip("libreoffice missing")
	}
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc missing")
	}
	dir := t.TempDir()
	src := makeDocx(t, dir)
	out := planSingle(src, "pdf")
	if _, err := convert(context.Background(), src, kindOf("docx"),
		Target{Ext: "pdf", Engine: "libreoffice"}, out, "", nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil || !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatalf("not a pdf: %v", err)
	}
}

func TestConvertCalibreEpubToTxt(t *testing.T) {
	if _, err := exec.LookPath("ebook-convert"); err != nil {
		t.Skip("calibre missing")
	}
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc missing")
	}
	dir := t.TempDir()
	src := makeDocx(t, dir)
	epub := filepath.Join(dir, "file-cli-note.epub")
	if _, err := convert(context.Background(), src, kindOf("docx"),
		Target{Ext: "epub", Engine: "pandoc"}, epub, "", nil); err != nil {
		t.Fatal(err)
	}
	out := planSingle(epub, "txt")
	if _, err := convert(context.Background(), epub, kindOf("epub"),
		Target{Ext: "txt", Engine: "calibre"}, out, "", nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil || !bytes.Contains(b, []byte("Hello world")) {
		t.Fatalf("bad text: %v %q", err, b)
	}
}

func TestArchive7zToTarGz(t *testing.T) {
	if _, err := exec.LookPath("7z"); err != nil {
		t.Skip("7zip missing")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	arc := filepath.Join(dir, "pack.7z")
	cmd := exec.Command("7z", "a", "-bd", arc, "a.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("7z a: %v %s", err, out)
	}
	k, err := detectFile(arc)
	if err != nil || k.ID != "7z" {
		t.Fatalf("detect 7z: %v %q", err, k.ID)
	}
	out := planSingle(arc, "tar.gz")
	if _, err := convert(context.Background(), arc, k,
		Target{Ext: "tar.gz", Engine: "archive"}, out, "", nil); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		names = append(names, hdr.Name)
	}
	if !strings.Contains(strings.Join(names, ","), "a.txt") {
		t.Fatalf("missing entry: %v", names)
	}
}

func TestConvertVideoToMP4(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg missing")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "clip.mkv")
	mustCmd(t, "ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=64x64:rate=10:duration=1",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-c:v", "libx264", "-c:a", "aac", "-shortest", src)

	out := planSingle(src, "mp4")
	if _, err := convert(context.Background(), src, kindOf("mkv"),
		Target{Ext: "mp4", Engine: "ffvideo", Arg: "mp4"}, out, "", nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil || len(b) < 12 || string(b[4:8]) != "ftyp" {
		t.Fatalf("not an mp4: %v", err)
	}
}

func TestConvertSRTToVTT(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg missing")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "sub.srt")
	srt := "1\n00:00:01,000 --> 00:00:02,000\nhello\n"
	if err := os.WriteFile(src, []byte(srt), 0o644); err != nil {
		t.Fatal(err)
	}
	k, err := detectFile(src)
	if err != nil || k.ID != "srt" {
		t.Fatalf("detect srt: %v %q", err, k.ID)
	}
	out := planSingle(src, "vtt")
	if _, err := convert(context.Background(), src, k,
		Target{Ext: "vtt", Engine: "ffsub", Arg: "vtt"}, out, "", nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil || !bytes.HasPrefix(b, []byte("WEBVTT")) {
		t.Fatalf("not vtt: %v %q", err, b)
	}
}

func TestConvertMarkdownToPDF(t *testing.T) {
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc missing")
	}
	hasPrint := false
	for _, b := range []string{"pdflatex", "xelatex", "lualatex", "tectonic", "typst", "soffice", "libreoffice"} {
		if _, err := exec.LookPath(b); err == nil {
			hasPrint = true
		}
	}
	if !hasPrint {
		t.Skip("no pdf engine and no libreoffice")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "note.md")
	if err := os.WriteFile(src, []byte("# Title\n\nHello world.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := planSingle(src, "pdf")
	if _, err := convert(context.Background(), src, kindOf("md"),
		Target{Ext: "pdf", Engine: "pandoc"}, out, "", nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil || !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatalf("not a pdf: %v", err)
	}
}

func TestConvertOCRImageToText(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract missing")
	}
	if _, err := exec.LookPath("magick"); err != nil {
		t.Skip("imagemagick missing")
	}
	if out, err := exec.Command("tesseract", "--list-langs").CombinedOutput(); err != nil || !strings.Contains(string(out), "\neng\n") {
		t.Skip("tesseract english data missing")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "label.png")
	mustCmd(t, "magick", "-size", "300x80", "xc:white",
		"-font", "DejaVu-Sans", "-pointsize", "36", "-fill", "black",
		"-annotate", "+10+50", "HELLO", src)

	k, err := detectFile(src)
	if err != nil || k.ID != "png" {
		t.Fatalf("detect png: %v %q", err, k.ID)
	}
	out := planSingle(src, "txt")
	if _, err := convert(context.Background(), src, k,
		Target{Ext: "txt", Engine: "ocr"}, out, "", nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(b), "HELLO") {
		t.Fatalf("bad ocr: %v %q", err, b)
	}
}
