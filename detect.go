package main

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kind is what a file is. Magic bytes decide, the extension only breaks ties.
type Kind struct {
	ID     string // canonical extension: png, jpg, tar.gz, docx, ...
	Family string // image, video, audio, pdf, archive, doc, sheet, ...
	Label  string // human label
}

type kindDef struct {
	family string
	label  string
}

var kinds = map[string]kindDef{
	"png":     {"image", "PNG image"},
	"jpg":     {"image", "JPEG image"},
	"gif":     {"image", "GIF image"},
	"webp":    {"image", "WebP image"},
	"bmp":     {"image", "BMP image"},
	"tiff":    {"image", "TIFF image"},
	"ico":     {"image", "Icon"},
	"avif":    {"image", "AVIF image"},
	"heic":    {"image", "HEIC image"},
	"jxl":     {"image", "JPEG XL image"},
	"psd":     {"image", "Photoshop document"},
	"tga":     {"image", "Targa image"},
	"svg":     {"vector", "SVG image"},
	"eps":     {"vector", "EPS vector"},
	"ps":      {"vector", "PostScript"},
	"pdf":     {"pdf", "PDF document"},
	"mp4":     {"video", "MP4 video"},
	"mkv":     {"video", "Matroska video"},
	"webm":    {"video", "WebM video"},
	"mov":     {"video", "QuickTime video"},
	"avi":     {"video", "AVI video"},
	"m4v":     {"video", "MPEG-4 video"},
	"mp3":     {"audio", "MP3 audio"},
	"wav":     {"audio", "WAV audio"},
	"flac":    {"audio", "FLAC audio"},
	"ogg":     {"audio", "OGG audio"},
	"opus":    {"audio", "Opus audio"},
	"m4a":     {"audio", "M4A audio"},
	"aac":     {"audio", "AAC audio"},
	"zip":     {"archive", "ZIP archive"},
	"tar":     {"archive", "TAR archive"},
	"tar.gz":  {"archive", "TAR.GZ archive"},
	"tar.xz":  {"archive", "TAR.XZ archive"},
	"tar.zst": {"archive", "TAR.ZST archive"},
	"tar.bz2": {"archive", "TAR.BZ2 archive"},
	"gz":      {"archive", "GZIP file"},
	"xz":      {"archive", "XZ file"},
	"zst":     {"archive", "Zstandard file"},
	"bz2":     {"archive", "BZIP2 file"},
	"7z":      {"archive", "7-Zip archive"},
	"rar":     {"archive", "RAR archive"},
	"srt":     {"subtitle", "SubRip subtitles"},
	"vtt":     {"subtitle", "WebVTT subtitles"},
	"ass":     {"subtitle", "ASS subtitles"},
	"ssa":     {"subtitle", "SSA subtitles"},
	"csv":     {"data", "CSV data"},
	"tsv":     {"data", "TSV data"},
	"json":    {"data", "JSON data"},
	"yaml":    {"data", "YAML data"},
	"yml":     {"data", "YAML data"},
	"xml":     {"data", "XML data"},
	"txt":     {"text", "Text file"},
	"md":      {"text", "Markdown"},
	"html":    {"text", "HTML page"},
	"log":     {"text", "Log file"},
	"tex":     {"text", "LaTeX source"},
	"docx":    {"doc", "Word document"},
	"doc":     {"doc", "Word document"},
	"odt":     {"doc", "OpenDocument text"},
	"rtf":     {"doc", "Rich text"},
	"epub":    {"ebook", "EPUB book"},
	"mobi":    {"ebook", "Mobipocket book"},
	"azw3":    {"ebook", "Kindle book"},
	"xlsx":    {"sheet", "Excel workbook"},
	"xls":     {"sheet", "Excel workbook"},
	"ods":     {"sheet", "OpenDocument sheet"},
	"pptx":    {"slide", "PowerPoint deck"},
	"ppt":     {"slide", "PowerPoint deck"},
	"odp":     {"slide", "OpenDocument slides"},
	"ttf":     {"font", "TrueType font"},
	"otf":     {"font", "OpenType font"},
	"woff":    {"font", "WOFF font"},
	"woff2":   {"font", "WOFF2 font"},
}

