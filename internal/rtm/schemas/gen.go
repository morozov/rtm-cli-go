//go:generate go tool rtm-gen schemas --spec=../../../spec.json --out=.

// Package schemas ships one JSON Schema (draft 2020-12) document
// per RTM method's response, embedded into the CLI binary. The
// *.json files in this directory are generated output (see
// gen.go's directive); schemas.go is the handwritten accessor.
package schemas
