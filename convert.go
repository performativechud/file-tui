package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Progress struct {
	Percent int // -1 when the tool cannot report progress
	Label   string
}

type Result struct {
	Out     string
	Size    int64
	Elapsed time.Duration
}

// srcBase is the original name without its extension, compound ones included.
func srcBase(p string) string {
	base := filepath.Base(p)
	lower := strings.ToLower(base)
	for _, c := range []string{".tar.gz", ".tar.xz", ".tar.zst", ".tar.bz2", ".tgz", ".txz", ".tzst", ".tbz2"} {
		if strings.HasSuffix(lower, c) {
			return base[:len(base)-len(c)]
		}
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func planSingle(src, ext string) string {
	dir := filepath.Dir(src)
	out := filepath.Join(dir, "file-tui-"+srcBase(src)+"."+ext)
	if _, err := os.Stat(out); os.IsNotExist(err) {
		return out
	}
	for i := 2; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("file-tui-%s-%d.%s", srcBase(src), i, ext))
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}

// planMultiBase returns the pdftoppm base. Output files are <base>-<page>.<ext>.
func planMultiBase(src, ext string) string {
	dir := filepath.Dir(src)
	base := filepath.Join(dir, "file-tui-"+srcBase(src))
	used := func(b string) bool {
		m, _ := filepath.Glob(b + "-*." + ext)
		return len(m) > 0
	}
	if !used(base) {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if !used(cand) {
			return cand
		}
	}
}

func planFor(src string, t Target) (out, base string) {
	if t.Multi {
		base = planMultiBase(src, t.Ext)
		return base + "-1." + t.Ext, base
	}
	return planSingle(src, t.Ext), ""
}

func convert(ctx context.Context, src string, k Kind, t Target, out, base string, emit func(Progress)) (Result, error) {
	start := time.Now()
	if emit == nil {
		emit = func(Progress) {}
	}

	var err error
	switch t.Engine {
	case "magick":
		err = runCmd(ctx, "magick", magickArgs(src, k, t, out), nil)
	case "pdfimg":
		args := []string{}
		switch t.Arg {
		case "png":
			args = append(args, "-png")
		case "jpg":
			args = append(args, "-jpeg", "-jpegopt", "quality=90")
		case "tiff":
			args = append(args, "-tiff", "-tiffcompression", "deflate")
		}
		args = append(args, "-r", "150", src, base)
		err = runCmd(ctx, "pdftoppm", args, nil)
	case "pdftotext":
		err = runCmd(ctx, "pdftotext", []string{"-layout", src, out}, nil)
	case "ffvideo", "ffaudio", "ffframe", "ffsub":
		args, dur := ffmpegArgs(ctx, src, t, out)
		err = runCmd(ctx, "ffmpeg", args, func(line string) {
			ffmpegProgress(line, dur, emit)
		})
	case "archive":
		err = convertArchive(ctx, src, k, t, out, emit)
	case "data":
		err = convertData(src, t, out)
	case "pandoc":
		if t.Ext == "pdf" {
			err = convertPandocPDF(ctx, src, out)
		} else {
			err = runCmd(ctx, "pandoc", []string{src, "-o", out}, nil)
		}
	case "libreoffice":
		err = convertLibreOffice(ctx, src, t, out)
	case "calibre":
		err = runCmd(ctx, "ebook-convert", []string{src, out}, nil)
	case "ocr":
		stem := strings.TrimSuffix(out, ".txt")
		err = runCmd(ctx, "tesseract", []string{src, stem, "-l", "eng"}, nil)
	default:
		err = fmt.Errorf("no converter for .%s", t.Ext)
	}
	if err != nil {
		// A killed converter can leave a truncated file behind; never let it
		// look like a finished conversion.
		removeOutputs(out, base, t)
		return Result{}, err
	}

	res := Result{Out: out, Elapsed: time.Since(start)}
	if t.Multi {
		res.Out = base + "-<page>." + t.Ext
		res.Size = globSize(base + "-*." + t.Ext)
		if res.Size == 0 {
			return Result{}, fmt.Errorf("converter reported success but wrote no pages")
		}
	} else {
		fi, statErr := os.Stat(out)
		if statErr != nil {
			return Result{}, fmt.Errorf("converter reported success but wrote no output")
		}
		res.Size = fi.Size()
	}
	return res, nil
}

func removeOutputs(out, base string, t Target) {
	if !t.Multi {
		_ = os.Remove(out)
		return
	}
	m, _ := filepath.Glob(base + "-*." + t.Ext)
	for _, p := range m {
		_ = os.Remove(p)
	}
}

func magickArgs(src string, k Kind, t Target, out string) []string {
	var a []string
	if k.Family == "vector" {
		a = append(a, "-density", "300")
	}
	a = append(a, src+"[0]")
	if t.Ext == "jpg" {
		// A transparent PNG flattened to JPEG would otherwise turn black.
		a = append(a, "-background", "white", "-alpha", "remove", "-alpha", "off")
	}
	return append(a, out)
}

func ffmpegArgs(ctx context.Context, src string, t Target, out string) ([]string, float64) {
	a := []string{"-hide_banner", "-loglevel", "error", "-nostats", "-progress", "pipe:1", "-y", "-i", src}

	switch t.Engine {
	case "ffvideo":
		switch t.Arg {
		case "mp4":
			a = append(a, "-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
				"-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart")
		case "mkv":
			a = append(a, "-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
				"-c:a", "aac", "-b:a", "192k")
		case "webm":
			a = append(a, "-c:v", "libvpx-vp9", "-crf", "34", "-b:v", "0",
				"-c:a", "libopus", "-b:a", "128k")
		case "mov":
			a = append(a, "-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
				"-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart")
		case "avi":
			a = append(a, "-c:v", "mpeg4", "-q:v", "4", "-c:a", "libmp3lame", "-q:a", "4")
		case "m4v":
			a = append(a, "-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
				"-c:a", "aac", "-b:a", "192k")
		case "gif":
			a = append(a, "-filter_complex",
				"fps=12,scale=480:-1:flags=lanczos,split[a][b];[a]palettegen[p];[b][p]paletteuse",
				"-loop", "0", "-an")
		}
	case "ffaudio":
		switch t.Arg {
		case "mp3":
			a = append(a, "-vn", "-c:a", "libmp3lame", "-q:a", "2")
		case "wav":
			a = append(a, "-vn", "-c:a", "pcm_s16le")
		case "flac":
			a = append(a, "-vn", "-c:a", "flac", "-compression_level", "5")
		case "ogg":
			a = append(a, "-vn", "-c:a", "libvorbis", "-q:a", "5")
		case "opus":
			a = append(a, "-vn", "-c:a", "libopus", "-b:a", "128k")
		case "m4a":
			a = append(a, "-vn", "-c:a", "aac", "-b:a", "192k")
		case "aac":
			a = append(a, "-vn", "-c:a", "aac", "-b:a", "192k")
		}
	case "ffframe":
		a = append(a, "-frames:v", "1")
		if t.Arg == "jpg" {
			a = append(a, "-q:v", "2")
		}
	case "ffsub":
		switch t.Arg {
		case "srt":
			a = append(a, "-c:s", "srt")
		case "vtt":
			a = append(a, "-c:s", "webvtt")
		case "ass":
			a = append(a, "-c:s", "ass")
		}
	}
	a = append(a, out)
	return a, probeDuration(ctx, src)
}

func probeDuration(ctx context.Context, src string) float64 {
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(c, "ffprobe", "-v", "error",
		"-show_entries", "format=duration", "-of", "csv=p=0", src).Output()
	if err != nil {
		return 0
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return d
}

func ffmpegProgress(line string, dur float64, emit func(Progress)) {
	val := ""
	switch {
	case strings.HasPrefix(line, "out_time_us="):
		val = strings.TrimPrefix(line, "out_time_us=")
	case strings.HasPrefix(line, "out_time_ms="):
		// ffmpeg reports out_time_ms in milliseconds, not microseconds.
		// Parse it with a 1e3 divisor so the percentage is correct.
		val = strings.TrimPrefix(line, "out_time_ms=")
		us, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err != nil {
			return
		}
		pct := int(float64(us) / 1e3 / dur * 100)
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		emit(Progress{Percent: pct})
		return
	case line == "progress=end":
		emit(Progress{Percent: 100})
		return
	default:
		return
	}
	if dur <= 0 {
		return
	}
	us, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
	if err != nil {
		return
	}
	pct := int(float64(us) / 1e6 / dur * 100)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	emit(Progress{Percent: pct})
}

// convertPandocPDF prefers a real TeX engine and falls back to printing
// pandoc's HTML with LibreOffice when none is installed.
func convertPandocPDF(ctx context.Context, src, out string) error {
	for _, b := range []string{"pdflatex", "xelatex", "lualatex", "tectonic", "typst"} {
		if _, err := exec.LookPath(b); err == nil {
			return runCmd(ctx, "pandoc", []string{src, "-o", out, "--pdf-engine=" + b}, nil)
		}
	}
	f, err := os.CreateTemp(filepath.Dir(out), ".file-tui-*.html")
	if err != nil {
		return err
	}
	html := f.Name()
	f.Close()
	defer os.Remove(html)
	if err := runCmd(ctx, "pandoc", []string{src, "-s", "-o", html}, nil); err != nil {
		return err
	}
	return convertLibreOffice(ctx, html, Target{Ext: "pdf"}, out)
}

func convertLibreOffice(ctx context.Context, src string, t Target, out string) error {
	tmp, err := os.MkdirTemp("", "file-tui-lo-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	bin := "soffice"
	if _, err := exec.LookPath(bin); err != nil {
		bin = "libreoffice"
	}
	if err := runCmd(ctx, bin, []string{"--headless", "--convert-to", t.Ext, "--outdir", tmp, src}, nil); err != nil {
		return err
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.EqualFold(filepath.Ext(e.Name()), "."+t.Ext) {
			srcPath := filepath.Join(tmp, e.Name())
			if err := copyFile(srcPath, out); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("LibreOffice produced no .%s output", t.Ext)
}

// copyFile copies src to dst, falling back to read+write when rename fails
// (e.g. src and dst are on different filesystems).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

type tailBuffer struct{ b []byte }

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if len(t.b) > 8192 {
		t.b = t.b[len(t.b)-8192:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return string(t.b) }

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l != "" {
			return l
		}
	}
	return ""
}

// runCmd runs a converter, streams stdout lines, and kills the whole process
// group on cancel so ffmpeg's children die too.
func runCmd(ctx context.Context, name string, args []string, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, name, args...)
	configureCmd(cmd)
	var errBuf tailBuffer
	cmd.Stderr = &errBuf
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if onLine != nil {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			onLine(sc.Text())
		}
	} else {
		_, _ = io.Copy(io.Discard, stdout)
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return context.Canceled
		}
		msg := lastLine(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s: %s", filepath.Base(name), msg)
	}
	return nil
}

func globSize(pattern string) int64 {
	m, _ := filepath.Glob(pattern)
	var n int64
	for _, p := range m {
		if fi, err := os.Stat(p); err == nil {
			n += fi.Size()
		}
	}
	return n
}

func convertData(src string, t Target, out string) error {
	in, err := readTable(src)
	if err != nil {
		return err
	}
	switch t.Ext {
	case "json":
		return writeJSONTable(out, in)
	case "csv":
		return writeCSVTable(out, in, ',')
	case "tsv":
		return writeCSVTable(out, in, '\t')
	}
	return fmt.Errorf("no data converter for .%s", t.Ext)
}

type table struct {
	header []string
	rows   [][]string
}

const maxDataFileSize = 64 << 20 // 64 MiB

func readTable(src string) (table, error) {
	fi, err := os.Stat(src)
	if err != nil {
		return table{}, err
	}
	if fi.Size() > maxDataFileSize {
		return table{}, fmt.Errorf("file too large for data conversion: %s", humanSize(fi.Size()))
	}
	switch strings.ToLower(filepath.Ext(src)) {
	case ".json":
		return readJSONTable(src)
	case ".csv":
		return readCSVTable(src, ',')
	case ".tsv":
		return readCSVTable(src, '\t')
	}
	return table{}, fmt.Errorf("data conversion supports csv, tsv and json")
}

func readCSVTable(src string, comma rune) (table, error) {
	f, err := os.Open(src)
	if err != nil {
		return table{}, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = comma
	r.FieldsPerRecord = -1
	all, err := r.ReadAll()
	if err != nil {
		return table{}, err
	}
	if len(all) == 0 {
		return table{}, nil
	}
	return table{header: all[0], rows: all[1:]}, nil
}

func readJSONTable(src string) (table, error) {
	b, err := os.ReadFile(src)
	if err != nil {
		return table{}, err
	}
	var objects []map[string]any
	if err := json.Unmarshal(b, &objects); err == nil {
		keys := map[string]bool{}
		for _, o := range objects {
			for k := range o {
				keys[k] = true
			}
		}
		header := make([]string, 0, len(keys))
		for k := range keys {
			header = append(header, k)
		}
		sort.Strings(header)
		rows := make([][]string, 0, len(objects))
		for _, o := range objects {
			row := make([]string, len(header))
			for i, k := range header {
				row[i] = jsonValue(o[k])
			}
			rows = append(rows, row)
		}
		return table{header: header, rows: rows}, nil
	}
	var arrays [][]any
	if err := json.Unmarshal(b, &arrays); err != nil {
		return table{}, fmt.Errorf("json must be an array of objects or arrays")
	}
	width := 0
	for _, a := range arrays {
		if len(a) > width {
			width = len(a)
		}
	}
	header := make([]string, width)
	for i := range header {
		header[i] = fmt.Sprintf("col%d", i+1)
	}
	rows := make([][]string, 0, len(arrays))
	for _, a := range arrays {
		row := make([]string, width)
		for i := range row {
			if i < len(a) {
				row[i] = jsonValue(a[i])
			}
		}
		rows = append(rows, row)
	}
	return table{header: header, rows: rows}, nil
}

func jsonValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func writeCSVTable(out string, t table, comma rune) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Comma = comma
	if len(t.header) > 0 {
		if err := w.Write(t.header); err != nil {
			return err
		}
	}
	for _, row := range t.rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeJSONTable(out string, t table) error {
	objects := make([]map[string]string, 0, len(t.rows))
	for _, row := range t.rows {
		o := make(map[string]string, len(t.header))
		for i, k := range t.header {
			if i < len(row) {
				o[k] = row[i]
			}
		}
		objects = append(objects, o)
	}
	b, err := json.MarshalIndent(objects, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(out, b, 0o644)
}
