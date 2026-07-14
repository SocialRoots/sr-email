package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"

	"github.com/SocialRoots/sr-email/pkg/email"
	"github.com/SocialRoots/sr-email/settings"
	"github.com/gin-gonic/gin"
)

// ginPayload mirrors MailGun form fields with Gin binding tags.
// Used by ShouldBind which handles both url-encoded and multipart.
// Worker code uses email.ParseRawPayload instead.
type ginPayload struct {
	Timestamp      string   `form:"timestamp"`
	Token          string   `form:"token"`
	Signature      string   `form:"signature"`
	Recipient      string   `form:"recipient"`
	Sender         string   `form:"sender"`
	From           string   `form:"from"`
	Subject        string   `form:"subject"`
	StrippedText   string   `form:"stripped-text"`
	StrippedHTML   string   `form:"stripped-html"`
	BodyHTML       []string `form:"body-html"`
	BodyPlain      string   `form:"body-plain"`
	MessageHeaders string   `form:"message-headers"`
}

func (s *server) handleInbound(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[sr-email] failed to read body: %v", err)
		c.JSON(500, gin.H{"error": "failed to read request body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))

	var p ginPayload
	if err := c.ShouldBind(&p); err != nil {
		log.Printf("[sr-email] failed to parse form: %v", err)
		c.JSON(400, gin.H{"error": "invalid form data"})
		return
	}

	if !verifyMailgunSignature(p.Timestamp, p.Token, p.Signature) {
		log.Printf("[sr-email] HMAC verification failed (timestamp=%s)", p.Timestamp)
		c.JSON(401, gin.H{"error": "invalid signature"})
		return
	}

	if _, err := email.SaveToInbox(raw, p.Timestamp, p.Recipient); err != nil {
		log.Printf("[sr-email] failed to save to inbox: %v", err)
		c.JSON(500, gin.H{"error": "failed to store email"})
		return
	}

	log.Printf("[sr-email] queued for %s", email.ExtractNoteLink(p.Recipient))
	c.JSON(200, gin.H{"status": "queued"})
}

func verifyMailgunSignature(timestamp, token, signature string) bool {
	if settings.MailgunAPIKey == "" {
		log.Println("[sr-email] MAILGUN_API_KEY not set — skipping HMAC verification")
		return true
	}
	mac := hmac.New(sha256.New, []byte(settings.MailgunAPIKey))
	mac.Write([]byte(timestamp + token))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}