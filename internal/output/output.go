// Package output formats the typed response body of a
// successful RTM API call for stdout. Each exported function
// satisfies commands.Formatter.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"gopkg.in/yaml.v3"
)

// JSON writes body as pretty-printed JSON with two-space
// indentation and a trailing newline. A nil or empty body is
// silent on success.
func JSON(w io.Writer, body any) error {
	if isEmpty(body) {
		return nil
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(body); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

// YAML writes body as YAML via gopkg.in/yaml.v3. A nil or empty
// body is silent on success.
//
// The implementation round-trips through json.Marshal so the
// generator's wrapper types (rtmBool, rtmInt, rtmTime) emit via
// their MarshalJSON methods rather than yaml.v3's reflection
// path. The intermediate JSON is parsed back into a yaml.Node
// — yaml.v3 reads JSON natively, and Node preserves the source
// document's key order, so YAML output keeps the same field
// order as the JSON output instead of sorting alphabetically.
func YAML(w io.Writer, body any) error {
	if isEmpty(body) {
		return nil
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode intermediate json: %w", err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return fmt.Errorf("decode intermediate json: %w", err)
	}
	clearStyle(&node)
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(&node); err != nil {
		return fmt.Errorf("encode yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("close yaml encoder: %w", err)
	}
	return nil
}

// clearStyle resets the source-style flag on every node so that
// JSON parsed via yaml.Unmarshal — which preserves the source's
// flow form and double-quoted scalars — re-emits as conventional
// block YAML with plain scalars (yaml.v3 still adds quotes when
// the content would otherwise be ambiguous).
func clearStyle(n *yaml.Node) {
	n.Style = 0
	for _, c := range n.Content {
		clearStyle(c)
	}
}

// isEmpty returns true when body carries no information worth
// rendering: a nil interface, a nil pointer, a zero-sized
// struct, or a struct whose JSON serialisation is `{}`. The last
// check catches response types generated for RTM methods that
// only confirm success.
func isEmpty(body any) bool {
	if body == nil {
		return true
	}
	v := reflect.ValueOf(body)
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return true
		}
		return isEmpty(v.Elem().Interface())
	case reflect.Struct:
		if v.NumField() == 0 {
			return true
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return false
		}
		return bytes.Equal(bytes.TrimSpace(raw), []byte("{}"))
	}
	return false
}
