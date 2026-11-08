package main

import (
	"sort"
	"strings"
)

// Target is one row in the convert menu.
type Target struct {
	Ext    string // output extension, may contain a dot: tar.gz
	Label  string // short name shown in the list
	Desc   string // one line human description
	Engine string // converter backend
	Arg    string // engine specific variant: codec key, subtitle format
	Need   string // capability probe key
	Multi  bool   // produces numbered files, one per page
	Warn   string // shown next to the description
}

func (t Target) Available(tools *Tools) (bool, string) {
	return tools.has(t.Need)
}

// Tools is the probed capability set. Nothing is assumed: a row is only
// offered when the machine can actually produce it.
type Tools struct {
	Bins     map[string]bool
	Magick   map[string]bool // writable ImageMagick format codes
	Encoders map[string]bool // ffmpeg encoders
	Langs    map[string]bool // tesseract language packs
}

// has reports whether a probe key is satisfied, with a human readable reason.
func (t *Tools) has(need string) (bool, string) {
	if need == "" || need == "go" {
		return true, ""
	}
	kind, val, ok := strings.Cut(need, ":")
	if !ok {
		kind, val = need, ""
	}
	switch kind {
	case "bin":
		names := strings.Split(val, "|")
		for _, n := range names {
			if t.Bins[n] {
				return true, ""
			}
		}
		return false, reasonFor(names[0])
	case "magick":
		if !t.Bins["magick"] {
			return false, reasonFor("magick")
		}
		if t.Magick[val] {
			return true, ""
		}
		return false, "ImageMagick cannot write " + val
	case "enc":
		if !t.Bins["ffmpeg"] {
			return false, reasonFor("ffmpeg")
		}
		for _, e := range strings.Split(val, ",") {
			if !t.Encoders[e] {
				return false, "ffmpeg build lacks " + e
			}
		}
		return true, ""
	case "ocr":
		if !t.Bins["tesseract"] {
			return false, reasonFor("tesseract")
		}
		if t.Langs[val] {
			return true, ""
		}
		return false, reasonFor("tesseract-data-" + val)
	case "pdfout":
		if !t.Bins["pandoc"] {
			return false, reasonFor("pandoc")
		}
		for _, b := range []string{"pdflatex", "xelatex", "lualatex", "tectonic", "typst"} {
			if t.Bins[b] {
				return true, ""
			}
		}
		if t.Bins["soffice"] || t.Bins["libreoffice"] {
			return true, ""
		}
		return false, "install libreoffice or texlive"
	}
	return false, "unknown requirement"
}

var needReason = map[string]string{
	"magick":             "install imagemagick",
	"ffmpeg":             "install ffmpeg",
	"pdftoppm":           "install poppler",
	"pdftotext":          "install poppler",
	"pandoc":             "install pandoc",
	"libreoffice":        "install libreoffice",
	"soffice":            "install libreoffice",
	"7z":                 "install 7zip",
	"tesseract":          "install tesseract",
	"tesseract-data-eng": "install tesseract-data-eng",
	"calibre":            "install calibre",
	"ebook-convert":      "install calibre",
	"xz":                 "install xz",
	"zstd":               "install zstd",
	"bzip2":              "install bzip2",
	"ghostscript":        "install ghostscript",
	"libx264":            "ffmpeg build lacks libx264",
	"aac":                "ffmpeg build lacks aac",
	"libvpx-vp9":         "ffmpeg build lacks libvpx-vp9",
	"libopus":            "ffmpeg build lacks libopus",
	"libmp3lame":         "ffmpeg build lacks libmp3lame",
	"libvorbis":          "ffmpeg build lacks libvorbis",
	"libtheora":          "ffmpeg build lacks libtheora",
	"mpeg4":              "ffmpeg build lacks mpeg4",
	"png":                "ffmpeg build lacks png",
	"mjpeg":              "ffmpeg build lacks mjpeg",
	"gif":                "ffmpeg build lacks gif",
	"flac":               "ffmpeg build lacks flac",
	"pcm_s16le":          "ffmpeg build lacks pcm_s16le",
}

func reasonFor(name string) string {
	if r, ok := needReason[name]; ok {
		return r
	}
	return "requires " + name
}

