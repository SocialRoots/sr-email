package email

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SocialRoots/sr-email/settings"
)

func TestSaveToInbox(t *testing.T) {
	dir := t.TempDir()
	orig := settings.EmailStoreDir
	settings.EmailStoreDir = dir
	defer func() { settings.EmailStoreDir = orig }()

	raw := []byte("From: test@example.com\nSubject: Re: note\n\nreply text")
	ts := "2026-07-10T12:00:00Z"
	recipient := "notekey123@mg.socialroots.org"

	name, err := SaveToInbox(raw, ts, recipient)
	if err != nil {
		t.Fatalf("SaveToInbox: %v", err)
	}

	if !strings.HasPrefix(name, "20260710T120000_notekey123") {
		t.Errorf("unexpected filename: %s", name)
	}
	if !strings.HasSuffix(name, ".eml") {
		t.Errorf("expected .eml extension, got: %s", name)
	}

	inboxPath := filepath.Join(dir, "inbox", name)
	content, err := os.ReadFile(inboxPath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if string(content) != string(raw) {
		t.Errorf("file content mismatch")
	}
}

func TestSaveToInbox_InvalidTimestamp(t *testing.T) {
	dir := t.TempDir()
	orig := settings.EmailStoreDir
	settings.EmailStoreDir = dir
	defer func() { settings.EmailStoreDir = orig }()

	_, err := SaveToInbox([]byte("content"), "not-a-timestamp", "key@domain.com")
	if err != nil {
		t.Fatalf("SaveToInbox: %v", err)
	}

	entries, _ := os.ReadDir(filepath.Join(dir, "inbox"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}
	if !strings.Contains(entries[0].Name(), "_key.eml") {
		t.Errorf("expected _key.eml suffix, got: %s", entries[0].Name())
	}
}

func TestSaveToInbox_CreatesDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "custom", "path")
	orig := settings.EmailStoreDir
	settings.EmailStoreDir = dir
	defer func() { settings.EmailStoreDir = orig }()

	_, err := SaveToInbox([]byte("content"), "2026-07-10T12:00:00Z", "key@domain.com")
	if err != nil {
		t.Fatalf("SaveToInbox: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "inbox")); os.IsNotExist(err) {
		t.Fatal("inbox directory was not created")
	}
}

func TestMoveToArchive(t *testing.T) {
	dir := t.TempDir()
	orig := settings.EmailStoreDir
	settings.EmailStoreDir = dir
	defer func() { settings.EmailStoreDir = orig }()

	name, _ := SaveToInbox([]byte("test"), "2026-07-10T12:00:00Z", "key@domain.com")

	if err := MoveToArchive(name); err != nil {
		t.Fatalf("MoveToArchive: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "inbox", name)); !os.IsNotExist(err) {
		t.Error("file still exists in inbox after archive")
	}
	if _, err := os.Stat(filepath.Join(dir, "archive", name)); os.IsNotExist(err) {
		t.Error("file not found in archive")
	}
}

func TestMoveToFailed(t *testing.T) {
	dir := t.TempDir()
	orig := settings.EmailStoreDir
	settings.EmailStoreDir = dir
	defer func() { settings.EmailStoreDir = orig }()

	name, _ := SaveToInbox([]byte("test"), "2026-07-10T12:00:00Z", "key@domain.com")

	if err := MoveToFailed(name); err != nil {
		t.Fatalf("MoveToFailed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "inbox", name)); !os.IsNotExist(err) {
		t.Error("file still exists in inbox after move to failed")
	}
	if _, err := os.Stat(filepath.Join(dir, "failed", name)); os.IsNotExist(err) {
		t.Error("file not found in failed")
	}
}