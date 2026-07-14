package email

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SocialRoots/sr-email/settings"
)

// inboxDir, archiveDir, failedDir are subdirectories of EmailStoreDir.
func inboxDir() string  { return filepath.Join(settings.EmailStoreDir, "inbox") }
func archiveDir() string { return filepath.Join(settings.EmailStoreDir, "archive") }
func failedDir() string  { return filepath.Join(settings.EmailStoreDir, "failed") }

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// SaveToInbox writes the raw MailGun payload to inbox/<timestamp>_<notelink>.eml.
// Returns the filename (without path) on success.
func SaveToInbox(raw []byte, timestamp, recipient string) (string, error) {
	dir := inboxDir()
	if err := ensureDir(dir); err != nil {
		return "", fmt.Errorf("create inbox dir: %w", err)
	}

	ts := safeTimestamp(timestamp)
	localPart := strings.SplitN(recipient, "@", 2)[0]
	if localPart == "" {
		localPart = "unknown"
	}

	name := fmt.Sprintf("%s_%s.eml", ts, localPart)
	path := filepath.Join(dir, name)

	if err := os.WriteFile(path, raw, 0644); err != nil {
		return "", fmt.Errorf("write inbox file: %w", err)
	}

	log.Printf("[sr-email] saved to inbox: %s (%d bytes)", name, len(raw))
	return name, nil
}

// MoveToArchive moves a file from inbox to archive/<name>.
func MoveToArchive(name string) error {
	return moveFile(filepath.Join(inboxDir(), name), filepath.Join(archiveDir(), name))
}

// MoveToFailed moves a file from inbox to failed/<name>.
func MoveToFailed(name string) error {
	return moveFile(filepath.Join(inboxDir(), name), filepath.Join(failedDir(), name))
}

func moveFile(src, dst string) error {
	if err := ensureDir(filepath.Dir(dst)); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

func safeTimestamp(ts string) string {
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.Format("20060102T150405")
	}
	return time.Now().UTC().Format("20060102T150405")
}