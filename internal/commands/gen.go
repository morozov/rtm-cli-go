//go:generate go tool rtm-gen cli --spec=../../spec.json --out=. --client-module=github.com/morozov/rtm-cli-go/internal/rtm

// Package commands is the generated cobra command tree for the
// Remember The Milk API. Host programs mount it onto their own root
// command via Register. See gen.go for the regeneration directive;
// every other file in this package is generated output.
package commands