func imageTargets() []Target {
	return []Target{
		{Ext: "png", Label: "PNG", Desc: "lossless, larger", Engine: "magick", Need: "magick:PNG"},
		{Ext: "jpg", Label: "JPG", Desc: "small, universal", Engine: "magick", Need: "magick:JPEG"},
		{Ext: "webp", Label: "WEBP", Desc: "small, modern", Engine: "magick", Need: "magick:WEBP"},
		{Ext: "avif", Label: "AVIF", Desc: "smallest, slow encode", Engine: "magick", Need: "magick:AVIF"},
		{Ext: "gif", Label: "GIF", Desc: "palette, can animate", Engine: "magick", Need: "magick:GIF"},
		{Ext: "bmp", Label: "BMP", Desc: "uncompressed", Engine: "magick", Need: "magick:BMP"},
		{Ext: "tiff", Label: "TIFF", Desc: "print, lossless", Engine: "magick", Need: "magick:TIFF"},
		{Ext: "ico", Label: "ICO", Desc: "favicon sizes", Engine: "magick", Need: "magick:ICO"},
		{Ext: "heic", Label: "HEIC", Desc: "apple photos", Engine: "magick", Need: "magick:HEIC"},
		{Ext: "jxl", Label: "JXL", Desc: "jpeg xl", Engine: "magick", Need: "magick:JXL"},
		{Ext: "psd", Label: "PSD", Desc: "photoshop layers", Engine: "magick", Need: "magick:PSD"},
		{Ext: "tga", Label: "TGA", Desc: "game texture", Engine: "magick", Need: "magick:TGA"},
		{Ext: "svg", Label: "SVG", Desc: "flattened vector", Engine: "magick", Need: "magick:SVG"},
	}
}

func audioTargets() []Target {
	return []Target{
		{Ext: "mp3", Label: "MP3", Desc: "universal lossy", Engine: "ffaudio", Arg: "mp3", Need: "enc:libmp3lame"},
		{Ext: "wav", Label: "WAV", Desc: "uncompressed pcm", Engine: "ffaudio", Arg: "wav", Need: "enc:pcm_s16le"},
		{Ext: "flac", Label: "FLAC", Desc: "lossless", Engine: "ffaudio", Arg: "flac", Need: "enc:flac"},
		{Ext: "ogg", Label: "OGG", Desc: "vorbis", Engine: "ffaudio", Arg: "ogg", Need: "enc:libvorbis"},
		{Ext: "opus", Label: "OPUS", Desc: "best at low bitrate", Engine: "ffaudio", Arg: "opus", Need: "enc:libopus"},
		{Ext: "m4a", Label: "M4A", Desc: "aac in mp4", Engine: "ffaudio", Arg: "m4a", Need: "enc:aac"},
		{Ext: "aac", Label: "AAC", Desc: "raw aac", Engine: "ffaudio", Arg: "aac", Need: "enc:aac"},
	}
}

func videoTargets() []Target {
	return []Target{
		{Ext: "mp4", Label: "MP4", Desc: "h264 + aac, universal", Engine: "ffvideo", Arg: "mp4", Need: "enc:libx264,aac"},
		{Ext: "mkv", Label: "MKV", Desc: "h264 + aac, container", Engine: "ffvideo", Arg: "mkv", Need: "enc:libx264,aac"},
		{Ext: "webm", Label: "WEBM", Desc: "vp9 + opus, web", Engine: "ffvideo", Arg: "webm", Need: "enc:libvpx-vp9,libopus"},
		{Ext: "mov", Label: "MOV", Desc: "quicktime", Engine: "ffvideo", Arg: "mov", Need: "enc:libx264,aac"},
		{Ext: "avi", Label: "AVI", Desc: "legacy mpeg4", Engine: "ffvideo", Arg: "avi", Need: "enc:mpeg4,libmp3lame"},
		{Ext: "m4v", Label: "M4V", Desc: "itunes video", Engine: "ffvideo", Arg: "m4v", Need: "enc:libx264,aac"},
		{Ext: "gif", Label: "GIF", Desc: "480px, 12 fps", Engine: "ffvideo", Arg: "gif", Need: "enc:gif"},
		{Ext: "mp3", Label: "MP3", Desc: "extract audio", Engine: "ffaudio", Arg: "mp3", Need: "enc:libmp3lame"},
		{Ext: "wav", Label: "WAV", Desc: "extract audio", Engine: "ffaudio", Arg: "wav", Need: "enc:pcm_s16le"},
		{Ext: "flac", Label: "FLAC", Desc: "extract audio", Engine: "ffaudio", Arg: "flac", Need: "enc:flac"},
		{Ext: "m4a", Label: "M4A", Desc: "extract audio", Engine: "ffaudio", Arg: "m4a", Need: "enc:aac"},
		{Ext: "opus", Label: "OPUS", Desc: "extract audio", Engine: "ffaudio", Arg: "opus", Need: "enc:libopus"},
		{Ext: "png", Label: "PNG", Desc: "first frame", Engine: "ffframe", Arg: "png", Need: "enc:png"},
		{Ext: "jpg", Label: "JPG", Desc: "first frame", Engine: "ffframe", Arg: "jpg", Need: "enc:mjpeg"},
	}
}

