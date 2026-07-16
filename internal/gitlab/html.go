package gitlab

import (
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
)

// ConvertHTMLToMarkdown parses simple HTML fragments from GitLab API (e.g. system notes)
// and converts them to a plain text / markdown representation suitable for a TUI.
func ConvertHTMLToMarkdown(input string) string {
	if input == "" {
		return ""
	}

	converter := md.NewConverter("", true, nil)
	converter.Use(plugin.GitHubFlavored())
	markdown, err := converter.ConvertString(input)
	if err != nil {
		return input
	}
	return markdown
}

