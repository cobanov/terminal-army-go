package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdatePlatform(t *testing.T) {
	got, err := updatePlatform("darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if got != "darwin_arm64" {
		t.Fatalf("platform = %q, want darwin_arm64", got)
	}
	if _, err := updatePlatform("windows", "amd64"); err == nil {
		t.Fatal("expected unsupported OS error")
	}
}

func TestReleaseArchiveName(t *testing.T) {
	got := releaseArchiveName("v0.1.7", "linux_amd64")
	want := "tarmy_0.1.7_linux_amd64.tar.gz"
	if got != want {
		t.Fatalf("archive name = %q, want %q", got, want)
	}
}

func TestTagFromLatestRedirect(t *testing.T) {
	cases := map[string]string{
		"https://github.com/cobanov/terminal-army-go/releases/tag/v0.1.7": "v0.1.7",
		"/cobanov/terminal-army-go/releases/tag/v0.1.8":                   "v0.1.8",
		"": "",
	}
	for location, want := range cases {
		if got := tagFromLatestRedirect(location); got != want {
			t.Fatalf("tagFromLatestRedirect(%q) = %q, want %q", location, got, want)
		}
	}
}

func TestChecksumForArchive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checksums.txt")
	data := "111 one.tar.gz\n222 *tarmy_0.1.7_darwin_arm64.tar.gz\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := checksumForArchive(path, "tarmy_0.1.7_darwin_arm64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != "222" {
		t.Fatalf("checksum = %q, want 222", got)
	}
}