func subtitleTargets() []Target {
	return []Target{
		{Ext: "srt", Label: "SRT", Desc: "subrip", Engine: "ffsub", Arg: "srt", Need: "bin:ffmpeg"},
		{Ext: "vtt", Label: "VTT", Desc: "webvtt", Engine: "ffsub", Arg: "vtt", Need: "bin:ffmpeg"},
		{Ext: "ass", Label: "ASS", Desc: "advanced subs", Engine: "ffsub", Arg: "ass", Need: "bin:ffmpeg"},
	}
}

func archiveTargets(srcArchive bool) []Target {
	return []Target{
		{Ext: "zip", Label: "ZIP", Desc: "deflate", Engine: "archive", Need: "go"},
		{Ext: "tar", Label: "TAR", Desc: "uncompressed", Engine: "archive", Need: "go"},
		{Ext: "tar.gz", Label: "TAR.GZ", Desc: "tar + gzip", Engine: "archive", Need: "go"},
		{Ext: "tar.xz", Label: "TAR.XZ", Desc: "tar + xz", Engine: "archive", Need: "bin:xz"},
		{Ext: "tar.zst", Label: "TAR.ZST", Desc: "tar + zstd", Engine: "archive", Need: "bin:zstd"},
	}
}

func dataTargets() []Target {
	return []Target{
		{Ext: "json", Label: "JSON", Desc: "objects, utf-8", Engine: "data", Need: "go"},
		{Ext: "csv", Label: "CSV", Desc: "comma separated", Engine: "data", Need: "go"},
		{Ext: "tsv", Label: "TSV", Desc: "tab separated", Engine: "data", Need: "go"},
	}
}

