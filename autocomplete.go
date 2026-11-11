package main

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// homeDirFunc is the home-directory resolver; tests may override it for determinism.
var homeDirFunc = os.UserHomeDir

// lookupUserFunc looks up a user by name; tests may override it for isolation.
var lookupUserFunc = user.Lookup

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
	var lookupDir string
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		if home, err := homeDirFunc(); err == nil {
			if dir == "~" {
				lookupDir = home
			} else {
				lookupDir = filepath.Join(home, dir[2:])
			}
		}
	} else if dir != "" && dir[0] == '~' && !strings.HasPrefix(dir, "~/") {
		// Handle ~username: dir starts with ~ but isn't ~/ (e.g., "~testuser")
		username := dir[1:] // strip the leading ~
		// Strip trailing slash if present for user lookup
		if username != "" && username[len(username)-1] == '/' {
			username = username[:len(username)-1]
		}
		if u, err := lookupUserFunc(username); err == nil {
			lookupDir = u.HomeDir
		} else {
			// User lookup failed; return no suggestions rather than falling back
			// to the current directory which would match everything.
			return nil
		}
	} else {
		// No ~ prefix: use the directory from splitPath
		lookupDir = dir
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

	return matches
}

// splitPath splits a path into (directory, prefix) for autocomplete.
// For "foo/bar/baz", returns ("foo/bar", "baz").
// For "foo", returns ("", "foo").
// For "~", returns ("~", "").
// For "~user", returns ("~user", "").
// For "~user/file", returns ("~user", "file").
func splitPath(p string) (string, string) {
	// Handle the special case of "~" alone.
	if p == "~" {
		return "~", ""
	}

	// Handle ~username: treat as directory name with optional /path
	if len(p) > 1 && p[0] == '~' && !strings.HasPrefix(p, "~/") {
		// Handle ~username/ (trailing slash) specially
		if p[len(p)-1] == '/' {
			// ~username/ : return ("~username/", "")
			return p[:len(p)-1] + "/", ""
		}
		// Find the first slash to separate username from path
		slashIdx := strings.Index(p[1:], "/")
		if slashIdx >= 0 {
			// ~username/path: return ("~username", "path")
			username := p[:slashIdx+1] // includes the ~
			path := p[slashIdx+1:]     // after the slash
			// Strip leading / from path
			if path[:1] == "/" {
				path = path[1:]
			}
			return username, path
		} else {
			// ~username (no slash): return ("~username", "")
			return p, ""
		}
	}

	// Handle trailing slash: user is browsing a directory.
	if strings.HasSuffix(p, "/") {
		return p, ""
	}

	// Find the last path separator. Backslash is only a separator on
	// Windows; on Unix it is a valid filename character.
	separators := "/"
	if runtime.GOOS == "windows" {
		separators = "/\\"
	}
	idx := strings.LastIndexAny(p, separators)
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

// expandUserDir resolves a ~user or ~user/path prefix to the user's home
// directory. On lookup failure the original dir is returned unchanged so the
// caller can fall back to a relative path lookup (which will then fail to
// stat and produce no suggestions, rather than crashing).
func expandUserDir(dir string) string {
	if dir == "~" {
		return "~"
	}

	// Strip the leading ~
	rest := dir[1:]

	// Find the first / to separate username from path
	slashIdx := strings.Index(rest, "/")
	var username, subpath string
	if slashIdx >= 0 {
		username = rest[:slashIdx]
		subpath = rest[slashIdx:] // includes the /
	} else {
		username = rest
		subpath = ""
	}

	// Look up the user
	if u, err := lookupUserFunc(username); err == nil {
		if subpath == "" {
			return filepath.ToSlash(u.HomeDir)
		}
		return filepath.ToSlash(filepath.Join(u.HomeDir, subpath[1:])) // skip leading /
	}

	// User not found; return original unchanged
	return dir
}

// fetchSuggestions is an async Bubble Tea command that reads directory entries.
// It returns a suggMsg with the request ID for stale-result prevention.
func fetchSuggestions(id int, raw string) tea.Cmd {
	return func() tea.Msg {
		return suggMsg{id: id, path: raw, sugg: autocomplete(raw)}
	}
}
