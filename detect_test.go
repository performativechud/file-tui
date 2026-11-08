package main

import (
	"bytes"
	"testing"
)

func TestDetectMagic(t *testing.T) {
	tar := make([]byte, 512)
	copy(tar[257:], "ustar\x0000")

	cases := []struct {
		name string
		b    []byte
		want string
	}{
		{"a.png", []byte("\x89PNG\r\n\x1a\n0000"), "png"},
		{"a.jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0}, "jpg"},
		{"a.gif", []byte("GIF89a..."), "gif"},
		{"a.webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "webp"},
		{"a.wav", []byte("RIFF\x00\x00\x00\x00WAVEfmt "), "wav"},
		{"a.avi", []byte("RIFF\x00\x00\x00\x00AVI LIST"), "avi"},
		{"a.tiff", []byte("II*\x00rest"), "tiff"},
		{"a.pdf", []byte("%PDF-1.7\n"), "pdf"},
		{"a.zip", []byte("PK\x03\x04rest"), "zip"},
		{"a.docx", []byte("PK\x03\x04rest"), "docx"},
		{"a.epub", []byte("PK\x03\x04rest"), "epub"},
		{"a.tar", tar, "tar"},
		{"a.gz", []byte{0x1F, 0x8B, 0x08, 0}, "gz"},
		{"a.tar.gz", []byte{0x1F, 0x8B, 0x08, 0}, "tar.gz"},
		{"a.tar.xz", append([]byte("\xFD7zXZ\x00"), 0, 0), "tar.xz"},
		{"a.xz", append([]byte("\xFD7zXZ\x00"), 0, 0), "xz"},
		{"a.zst", []byte{0x28, 0xB5, 0x2F, 0xFD}, "zst"},
		{"a.flac", []byte("fLaC0000"), "flac"},
		{"a.mp3", []byte("ID3\x04\x00"), "mp3"},
		{"a.mkv", []byte("\x1A\x45\xDF\xA3\x01\x00"), "mkv"},
		{"a.webm", []byte("\x1A\x45\xDF\xA3webm\x00"), "webm"},
		{"a.mp4", []byte("\x00\x00\x00\x18ftypisom"), "mp4"},
		{"a.mov", []byte("\x00\x00\x00\x18ftypqt  "), "mov"},
		{"a.avif", []byte("\x00\x00\x00\x18ftypavif"), "avif"},
		{"a.heic", []byte("\x00\x00\x00\x18ftypheic"), "heic"},
		{"a.m4a", []byte("\x00\x00\x00\x18ftypM4A "), "m4a"},
		{"a.svg", []byte(`<?xml version="1.0"?><svg width="1"/>`), "svg"},
		{"a.json", []byte(`[{"a":1}]`), "json"},
		{"a.csv", []byte("a,b\n1,2\n"), "csv"},
		{"a.srt", []byte("1\n00:00:01,000 --> 00:00:02,000\nhi\n"), "srt"},
		{"a.md", []byte("# title\n"), "md"},
		{"noext", []byte{0x00, 0x01, 0x02, 0xFF}, "bin"},
	}

	for _, c := range cases {
		got := detect(c.b, c.name)
		if got.ID != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got.ID, c.want)
		}
	}
}

func TestDetectExtensionOnlyWhenMagicUnknown(t *testing.T) {
	// A jpg named .png must still be detected as jpg.
	got := detect([]byte{0xFF, 0xD8, 0xFF, 0xE0}, "photo.png")
	if got.ID != "jpg" {
		t.Fatalf("magic must win over extension, got %q", got.ID)
	}
}

func TestBinaryTextGuard(t *testing.T) {
	if binaryControl([]byte("hello\nworld")) {
		t.Fatal("plain text flagged as binary")
	}
	if !binaryControl([]byte{0x00, 0x01, 0x02}) {
		t.Fatal("control bytes must flag binary")
	}
	if !bytes.Equal([]byte("x"), []byte("x")) {
		t.Fatal("sanity")
	}
}