// targetsFor returns every logical target for a source kind. Rows a missing
// tool cannot serve stay in the list so the UI can explain why they are locked.
func targetsFor(k Kind, tools *Tools) []Target {
	var out []Target
	switch k.Family {
	case "image":
		out = append(out, imageTargets()...)
		out = append(out,
			Target{Ext: "pdf", Label: "PDF", Desc: "document", Engine: "magick", Need: "magick:PDF"},
			Target{Ext: "txt", Label: "TXT", Desc: "ocr text", Engine: "ocr", Need: "ocr:eng"},
		)
	case "vector":
		if k.ID == "svg" {
			out = append(out, imageTargets()...)
			out = append(out, Target{Ext: "pdf", Label: "PDF", Desc: "document", Engine: "magick", Need: "magick:PDF"})
		} else {
			for _, t := range imageTargets() {
				t.Need = "bin:ghostscript"
				out = append(out, t)
			}
			out = append(out, Target{Ext: "pdf", Label: "PDF", Desc: "document", Engine: "magick", Need: "bin:ghostscript"})
		}
	case "pdf":
		out = append(out,
			Target{Ext: "png", Label: "PNG", Desc: "pages as images", Engine: "pdfimg", Arg: "png", Need: "bin:pdftoppm", Multi: true},
			Target{Ext: "jpg", Label: "JPG", Desc: "pages as images", Engine: "pdfimg", Arg: "jpg", Need: "bin:pdftoppm", Multi: true},
			Target{Ext: "tiff", Label: "TIFF", Desc: "pages as images", Engine: "pdfimg", Arg: "tiff", Need: "bin:pdftoppm", Multi: true},
			Target{Ext: "txt", Label: "TXT", Desc: "extracted text", Engine: "pdftotext", Need: "bin:pdftotext"},
			// PDFs open in LibreOffice Draw, which has no DOCX export filter.
			Target{Ext: "html", Label: "HTML", Desc: "text as web page", Engine: "libreoffice", Need: "bin:libreoffice|soffice", Warn: "text only, no layout"},
		)
	case "video":
		out = append(out, videoTargets()...)
	case "audio":
		out = append(out, audioTargets()...)
	case "subtitle":
		out = append(out, subtitleTargets()...)
	case "archive":
		out = append(out, archiveTargets(true)...)
		if k.ID == "7z" || k.ID == "rar" {
			for i := range out {
				out[i].Need = "bin:7z|unar"
			}
		}
	case "data":
		out = append(out, dataTargets()...)
	case "doc":
		out = append(out,
			Target{Ext: "pdf", Label: "PDF", Desc: "fixed layout", Engine: "libreoffice", Need: "bin:libreoffice|soffice"},
			Target{Ext: "html", Label: "HTML", Desc: "web page", Engine: "pandoc", Need: "bin:pandoc"},
			Target{Ext: "md", Label: "MD", Desc: "markdown", Engine: "pandoc", Need: "bin:pandoc"},
			Target{Ext: "txt", Label: "TXT", Desc: "plain text", Engine: "pandoc", Need: "bin:pandoc"},
			Target{Ext: "docx", Label: "DOCX", Desc: "word document", Engine: "pandoc", Need: "bin:pandoc"},
			Target{Ext: "epub", Label: "EPUB", Desc: "ebook", Engine: "pandoc", Need: "bin:pandoc"},
		)
	case "sheet":
		out = append(out,
			Target{Ext: "csv", Label: "CSV", Desc: "first sheet", Engine: "libreoffice", Need: "bin:libreoffice|soffice"},
			Target{Ext: "xlsx", Label: "XLSX", Desc: "excel workbook", Engine: "libreoffice", Need: "bin:libreoffice|soffice"},
			Target{Ext: "ods", Label: "ODS", Desc: "opendocument", Engine: "libreoffice", Need: "bin:libreoffice|soffice"},
			Target{Ext: "pdf", Label: "PDF", Desc: "fixed layout", Engine: "libreoffice", Need: "bin:libreoffice|soffice"},
		)
	case "slide":
		out = append(out,
			Target{Ext: "pdf", Label: "PDF", Desc: "fixed layout", Engine: "libreoffice", Need: "bin:libreoffice|soffice"},
			Target{Ext: "png", Label: "PNG", Desc: "first slide", Engine: "libreoffice", Need: "bin:libreoffice|soffice", Warn: "first slide only"},
		)
	case "ebook":
		out = append(out,
			Target{Ext: "epub", Label: "EPUB", Desc: "open ebook", Engine: "calibre", Need: "bin:ebook-convert"},
			Target{Ext: "mobi", Label: "MOBI", Desc: "kindle legacy", Engine: "calibre", Need: "bin:ebook-convert"},
			Target{Ext: "pdf", Label: "PDF", Desc: "fixed layout", Engine: "calibre", Need: "bin:ebook-convert"},
			Target{Ext: "txt", Label: "TXT", Desc: "plain text", Engine: "calibre", Need: "bin:ebook-convert"},
		)
	case "text":
		out = append(out,
			Target{Ext: "pdf", Label: "PDF", Desc: "document", Engine: "pandoc", Need: "pdfout"},
			Target{Ext: "html", Label: "HTML", Desc: "web page", Engine: "pandoc", Need: "bin:pandoc"},
			Target{Ext: "docx", Label: "DOCX", Desc: "word document", Engine: "pandoc", Need: "bin:pandoc"},
			Target{Ext: "epub", Label: "EPUB", Desc: "ebook", Engine: "pandoc", Need: "bin:pandoc"},
			Target{Ext: "md", Label: "MD", Desc: "markdown", Engine: "pandoc", Need: "bin:pandoc", Warn: "text only"},
		)
	}

	// Drop no-op rows: png -> png, mp3 -> mp3, zip -> zip.
	filtered := out[:0]
	for _, t := range out {
		if t.Ext == k.ID {
			continue
		}
		filtered = append(filtered, t)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		ai, _ := filtered[i].Available(tools)
		aj, _ := filtered[j].Available(tools)
		if ai != aj {
			return ai
		}
		return false
	})
	return filtered
}

func toolName(t Target) string {
	switch t.Engine {
	case "magick":
		return "ImageMagick"
	case "pdfimg", "pdftotext":
		return "poppler"
	case "ffvideo", "ffaudio", "ffframe", "ffsub":
		return "ffmpeg"
	case "archive":
		return "file-tui"
	case "data":
		return "file-tui"
	case "libreoffice":
		return "LibreOffice"
	case "pandoc":
		return "pandoc"
	case "calibre":
		return "Calibre"
	case "ocr":
		return "tesseract"
	}
	return t.Engine
}
