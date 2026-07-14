package email

import (
	"net/url"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
)

// MailgunPayload mirrors the form fields MailGun sends in a forward() POST.
type MailgunPayload struct {
	Timestamp      string
	Token          string
	Signature      string
	Recipient      string
	Sender         string
	From           string
	Subject        string
	StrippedText   string
	StrippedHTML   string
	BodyHTML       []string
	BodyPlain      string
	MessageHeaders string
}

// ParseRawPayload decodes a raw form-encoded MailGun POST body.
// Handles application/x-www-form-urlencoded content type.
// TODO: support multipart/form-data when attachments are present.
func ParseRawPayload(raw []byte) (MailgunPayload, error) {
	vals, err := url.ParseQuery(string(raw))
	if err != nil {
		return MailgunPayload{}, err
	}

	return MailgunPayload{
		Timestamp:      vals.Get("timestamp"),
		Token:          vals.Get("token"),
		Signature:      vals.Get("signature"),
		Recipient:      vals.Get("recipient"),
		Sender:         vals.Get("sender"),
		From:           vals.Get("from"),
		Subject:        vals.Get("subject"),
		StrippedText:   vals.Get("stripped-text"),
		StrippedHTML:   vals.Get("stripped-html"),
		BodyHTML:       vals["body-html"],
		BodyPlain:      vals.Get("body-plain"),
		MessageHeaders: vals.Get("message-headers"),
	}, nil
}

// ExtractReplyText returns the clean reply text, prioritizing stripped fields.
func ExtractReplyText(strippedText, strippedHTML, bodyHTML string) string {
	p := bluemonday.StrictPolicy()

	if strippedText != "" {
		return strings.TrimSpace(strippedText)
	}
	if strippedHTML != "" {
		return strings.TrimSpace(p.Sanitize(strippedHTML))
	}
	if bodyHTML != "" {
		parts := strings.SplitN(bodyHTML, "------ Original Message ------", 2)
		return strings.TrimSpace(p.Sanitize(parts[0]))
	}
	return ""
}

// ExtractNoteLink extracts the local-part (before @) from the recipient email.
func ExtractNoteLink(recipient string) string {
	if idx := strings.Index(recipient, "@"); idx != -1 {
		return recipient[:idx]
	}
	return recipient
}

// ExtractReplyName extracts the display name from a From header like
// "Jane Smith <jane@example.com>" or bare "jane@example.com".
func ExtractReplyName(from string) string {
	if idx := strings.Index(from, "<"); idx != -1 {
		return strings.TrimSpace(from[:idx])
	}
	return from
}

// FormatTimestamp parses common date formats and returns RFC3339.
func FormatTimestamp(dateHeader string) string {
	t, err := time.Parse(time.RFC1123Z, dateHeader)
	if err != nil {
		t, err = time.Parse(time.RFC3339, dateHeader)
		if err != nil {
			return dateHeader
		}
	}
	return t.UTC().Format(time.RFC3339)
}