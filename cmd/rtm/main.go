// Command rtm is the Remember The Milk command-line client.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

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
		func(w io.Writer, body json.RawMessage) error { return formatter(w, body) },
	)
	root.AddCommand(newManifestCommand())

	return root
}
