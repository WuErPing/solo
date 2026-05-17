package output

import (
	"fmt"
)

// Render renders a CommandResult to stdout based on OutputOptions.
func Render(result *CommandResult, opts OutputOptions) error {
	if result == nil {
		return nil
	}

	var err error
	switch opts.Format {
	case FormatJSON:
		err = renderJSON(Stdout, result, opts)
	case FormatYAML:
		err = renderYAML(Stdout, result, opts)
	case FormatQuiet:
		err = renderQuiet(Stdout, result, opts)
	default: // table
		err = renderTable(Stdout, result, opts)
	}
	return err
}

// RenderError renders a CommandError to stderr based on OutputOptions.
func RenderError(ce *CommandError, opts OutputOptions) {
	switch opts.Format {
	case FormatJSON:
		renderJSONError(Stderr, ce)
	case FormatYAML:
		renderYAMLError(Stderr, ce)
	default:
		msg := ErrorText(ce.Message)
		if ce.Details != "" {
			msg += "\n" + ce.Details
		}
		fmt.Fprintln(Stderr, msg)
	}
}

// PrintResult is a convenience function that renders a result and handles errors.
// It returns the appropriate exit code: 0 for success, 1 for error.
func PrintResult(result *CommandResult, opts OutputOptions) int {
	if result == nil {
		return 0
	}
	if err := Render(result, opts); err != nil {
		RenderError(&CommandError{
			Code:    "RENDER_FAILED",
			Message: err.Error(),
		}, opts)
		return 1
	}
	return 0
}
