package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

type styledCell struct {
	r     rune
	style string
	width int
}

func parseStyledLine(line string) []styledCell {
	var cells []styledCell
	var currentStyle string

	i := 0
	n := len(line)
	for i < n {
		if i+1 < n && line[i] == '\x1b' && line[i+1] == '[' {
			// Found ANSI sequence
			start := i
			i += 2 // skip \x1b[
			for i < n {
				c := line[i]
				i++
				// ANSI parameters are typically digits, semicolons, etc.
				// The sequence ends with a letter (usually 'm' for styling).
				if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
					break
				}
			}
			currentStyle += line[start:i]
		} else {
			// Normal rune
			r, size := utf8.DecodeRuneInString(line[i:])
			if size == 0 {
				break
			}
			i += size

			w := runewidth.RuneWidth(r)
			if w == 0 {
				w = 1 // fallback
			}

			cells = append(cells, styledCell{
				r:     r,
				style: currentStyle,
				width: w,
			})
		}
	}
	return cells
}

type gridCell struct {
	r      rune
	style  string
	isCont bool // true if this is a continuation column of a multi-column rune
}

func lineToGrid(line string, width int) []gridCell {
	grid := make([]gridCell, width)
	for i := range grid {
		grid[i] = gridCell{r: ' ', style: ""}
	}

	cells := parseStyledLine(line)
	col := 0
	for _, cell := range cells {
		if col >= width {
			break
		}
		grid[col] = gridCell{r: cell.r, style: cell.style}
		for w := 1; w < cell.width; w++ {
			if col+w < width {
				grid[col+w] = gridCell{r: ' ', style: cell.style, isCont: true}
			}
		}
		col += cell.width
	}
	return grid
}

func gridToLine(grid []gridCell) string {
	var sb strings.Builder
	var lastStyle string
	for _, cell := range grid {
		if cell.isCont {
			continue
		}
		if cell.style != lastStyle {
			if cell.style == "" {
				sb.WriteString("\x1b[0m")
			} else {
				sb.WriteString(cell.style)
			}
			lastStyle = cell.style
		}
		sb.WriteRune(cell.r)
	}
	if lastStyle != "" {
		sb.WriteString("\x1b[0m")
	}
	return sb.String()
}

func overlayLines(bgLine, fgLine string, startX, width int) string {
	bgGrid := lineToGrid(bgLine, width)

	fgCells := parseStyledLine(fgLine)
	col := startX
	for _, cell := range fgCells {
		if col >= width {
			break
		}
		if col >= 0 {
			bgGrid[col] = gridCell{r: cell.r, style: cell.style}
			for w := 1; w < cell.width; w++ {
				if col+w < width && col+w >= 0 {
					bgGrid[col+w] = gridCell{r: ' ', style: cell.style, isCont: true}
				}
			}
		}
		col += cell.width
	}

	return gridToLine(bgGrid)
}

func overlay(background, foreground string, width, height, startX, startY int) string {
	bgLines := strings.Split(background, "\n")
	fgLines := strings.Split(foreground, "\n")

	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}

	for i, fgLine := range fgLines {
		y := startY + i
		if y >= 0 && y < height {
			bgLines[y] = overlayLines(bgLines[y], fgLine, startX, width)
		}
	}

	return strings.Join(bgLines, "\n")
}
