package email

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/url"
	"regexp"
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

// ParseRawPayload decodes a raw MailGun POST body.
// Detects format: multipart/form-data (starts with "--") or
// application/x-www-form-urlencoded (fallback).
func ParseRawPayload(raw []byte) (MailgunPayload, error) {
	if len(raw) == 0 {
		return MailgunPayload{}, io.ErrUnexpectedEOF
	}
	if bytes.HasPrefix(raw, []byte("--")) {
		return parseMultipart(raw)
	}
	return parseURLEncoded(raw)
}

func parseURLEncoded(raw []byte) (MailgunPayload, error) {
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

// parseMultipart parses a multipart/form-data body sent by MailGun.
// The boundary is extracted from the first line (which is "--BOUNDARY").
func parseMultipart(raw []byte) (MailgunPayload, error) {
	// Extract boundary from first line: "--BOUNDARY\r\n" or "--BOUNDARY\n"
	firstLine := raw
	if idx := bytes.IndexByte(raw, '\n'); idx != -1 {
		firstLine = raw[:idx]
	}
	boundary := strings.TrimPrefix(string(firstLine), "--")
	boundary = strings.TrimRight(boundary, "\r")
	if boundary == "" {
		return MailgunPayload{}, io.ErrUnexpectedEOF
	}

	reader := multipart.NewReader(bytes.NewReader(raw), boundary)

	var payload MailgunPayload
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return MailgunPayload{}, err
		}

		name := part.FormName()
		if name == "" {
			part.Close()
			continue
		}

		value, err := io.ReadAll(part)
		part.Close()
		if err != nil {
			return MailgunPayload{}, err
		}

		// Skip attachments (parts with a filename)
		if part.FileName() != "" {
			continue
		}

		switch name {
		case "timestamp":
			payload.Timestamp = string(value)
		case "token":
			payload.Token = string(value)
		case "signature":
			payload.Signature = string(value)
		case "recipient":
			payload.Recipient = string(value)
		case "sender":
			payload.Sender = string(value)
		case "from":
			payload.From = string(value)
		case "subject":
			payload.Subject = string(value)
		case "stripped-text":
			payload.StrippedText = string(value)
		case "stripped-html":
			payload.StrippedHTML = string(value)
		case "body-html":
			payload.BodyHTML = append(payload.BodyHTML, string(value))
		case "body-plain":
			payload.BodyPlain = string(value)
		case "message-headers":
			payload.MessageHeaders = string(value)
		}
	}

	return payload, nil
}

var (
	onWroteRegexp    = regexp.MustCompile(`(?i)\nOn\s+.+\s+wrote:\s*\n`)
	originalMsgRegexp = regexp.MustCompile(`(?i)(?:\n|^)-{3,}\s*Original\s+Message\s*-{3,}(?:\s*\n|$)`)
)

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
		// Try "------ Original Message ------" variants.
		if loc := originalMsgRegexp.FindStringIndex(bodyHTML); loc != nil {
			return strings.TrimSpace(p.Sanitize(bodyHTML[:loc[0]]))
		}
		// Try "On [date], [name] wrote:" pattern (ProtonMail, Gmail, etc.)
		if loc := onWroteRegexp.FindStringIndex(bodyHTML); loc != nil {
			return strings.TrimSpace(p.Sanitize(bodyHTML[:loc[0]]))
		}
		return strings.TrimSpace(p.Sanitize(bodyHTML))
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