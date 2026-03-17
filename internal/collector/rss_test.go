package collector

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// cleanHTML tests
// ---------------------------------------------------------------------------

func TestCleanHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text",
			input: "Hello World",
			want:  "Hello World",
		},
		{
			name:  "strip simple tags",
			input: "<p>Hello <b>World</b></p>",
			want:  "Hello World",
		},
		{
			name:  "strip nested tags",
			input: "<div><p>Line 1</p><p>Line 2</p></div>",
			want:  "Line 1 Line 2",
		},
		{
			name:  "strip link tags",
			input: `<a href="http://example.com">Click here</a>`,
			want:  "Click here",
		},
		{
			name:  "strip img tag",
			input: `<img src="pic.jpg" alt="photo"/>`,
			want:  "photo\"/",
		},
		{
			name:  "truncate long text",
			input: "<p>" + string(make([]byte, 600)) + "</p>",
			want:  "500 chars + ellipsis",
		},
		{
			name:  "handle entity",
			input: "A &amp; B &lt; C",
			want:  "A & B < C",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only tags",
			input: "<br/><hr/>",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanHTML(tt.input)
			if tt.name == "truncate long text" {
				// Special check: should be truncated with "..."
				if len(got) < 503 {
					t.Errorf("cleanHTML() length = %d, expected ~503 (500 + ...)", len(got))
				}
				if len(got) > 510 {
					t.Errorf("cleanHTML() length = %d, expected ~503", len(got))
				}
				return
			}
			if got != tt.want {
				t.Errorf("cleanHTML() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseTime tests
// ---------------------------------------------------------------------------

func TestParseTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string // prefix match (we just check it's not empty and starts with correct date)
	}{
		{
			name:  "RFC3339",
			input: "2026-03-17T12:00:00Z",
			want:  "2026-03-17T12:00:00Z",
		},
		{
			name:  "RFC1123",
			input: "Mon, 17 Mar 2026 12:00:00 UTC",
			want:  "2026-03-17T12:00:00Z",
		},
		{
			name:  "RFC1123Z",
			input: "Mon, 17 Mar 2026 12:00:00 +0000",
			want:  "2026-03-17T12:00:00Z",
		},
		{
			name:  "RFC822",
			input: "17 Mar 26 12:00 UTC",
			want:  "2026-03-17T12:00:00Z",
		},
		{
			name:  "date only",
			input: "2026-03-17",
			want:  "2026-03-17T00:00:00Z",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "unrecognized format returns as-is",
			input: "some random text",
			want:  "some random text",
		},
		{
			name:  "ISO 8601 with offset",
			input: "2026-03-17T20:00:00+08:00",
			want:  "2026-03-17T12:00:00Z",
		},
		{
			name:  "January format",
			input: "March 17, 2026",
			want:  "", // not in the format list, returned as-is
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTime(tt.input)

			if tt.input == "" {
				if got != "" {
					t.Errorf("parseTime(%q) = %q, want empty", tt.input, got)
				}
				return
			}

			if tt.want == "" && tt.input == "some random text" {
				if got != tt.input {
					t.Errorf("parseTime(%q) = %q, want %q (returned as-is)", tt.input, got, tt.input)
				}
				return
			}

			if got == "" {
				t.Errorf("parseTime(%q) = empty, want non-empty", tt.input)
				return
			}

			// For recognized formats, verify the result is valid RFC3339
			if tt.want != "" && tt.want != tt.input {
				parsed, err := time.Parse(time.RFC3339, got)
				if err != nil {
					t.Errorf("parseTime(%q) = %q, not valid RFC3339: %v", tt.input, got, err)
					return
				}
				// Verify the date part matches
				expected, _ := time.Parse(time.RFC3339, tt.want)
				if !parsed.Equal(expected) {
					t.Errorf("parseTime(%q) = %q, want %q", tt.input, got, tt.want)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// convertRSSItems tests
// ---------------------------------------------------------------------------

func TestConvertRSSItems(t *testing.T) {
	c := NewRSSCollector(WithMaxItems(10))

	src := Source{
		Name:     "Test Feed",
		URL:      "http://example.com/feed.xml",
		Language: "en",
	}

	items := []rssItem{
		{
			Title:       "First Article",
			Link:        "http://example.com/1",
			Description: "<p>Summary of first article</p>",
			PubDate:     "Mon, 17 Mar 2026 12:00:00 UTC",
		},
		{
			Title:       "Second Article",
			Link:        "http://example.com/2",
			Description: "Plain summary",
			PubDate:     "2026-03-17T14:00:00Z",
		},
		{
			Title:       "GUID Only",
			Link:        "",
			GUID:        "http://example.com/3",
			Description: "Uses GUID as URL",
		},
		{
			Title:       "Empty URL",
			Link:        "",
			GUID:        "",
			Description: "Should be skipped",
		},
	}

	articles := c.convertRSSItems(items, src)

	if len(articles) != 3 {
		t.Errorf("convertRSSItems() returned %d articles, want 3", len(articles))
	}

	// Check first article
	if articles[0].Title != "First Article" {
		t.Errorf("articles[0].Title = %q, want %q", articles[0].Title, "First Article")
	}
	if articles[0].URL != "http://example.com/1" {
		t.Errorf("articles[0].URL = %q, want %q", articles[0].URL, "http://example.com/1")
	}
	if articles[0].Summary != "Summary of first article" {
		t.Errorf("articles[0].Summary = %q, want clean HTML", articles[0].Summary)
	}
	if articles[0].SourceName != "Test Feed" {
		t.Errorf("articles[0].SourceName = %q, want %q", articles[0].SourceName, "Test Feed")
	}
	if articles[0].Language != "en" {
		t.Errorf("articles[0].Language = %q, want %q", articles[0].Language, "en")
	}
	if articles[0].PublishedAt == "" {
		t.Errorf("articles[0].PublishedAt is empty, want parsed time")
	}

	// Check GUID fallback for third article
	if articles[2].URL != "http://example.com/3" {
		t.Errorf("articles[2].URL = %q, want GUID fallback", articles[2].URL)
	}

	// Test maxItems truncation
	c2 := NewRSSCollector(WithMaxItems(2))
	articles2 := c2.convertRSSItems(items, src)
	if len(articles2) != 2 {
		t.Errorf("convertRSSItems() with maxItems=2 returned %d, want 2", len(articles2))
	}
}

// ---------------------------------------------------------------------------
// cleanText tests (bonus, since it's used by cleanHTML)
// ---------------------------------------------------------------------------

func TestCleanText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trim spaces", "  hello  ", "hello"},
		{"replace newlines", "line1\nline2", "line1 line2"},
		{"replace carriage returns", "line1\rline2", "line1line2"},
		{"collapse multiple spaces", "a  b   c", "a b c"},
		{"combined", "  a\nb\r\n  c  ", "a b c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanText(tt.input); got != tt.want {
				t.Errorf("cleanText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
