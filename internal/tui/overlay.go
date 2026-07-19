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
			style := cell.style
			if !styleHasBackground(style) {
				style = style + colorBgPanelANSI
			}
			bgGrid[col] = gridCell{r: cell.r, style: style}
			for w := 1; w < cell.width; w++ {
				if col+w < width && col+w >= 0 {
					bgGrid[col+w] = gridCell{r: ' ', style: style, isCont: true}
				}
			}
		}
		col += cell.width
	}

	return gridToLine(bgGrid)
}

func styleHasBackground(style string) bool {
	hasBg := false
	i := 0
	n := len(style)
	for i < n {
		if i+1 < n && style[i] == '\x1b' && style[i+1] == '[' {
			i += 2
			start := i
			for i < n {
				c := style[i]
				if c == 'm' {
					break
				}
				if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
					break
				}
				i++
			}
			seqContent := style[start:i]
			if i < n && style[i] == 'm' {
				i++
			}
			
			if seqContent == "" || seqContent == "0" {
				hasBg = false
				continue
			}
			
			params := strings.Split(seqContent, ";")
			pIdx := 0
			for pIdx < len(params) {
				param := params[pIdx]
				if param == "0" {
					hasBg = false
					pIdx++
				} else if param == "38" {
					if pIdx+1 < len(params) {
						subType := params[pIdx+1]
						if subType == "5" {
							pIdx += 3
						} else if subType == "2" {
							pIdx += 5
						} else {
							pIdx++
						}
					} else {
						pIdx++
					}
				} else if param == "48" {
					hasBg = true
					if pIdx+1 < len(params) {
						subType := params[pIdx+1]
						if subType == "5" {
							pIdx += 3
						} else if subType == "2" {
							pIdx += 5
						} else {
							pIdx++
						}
					} else {
						pIdx++
					}
				} else {
					val := 0
					for k := 0; k < len(param); k++ {
						if param[k] >= '0' && param[k] <= '9' {
							val = val*10 + int(param[k]-'0')
						} else {
							break
						}
					}
					if (val >= 40 && val <= 47) || (val >= 100 && val <= 107) {
						hasBg = true
					} else if val == 49 {
						hasBg = false
					}
					pIdx++
				}
			}
		} else {
			i++
		}
	}
	return hasBg
}

func overlay(background, foreground string, width, height, startX, startY int) string {
	if height <= 0 || width <= 0 {
		return background
	}
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

	if len(bgLines) > height {
		bgLines = bgLines[:height]
	}

	return strings.Join(bgLines, "\n")
}
