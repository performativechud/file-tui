package main

import (
	"os/exec"
	"strings"
)

func probeTools() *Tools {
	t := &Tools{
		Bins:     map[string]bool{},
		Magick:   map[string]bool{},
		Encoders: map[string]bool{},
		Langs:    map[string]bool{},
	}
	for _, b := range []string{
		"magick", "ffmpeg", "ffprobe", "pdftoppm", "pdftotext", "pdfinfo",
		"xz", "zstd", "bzip2", "7z", "7zz", "unar", "pandoc", "soffice",
		"libreoffice", "tesseract", "ebook-convert", "xdg-open", "ghostscript", "gs",
		"pdflatex", "xelatex", "lualatex", "tectonic", "typst",
	} {
		if _, err := exec.LookPath(b); err == nil {
			t.Bins[b] = true
		}
	}
	if t.Bins["ghostscript"] || t.Bins["gs"] {
		t.Bins["ghostscript"] = true
	}
	if t.Bins["7zz"] {
		t.Bins["7z"] = true
	}

	if t.Bins["magick"] {
		if out, err := exec.Command("magick", "-list", "format").Output(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				f := strings.Fields(line)
				if len(f) < 3 {
					continue
				}
				code := strings.TrimRight(f[0], "*")
				mode := f[2]
				if strings.Contains(mode, "w") {
					t.Magick[code] = true
				}
			}
		}
	}

	if t.Bins["ffmpeg"] {
		if out, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").Output(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				f := strings.Fields(line)
				if len(f) >= 2 && len(f[0]) == 6 && strings.ContainsAny(f[0][:1], "VAS") {
					t.Encoders[f[1]] = true
				}
			}
		}
	}

	// The tesseract package ships no language data on Arch; eng comes from
	// tesseract-data-eng. Without it every OCR run fails.
	if t.Bins["tesseract"] {
		if out, err := exec.Command("tesseract", "--list-langs").CombinedOutput(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.ContainsAny(line, ": ") {
					continue
				}
				t.Langs[line] = true
			}
		}
	}
	return t
}
