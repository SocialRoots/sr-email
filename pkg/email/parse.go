package email

import (
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
)

func ExtractReplyText(strippedText, strippedHTML, bodyHTML string) string {
	p := bluemonday.StrictPolicy()

	if strippedText != "" {
		return strippedText
	}
	if strippedHTML != "" {
		return p.Sanitize(strippedHTML)
	}
	if bodyHTML != "" {
		parts := strings.SplitN(bodyHTML, "------ Original Message ------", 2)
		return p.Sanitize(parts[0])
	}
	return ""
}

func ExtractNoteLink(recipient string) string {
	if idx := strings.Index(recipient, "@"); idx != -1 {
		return recipient[:idx]
	}
	return recipient
}

func ExtractReplyName(from string) string {
	if idx := strings.Index(from, "<"); idx != -1 {
		return strings.TrimSpace(from[:idx])
	}
	return from
}

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