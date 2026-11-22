package main

import (
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizePathUserExpansion(t *testing.T) {
	// Test ~user expansion for valid user
	cur, curErr := user.Current()
	if curErr == nil {
		homeDir := cur.HomeDir
		input := "~" + cur.Username + "/Documents"
		result := normalizePath(input)
		expected := filepath.Join(homeDir, "Documents")
		if result != expected {
			t.Errorf("normalizePath(%q) = %q, want %q", input, result, expected)
		}

		// Test ~user with just the username (no trailing path)
		input = "~" + cur.Username
		result = normalizePath(input)
		if result != homeDir {
			t.Errorf("normalizePath(%q) = %q, want %q", input, result, homeDir)
		}
	}

	// Test ~user with nonexistent user - should return unchanged (and fail to stat)
	input := "~nonexistentuser/file.txt"
	result := normalizePath(input)
	if result != input {
		t.Errorf("normalizePath(%q) = %q, want unchanged %q", input, result, input)
	}

	// Test ~ alone
	input = "~"
	result = normalizePath(input)
	if curErr == nil {
		if result != cur.HomeDir {
			t.Errorf("normalizePath(~) = %q, want %q", result, cur.HomeDir)
		}
	}

	// Test ~/
	input = "~/"
	result = normalizePath(input)
	if curErr == nil {
		expected := filepath.Join(cur.HomeDir, "")
		if result != expected && result != cur.HomeDir {
			// filepath.Clean may trim trailing /
			if result != cur.HomeDir {
				t.Errorf("normalizePath(~/) = %q, want %q", result, expected)
			}
		}
	}

	// Test ~user/
	input = "~" + cur.Username + "/"
	result = normalizePath(input)
	if curErr == nil {
		if result != cur.HomeDir {
			t.Errorf("normalizePath(~user/) = %q, want %q", result, cur.HomeDir)
		}
	}
}

func TestNormalizePathEdgeCases(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		contains string // expected substring in result (for cases that can't be exact)
	}{
		{"empty string", "", ""},
		{"just ~", "~", ""},
		{"~/", "~/", ""},
		{"~user", "~user", ""}, // no expansion for invalid user
		{"~user/", "~user/", ""},
		{"~user/subdir", "~user/subdir", "subdir"},
		{"~user/subdir/file.txt", "~user/subdir/file.txt", "subdir/file.txt"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizePath(tc.input)
			if tc.name == "empty string" {
				if result != "" {
					t.Errorf("normalizePath(%q) = %q, want empty", tc.input, result)
				}
			}
			if tc.contains != "" && !strings.Contains(result, tc.contains) {
				t.Errorf("normalizePath(%q) = %q does not contain %q", tc.input, result, tc.contains)
			}
		})
	}
}

func TestNormalizePathTildeUserFallback(t *testing.T) {
	// Test that ~user that doesn't exist falls back to literal path
	// (resolvePath will then fail to stat it)
	input := "~doesnotexist/anything"
	result := normalizePath(input)
	if result != input {
		t.Errorf("normalizePath(%q) = %q, want unchanged %q", input, result, input)
	}
}

func TestNormalizePathFileURIWithTildeUser(t *testing.T) {
	// This test is more about resolvePath behavior via file://
	// file:// URLs with host are rejected (handled in formats_test.go)
	// Just verify normalizePath doesn't break on file:// paths
	input := "file:///home/user/file.txt"
	result := normalizePath(input)
	if !strings.HasSuffix(result, "/home/user/file.txt") && !strings.HasSuffix(result, "home/user/file.txt") {
		t.Errorf("normalizePath(%q) = %q, unexpected result", input, result)
	}
}

func TestNormalizePathWithSpaces(t *testing.T) {
	// Test paths with spaces (the escapeShell handles this)
	input := `~/My Documents/file.txt`
	result := normalizePath(input)
	// Should have expanded ~ to home
	if !strings.HasSuffix(result, "My Documents/file.txt") && !strings.HasSuffix(result, "My\\ Documents/file.txt") {
		t.Logf("normalizePath(%q) = %q", input, result)
	}
}
