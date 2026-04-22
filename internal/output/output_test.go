package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sampleBody struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
	Flag  bool   `json:"flag"`
}

func TestJSONEmptyBodyIsSilent(t *testing.T) {
	for _, body := range []any{nil, (*sampleBody)(nil), struct{}{}, &struct{}{}} {
		var buf bytes.Buffer
		require.NoError(t, JSON(&buf, body))
		assert.Empty(t, buf.String(), "empty body %#v should produce no output", body)
	}
}

func TestJSONPrettyPrints(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, JSON(&buf, sampleBody{Name: "foo", Count: 42, Flag: true}))
	assert.Equal(t, "{\n  \"name\": \"foo\",\n  \"count\": 42,\n  \"flag\": true\n}\n", buf.String())
}

func TestYAMLEmitsProperTypes(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, YAML(&buf, sampleBody{Name: "foo", Count: 1674089, Flag: true}))
	out := buf.String()
	assert.Contains(t, out, "name: foo")
	assert.Contains(t, out, "count: 1674089")
	assert.Contains(t, out, "flag: true")
	// Should NOT render count in scientific notation.
	assert.NotContains(t, out, "1.674089e+06")
}

func TestYAMLEmptyBodyIsSilent(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, YAML(&buf, struct{}{}))
	assert.Empty(t, buf.String())
}
