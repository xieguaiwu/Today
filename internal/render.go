package internal

import (
	"fmt"
	"os"
	"strings"
)

// Rendering primitives for the streak-style visualisations.
//
// Colour is applied as 24-bit ANSI only when the environment asks for it; on a
// dumb terminal or with NO_COLOR the same information is still conveyed through
// the glyphs, so nothing is colour-only.

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	switch os.Getenv("TERM") {
	case "", "dumb", "linux":
		return false
	}
	return true
}

// rgb parses #RGB / #RRGGBB into components.
func rgb(hex string) (r, g, b int, ok bool) {
	s := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if s == "" {
		return 0, 0, 0, false
	}
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	conv := func(pair string) (int, bool) {
		n := 0
		for _, c := range pair {
			v := 0
			switch {
			case c >= '0' && c <= '9':
				v = int(c - '0')
			case c >= 'a' && c <= 'f':
				v = int(c-'a') + 10
			case c >= 'A' && c <= 'F':
				v = int(c-'A') + 10
			default:
				return 0, false
			}
			n = n*16 + v
		}
		return n, true
	}
	if r, ok1 := conv(s[0:2]); ok1 {
		if g, ok2 := conv(s[2:4]); ok2 {
			if b, ok3 := conv(s[4:6]); ok3 {
				return r, g, b, true
			}
		}
	}
	return 0, 0, 0, false
}

// fg paints text in a truecolour foreground when possible.
func fg(hex, s string) string {
	if !colorEnabled() {
		return s
	}
	r, g, b, ok := rgb(hex)
	if !ok {
		return s
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", r, g, b, s)
}

const (
	cellFull   = "■" // all steps done
	cellHigh   = "▓" // ~2/3..3/4 done
	cellMid    = "▒" // ~1/3..2/3 done
	cellLow    = "░" // some steps, below 1/3
	cellOff    = "□" // nothing done, past or present
	cellDim    = "·" // future day
	cellToday  = "◉" // fully done today
	cellTodayO = "○" // today, not fully done yet
)

// levelGlyph maps a 0..1 completion ratio onto a glyph. The steps are visible
// without colour too, which matters because the source app's shading is the
// only place its step counts are shown.
func levelGlyph(level float64, isToday bool) string {
	switch {
	case level >= 1.0 && isToday:
		return cellToday
	case level >= 1.0:
		return cellFull
	case isToday:
		return cellTodayO
	case level >= 0.75:
		return cellHigh
	case level >= 0.5:
		return cellMid
	case level > 0:
		return cellLow
	default:
		return cellOff
	}
}

// shadeFor dims partial fills so a 1/4 day reads lighter than a 4/4 one.
func shadeFor(hex string, level float64, s string) string {
	if level >= 1.0 || level <= 0 {
		return fg(hex, s)
	}
	r, g, b, ok := rgb(hex)
	if !ok || !colorEnabled() {
		return s
	}
	// Blend toward the background proportionally to how little was done.
	mix := 0.35 + 0.65*level
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m",
		int(float64(r)*mix), int(float64(g)*mix), int(float64(b)*mix), s)
}

// weekCells renders Monday..Sunday for one item.
func weekCells(cells []WeekCell, color string) string {
	var b strings.Builder
	for _, c := range cells {
		if c.Future {
			b.WriteString(dim(cellDim))
			continue
		}
		g := levelGlyph(c.Level, c.IsToday)
		if c.Level <= 0 {
			b.WriteString(dim(g))
			continue
		}
		b.WriteString(shadeFor(color, c.Level, g))
	}
	return b.String()
}

// statCard renders one of the four metric boxes.
func statCard(label, value, sub string, width int) string {
	pad := func(s string, n int) string {
		if displayWidth(s) >= n {
			return s
		}
		return s + strings.Repeat(" ", n-displayWidth(s))
	}
	inner := width - 2
	return fmt.Sprintf("┌%s┐\n│%s│\n│%s│\n│%s│\n└%s┘",
		strings.Repeat("─", inner),
		pad(" "+dim(label), inner),
		pad(" "+bold(value), inner),
		pad(" "+dim(sub), inner),
		strings.Repeat("─", inner))
}