func kindOf(id string) Kind {
	if d, ok := kinds[id]; ok {
		return Kind{ID: id, Family: d.family, Label: d.label}
	}
	return Kind{ID: id, Family: "unknown", Label: strings.ToUpper(id) + " file"}
}

var textExt = map[string]string{
	"srt": "srt", "vtt": "vtt", "ass": "ass", "ssa": "ssa",
	"csv": "csv", "tsv": "tsv", "json": "json",
	"yaml": "yaml", "yml": "yml", "xml": "xml",
	"txt": "txt", "md": "md", "markdown": "md", "html": "html", "htm": "html",
	"log": "log", "tex": "tex",
	"docx": "docx", "odt": "odt", "rtf": "rtf",
	"mobi": "mobi", "azw3": "azw3",
}

type compoundExt struct{ suffix, id string }

var compoundExts = []compoundExt{
	{".tar.gz", "tar.gz"}, {".tgz", "tar.gz"},
	{".tar.xz", "tar.xz"}, {".txz", "tar.xz"},
	{".tar.zst", "tar.zst"}, {".tzst", "tar.zst"},
	{".tar.bz2", "tar.bz2"}, {".tbz2", "tar.bz2"},
}

// splitExt returns the canonical compound extension and the plain one.
func splitExt(name string) (compound, plain string) {
	lower := strings.ToLower(filepath.Base(name))
	for _, c := range compoundExts {
		if strings.HasSuffix(lower, c.suffix) {
			return c.id, c.id
		}
	}
	ext := strings.TrimPrefix(path.Ext(lower), ".")
	return "", ext
}

// detectFile reads the first 4 KB of the file for magic-byte detection.
// File types whose magic signature lives beyond 4 KB fall back to extension
// guessing instead of being misidentified.
func detectFile(p string) (Kind, error) {
	f, err := os.Open(p)
	if err != nil {
		return Kind{}, err
	}
	defer f.Close()
	head := make([]byte, 4096)
	n, _ := f.Read(head)
	return detect(head[:n], p), nil
}

