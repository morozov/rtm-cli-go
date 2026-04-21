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
	"strings"

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
// path. The JSON is then re-decoded with json.Decoder.UseNumber,
// and numeric-shaped json.Number values are narrowed into int64
// (when there's no decimal or exponent) or float64 — otherwise
// yaml.v3 would render every integer as a float in scientific
// notation (`1.674089e+06` for a 7-digit RTM id).
func YAML(w io.Writer, body any) error {
	if isEmpty(body) {
		return nil
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode intermediate json: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("decode intermediate json: %w", err)
	}
	v = narrowNumbers(v)
	out, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode yaml: %w", err)
	}
	if _, err := w.Write(out); err != nil {
		return fmt.Errorf("write yaml: %w", err)
	}
	return nil
}

// narrowNumbers walks v and replaces every json.Number with
// either int64 (when the source had no decimal point or
// exponent) or float64. Leaves other types untouched.
func narrowNumbers(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			x[k] = narrowNumbers(val)
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = narrowNumbers(val)
		}
		return x
	case json.Number:
		s := string(x)
		if !strings.ContainsAny(s, ".eE") {
			if n, err := x.Int64(); err == nil {
				return n
			}
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return s
	}
	return v
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
