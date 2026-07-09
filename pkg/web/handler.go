package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/SocialRoots/sr-email/pkg/email"
	"github.com/SocialRoots/sr-email/settings"
	"github.com/gin-gonic/gin"
)

type mailgunPayload struct {
	Timestamp    string `form:"timestamp"`
	Token        string `form:"token"`
	Signature    string `form:"signature"`
	Recipient    string `form:"recipient"`
	Sender       string `form:"sender"`
	From         string `form:"from"`
	Subject      string `form:"subject"`
	StrippedText string `form:"stripped-text"`
	StrippedHTML string `form:"stripped-html"`
	BodyHTML     string `form:"body-html"`
	BodyPlain    string `form:"body-plain"`
}

type replyPayload struct {
	Link       string `json:"link"`
	Reply      string `json:"reply"`
	ReplyName  string `json:"reply_name"`
	ReplyEmail string `json:"reply_email"`
}

func (s *server) handleInbound(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[sr-email] failed to read body: %v", err)
		c.JSON(500, gin.H{"error": "failed to read request body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))

	var p mailgunPayload
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

	go email.SaveRaw(raw, p.Timestamp, p.Recipient)

	replyText := email.ExtractReplyText(p.StrippedText, p.StrippedHTML, p.BodyHTML)
	noteLink := email.ExtractNoteLink(p.Recipient)
	replyName := email.ExtractReplyName(p.From)

	if err := forwardReply(replyPayload{
		Link:       noteLink,
		Reply:      replyText,
		ReplyName:  replyName,
		ReplyEmail: p.Sender,
	}); err != nil {
		log.Printf("[sr-email] failed to forward reply: %v", err)
		c.JSON(200, gin.H{
			"status":  "stored",
			"message": "email saved, reply forwarding failed — will retry",
		})
		return
	}

	log.Printf("[sr-email] reply forwarded for %s", noteLink)
	c.JSON(200, gin.H{"status": "ok"})
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

func forwardReply(rp replyPayload) error {
	if settings.ResponsesServiceURL == "" {
		return fmt.Errorf("ROOTSHOOT_RESPONSES_SERVICE not set")
	}

	body, err := json.Marshal(rp)
	if err != nil {
		return fmt.Errorf("marshal reply: %w", err)
	}

	url := settings.ResponsesServiceURL + "/response/reply"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-SR-Internal-Token", settings.InternalToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("RS-RESPONSES returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}