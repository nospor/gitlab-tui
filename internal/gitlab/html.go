package gitlab

import (
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
)

var (
	reDiffAdd = regexp.MustCompile(`\{\+\s*(.*?)\s*\+\}`)
	reDiffDel = regexp.MustCompile(`\[\-\s*(.*?)\s*\-\]`)
	reBold    = regexp.MustCompile(`\*\*(.*?)\*\*`)
	reBold2   = regexp.MustCompile(`__(.*?)__`)
)

// ConvertHTMLToMarkdown parses simple HTML fragments from GitLab API (e.g. system notes)
// and converts them to a plain text / markdown representation suitable for a TUI.
func ConvertHTMLToMarkdown(input string) string {
	if input == "" {
		return ""
	}

	opt := &md.Options{
		EscapeMode: "disabled",
	}
	converter := md.NewConverter("", true, opt)
	converter.Use(plugin.GitHubFlavored())
	markdown, err := converter.ConvertString(input)
	if err != nil {
		markdown = input
	}
	return UnescapeMarkdown(markdown)
}

// UnescapeMarkdown strips backslash escapes for markdown formatting characters.
func UnescapeMarkdown(s string) string {
	s = strings.ReplaceAll(s, "\\**", "**")
	s = strings.ReplaceAll(s, "\\*", "*")
	s = strings.ReplaceAll(s, "\\_", "_")
	s = strings.ReplaceAll(s, "\\-", "-")
	s = strings.ReplaceAll(s, "\\[", "[")
	s = strings.ReplaceAll(s, "\\]", "]")
	s = strings.ReplaceAll(s, "\\`", "`")
	s = strings.ReplaceAll(s, "\\~", "~")
	s = strings.ReplaceAll(s, "\\#", "#")
	return s
}

// FormatSystemNote cleans system note bodies for TUI display:
// unescapes markdown backslashes, formats diff additions/deletions,
// and strips raw markdown asterisks/underscores.
func FormatSystemNote(input string) string {
	if input == "" {
		return ""
	}
	s := UnescapeMarkdown(input)
	s = reDiffAdd.ReplaceAllString(s, " +$1")
	s = reDiffDel.ReplaceAllString(s, " -$1")
	s = reBold.ReplaceAllString(s, "$1")
	s = reBold2.ReplaceAllString(s, "$1")
	return s
}

