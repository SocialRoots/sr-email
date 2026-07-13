package email

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SocialRoots/sr-email/settings"
)

func TestSaveRaw(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EMAIL_STORE_DIR", dir)

	// Re-initialize the config value since it was captured at package init.
	// We override it by re-reading the env var — the settings package reads
	// it lazily via a closure.
	orig := settings.EmailStoreDir
	settings.EmailStoreDir = dir
	defer func() { settings.EmailStoreDir = orig }()

	raw := []byte("From: test@example.com\nSubject: Re: note\n\nreply text")
	ts := "2026-07-09T12:00:00Z"
	recipient := "notekey123@mg.socialroots.org"

	SaveRaw(raw, ts, recipient)

	// Check the file was written.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read store dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	filename := entries[0].Name()
	if !strings.HasPrefix(filename, "20260709T120000_notekey123") {
		t.Errorf("unexpected filename: %s", filename)
	}
	if !strings.HasSuffix(filename, ".eml") {
		t.Errorf("expected .eml extension, got: %s", filename)
	}

	content, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if string(content) != string(raw) {
		t.Errorf("file content mismatch:\ngot:  %q\nwant: %q", string(content), string(raw))
	}
}

func TestSaveRaw_InvalidTimestamp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EMAIL_STORE_DIR", dir)

	orig := settings.EmailStoreDir
	settings.EmailStoreDir = dir
	defer func() { settings.EmailStoreDir = orig }()

	raw := []byte("content")
	ts := "not-a-timestamp"
	recipient := "key@domain.com"

	SaveRaw(raw, ts, recipient)

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	// Should contain a timestamp-like prefix (current time fallback).
	name := entries[0].Name()
	if !strings.Contains(name, "_key.eml") {
		t.Errorf("expected _key.eml suffix, got: %s", name)
	}
}

func TestSaveRaw_NoAtSign(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EMAIL_STORE_DIR", dir)

	orig := settings.EmailStoreDir
	settings.EmailStoreDir = dir
	defer func() { settings.EmailStoreDir = orig }()

	raw := []byte("content")
	ts := "2026-07-09T12:00:00Z"
	recipient := "justakey"

	SaveRaw(raw, ts, recipient)

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	name := entries[0].Name()
	if !strings.Contains(name, "_justakey.eml") {
		t.Errorf("expected _justakey.eml suffix, got: %s", name)
	}
}

func TestSaveRaw_CreatesDir(t *testing.T) {
	// Use a non-existent nested path to test MkdirAll.
	base := t.TempDir()
	dir := filepath.Join(base, "subdir", "emails")

	orig := settings.EmailStoreDir
	settings.EmailStoreDir = dir
	defer func() { settings.EmailStoreDir = orig }()

	raw := []byte("content")
	SaveRaw(raw, "2026-07-09T12:00:00Z", "key@domain.com")

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("directory was not created")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Fatal("no files in created directory")
	}
}