func detect(b []byte, name string) Kind {
	has := func(off int, s string) bool {
		return len(b) >= off+len(s) && string(b[off:off+len(s)]) == s
	}
	has0 := func(s string) bool { return bytes.HasPrefix(b, []byte(s)) }
	compound, ext := splitExt(name)

	switch {
	case len(b) >= 12 && string(b[4:8]) == "ftyp":
		brand := string(b[8:12])
		switch brand {
		case "avif", "avis":
			return kindOf("avif")
		case "heic", "heix", "hevc", "heim", "heis", "mif1", "msf1", "hevm":
			return kindOf("heic")
		case "qt  ":
			return kindOf("mov")
		case "M4A ", "M4B ", "M4P ":
			return kindOf("m4a")
		case "M4V ", "M4VH", "M4VP":
			return kindOf("m4v")
		}
		if ext == "mov" {
			return kindOf("mov")
		}
		if ext == "m4a" || ext == "m4b" {
			return kindOf("m4a")
		}
		return kindOf("mp4")
	case has(0, "\x89PNG\r\n\x1a\n"):
		return kindOf("png")
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return kindOf("jpg")
	case has0("GIF87a") || has0("GIF89a"):
		return kindOf("gif")
	case len(b) >= 12 && has0("RIFF"):
		switch string(b[8:12]) {
		case "WEBP":
			return kindOf("webp")
		case "WAVE":
			return kindOf("wav")
		case "AVI ":
			return kindOf("avi")
		}
	case len(b) >= 26 && b[0] == 'B' && b[1] == 'M' && b[6] == 0 && b[7] == 0 && b[8] == 0 && b[9] == 0:
		return kindOf("bmp")
	case len(b) >= 4 && (has0("II*\x00") || has0("MM\x00*")):
		return kindOf("tiff")
	case len(b) >= 4 && has0("\x00\x00\x01\x00"):
		return kindOf("ico")
	case has0("8BPS"):
		return kindOf("psd")
	case len(b) >= 12 && string(b[4:12]) == "JXL \r\n\x87\n":
		return kindOf("jxl")
	case len(b) >= 2 && b[0] == 0xFF && b[1] == 0x0A:
		return kindOf("jxl")
	case has0("%PDF"):
		return kindOf("pdf")
	case has0("%!PS"):
		if bytes.Contains(b, []byte("EPSF")) {
			return kindOf("eps")
		}
		return kindOf("ps")
	case has0("PK\x03\x04") || has0("PK\x05\x06") || has0("PK\x07\x08"):
		switch ext {
		case "docx", "doc":
			return kindOf("docx")
		case "xlsx", "xls":
			return kindOf("xlsx")
		case "pptx", "ppt":
			return kindOf("pptx")
		case "odt", "ods", "odp":
			return kindOf(ext)
		case "epub":
			return kindOf("epub")
		}
		return kindOf("zip")
	case has0("7z\xBC\xAF\x27\x1C"):
		return kindOf("7z")
	case has0("Rar!\x1a\x07"):
		return kindOf("rar")
	case len(b) >= 2 && b[0] == 0x1F && b[1] == 0x8B:
		if compound == "tar.gz" {
			return kindOf("tar.gz")
		}
		return kindOf("gz")
	case has0("BZh"):
		if compound == "tar.bz2" {
			return kindOf("tar.bz2")
		}
		return kindOf("bz2")
	case has0("\xFD7zXZ\x00"):
		if compound == "tar.xz" {
			return kindOf("tar.xz")
		}
		return kindOf("xz")
	case has0("\x28\xB5\x2F\xFD"):
		if compound == "tar.zst" {
			return kindOf("tar.zst")
		}
		return kindOf("zst")
	case len(b) >= 265 && has(257, "ustar"):
		return kindOf("tar")
	case has0("OggS"):
		return kindOf("ogg")
	case has0("fLaC"):
		return kindOf("flac")
	case has0("ID3"):
		return kindOf("mp3")
	case len(b) >= 2 && b[0] == 0xFF && b[1]&0xE0 == 0xE0:
		return kindOf("mp3")
	case has0("\x1A\x45\xDF\xA3"):
		if bytes.Contains(b, []byte("webm")) {
			return kindOf("webm")
		}
		return kindOf("mkv")
	case len(b) >= 4 && (has0("\x00\x01\x00\x00") || has0("true") || has0("typ1")):
		return kindOf("ttf")
	case has0("OTTO"):
		return kindOf("otf")
	case has0("wOFF"):
		return kindOf("woff")
	case has0("wOF2"):
		return kindOf("woff2")
	}

	if utf8.Valid(b) && !binaryControl(b) {
		low := strings.ToLower(string(b))
		if strings.Contains(low, "<svg") {
			return kindOf("svg")
		}
		if id, ok := textExt[ext]; ok {
			return kindOf(id)
		}
		trimmed := strings.TrimSpace(low)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			return kindOf("json")
		}
		return kindOf("txt")
	}

	if _, ok := kinds[ext]; ok {
		return kindOf(ext)
	}
	id := ext
	if id == "" {
		id = "bin"
	}
	return Kind{ID: id, Family: "unknown", Label: strings.ToUpper(id) + " file"}
}

func binaryControl(b []byte) bool {
	for _, c := range b {
		if c < 0x09 || (c > 0x0D && c < 0x20) {
			return true
		}
		if !unicode.IsPrint(rune(c)) && c >= 0x80 {
			// allow valid UTF-8 multibyte sequences; utf8.Valid already checked
			continue
		}
	}
	return false
}
