package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SocialRoots/sr-email/settings"
	"github.com/gin-gonic/gin"
)

// Test fixtures — constants shared across handler tests.
const (
	testAPIKey         = "test-mailgun-key-abc123"
	testInternalToken  = "test-internal-token-xyz"
	testTimestamp      = "1721234567"
	testToken          = "abcdefghijabcdefghijabcdefghijabcdefghijab"
	testRecipient      = "notekey123@mg.socialroots.org"
	testSender         = "jane@example.com"
	testFrom           = "Jane Smith <jane@example.com>"
	testSubject        = "Re: Your note"
	testStrippedText   = "Here is my reply"
)

// computeSignature generates the HMAC-SHA256 signature MailGun would send.
func computeSignature(key, timestamp, token string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(timestamp + token))
	return hex.EncodeToString(mac.Sum(nil))
}

// save and restore settings to isolate each test.
func saveSettings() func() {
	origKey := settings.MailgunAPIKey
	origURL := settings.ResponsesServiceURL
	origToken := settings.InternalToken
	origStore := settings.EmailStoreDir
	return func() {
		settings.MailgunAPIKey = origKey
		settings.ResponsesServiceURL = origURL
		settings.InternalToken = origToken
		settings.EmailStoreDir = origStore
	}
}

func TestHandleInbound_Valid(t *testing.T) {
	restore := saveSettings()
	defer restore()

	settings.MailgunAPIKey = testAPIKey
	settings.EmailStoreDir = t.TempDir()

	// Mock RS-RESPONSES server.
	var gotPayload json.RawMessage
	mux := http.NewServeMux()
	mux.HandleFunc("/response/reply", func(w http.ResponseWriter, r *http.Request) {
		// Verify internal token was sent.
		if r.Header.Get("X-SR-Internal-Token") != testInternalToken {
			t.Errorf("missing or wrong X-SR-Internal-Token: %q", r.Header.Get("X-SR-Internal-Token"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		gotPayload = make(json.RawMessage, 0)
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
	})
	mock := httptest.NewServer(mux)
	defer mock.Close()
	settings.ResponsesServiceURL = mock.URL
	settings.InternalToken = testInternalToken

	// Build the form-encoded POST body.
	sig := computeSignature(testAPIKey, testTimestamp, testToken)
	form := url.Values{
		"timestamp":     {testTimestamp},
		"token":         {testToken},
		"signature":     {sig},
		"recipient":     {testRecipient},
		"sender":        {testSender},
		"from":          {testFrom},
		"subject":       {testSubject},
		"stripped-text": {testStrippedText},
		"body-plain":    {testStrippedText + "\n\n> Original"},
		"body-html":     {"<p>" + testStrippedText + "</p>"},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/email/inbound",
		strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s := &server{env: "test"}
	s.handleInbound(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	// Verify the forwarded payload.
	var rp replyPayload
	if err := json.Unmarshal(gotPayload, &rp); err != nil {
		t.Fatalf("failed to unmarshal forwarded payload: %v", err)
	}
	if rp.Link != "notekey123" {
		t.Errorf("Link = %q, want %q", rp.Link, "notekey123")
	}
	if rp.Reply != testStrippedText {
		t.Errorf("Reply = %q, want %q", rp.Reply, testStrippedText)
	}
	if rp.ReplyName != "Jane Smith" {
		t.Errorf("ReplyName = %q, want %q", rp.ReplyName, "Jane Smith")
	}
	if rp.ReplyEmail != testSender {
		t.Errorf("ReplyEmail = %q, want %q", rp.ReplyEmail, testSender)
	}

	// Verify the raw email was saved to disk.
	// SaveRaw runs as a goroutine; give it time to complete.
	var files []os.FileInfo
	for i := 0; i < 10; i++ {
		entries, _ := os.ReadDir(settings.EmailStoreDir)
		if len(entries) > 0 {
			files = make([]os.FileInfo, len(entries))
			for j, e := range entries {
				info, _ := e.Info()
				files[j] = info
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(files) == 0 {
		t.Fatal("no email file was written to disk")
	}
	if !strings.HasSuffix(files[0].Name(), ".eml") {
		t.Errorf("expected .eml file, got %s", files[0].Name())
	}
	if files[0].Size() == 0 {
		t.Errorf("email file is empty")
	}
}

func TestHandleInbound_BadSignature(t *testing.T) {
	restore := saveSettings()
	defer restore()

	settings.MailgunAPIKey = testAPIKey
	settings.EmailStoreDir = t.TempDir()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	form := url.Values{
		"timestamp": {testTimestamp},
		"token":     {testToken},
		"signature": {"definitely-not-valid"},
		"recipient": {testRecipient},
	}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/email/inbound",
		strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s := &server{env: "test"}
	s.handleInbound(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401; body = %s", w.Code, w.Body.String())
	}
}

func TestHandleInbound_NoKeyDevMode(t *testing.T) {
	restore := saveSettings()
	defer restore()

	// Leave MailgunAPIKey empty — dev promiscuous mode accepts all.
	settings.MailgunAPIKey = ""
	settings.EmailStoreDir = t.TempDir()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	form := url.Values{
		"timestamp": {testTimestamp},
		"token":     {testToken},
		"signature": {"anything"},
		"recipient": {testRecipient},
	}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/email/inbound",
		strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s := &server{env: "test"}
	s.handleInbound(c)

	if w.Code == http.StatusUnauthorized {
		t.Errorf("dev mode should accept without key, got 401")
	}
}

func TestHandleInbound_MissingFormData(t *testing.T) {
	restore := saveSettings()
	defer restore()

	settings.EmailStoreDir = t.TempDir()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// Send an empty body — form parsing succeeds (no fields) but no
	// recipient/sender means the reply will be incomplete.
	c.Request = httptest.NewRequest(http.MethodPost, "/api/email/inbound",
		strings.NewReader(""))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s := &server{env: "test"}
	s.handleInbound(c)

	// Should return 200 because the email is saved to file.
	// The forward will fail with an empty link, but that's a soft error.
	if w.Code != http.StatusOK {
		t.Errorf("empty form should return 200, got %d; body = %s",
			w.Code, w.Body.String())
	}
}

func TestHandleInbound_ForwardFailStill200(t *testing.T) {
	restore := saveSettings()
	defer restore()

	settings.MailgunAPIKey = testAPIKey
	settings.EmailStoreDir = t.TempDir()
	// Point to a server that will refuse the connection.
	settings.ResponsesServiceURL = "http://127.0.0.1:1"
	settings.InternalToken = testInternalToken

	sig := computeSignature(testAPIKey, testTimestamp, testToken)
	form := url.Values{
		"timestamp":     {testTimestamp},
		"token":         {testToken},
		"signature":     {sig},
		"recipient":     {testRecipient},
		"sender":        {testSender},
		"from":          {testFrom},
		"stripped-text": {testStrippedText},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/email/inbound",
		strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s := &server{env: "test"}
	s.handleInbound(c)

	// Should still return 200 since the email was saved.
	if w.Code != http.StatusOK {
		t.Errorf("should return 200 even if forward fails, got %d; body = %s",
			w.Code, w.Body.String())
	}
}

func TestHandleInbound_MultipartFormData(t *testing.T) {
	restore := saveSettings()
	defer restore()

	settings.MailgunAPIKey = testAPIKey
	settings.EmailStoreDir = t.TempDir()

	mux := http.NewServeMux()
	mux.HandleFunc("/response/reply", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	mock := httptest.NewServer(mux)
	defer mock.Close()
	settings.ResponsesServiceURL = mock.URL
	settings.InternalToken = testInternalToken

	sig := computeSignature(testAPIKey, testTimestamp, testToken)

	// Build multipart body using a buffer.
	var buf bytes.Buffer
	mp := `--BOUNDARY
Content-Disposition: form-data; name="timestamp"

` + testTimestamp + `
--BOUNDARY
Content-Disposition: form-data; name="token"

` + testToken + `
--BOUNDARY
Content-Disposition: form-data; name="signature"

` + sig + `
--BOUNDARY
Content-Disposition: form-data; name="recipient"

` + testRecipient + `
--BOUNDARY
Content-Disposition: form-data; name="sender"

` + testSender + `
--BOUNDARY
Content-Disposition: form-data; name="from"

` + testFrom + `
--BOUNDARY
Content-Disposition: form-data; name="stripped-text"

` + testStrippedText + `
--BOUNDARY--`
	buf.WriteString(mp)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/email/inbound",
		&buf)
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=BOUNDARY")

	s := &server{env: "test"}
	s.handleInbound(c)

	if w.Code != http.StatusOK {
		t.Errorf("multipart: status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
}