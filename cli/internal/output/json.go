package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// renderJSON writes the command result as formatted JSON.
func renderJSON(w io.Writer, result *CommandResult, _ OutputOptions) error {
	var data interface{}
	if result.IsSingle {
		data = result.Single
	} else {
		data = result.List
	}

	if result.Schema != nil && result.Schema.Serialize != nil {
		data = result.Schema.Serialize(data)
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	_, err = fmt.Fprintln(w, string(b))
	return err
}

// renderJSONError writes a CommandError as JSON.
func renderJSONError(w io.Writer, err *CommandError) {
	b, _ := json.MarshalIndent(map[string]*CommandError{"error": err}, "", "  ")
	_, _ = fmt.Fprintln(w, string(b))
}
