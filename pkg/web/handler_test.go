package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	emailStore := t.TempDir()
	settings.MailgunAPIKey = testAPIKey
	settings.EmailStoreDir = emailStore

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

	// Verify the file landed in inbox/.
	inboxDir := filepath.Join(emailStore, "inbox")
	entries, _ := os.ReadDir(inboxDir)
	if len(entries) == 0 {
		t.Fatal("no file in inbox")
	}
	if !strings.HasSuffix(entries[0].Name(), ".eml") {
		t.Errorf("expected .eml file, got %s", entries[0].Name())
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
	c.Request = httptest.NewRequest(http.MethodPost, "/api/email/inbound",
		strings.NewReader(""))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s := &server{env: "test"}
	s.handleInbound(c)

	// Empty body: ParseRawPayload returns empty fields, file saved to inbox.
	if w.Code != http.StatusOK {
		t.Errorf("empty form should return 200, got %d; body = %s",
			w.Code, w.Body.String())
	}

	// File should still be saved.
	inbox := filepath.Join(settings.EmailStoreDir, "inbox")
	entries, _ := os.ReadDir(inbox)
	if len(entries) == 0 {
		t.Error("expected a file in inbox even with empty body")
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