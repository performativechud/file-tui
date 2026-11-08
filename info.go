package main

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// probeInfo returns one short metadata line for the source card.
func probeInfo(p string, k Kind, tools *Tools) string {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	switch k.Family {
	case "image", "vector":
		if tools.Bins["magick"] {
			// Feed the file via stdin so a filename containing "[" or "]" is
			// never interpreted as an ImageMagick frame selector or coder.
			if f, err := os.Open(p); err == nil {
				cmd := exec.CommandContext(ctx, "magick", "identify", "-format", "%wx%h", "fd:0")
				cmd.Stdin = f
				out, err := cmd.Output()
				f.Close()
				if err == nil {
					return strings.TrimSpace(string(out))
				}
			}
		}
	case "video", "audio":
		if tools.Bins["ffprobe"] {
			return ffprobeInfo(ctx, p, k.Family == "video")
		}
	case "pdf":
		if tools.Bins["pdfinfo"] {
			out, err := exec.CommandContext(ctx, "pdfinfo", p).Output()
			if err == nil {
				return pdfInfo(out)
			}
		}
	}
	return ""
}

func ffprobeInfo(ctx context.Context, p string, video bool) string {
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error",
		"-show_entries", "format=duration:stream=codec_name,width,height",
		"-of", "default=noprint_wrappers=1", p).Output()
	if err != nil {
		return ""
	}
	var codec, dur string
	var w, h int
	for _, line := range strings.Split(string(out), "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "codec_name":
			if codec == "" && val != "" {
				codec = val
			}
		case "width":
			w, _ = strconv.Atoi(val)
		case "height":
			h, _ = strconv.Atoi(val)
		case "duration":
			dur = val
		}
	}
	var parts []string
	if video && w > 0 && h > 0 {
		parts = append(parts, strconv.Itoa(w)+"×"+strconv.Itoa(h))
	}
	if codec != "" {
		parts = append(parts, codec)
	}
	if d, err := strconv.ParseFloat(dur, 64); err == nil && d > 0 {
		parts = append(parts, strconv.FormatFloat(d, 'f', 1, 64)+"s")
	}
	return strings.Join(parts, " · ")
}

func pdfInfo(out []byte) string {
	var pages, size string
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "Pages:"):
			pages = strings.TrimSpace(strings.TrimPrefix(line, "Pages:"))
		case strings.HasPrefix(line, "Page size:"):
			size = strings.TrimSpace(strings.TrimPrefix(line, "Page size:"))
			if i := strings.Index(size, " ("); i > 0 {
				size = size[:i]
			}
		}
	}
	var parts []string
	if pages != "" {
		parts = append(parts, pages+" pages")
	}
	if size != "" {
		parts = append(parts, size)
	}
	return strings.Join(parts, " · ")
}
