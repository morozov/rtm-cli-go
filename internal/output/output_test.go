package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONEmptyBodyIsSilent(t *testing.T) {
	for _, body := range []string{"", "{}", "  {}  "} {
		var buf bytes.Buffer
		require.NoError(t, JSON(&buf, json.RawMessage(body)))
		assert.Empty(t, buf.String(), "empty body %q should produce no output", body)
	}
}

func TestJSONPrettyPrints(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, JSON(&buf, json.RawMessage(`{"frob":"abc"}`)))
	assert.Equal(t, "{\n  \"frob\": \"abc\"\n}\n", buf.String())
}

func TestYAMLEmptyBodyIsSilent(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, YAML(&buf, json.RawMessage(`{}`)))
	assert.Empty(t, buf.String())
}

func TestYAMLEmitsYAML(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, YAML(&buf, json.RawMessage(`{"frob":"abc","nested":{"k":1}}`)))
	out := buf.String()
	assert.Contains(t, out, "frob: abc")
	assert.Contains(t, out, "nested:")
	assert.Contains(t, out, "k: 1")
}

func TestJSONRejectsInvalidBody(t *testing.T) {
	err := JSON(&bytes.Buffer{}, json.RawMessage(`not json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response body")
}
