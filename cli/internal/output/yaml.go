package output

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// renderYAML writes the command result as YAML.
func renderYAML(w io.Writer, result *CommandResult, _ OutputOptions) error {
	var data interface{}
	if result.IsSingle {
		data = result.Single
	} else {
		data = result.List
	}

	if result.Schema != nil && result.Schema.Serialize != nil {
		data = result.Schema.Serialize(data)
	}

	b, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal YAML: %w", err)
	}

	if _, err := w.Write(b); err != nil {
		return err
	}
	return nil
}

// renderYAMLError writes a CommandError as YAML.
func renderYAMLError(w io.Writer, err *CommandError) {
	b, _ := yaml.Marshal(map[string]*CommandError{"error": err})
	_, _ = w.Write(b)
}
