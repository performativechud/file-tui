package main

import (
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// resolvePath accepts everything a terminal might paste for a dropped file:
// plain paths, quoted paths, backslash escaped paths, file:// URIs, ~ and
// multi-line drops.
func resolvePath(raw string) (string, error) {
	for _, c := range inputCandidates(raw) {
		p := normalizePath(c)
		if p == "" {
			continue
		}
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			return "", fmt.Errorf("that is a directory, drop a file")
		}
		// Absolute paths keep decoder tools (ImageMagick, ffmpeg, poppler)
		// from reading a leading "|" or a URL scheme as a protocol.
		if abs, err := filepath.Abs(p); err == nil {
			return abs, nil
		}
		return p, nil
	}
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("drop a file or paste a path")
	}
	return "", fmt.Errorf("no such file: %s", trunc(s, 60))
}

func inputCandidates(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	cands := []string{raw}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			cands = append(cands, line)
		}
	}
	if len(strings.Fields(raw)) > 1 {
		cands = append(cands, strings.Fields(raw)...)
	}
	return cands
}

func normalizePath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	if strings.HasPrefix(strings.ToLower(s), "file://") {
		rest := s[len("file://"):]
		// Windows drive-letter or UNC paths are not valid URIs; handle directly.
		if len(rest) >= 2 && ((rest[0] >= 'A' && rest[0] <= 'Z') || (rest[0] >= 'a' && rest[0] <= 'z')) && rest[1] == ':' {
			s = rest
		} else if strings.HasPrefix(rest, "/") && len(rest) >= 3 && ((rest[1] >= 'A' && rest[1] <= 'Z') || (rest[1] >= 'a' && rest[1] <= 'z')) && rest[2] == ':' {
			s = rest[1:]
		} else if u, err := url.Parse(s); err == nil {
			if u.Host != "" {
				return ""
			}
			s = u.Path
			if len(s) >= 3 && s[0] == '/' && ((s[1] >= 'A' && s[1] <= 'Z') || (s[1] >= 'a' && s[1] <= 'z')) && s[2] == ':' {
				s = s[1:]
			}
		}
	}
	s = unescapeShell(s)
	if s == "~" || strings.HasPrefix(s, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			s = home + s[1:]
		}
	} else if strings.HasPrefix(s, "~") {
		// ~user or ~user/path: expand via os/user.Lookup.
		rest := s[1:]
		var userPart, tail string
		if idx := strings.IndexByte(rest, '/'); idx >= 0 {
			userPart, tail = rest[:idx], rest[idx:]
		} else {
			userPart, tail = rest, ""
		}
		if userPart != "" {
			if u, err := user.Lookup(userPart); err == nil {
				s = u.HomeDir + tail
			}
			// On lookup failure, leave s unchanged so callers see the literal path.
		}
	}
	if s == "" {
		return ""
	}
	return filepath.Clean(s)
}

// unescapeShell drops backslashes that quote punctuation, which is how many
// terminals paste a path containing spaces.
func unescapeShell(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			n := s[i+1]
			if !isAlnumByte(n) {
				b.WriteByte(n)
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isAlnumByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
