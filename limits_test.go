package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractBudgetCapsBytes(t *testing.T) {
	t.Setenv("FILE_CLI_MAX_EXTRACT_MB", "1")
	b := newExtractBudget()
	if b.maxBytes != 1<<20 {
		t.Fatalf("max bytes = %d, want 1 MiB", b.maxBytes)
	}
	dst := filepath.Join(t.TempDir(), "big.bin")
	err := writeStream(dst, bytes.NewReader(make([]byte, 1<<20+1)), 0o644, b)
	if err == nil || !strings.Contains(err.Error(), "expands past") {
		t.Fatalf("want cap error, got %v", err)
	}
	if _, statErr := os.Stat(dst); statErr != nil {
		t.Fatalf("partial file should exist only until the caller cleans up: %v", statErr)
	}
}

func TestExtractBudgetCapsEntries(t *testing.T) {
	t.Setenv("FILE_CLI_MAX_EXTRACT_ENTRIES", "2")
	b := newExtractBudget()
	for i := 0; i < 2; i++ {
		if err := b.entry(); err != nil {
			t.Fatalf("entry %d: %v", i, err)
		}
	}
	if err := b.entry(); err == nil {
		t.Fatal("third entry must be refused")
	}
}
