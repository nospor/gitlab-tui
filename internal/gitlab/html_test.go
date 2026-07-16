package gitlab

import (
	"testing"
)

func TestConvertHTMLToMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Plain text unchanged",
			input: "Hello World",
			want:  "Hello World",
		},
		{
			name:  "Unescape HTML entities in plain text",
			input: "Hello &amp; World &lt;3",
			want:  "Hello & World <3",
		},
		{
			name:  "Simple list from GitLab system note",
			input: "added 1 commit:\n\n<ul><li>93e7cd05 - fix: Fixed linting errors</li></ul>",
			want:  "added 1 commit:\n\n- 93e7cd05 - fix: Fixed linting errors",
		},
		{
			name:  "List with link and attributes",
			input: `<ul class="commits-list"><li class="commit"><a href="/proj/commit/1234">1234</a> - feat: done</li></ul>`,
			want:  "- [1234](/proj/commit/1234) \\- feat: done",
		},
		{
			name:  "Formatted text tags",
			input: "This is <strong>bold</strong>, <em>italic</em>, <code>code</code>, and <s>deleted</s>.",
			want:  "This is **bold**, _italic_, `code`, and ~~deleted~~.",
		},
		{
			name:  "Paragraphs and line breaks",
			input: "<p>First paragraph.</p><br/><p>Second paragraph.</p>",
			want:  "First paragraph.\n\nSecond paragraph.",
		},
		{
			name:  "Pre and Code block",
			input: "Look at this:<pre><code>func main() {}</code></pre>end",
			want:  "Look at this:\n\n```\nfunc main() {}\n```\n\nend",
		},
		{
			name:  "Nested lists",
			input: "<ul><li>Item 1<ul><li>Nested 1</li></ul></li><li>Item 2</li></ul>",
			want:  "- Item 1\n  - Nested 1\n- Item 2",
		},
		{
			name:  "Consecutive newlines cleaned",
			input: "\n\n\nHello\n\n\n\n\nWorld\n\n\n",
			want:  "Hello\n\nWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertHTMLToMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("ConvertHTMLToMarkdown() = %q, want %q", got, tt.want)
			}
		})
	}
}
