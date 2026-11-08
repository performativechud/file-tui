package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// homeDirFunc is the home-directory resolver; tests may override it for determinism.
var homeDirFunc = os.UserHomeDir

// suggestion is a single autocomplete entry.
type suggestion struct {
	name  string // display name (with trailing / for dirs)
	path  string // full path to complete to
	isDir bool
}

// autocomplete returns up to max suggestions matching the given input prefix.
// It expands ~ to the home directory for lookup but returns paths using the
// original prefix style so completion feels natural.
func autocomplete(raw string) []suggestion {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// Determine the directory to scan and the prefix to filter by.
	dir, prefix := splitPath(raw)

	// Expand ~ for filesystem lookup.
	lookupDir := dir
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		if home, err := homeDirFunc(); err == nil {
			if dir == "~" {
				lookupDir = home
			} else {
				lookupDir = filepath.Join(home, dir[2:])
			}
		}
	}
	if lookupDir == "" {
		lookupDir = "."
	}

	entries, err := os.ReadDir(lookupDir)
	if err != nil {
		return nil
	}

	var matches []suggestion
	lowerPrefix := strings.ToLower(prefix)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(strings.ToLower(name), lowerPrefix) {
			continue
		}
		isDir := e.IsDir()
		displayName := name
		if isDir {
			displayName += "/"
		}
		// Build the completed path preserving the user's prefix style.
		completedPath := joinPath(dir, name)
		if isDir {
			completedPath += "/"
		}
		matches = append(matches, suggestion{
			name:  displayName,
			path:  completedPath,
			isDir: isDir,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		// Directories first, then alphabetical.
		if matches[i].isDir != matches[j].isDir {
			return matches[i].isDir
		}
		return strings.ToLower(matches[i].name) < strings.ToLower(matches[j].name)
	})

	if len(matches) > 8 {
		matches = matches[:8]
	}
	return matches
}

// splitPath splits a path into (directory, prefix) for autocomplete.
// For "foo/bar/baz", returns ("foo/bar", "baz").
// For "foo", returns (".", "foo") or ("", "foo") depending on whether
// it looks like an absolute/relative path.
func splitPath(p string) (string, string) {
	// Handle trailing slash: user is browsing a directory.
	if strings.HasSuffix(p, "/") {
		return p, ""
	}

	// Find the last path separator.
	idx := strings.LastIndexAny(p, "/\\")
	if idx < 0 {
		// No separator: treat as prefix in current directory.
		return "", p
	}

	dir := p[:idx]
	prefix := p[idx+1:]
	if dir == "" {
		dir = "/"
	}
	return dir, prefix
}

// fetchSuggestions is an async Bubble Tea command that reads directory entries.
// It returns a suggMsg with the request ID for stale-result prevention.
func fetchSuggestions(id int, raw string) tea.Cmd {
	return func() tea.Msg {
		return suggMsg{id: id, path: raw, sugg: autocomplete(raw)}
	}
}

// joinPath joins a directory and name, handling empty dir and ~ prefix.
func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	if strings.HasSuffix(dir, "/") {
		return dir + name
	}
	return dir + "/" + name
}
