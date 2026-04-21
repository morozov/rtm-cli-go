// Command rtm is the Remember The Milk command-line client.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/morozov/rtm-cli-go/internal/commands"
	"github.com/morozov/rtm-cli-go/internal/config"
	"github.com/morozov/rtm-cli-go/internal/output"
	"github.com/morozov/rtm-cli-go/internal/rtm"
)

// ErrMissingCredentials is returned when the resolved
// configuration does not carry both --key and --secret (via flag,
// env, or config file).
var ErrMissingCredentials = errors.New("missing RTM credentials")

// ErrUnknownOutput is returned when --output is set to a value
// other than the supported formatters.
var ErrUnknownOutput = errors.New("unknown output format")

func main() {
	err := newRootCommand().Execute()
	switch {
	case err == nil:
		return
	case errors.Is(err, rtm.ErrRTMAPI):
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(2)
	default:
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var (
		client       *rtm.Client
		formatter    commands.Formatter
		outputFormat string
	)

	root := &cobra.Command{
		Use:           "rtm",
		Short:         "Remember The Milk CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String("key", "", "RTM API key (or $RTM_API_KEY, or api_key in config)")
	root.PersistentFlags().String("secret", "", "RTM API secret (or $RTM_API_SECRET, or api_secret in config)")
	root.PersistentFlags().String("token", "", "RTM auth token (or $RTM_AUTH_TOKEN, or auth_token in config)")
	root.PersistentFlags().StringVarP(&outputFormat, "output", "o", "json", "output format: json or yaml")

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if cmd.Name() == manifestCommandName {
			return nil
		}
		cfg, err := config.Load(cmd.Root().PersistentFlags())
		if err != nil {
			return err
		}
		if cfg.APIKey == "" || cfg.APISecret == "" {
			return fmt.Errorf("--key and --secret must be set via flag, environment, or config file: %w", ErrMissingCredentials)
		}
		client = rtm.NewClient(cfg.APIKey, cfg.APISecret, cfg.AuthToken)

		switch outputFormat {
		case "json":
			formatter = output.JSON
		case "yaml":
			formatter = output.YAML
		default:
			return fmt.Errorf("%w %q; valid values: json, yaml", ErrUnknownOutput, outputFormat)
		}
		return nil
	}

	commands.Register(
		root,
		func() *rtm.Client { return client },
		func(w io.Writer, body any) error { return formatter(w, body) },
	)
	root.AddCommand(newManifestCommand())

	// Wrap the help function on root; cobra inherits it down the
	// tree. The wrapper appends a References section whenever the
	// command carries `ref.N` annotations (emitted by the
	// generator for commands whose descriptions had anchors).
	root.SetHelpFunc(withReferencesFooter())

	return root
}

// referencePrefix keys the per-command footnote annotations the
// generator emits into each cobra.Command (ref.1, ref.2, …).
const referencePrefix = "ref."

// withReferencesFooter returns a cobra help function that runs
// cobra's default help output and then appends a "References:"
// footer when the command has any `ref.N` annotations.
func withReferencesFooter() func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, _ []string) {
		out := cmd.OutOrStderr()
		// Replicate cobra's default template: long-or-short
		// description, blank line, usage.
		if desc := strings.TrimRight(cmd.Long, " \t\n"); desc != "" {
			fmt.Fprintln(out, desc)
			fmt.Fprintln(out)
		} else if desc := strings.TrimRight(cmd.Short, " \t\n"); desc != "" {
			fmt.Fprintln(out, desc)
			fmt.Fprintln(out)
		}
		if cmd.Runnable() || cmd.HasSubCommands() {
			fmt.Fprint(out, cmd.UsageString())
		}
		if refs := collectReferences(cmd); len(refs) > 0 {
			fmt.Fprintln(out)
			fmt.Fprintln(out, "References:")
			for _, r := range refs {
				fmt.Fprintf(out, "  [^%d] %s\n", r.N, r.URL)
			}
		}
	}
}

// commandReference is the in-host view of a single footnote entry.
type commandReference struct {
	N   int
	URL string
}

// collectReferences pulls `ref.1`, `ref.2`, … from cmd.Annotations
// and returns them in numeric order. Missing numbers stop the
// walk.
func collectReferences(cmd *cobra.Command) []commandReference {
	var refs []commandReference
	for n := 1; ; n++ {
		url, ok := cmd.Annotations[fmt.Sprintf("%s%d", referencePrefix, n)]
		if !ok {
			break
		}
		refs = append(refs, commandReference{N: n, URL: url})
	}
	return refs
}
