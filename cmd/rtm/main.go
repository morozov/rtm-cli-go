// Command rtm is the Remember The Milk command-line client.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/morozov/rtm-cli-go/internal/commands"
	"github.com/morozov/rtm-cli-go/internal/rtm"
)

// ErrMissingCredentials is returned when the CLI is invoked without
// the --key and --secret flags set.
var ErrMissingCredentials = errors.New("missing RTM credentials")

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var (
		apiKey    string
		apiSecret string
		authToken string
		client    *rtm.Client
	)

	root := &cobra.Command{
		Use:           "rtm",
		Short:         "Remember The Milk CLI",
		SilenceUsage:  true,
		SilenceErrors: false,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			if apiKey == "" || apiSecret == "" {
				return fmt.Errorf("--key and --secret are required: %w", ErrMissingCredentials)
			}
			client = rtm.NewClient(apiKey, apiSecret, authToken)
			return nil
		},
	}
	root.PersistentFlags().StringVar(&apiKey, "key", "", "RTM API key")
	root.PersistentFlags().StringVar(&apiSecret, "secret", "", "RTM API secret")
	root.PersistentFlags().StringVar(&authToken, "token", "", "RTM auth token (required for logged-in methods)")

	commands.Register(root, func() *rtm.Client { return client })

	return root
}
