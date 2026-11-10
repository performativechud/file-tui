package main

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutocompleteUserExpansion(t *testing.T) {
	// Create a temporary home directory with a Documents subdirectory
	home := t.TempDir()
	err := os.MkdirAll(filepath.Join(home, "Documents"), 0o755)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	os.WriteFile(filepath.Join(home, "testfile.txt"), []byte("x"), 0o644)

	// Override homeDirFunc and lookupUserFunc for test isolation
	origHome := homeDirFunc
	origLookup := lookupUserFunc
	homeDirFunc = func() (string, error) { return home, nil }
	systemLookup := user.Lookup
	lookupUserFunc = func(username string) (*user.User, error) {
		if username == "testuser" {
			return &user.User{HomeDir: home}, nil
		}
		return systemLookup(username)
	}
	defer func() {
		homeDirFunc = origHome
		lookupUserFunc = origLookup
	}()

	// Test ~testuser expansion: should scan home and find Documents + testfile.txt
	suggs := autocomplete("~testuser")
	if len(suggs) == 0 {
		t.Fatal("expected suggestions for ~testuser")
	}
	found := false
	for _, s := range suggs {
		if s.name == "Documents/" || s.name == "testfile.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Documents or testfile.txt in suggestions, got %+v", suggs)
	}

	// Test ~testuser/ prefix
	suggs = autocomplete("~testuser/")
	if len(suggs) == 0 {
		t.Fatal("expected suggestions for ~testuser/")
	}

	// Test ~testuser/Doc prefix (should match Documents)
	suggs = autocomplete("~testuser/Doc")
	if len(suggs) == 0 {
		t.Fatal("expected suggestions for ~testuser/Doc")
	}
	found = false
	for _, s := range suggs {
		if strings.HasPrefix(strings.ToLower(s.name), "doc") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Documents in suggestions for ~testuser/Doc, got %+v", suggs)
	}

	// Test ~invaliduser should not crash and return empty (user lookup fails)
	suggs = autocomplete("~invaliduser")
	if len(suggs) != 0 {
		t.Fatalf("expected no suggestions for ~invaliduser, got %d", len(suggs))
	}
}

func TestExpandUserDir(t *testing.T) {
	systemLookup := user.Lookup
	lookupUserFunc = func(username string) (*user.User, error) {
		if username == "testuser" {
			return &user.User{HomeDir: "/home/testuser"}, nil
		}
		return systemLookup(username)
	}
	defer func() { lookupUserFunc = user.Lookup }()

	// Test ~user
	path := expandUserDir("~testuser")
	if path != "/home/testuser" {
		t.Errorf("expandUserDir(~testuser) = %q, want /home/testuser", path)
	}

	// Test ~user/subdir
	path = expandUserDir("~testuser/subdir")
	if path != "/home/testuser/subdir" {
		t.Errorf("expandUserDir(~testuser/subdir) = %q, want /home/testuser/subdir", path)
	}

	// Test invalid user
	path = expandUserDir("~invaliduser")
	if path != "~invaliduser" {
		t.Errorf("expandUserDir(~invaliduser) = %q, want ~invaliduser (unchanged)", path)
	}

	// Test ~ alone
	path = expandUserDir("~")
	if path != "~" {
		t.Errorf("expandUserDir(~) = %q, want ~", path)
	}

	// Test ~user with trailing slash: expandUserDir resolves it to home
	path = expandUserDir("~testuser/")
	if path != "/home/testuser" {
		t.Errorf("expandUserDir(~testuser/) = %q, want /home/testuser", path)
	}
}

func TestAutocompleteSplitPathUser(t *testing.T) {
	// splitPath should treat ~user as directory part
	dir, prefix := splitPath("~testuser/file")
	if dir != "~testuser" {
		t.Errorf("splitPath(~testuser/file): dir = %q, want ~testuser", dir)
	}
	if prefix != "file" {
		t.Errorf("splitPath(~testuser/file): prefix = %q, want file", prefix)
	}

	// Test ~user/ prefix
	dir, prefix = splitPath("~testuser/")
	if dir != "~testuser/" {
		t.Errorf("splitPath(~testuser/): dir = %q, want ~testuser/", dir)
	}
	if prefix != "" {
		t.Errorf("splitPath(~testuser/): prefix = %q, want empty", prefix)
	}

	// Test ~ alone
	dir, prefix = splitPath("~")
	if dir != "~" {
		t.Errorf("splitPath(~): dir = %q, want ~", dir)
	}
	if prefix != "" {
		t.Errorf("splitPath(~): prefix = %q, want empty", prefix)
	}
}

func TestAutocompleteUserFallback(t *testing.T) {
	// When user lookup fails, autocomplete should not crash and return empty
	// This simulates a scenario where the system user doesn't exist
	home := t.TempDir()
	err := os.MkdirAll(filepath.Join(home, "someuser"), 0o755)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	origHome := homeDirFunc
	origLookup := lookupUserFunc
	homeDirFunc = func() (string, error) { return home, nil }
	lookupUserFunc = func(username string) (*user.User, error) {
		// Always fail lookup
		return nil, os.ErrNotExist
	}
	defer func() {
		homeDirFunc = origHome
		lookupUserFunc = origLookup
	}()

	// Should not crash; may return empty or fallback
	suggs := autocomplete("~nonexistentuser")
	// No assertions on result, just that it didn't crash
	_ = suggs

	// Also test ~nonexistentuser/subdir
	suggs = autocomplete("~nonexistentuser/subdir")
	_ = suggs
}
