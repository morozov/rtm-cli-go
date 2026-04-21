// Command rtm is the Remember The Milk command-line client.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/morozov/rtm-cli-go/internal/commands"
	"github.com/morozov/rtm-cli-go/internal/config"
	"github.com/morozov/rtm-cli-go/internal/rtm"
)

// ErrMissingCredentials is returned when the resolved
// configuration does not carry both --key and --secret (via flag,
// env, or config file).
var ErrMissingCredentials = errors.New("missing RTM credentials")

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var client *rtm.Client

	root := &cobra.Command{
		Use:          "rtm",
		Short:        "Remember The Milk CLI",
		SilenceUsage: true,
	}
	root.PersistentFlags().String("key", "", "RTM API key (or $RTM_API_KEY, or api_key in config)")
	root.PersistentFlags().String("secret", "", "RTM API secret (or $RTM_API_SECRET, or api_secret in config)")
	root.PersistentFlags().String("token", "", "RTM auth token (or $RTM_AUTH_TOKEN, or auth_token in config)")

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(cmd.Root().PersistentFlags())
		if err != nil {
			return err
		}
		if cfg.APIKey == "" || cfg.APISecret == "" {
			return fmt.Errorf("--key and --secret must be set via flag, environment, or config file: %w", ErrMissingCredentials)
		}
		client = rtm.NewClient(cfg.APIKey, cfg.APISecret, cfg.AuthToken)
		return nil
	}

	commands.Register(root, func() *rtm.Client { return client })

	return root
}
