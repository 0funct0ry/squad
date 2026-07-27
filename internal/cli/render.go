package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
)

// colorable modes get the item-4 color treatment (bold headers, dimmed NULL,
// alternating row shading); csv/json/list/tabs/ascii always render plain,
// since those are meant to be piped/parsed.
func colorEligible(mode OutputMode) bool {
	switch mode {
	case ModeColumn, ModeTable, ModeBox, ModeMarkdown:
		return true
	default:
		return false
	}
}

// Render writes cols/rows to s.Out using the current .mode, honoring
// .headers/.nullvalue, and applying the item-4 color treatment when stdout
// is a TTY and the mode is color-eligible.
func (s *State) Render(cols []string, rows [][]any) error {
	return s.renderTo(s.Out, cols, rows)
}

// renderTo is Render's implementation, parameterized on the destination
// writer so ".save" can render a one-shot query's output to a file without
// touching s.Out/.once state.
func (s *State) renderTo(w io.Writer, cols []string, rows [][]any) error {
	strRows := make([][]string, len(rows))
	for i, row := range rows {
		strRows[i] = make([]string, len(row))
		for j, v := range row {
			strRows[i][j] = s.formatValue(v)
		}
	}

	useColor := s.Colorized && colorEligible(s.Mode)

	switch s.Mode {
	case ModeCSV:
		return renderDelim(w, cols, strRows, s.Headers, ',', true)
	case ModeTabs:
		return renderDelim(w, cols, strRows, s.Headers, '\t', false)
	case ModeList:
		return renderDelim(w, cols, strRows, s.Headers, '|', false)
	case ModeAscii:
		return renderAscii(w, cols, strRows, s.Headers)
	case ModeJSON:
		return renderJSON(w, cols, strRows)
	case ModeMarkdown:
		return renderMarkdown(w, cols, strRows, useColor)
	case ModeColumn:
		return renderColumn(w, cols, strRows, s.Headers, useColor)
	case ModeTable:
		return renderBoxLike(w, cols, strRows, s.Headers, useColor, tableBorder)
	case ModeBox:
		return renderBoxLike(w, cols, strRows, s.Headers, useColor, boxBorder)
	default:
		return renderColumn(w, cols, strRows, s.Headers, useColor)
	}
}

func (s *State) formatValue(v any) string {
	if v == nil {
		return s.NullValue
	}
	return fmt.Sprintf("%v", v)
}

