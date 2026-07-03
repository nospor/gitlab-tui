package gitlab

import (
	"html"
	"strings"
)

// ConvertHTMLToMarkdown parses simple HTML fragments from GitLab API (e.g. system notes)
// and converts them to a plain text / markdown representation suitable for a TUI.
func ConvertHTMLToMarkdown(input string) string {
	if input == "" {
		return ""
	}

	// If the input doesn't contain '<', it's highly likely plain text/markdown already.
	if !strings.Contains(input, "<") {
		return cleanResult(html.UnescapeString(input))
	}

	var buf strings.Builder
	listLevel := 0

	writeNewline := func() {
		s := buf.String()
		if len(s) == 0 {
			return
		}
		if !strings.HasSuffix(s, "\n") {
			buf.WriteByte('\n')
		}
	}

	writeDoubleNewline := func() {
		s := buf.String()
		if len(s) == 0 {
			return
		}
		if strings.HasSuffix(s, "\n\n") {
			return
		} else if strings.HasSuffix(s, "\n") {
			buf.WriteByte('\n')
		} else {
			buf.WriteString("\n\n")
		}
	}

	inTag := false
	var tagBuf strings.Builder
	i := 0
	n := len(input)

	for i < n {
		c := input[i]
		if c == '<' {
			inTag = true
			tagBuf.Reset()
			i++
			continue
		}
		if c == '>' && inTag {
			inTag = false
			tag := tagBuf.String()
			handleTag(tag, &buf, &listLevel, writeNewline, writeDoubleNewline)
			i++
			continue
		}

		if inTag {
			tagBuf.WriteByte(c)
		} else {
			buf.WriteByte(c)
		}
		i++
	}

	res := html.UnescapeString(buf.String())
	return cleanResult(res)
}

func handleTag(tag string, buf *strings.Builder, listLevel *int, writeNewline, writeDoubleNewline func()) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return
	}
	isClosing := strings.HasPrefix(tag, "/")
	tagName := tag
	if isClosing {
		tagName = tag[1:]
	} else {
		// Split by spaces to ignore attributes
		fields := strings.Fields(tag)
		if len(fields) > 0 {
			tagName = fields[0]
		}
	}
	tagName = strings.ToLower(strings.TrimSuffix(tagName, "/"))

	switch tagName {
	case "ul", "ol":
		if isClosing {
			if *listLevel > 0 {
				*listLevel--
			}
			if *listLevel == 0 {
				writeDoubleNewline()
			} else {
				writeNewline()
			}
		} else {
			if *listLevel == 0 {
				writeDoubleNewline()
			} else {
				writeNewline()
			}
			*listLevel++
		}
	case "li":
		if !isClosing {
			writeNewline()
			// Indentation
			for idx := 0; idx < *listLevel-1; idx++ {
				buf.WriteString("  ")
			}
			buf.WriteString("- ")
		}
	case "p":
		writeDoubleNewline()
	case "br":
		writeNewline()
	case "strong", "b":
		buf.WriteString("**")
	case "em", "i":
		buf.WriteString("*")
	case "code":
		buf.WriteString("`")
	case "pre":
		if !isClosing {
			writeDoubleNewline()
			buf.WriteString("```\n")
		} else {
			writeNewline()
			buf.WriteString("```")
			writeDoubleNewline()
		}
	case "del", "s":
		buf.WriteString("~~")
	}
}

func cleanResult(s string) string {
	lines := strings.Split(s, "\n")
	var cleanedLines []string

	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		cleanedLines = append(cleanedLines, trimmed)
	}

	var finalLines []string
	consecutiveBlanks := 0
	for _, line := range cleanedLines {
		if line == "" {
			consecutiveBlanks++
			if consecutiveBlanks <= 1 {
				finalLines = append(finalLines, "")
			}
		} else {
			consecutiveBlanks = 0
			finalLines = append(finalLines, line)
		}
	}

	res := strings.Join(finalLines, "\n")
	return strings.Trim(res, "\n")
}
