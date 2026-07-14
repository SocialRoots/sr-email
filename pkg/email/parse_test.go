package email

import (
	"os"
	"strings"
	"testing"
)

func TestExtractReplyText(t *testing.T) {
	tests := []struct {
		name         string
		strippedText string
		strippedHTML string
		bodyHTML     string
		want         string
	}{
		{
			name:         "stripped-text takes priority",
			strippedText: "this is my reply",
			strippedHTML: "<p>html reply</p>",
			bodyHTML:     "<p>full html</p>",
			want:         "this is my reply",
		},
		{
			name:         "stripped-html fallback",
			strippedText: "",
			strippedHTML: "<p>html <strong>reply</strong></p>",
			bodyHTML:     "<p>full html</p>",
			want:         "html reply",
		},
		{
			name:         "body-html fallback with original message trim",
			strippedText: "",
			strippedHTML: "",
			bodyHTML:     "<p>my reply text</p>\n------ Original Message ------\n<p>original</p>",
			want:         "my reply text",
		},
		{
			name:         "body-html full when no separator",
			strippedText: "",
			strippedHTML: "",
			bodyHTML:     "<p>just this</p>",
			want:         "just this",
		},
		{
			name:         "all empty returns empty",
			strippedText: "",
			strippedHTML: "",
			bodyHTML:     "",
			want:         "",
		},
		{
			name:         "body-html handles missing separator gracefully",
			strippedText: "",
			strippedHTML: "",
			bodyHTML:     "<p>text</p>\n------ Original Message ------",
			want:         "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractReplyText(tt.strippedText, tt.strippedHTML, tt.bodyHTML)
			if got != tt.want {
				t.Errorf("ExtractReplyText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractNoteLink(t *testing.T) {
	tests := []struct {
		name      string
		recipient string
		want      string
	}{
		{
			name:      "standard format",
			recipient: "notekey123@mg.socialroots.org",
			want:      "notekey123",
		},
		{
			name:      "no @ returns as-is",
			recipient: "notekey123",
			want:      "notekey123",
		},
		{
			name:      "empty string",
			recipient: "",
			want:      "",
		},
		{
			name:      "multiple @ uses first segment",
			recipient: "key@sub@domain.com",
			want:      "key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractNoteLink(tt.recipient)
			if got != tt.want {
				t.Errorf("ExtractNoteLink(%q) = %q, want %q", tt.recipient, got, tt.want)
			}
		})
	}
}

func TestExtractReplyName(t *testing.T) {
	tests := []struct {
		name string
		from string
		want string
	}{
		{
			name: "display name with angle brackets",
			from: "Jane Smith <jane@example.com>",
			want: "Jane Smith",
		},
		{
			name: "bare email returns as-is",
			from: "jane@example.com",
			want: "jane@example.com",
		},
		{
			name: "empty string",
			from: "",
			want: "",
		},
		{
			name: "whitespace around name",
			from: "  Bob  <bob@example.com>",
			want: "Bob",
		},
		{
			name: "special characters in name",
			from: "\"O'Brien, Jane\" <jane@example.com>",
			want: "\"O'Brien, Jane\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractReplyName(tt.from)
			if got != tt.want {
				t.Errorf("ExtractReplyName(%q) = %q, want %q", tt.from, got, tt.want)
			}
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name        string
		dateHeader  string
		want        string
	}{
		{
			name:       "RFC1123Z",
			dateHeader: "Mon, 02 Jan 2006 15:04:05 -0700",
			want:       "2006-01-02T22:04:05Z",
		},
		{
			name:       "RFC3339",
			dateHeader: "2006-01-02T15:04:05Z",
			want:       "2006-01-02T15:04:05Z",
		},
		{
			name:       "invalid returns original",
			dateHeader: "not-a-date",
			want:       "not-a-date",
		},
		{
			name:       "empty string",
			dateHeader: "",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTimestamp(tt.dateHeader)
			if got != tt.want {
				t.Errorf("FormatTimestamp(%q) = %q, want %q", tt.dateHeader, got, tt.want)
			}
		})
	}
}

func TestParseRawPayload_Multipart(t *testing.T) {
	raw := `--BOUNDARY
Content-Disposition: form-data; name="timestamp"

1784068778
--BOUNDARY
Content-Disposition: form-data; name="token"

abc123
--BOUNDARY
Content-Disposition: form-data; name="signature"

sig456
--BOUNDARY
Content-Disposition: form-data; name="recipient"

notekey@reply.example.com
--BOUNDARY
Content-Disposition: form-data; name="sender"

jane@example.com
--BOUNDARY
Content-Disposition: form-data; name="from"

Jane Smith <jane@example.com>
--BOUNDARY
Content-Disposition: form-data; name="subject"

Re: test
--BOUNDARY
Content-Disposition: form-data; name="stripped-text"

this is my reply
--BOUNDARY
Content-Disposition: form-data; name="body-plain"

full body text
--BOUNDARY
Content-Disposition: form-data; name="body-html"

<p>html part 1</p>
--BOUNDARY
Content-Disposition: form-data; name="body-html"

<p>html part 2</p>
--BOUNDARY
Content-Disposition: form-data; name="message-headers"

[["From","jane@example.com"]]
--BOUNDARY--
`

	p, err := ParseRawPayload([]byte(raw))
	if err != nil {
		t.Fatalf("ParseRawPayload() error = %v", err)
	}

	if p.Timestamp != "1784068778" {
		t.Errorf("Timestamp = %q, want %q", p.Timestamp, "1784068778")
	}
	if p.Token != "abc123" {
		t.Errorf("Token = %q, want %q", p.Token, "abc123")
	}
	if p.Signature != "sig456" {
		t.Errorf("Signature = %q, want %q", p.Signature, "sig456")
	}
	if p.Recipient != "notekey@reply.example.com" {
		t.Errorf("Recipient = %q", p.Recipient)
	}
	if p.Sender != "jane@example.com" {
		t.Errorf("Sender = %q", p.Sender)
	}
	if p.From != "Jane Smith <jane@example.com>" {
		t.Errorf("From = %q", p.From)
	}
	if p.Subject != "Re: test" {
		t.Errorf("Subject = %q", p.Subject)
	}
	if p.StrippedText != "this is my reply" {
		t.Errorf("StrippedText = %q", p.StrippedText)
	}
	if p.BodyPlain != "full body text" {
		t.Errorf("BodyPlain = %q", p.BodyPlain)
	}
	if len(p.BodyHTML) != 2 {
		t.Errorf("len(BodyHTML) = %d, want 2", len(p.BodyHTML))
	} else {
		if p.BodyHTML[0] != "<p>html part 1</p>" {
			t.Errorf("BodyHTML[0] = %q", p.BodyHTML[0])
		}
		if p.BodyHTML[1] != "<p>html part 2</p>" {
			t.Errorf("BodyHTML[1] = %q", p.BodyHTML[1])
		}
	}
	if p.MessageHeaders != "[[\"From\",\"jane@example.com\"]]" {
		t.Errorf("MessageHeaders = %q", p.MessageHeaders)
	}
}

func TestParseRawPayload_MultipartWithAttachment(t *testing.T) {
	// Attachments (parts with filename) should be skipped.
	raw := `--BOUNDARY
Content-Disposition: form-data; name="timestamp"

1784068778
--BOUNDARY
Content-Disposition: form-data; name="stripped-text"

reply text
--BOUNDARY
Content-Disposition: form-data; name="attachment-1"; filename="signature.asc"
Content-Type: application/pgp-signature

-----BEGIN PGP SIGNATURE-----
ignore
-----END PGP SIGNATURE-----
--BOUNDARY--
`

	p, err := ParseRawPayload([]byte(raw))
	if err != nil {
		t.Fatalf("ParseRawPayload() error = %v", err)
	}

	if p.Timestamp != "1784068778" {
		t.Errorf("Timestamp = %q", p.Timestamp)
	}
	if p.StrippedText != "reply text" {
		t.Errorf("StrippedText = %q", p.StrippedText)
	}
}

func TestParseRawPayload_URLEncoded(t *testing.T) {
	// URL-encoded fallback must still work.
	raw := "timestamp=1784068778&token=abc&signature=sig&recipient=key@ex.com&from=Jane&stripped-text=reply"

	p, err := ParseRawPayload([]byte(raw))
	if err != nil {
		t.Fatalf("ParseRawPayload() error = %v", err)
	}

	if p.Timestamp != "1784068778" {
		t.Errorf("Timestamp = %q", p.Timestamp)
	}
	if p.StrippedText != "reply" {
		t.Errorf("StrippedText = %q", p.StrippedText)
	}
	if p.From != "Jane" {
		t.Errorf("From = %q", p.From)
	}
}

func TestParseRawPayload_EmptyBody(t *testing.T) {
	_, err := ParseRawPayload([]byte(""))
	if err == nil {
		t.Error("expected error for empty body")
	}
}

func TestExtractReplyText_OnWroteSeparator(t *testing.T) {
	got := ExtractReplyText("", "", `<p>my reply text</p>

On Wed, Jul 14, 2026 at 10:39 PM, Jane Smith <jane@example.com> wrote:
> original message
`)
	want := "my reply text"
	if got != want {
		t.Errorf("ExtractReplyText() = %q, want %q", got, want)
	}
}

func TestExtractReplyText_MultipleSeparators(t *testing.T) {
	got := ExtractReplyText("", "", `<p>my reply</p>
-------- Original Message --------
<p>original</p>`)
	want := "my reply"
	if got != want {
		t.Errorf("ExtractReplyText() = %q, want %q", got, want)
	}
}

// Regression test: parse a real MailGun multipart payload captured from production.
func TestParseRawPayload_RegressionFromProduction(t *testing.T) {
	raw, err := os.ReadFile("testdata/20260714T223938_9479e7adea1b4076bad68b0177071e46.eml")
	if err != nil {
		t.Skipf("production capture file not found: %v", err)
	}

	p, err := ParseRawPayload(raw)
	if err != nil {
		t.Fatalf("ParseRawPayload() error = %v", err)
	}

	if p.Timestamp != "1784068778" {
		t.Errorf("Timestamp = %q, want %q", p.Timestamp, "1784068778")
	}
	if p.Token != "5f9e963e30e1c71fa6f7fe04f331724147c2db2d006a0c0c1c" {
		t.Errorf("Token = %q", p.Token)
	}
	if p.Recipient != "9479e7adea1b4076bad68b0177071e46@reply.alpha.socialroots.io" {
		t.Errorf("Recipient = %q", p.Recipient)
	}
	if p.Sender != "social+roots@nigini.me" {
		t.Errorf("Sender = %q", p.Sender)
	}
	if p.From != "Nigini Oliveira <social+roots@nigini.me>" {
		t.Errorf("From = %q", p.From)
	}
	if p.Subject != "Re: New replies in Welcome to secured-up Socialroots..." {
		t.Errorf("Subject = %q", p.Subject)
	}
	if p.StrippedText == "" {
		t.Error("StrippedText is empty — expected reply text")
	}
	if !strings.Contains(p.StrippedText, "Can I reply to this via email") {
		t.Errorf("StrippedText = %q, should contain reply body", p.StrippedText)
	}
	if len(p.BodyHTML) == 0 {
		t.Error("BodyHTML is empty — expected at least one HTML body")
	}
	if p.BodyPlain == "" {
		t.Error("BodyPlain is empty — expected plain text body")
	}
	if p.MessageHeaders == "" {
		t.Error("MessageHeaders is empty — expected headers array")
	}

	// End-to-end: verify the full reply extraction pipeline
	rawHTML := strings.Join(p.BodyHTML, "\n")
	if rawHTML == "" {
		rawHTML = p.BodyPlain
	}
	replyText := ExtractReplyText(p.StrippedText, p.StrippedHTML, rawHTML)
	if replyText == "" {
		t.Error("ExtractReplyText produced empty result — expected text")
	}
	if !strings.Contains(replyText, "Can I reply to this via email") {
		t.Errorf("ExtractReplyText = %q, should contain reply body", replyText)
	}

	noteLink := ExtractNoteLink(p.Recipient)
	if noteLink != "9479e7adea1b4076bad68b0177071e46" {
		t.Errorf("ExtractNoteLink = %q, want %q", noteLink, "9479e7adea1b4076bad68b0177071e46")
	}

	replyName := ExtractReplyName(p.From)
	if replyName != "Nigini Oliveira" {
		t.Errorf("ExtractReplyName = %q, want %q", replyName, "Nigini Oliveira")
	}
}