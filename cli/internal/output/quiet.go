package output

import (
	"fmt"
	"io"
)

// renderQuiet writes only the ID field of each item, one per line.
func renderQuiet(w io.Writer, result *CommandResult, _ OutputOptions) error {
	if result.Schema == nil || result.Schema.IDField == nil {
		return nil
	}

	if result.IsSingle {
		id := result.Schema.IDField(result.Single)
		_, err := fmt.Fprintln(w, id)
		return err
	}

	for _, item := range result.List {
		id := result.Schema.IDField(item)
		if _, err := fmt.Fprintln(w, id); err != nil {
			return err
		}
	}
	return nil
}
