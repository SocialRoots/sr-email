package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SocialRoots/sr-email/settings"
)

type replyPayload struct {
	Link       string `json:"link"`
	Reply      string `json:"reply"`
	ReplyName  string `json:"reply_name"`
	ReplyEmail string `json:"reply_email"`
}

// ProcessInbox scans the inbox directory and processes each file.
// Called by the cron loop; safe to run concurrently with SaveToInbox
// because os.Rename is atomic within the same filesystem.
func ProcessInbox() {
	dir := inboxDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return // nothing to do
		}
		log.Printf("[sr-email] inbox read error: %v", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(dir, name)

		processFile(path, name)
	}
}

func processFile(path, name string) {
	log.Printf("PROCESSING %s", name)

	raw, err := os.ReadFile(path)
	if err != nil {
		log.Printf("FAIL %s: read error: %v", name, err)
		MoveToFailed(name)
		return
	}

	payload, err := ParseRawPayload(raw)
	if err != nil {
		log.Printf("FAIL %s: parse error: %v", name, err)
		MoveToFailed(name)
		return
	}

	// Build the reply payload.
	rawHTML := strings.Join(payload.BodyHTML, "\n")
	if rawHTML == "" {
		rawHTML = payload.BodyPlain
	}
	replyText := ExtractReplyText(payload.StrippedText, payload.StrippedHTML, rawHTML)
	noteLink := ExtractNoteLink(payload.Recipient)
	replyName := ExtractReplyName(payload.From)

	rp := replyPayload{
		Link:       noteLink,
		Reply:      replyText,
		ReplyName:  replyName,
		ReplyEmail: payload.Sender,
	}

	if err := forwardReply(rp); err != nil {
		log.Printf("FAIL %s: forward error: %v", name, err)
		MoveToFailed(name)
		return
	}

	log.Printf("SUCCESS %s", name)
	MoveToArchive(name)
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