func renderDelim(w io.Writer, cols []string, rows [][]string, headers bool, sep byte, quote bool) error {
	sepStr := string(sep)
	writeLine := func(fields []string) {
		out := make([]string, len(fields))
		for i, f := range fields {
			if quote && (strings.ContainsAny(f, string(sep)+"\"\n")) {
				out[i] = `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
			} else {
				out[i] = f
			}
		}
		fmt.Fprintln(w, strings.Join(out, sepStr))
	}
	if headers {
		writeLine(cols)
	}
	for _, row := range rows {
		writeLine(row)
	}
	return nil
}

func renderAscii(w io.Writer, cols []string, rows [][]string, headers bool) error {
	const unitSep = "\x1f"
	const recordSep = "\x1e"
	if headers {
		fmt.Fprint(w, strings.Join(cols, unitSep)+recordSep)
	}
	for _, row := range rows {
		fmt.Fprint(w, strings.Join(row, unitSep)+recordSep)
	}
	fmt.Fprintln(w)
	return nil
}

func renderJSON(w io.Writer, cols []string, rows [][]string) error {
	out := make([]map[string]string, len(rows))
	for i, row := range rows {
		obj := make(map[string]string, len(cols))
		for j, c := range cols {
			if j < len(row) {
				obj[c] = row[j]
			}
		}
		out[i] = obj
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func colWidths(cols []string, rows [][]string) []int {
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = len([]rune(c))
	}
	for _, row := range rows {
		for i, v := range row {
			if i >= len(widths) {
				continue
			}
			if l := len([]rune(v)); l > widths[i] {
				widths[i] = l
			}
		}
	}
	return widths
}

func padRight(s string, width int) string {
	l := len([]rune(s))
	if l >= width {
		return s
	}
	return s + strings.Repeat(" ", width-l)
}

func renderColumn(w io.Writer, cols []string, rows [][]string, headers bool, useColor bool) error {
	widths := colWidths(cols, rows)
	if headers {
		parts := make([]string, len(cols))
		for i, c := range cols {
			cell := padRight(c, widths[i])
			if useColor {
				cell = color.New(color.Bold).Sprint(cell)
			}
			parts[i] = cell
		}
		fmt.Fprintln(w, strings.Join(parts, "  "))
		if headers {
			sep := make([]string, len(cols))
			for i := range cols {
				sep[i] = strings.Repeat("-", widths[i])
			}
			fmt.Fprintln(w, strings.Join(sep, "  "))
		}
	}
	for ri, row := range rows {
		parts := make([]string, len(cols))
		for i := range cols {
			var v string
			if i < len(row) {
				v = row[i]
			}
			cell := padRight(v, widths[i])
			cell = colorizeCell(cell, v, ri, useColor)
			parts[i] = cell
		}
		fmt.Fprintln(w, strings.Join(parts, "  "))
	}
	return nil
}

// colorizeCell dims NULL-rendered cells and applies subtle alternating-row
// shading on colorable modes; foreground/dim/bold only, no background color.
func colorizeCell(padded, raw string, rowIdx int, useColor bool) string {
	if !useColor {
		return padded
	}
	if strings.TrimRight(raw, " ") == "" {
		return color.New(color.Faint).Sprint(padded)
	}
	if rowIdx%2 == 1 {
		return color.New(color.FgHiWhite).Sprint(padded)
	}
	return padded
}

func renderMarkdown(w io.Writer, cols []string, rows [][]string, useColor bool) error {
	widths := colWidths(cols, rows)
	headerParts := make([]string, len(cols))
	for i, c := range cols {
		cell := padRight(c, widths[i])
		if useColor {
			cell = color.New(color.Bold).Sprint(cell)
		}
		headerParts[i] = cell
	}
	fmt.Fprintln(w, "| "+strings.Join(headerParts, " | ")+" |")
	sepParts := make([]string, len(cols))
	for i := range cols {
		sepParts[i] = strings.Repeat("-", widths[i])
	}
	fmt.Fprintln(w, "| "+strings.Join(sepParts, " | ")+" |")
	for ri, row := range rows {
		parts := make([]string, len(cols))
		for i := range cols {
			var v string
			if i < len(row) {
				v = row[i]
			}
			cell := padRight(v, widths[i])
			cell = colorizeCell(cell, v, ri, useColor)
			parts[i] = cell
		}
		fmt.Fprintln(w, "| "+strings.Join(parts, " | ")+" |")
	}
	return nil
}

type borderChars struct {
	horiz, vert               string
	topLeft, topMid, topRight string
	midLeft, midMid, midRight string
	botLeft, botMid, botRight string
}

var tableBorder = borderChars{
	horiz: "-", vert: "|",
	topLeft: "+", topMid: "+", topRight: "+",
	midLeft: "+", midMid: "+", midRight: "+",
	botLeft: "+", botMid: "+", botRight: "+",
}

var boxBorder = borderChars{
	horiz: "─", vert: "│",
	topLeft: "┌", topMid: "┬", topRight: "┐",
	midLeft: "├", midMid: "┼", midRight: "┤",
	botLeft: "└", botMid: "┴", botRight: "┘",
}

func renderBoxLike(w io.Writer, cols []string, rows [][]string, headers bool, useColor bool, b borderChars) error {
	widths := colWidths(cols, rows)

	hline := func(left, mid, right string) string {
		segs := make([]string, len(widths))
		for i, wd := range widths {
			segs[i] = strings.Repeat(b.horiz, wd+2)
		}
		return left + strings.Join(segs, mid) + right
	}

	fmt.Fprintln(w, hline(b.topLeft, b.topMid, b.topRight))
	if headers {
		parts := make([]string, len(cols))
		for i, c := range cols {
			cell := " " + padRight(c, widths[i]) + " "
			if useColor {
				cell = color.New(color.Bold).Sprint(cell)
			}
			parts[i] = cell
		}
		fmt.Fprintln(w, b.vert+strings.Join(parts, b.vert)+b.vert)
		fmt.Fprintln(w, hline(b.midLeft, b.midMid, b.midRight))
	}
	for ri, row := range rows {
		parts := make([]string, len(cols))
		for i := range cols {
			var v string
			if i < len(row) {
				v = row[i]
			}
			cell := " " + padRight(v, widths[i]) + " "
			cell = colorizeCell(cell, v, ri, useColor)
			parts[i] = cell
		}
		fmt.Fprintln(w, b.vert+strings.Join(parts, b.vert)+b.vert)
	}
	fmt.Fprintln(w, hline(b.botLeft, b.botMid, b.botRight))
	return nil
}
