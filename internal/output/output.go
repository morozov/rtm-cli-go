// Package output formats the inner body of a successful RTM API
// response for stdout. Each exported function satisfies
// commands.Formatter.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// JSON writes body as pretty-printed JSON with two-space
// indentation and a trailing newline. An empty body (`{}`) is
// silent on success.
func JSON(w io.Writer, body json.RawMessage) error {
	if isEmpty(body) {
		return nil
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

// YAML writes body as YAML via gopkg.in/yaml.v3. An empty body
// (`{}`) is silent on success.
func YAML(w io.Writer, body json.RawMessage) error {
	if isEmpty(body) {
		return nil
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	out, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode yaml: %w", err)
	}
	if _, err := w.Write(out); err != nil {
		return fmt.Errorf("write yaml: %w", err)
	}
	return nil
}

func isEmpty(body json.RawMessage) bool {
	trimmed := bytes.TrimSpace(body)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("{}"))
}
