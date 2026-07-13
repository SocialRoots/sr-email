package email

import (
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
			bodyHTML:     "------ Original Message ------",
			want:         "",
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