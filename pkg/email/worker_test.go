package email

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/SocialRoots/sr-email/settings"
)

func TestProcessInbox_ForwardsAndArchives(t *testing.T) {
	dir := t.TempDir()
	orig := settings.EmailStoreDir
	origURL := settings.ResponsesServiceURL
	origToken := settings.InternalToken
	settings.EmailStoreDir = dir
	defer func() {
		settings.EmailStoreDir = orig
		settings.ResponsesServiceURL = origURL
		settings.InternalToken = origToken
	}()

	// Mock RS-RESPONSES.
	var gotPayload json.RawMessage
	mux := http.NewServeMux()
	mux.HandleFunc("/response/reply", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-SR-Internal-Token") != "test-token" {
			t.Errorf("missing internal token")
		}
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
	})
	mock := httptest.NewServer(mux)
	defer mock.Close()
	settings.ResponsesServiceURL = mock.URL
	settings.InternalToken = "test-token"

	// Seed the inbox with a valid MailGun payload.
	raw := []byte("timestamp=1721234567&token=abc&signature=whatever&recipient=notekey123@mg.org&sender=jane@example.com&from=Jane+Smith+%3Cjane%40example.com%3E&stripped-text=my+reply")
	SaveToInbox(raw, "2026-07-10T12:00:00Z", "notekey123@mg.org")

	// Process.
	ProcessInbox()

	// Check file was moved to archive.
	entries, _ := filepath.Glob(filepath.Join(dir, "archive", "*.eml"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in archive, got %d", len(entries))
	}

	// Check forwarded payload.
	var rp replyPayload
	if err := json.Unmarshal(gotPayload, &rp); err != nil {
		t.Fatalf("unmarshal forwarded payload: %v", err)
	}
	if rp.Link != "notekey123" {
		t.Errorf("Link = %q, want notekey123", rp.Link)
	}
	if rp.Reply != "my reply" {
		t.Errorf("Reply = %q, want 'my reply'", rp.Reply)
	}
	if rp.ReplyName != "Jane Smith" {
		t.Errorf("ReplyName = %q, want 'Jane Smith'", rp.ReplyName)
	}
	if rp.ReplyEmail != "jane@example.com" {
		t.Errorf("ReplyEmail = %q, want jane@example.com", rp.ReplyEmail)
	}
}

func TestProcessInbox_MovesToFailedOnForwardError(t *testing.T) {
	dir := t.TempDir()
	orig := settings.EmailStoreDir
	origURL := settings.ResponsesServiceURL
	settings.EmailStoreDir = dir
	settings.ResponsesServiceURL = "http://127.0.0.1:1" // connection refused
	defer func() {
		settings.EmailStoreDir = orig
		settings.ResponsesServiceURL = origURL
	}()

	raw := []byte("timestamp=1&token=a&signature=s&recipient=k@mg.org&sender=x@y.com")
	SaveToInbox(raw, "2026-07-10T12:00:00Z", "k@mg.org")

	ProcessInbox()

	entries, _ := filepath.Glob(filepath.Join(dir, "failed", "*.eml"))
	if len(entries) == 0 {
		t.Fatal("expected file in failed on forward error")
	}

	// Inbox should be empty.
	inboxEntries, _ := filepath.Glob(filepath.Join(dir, "inbox", "*.eml"))
	if len(inboxEntries) != 0 {
		t.Errorf("inbox should be empty after processing, got %d files", len(inboxEntries))
	}
}

func TestProcessInbox_SkipsUnparseablePayload(t *testing.T) {
	dir := t.TempDir()
	orig := settings.EmailStoreDir
	settings.EmailStoreDir = dir
	defer func() { settings.EmailStoreDir = orig }()

	// Garbage that can't be parsed as form data.
	SaveToInbox([]byte("\x00\x01\x02"), "2026-07-10T12:00:00Z", "bad@mg.org")

	ProcessInbox()

	entries, _ := filepath.Glob(filepath.Join(dir, "failed", "*.eml"))
	if len(entries) == 0 {
		t.Fatal("expected unparseable file in failed")
	}
}

func TestProcessInbox_EmptyInboxIsNoOp(t *testing.T) {
	dir := t.TempDir()
	orig := settings.EmailStoreDir
	settings.EmailStoreDir = dir
	defer func() { settings.EmailStoreDir = orig }()

	// No panic, no error — just returns silently.
	ProcessInbox()

	entries, _ := filepath.Glob(filepath.Join(dir, "archive", "*.eml"))
	if len(entries) != 0 {
		t.Errorf("unexpected files in archive")
	}
}

func TestProcessInbox_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	orig := settings.EmailStoreDir
	origURL := settings.ResponsesServiceURL
	origToken := settings.InternalToken
	settings.EmailStoreDir = dir
	settings.InternalToken = "t"
	defer func() {
		settings.EmailStoreDir = orig
		settings.ResponsesServiceURL = origURL
		settings.InternalToken = origToken
	}()

	var callCount int
	mux := http.NewServeMux()
	mux.HandleFunc("/response/reply", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
	})
	mock := httptest.NewServer(mux)
	defer mock.Close()
	settings.ResponsesServiceURL = mock.URL

	for i := 0; i < 3; i++ {
		ts := fmt.Sprintf("2026-07-10T12:00:0%dZ", i)
		SaveToInbox([]byte("timestamp=1&token=a&signature=s&recipient=k@mg.org&sender=x@y.com"), ts, "k@mg.org")
	}

	ProcessInbox()

	if callCount != 3 {
		t.Errorf("expected 3 forward calls, got %d", callCount)
	}
	archiveEntries, _ := filepath.Glob(filepath.Join(dir, "archive", "*.eml"))
	if len(archiveEntries) != 3 {
		t.Errorf("expected 3 archive files, got %d", len(archiveEntries))
	}
}