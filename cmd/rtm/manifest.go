package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// manifestCommandName is the subcommand a consumer invokes to
// dump the CLI's command tree as JSON. Kept in a constant so
// the root's PersistentPreRunE can skip credential-loading for
// it.
const manifestCommandName = "manifest"

// manifestNode is the JSON shape `rtm manifest` emits per cobra
// command. The tree is rooted at `rtm` itself.
type manifestNode struct {
	Name            string              `json:"name"`
	Short           string              `json:"short,omitempty"`
	Long            string              `json:"long,omitempty"`
	Flags           []manifestFlag      `json:"flags,omitempty"`
	PersistentFlags []manifestFlag      `json:"persistent_flags,omitempty"`
	References      []manifestReference `json:"references,omitempty"`
	Commands        []manifestNode      `json:"commands,omitempty"`
}

// manifestReference is the JSON shape for a single footnote.
type manifestReference struct {
	Marker string `json:"marker"`
	URL    string `json:"url"`
}

// manifestFlag is the JSON shape for a single cobra flag.
type manifestFlag struct {
	Name        string   `json:"name"`
	Shorthand   string   `json:"shorthand,omitempty"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Default     string   `json:"default,omitempty"`
	Required    bool     `json:"required,omitempty"`
	EnumValues  []string `json:"enum_values,omitempty"`
}

func newManifestCommand() *cobra.Command {
	return &cobra.Command{
		Use:   manifestCommandName,
		Short: "Dump the full CLI command tree as JSON for programmatic discovery",
		Long: "Dump the full CLI command tree as JSON for programmatic discovery.\n\n" +
			"Intended for tooling and AI agents that need to enumerate every\n" +
			"available command and flag without recursively crawling `--help`\n" +
			"output. Credentials are not required.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tree := collectManifestNode(cmd.Root())
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(tree); err != nil {
				return fmt.Errorf("encode manifest: %w", err)
			}
			return nil
		},
	}
}

func collectManifestNode(c *cobra.Command) manifestNode {
	node := manifestNode{
		Name:  c.Name(),
		Short: c.Short,
		Long:  c.Long,
	}
	for _, r := range collectReferences(c) {
		node.References = append(node.References, manifestReference{
			Marker: fmt.Sprintf("[^%d]", r.N),
			URL:    r.URL,
		})
	}
	// Cobra considers locally-defined persistent flags to also be
	// "local". De-dupe so each flag appears in exactly one slot.
	persistentNames := map[string]struct{}{}
	c.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		persistentNames[f.Name] = struct{}{}
		node.PersistentFlags = append(node.PersistentFlags, manifestFlagFrom(f))
	})
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		if _, persistent := persistentNames[f.Name]; persistent {
			return
		}
		node.Flags = append(node.Flags, manifestFlagFrom(f))
	})
	for _, sub := range c.Commands() {
		if sub.Hidden {
			continue
		}
		switch sub.Name() {
		case "help", "completion", manifestCommandName:
			continue
		}
		node.Commands = append(node.Commands, collectManifestNode(sub))
	}
	return node
}

func manifestFlagFrom(f *pflag.Flag) manifestFlag {
	required := false
	if v, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok && len(v) > 0 && v[0] == "true" {
		required = true
	}
	return manifestFlag{
		Name:        f.Name,
		Shorthand:   f.Shorthand,
		Type:        f.Value.Type(),
		Description: f.Usage,
		Default:     f.DefValue,
		Required:    required,
		EnumValues:  f.Annotations["rtm-gen.enum"],
	}
}
