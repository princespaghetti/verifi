package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Color codes using ANSI escape sequences
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// colorsEnabled determines if color output is enabled
var colorsEnabled = true

func init() {
	// Disable colors if NO_COLOR environment variable is set
	// or if stdout is not a terminal
	if os.Getenv("NO_COLOR") != "" {
		colorsEnabled = false
	}
}

// Color wraps text with ANSI color codes if colors are enabled
func Color(text, color string) string {
	if !colorsEnabled {
		return text
	}
	return color + text + colorReset
}

// colorize applies color to text, with a fallback if colors are disabled
func colorize(text, color string) string {
	return Color(text, color)
}

// Success prints a success message with a green checkmark
func Success(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	icon := colorize("✓", colorGreen)
	fmt.Printf("%s %s\n", icon, msg)
}

// Error prints an error message with a red X to stderr
func Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	icon := colorize("✗", colorRed)
	fmt.Fprintf(os.Stderr, "%s Error: %s\n", icon, msg)
}

// Warning prints a warning message with a yellow warning sign
func Warning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	icon := colorize("⚠", colorYellow)
	fmt.Printf("%s Warning: %s\n", icon, msg)
}

// Info prints an informational message
func Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(msg)
}

// Header prints a section header with optional underline
func Header(text string) {
	fmt.Println(colorize(text, colorBold))
	fmt.Println(strings.Repeat("=", len(text)))
	fmt.Println()
}

// Subheader prints a subsection header
func Subheader(text string) {
	fmt.Println(colorize(text, colorBold))
	fmt.Println(strings.Repeat("-", len(text)))
}

// Section prints a simple section divider
func Section(text string) {
	fmt.Println()
	fmt.Println(colorize(text, colorBold))
	fmt.Println(strings.Repeat("-", len(text)))
}

// Field prints a labeled field (key-value pair)
func Field(label, value string) {
	labelFormatted := fmt.Sprintf("%-16s", label+":")
	fmt.Printf("%s %s\n", colorize(labelFormatted, colorGray), value)
}

// FieldIndented prints an indented labeled field
func FieldIndented(label, value string, indent int) {
	indentStr := strings.Repeat(" ", indent)
	labelFormatted := fmt.Sprintf("%-16s", label+":")
	fmt.Printf("%s%s %s\n", indentStr, labelFormatted, value)
}

// Table represents a simple text table
type Table struct {
	Headers []string
	Rows    [][]string
	writer  io.Writer
}

// NewTable creates a new table with the given headers
func NewTable(headers ...string) *Table {
	return &Table{
		Headers: headers,
		Rows:    [][]string{},
		writer:  os.Stdout,
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(values ...string) {
	t.Rows = append(t.Rows, values)
}

// Print renders the table
func (t *Table) Print() {
	if len(t.Headers) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(t.Headers))
	for i, header := range t.Headers {
		widths[i] = len(header)
	}
	for _, row := range t.Rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Build format string
	formats := make([]string, len(t.Headers))
	for i := range widths {
		formats[i] = fmt.Sprintf("%%-%ds", widths[i])
	}
	formatStr := strings.Join(formats, "  ")

	// Print headers
	headerVals := make([]interface{}, len(t.Headers))
	for i, h := range t.Headers {
		headerVals[i] = colorize(h, colorBold)
	}
	_, _ = fmt.Fprintf(t.writer, formatStr+"\n", headerVals...) // Ignore write errors - main operation succeeded

	// Print separator
	totalWidth := 0
	for _, w := range widths {
		totalWidth += w
	}
	totalWidth += 2 * (len(widths) - 1)                            // Add space for separators
	_, _ = fmt.Fprintln(t.writer, strings.Repeat("-", totalWidth)) // Ignore write errors - main operation succeeded

	// Print rows
	for _, row := range t.Rows {
		rowVals := make([]interface{}, len(row))
		for i, cell := range row {
			rowVals[i] = cell
		}
		_, _ = fmt.Fprintf(t.writer, formatStr+"\n", rowVals...) // Ignore write errors - main operation succeeded
	}
}

// PrintCompact renders the table in a more compact format
func (t *Table) PrintCompact() {
	if len(t.Rows) == 0 {
		return
	}

	for _, row := range t.Rows {
		fmt.Println(strings.Join(row, " "))
	}
}

// JSON marshals and prints data as indented JSON
func JSON(v interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// FormatBytes formats byte sizes in human-readable format (B, KB, MB, etc.)
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// RepeatString repeats a string n times
func RepeatString(s string, n int) string {
	return strings.Repeat(s, n)
}

// TruncateString truncates a string to maxLen with ellipsis
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// StatusIcon returns a colored status icon based on status string
func StatusIcon(status string) string {
	switch strings.ToLower(status) {
	case "pass", "ok", "valid", "success":
		return colorize("✓", colorGreen)
	case "warn", "warning":
		return colorize("⚠", colorYellow)
	case "fail", "error", "expired", "invalid":
		return colorize("✗", colorRed)
	default:
		return "•"
	}
}

// PrintList prints a bulleted list
func PrintList(items []string) {
	for _, item := range items {
		fmt.Printf("  • %s\n", item)
	}
}

// PrintNumberedList prints a numbered list
func PrintNumberedList(items []string) {
	for i, item := range items {
		fmt.Printf("  %d. %s\n", i+1, item)
	}
}

// EmptyLine prints an empty line
func EmptyLine() {
	fmt.Println()
}

// Separator prints a horizontal line separator
func Separator(char string, length int) {
	fmt.Println(strings.Repeat(char, length))
}

// ConfirmPrompt asks the user for confirmation (y/n)
// Returns true if user confirms, false otherwise
func ConfirmPrompt(message string) bool {
	fmt.Printf("%s [y/N]: ", message)
	var response string
	_, _ = fmt.Scanln(&response) // Ignore error, treat as no confirmation if failed
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// EnableColors enables color output
func EnableColors() {
	colorsEnabled = true
}

// DisableColors disables color output
func DisableColors() {
	colorsEnabled = false
}