func bold(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\x1b[1m" + s + "\x1b[0m"
}

// dim wraps text in the ANSI "faint" sequence unless colour is disabled.
func dim(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\x1b[2m" + s + "\x1b[0m"
}

// statRow lays four cards out side by side.
func statRow(cards []string) string {
	split := make([][]string, len(cards))
	lines := 0
	for i, c := range cards {
		split[i] = strings.Split(c, "\n")
		if len(split[i]) > lines {
			lines = len(split[i])
		}
	}
	var b strings.Builder
	for l := 0; l < lines; l++ {
		for i := range split {
			if l < len(split[i]) {
				b.WriteString(split[i][l])
			} else {
				b.WriteString(strings.Repeat(" ", displayWidth(split[i][0])))
			}
			b.WriteString(" ")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// yearHeatmap renders a Monday-first GitHub-style grid of completion levels,
// clipped to the columns that actually carry data so a young habit does not
// waste the screen.
func yearHeatmap(grid [][]float64, marks []MonthMark, color string, maxCols int) string {
	if len(grid) == 0 || len(grid[0]) == 0 {
		return dim("(无记录)") + "\n"
	}
	cols := len(grid[0])

	last := 0
	for c := 0; c < cols; c++ {
		for r := 0; r < 7; r++ {
			if grid[r][c] > 0 {
				last = c
			}
		}
	}
	start := 0
	if maxCols > 0 && cols > maxCols {
		start = last - maxCols + 1
		if start < 0 {
			start = 0
		}
		if start+maxCols < last+1 {
			start = last - maxCols + 1
		}
	}
	end := cols
	if maxCols > 0 && end > start+maxCols {
		end = start + maxCols
	}

	monthAt := map[int]string{}
	for _, m := range marks {
		if m.Col >= start && m.Col < end {
			if _, taken := monthAt[m.Col]; !taken {
				monthAt[m.Col] = m.Label
			}
		}
	}

	var b strings.Builder
	// month ruler
	labelRow := make([]string, end-start)
	for c := start; c < end; c++ {
		if l, ok := monthAt[c]; ok {
			labelRow[c-start] = l
		}
	}
	b.WriteString(dim(strings.Join(padColumns(labelRow, 1), "")) + "\n")

	names := []string{"一", "二", "三", "四", "五", "六", "日"}
	for r := 0; r < 7; r++ {
		b.WriteString(dim(names[r]) + " ")
		for c := start; c < end; c++ {
			lv := grid[r][c]
			if lv <= 0 {
				b.WriteString(dim(cellDim))
				continue
			}
			b.WriteString(shadeFor(color, lv, levelGlyph(lv, false)))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// padColumns gives each label its own column run, blank where unlabelled.
func padColumns(labels []string, minW int) []string {
	out := make([]string, len(labels))
	for i, l := range labels {
		if l == "" {
			out[i] = strings.Repeat(" ", minW)
			continue
		}
		out[i] = l
	}
	return out
}

// weekdayChart renders the "你在什么时候最能持之以恒" bars, Monday-first.
func weekdayChart(hist [7]int, color string) string {
	peak := 1
	for _, n := range hist {
		if n > peak {
			peak = n
		}
	}
	names := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
	var b strings.Builder
	for i, n := range hist {
		bar := ""
		if n > 0 {
			bar = strings.Repeat("█", max(1, n*24/peak))
		}
		b.WriteString(fmt.Sprintf("%s %s %s\n", dim(names[i]), fg(color, bar), dim(fmt.Sprintf("%d", n))))
	}
	return b.String()
}

// displayWidth counts CJK runes as two columns, which is what the terminal does.
func displayWidth(s string) int {
	w := 0
	for _, r := range stripANSI(s) {
		if r >= 0x1100 && (r <= 0x115F ||
			(r >= 0x2E80 && r <= 0xA4CF) ||
			(r >= 0xAC00 && r <= 0xD7A3) ||
			(r >= 0xF900 && r <= 0xFAFF) ||
			(r >= 0xFE30 && r <= 0xFE6F) ||
			(r >= 0xFF00 && r <= 0xFF60) ||
			(r >= 0xFFE0 && r <= 0xFFE6)) {
			w += 2
			continue
		}
		w++
	}
	return w
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEsc = true
			continue
		case inEsc:
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
