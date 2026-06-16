// Package output renders CLI command results to JSON, YAML, tables, and quiet ID lists.
package output

import (
	"github.com/fatih/color"
)

var (
	// HeaderColor for table headers.
	HeaderColor = color.New(color.Bold)
	// SuccessColor for positive statuses.
	SuccessColor = color.New(color.FgGreen)
	// ErrorColor for errors.
	ErrorColor = color.New(color.FgRed, color.Bold)
	// WarnColor for warnings.
	WarnColor = color.New(color.FgYellow)
	// DimColor for secondary info.
	DimColor = color.New(color.FgHiBlack)
	// IDColor for agent/item IDs.
	IDColor = color.New(color.FgCyan)
)

// Colorize applies a named color to a string.
func Colorize(colorName, text string) string {
	switch colorName {
	case "red":
		return color.RedString(text)
	case "green":
		return color.GreenString(text)
	case "yellow":
		return color.YellowString(text)
	case "cyan":
		return color.CyanString(text)
	case "blue":
		return color.BlueString(text)
	case "magenta":
		return color.MagentaString(text)
	case "dim":
		return DimColor.Sprint(text)
	case "bold":
		return color.New(color.Bold).Sprint(text)
	default:
		return text
	}
}

// Bold formats text in bold.
func Bold(text string) string {
	return color.New(color.Bold).Sprint(text)
}

// Red formats text in red.
func Red(text string) string {
	return color.RedString(text)
}

// Green formats text in green.
func Green(text string) string {
	return color.GreenString(text)
}

// Yellow formats text in yellow.
func Yellow(text string) string {
	return color.YellowString(text)
}

// Cyan formats text in cyan.
func Cyan(text string) string {
	return color.CyanString(text)
}

// Dim formats text in dim color.
func Dim(text string) string {
	return DimColor.Sprint(text)
}

// ErrorText formats an error message.
func ErrorText(msg string) string {
	return Red("Error: " + msg)
}

// DisableColor disables all color output (e.g., when piped).
func DisableColor() {
	color.NoColor = true
}
