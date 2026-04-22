package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/morozov/rtm-cli-go/internal/config"
	"github.com/morozov/rtm-cli-go/internal/rtm"
)

// authBrowserURL is the base URL for the user-facing approval
// page. Parameters (api_key, perms, frob, api_sig) are appended
// per RTM's authentication flow.
const authBrowserURL = "https://www.rememberthemilk.com/services/auth/"

// validPerms enumerates the RTM permission levels in ascending
// order. A token issued with one level can be used for methods
// whose requiredperms is that level or lower.
var validPerms = map[string]struct{}{
	"read":   {},
	"write":  {},
	"delete": {},
}

// ErrExistingToken is returned when the config file already
// holds an auth_token and --force was not supplied.
var ErrExistingToken = errors.New("config file already holds an auth token")

// ErrInvalidPerms is returned when --perms is not one of
// read/write/delete.
var ErrInvalidPerms = errors.New("invalid --perms value")

func newAuthLoginCmd(clientFor func() *rtm.Client) *cobra.Command {
	var (
		perms     string
		noBrowser bool
		force     bool
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Run the RTM approval flow and store the resulting auth token",
		Long: "Run the RTM approval flow end-to-end: request a frob, open the\n" +
			"browser to RTM's approval page, wait for confirmation, exchange\n" +
			"the frob for an auth token, verify it, and write it to the\n" +
			"config file.\n\n" +
			"The persistent --key and --secret flags (or their env / config\n" +
			"equivalents) must be set. --token is ignored — login produces one.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, ok := validPerms[perms]; !ok {
				return fmt.Errorf("%w: %q (expected read, write, or delete)", ErrInvalidPerms, perms)
			}
			return runAuthLogin(
				cmd.Context(),
				clientFor(),
				cmd.OutOrStderr(),
				cmd.InOrStdin(),
				authLoginOptions{
					Perms:     perms,
					NoBrowser: noBrowser,
					Force:     force,
				},
			)
		},
	}
	cmd.Flags().StringVar(&perms, "perms", "read", "permissions to request: read, write, or delete")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "do not attempt to open the approval URL in a browser")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing auth_token in the config file")
	return cmd
}

type authLoginOptions struct {
	Perms     string
	NoBrowser bool
	Force     bool
}

func runAuthLogin(ctx context.Context, client *rtm.Client, progress io.Writer, stdin io.Reader, opts authLoginOptions) error {
	cfgPath, err := config.Path()
	if err != nil {
		return err
	}

	existing, existingExists, err := config.Read(cfgPath)
	if err != nil {
		return err
	}
	if existingExists && existing.AuthToken != "" && !opts.Force {
		// Only refuse when the existing token actually works.
		// A dead token protects nothing, so there's no value in
		// making the user spell out --force to replace it.
		client.AuthToken = existing.AuthToken
		if _, err := client.Auth.CheckToken(ctx); err == nil {
			return fmt.Errorf("%w at %s: re-run with --force to replace", ErrExistingToken, cfgPath)
		}
		client.AuthToken = ""
	}

	_, _ = fmt.Fprintln(progress, "Requesting a frob…")
	frobResp, err := client.Auth.GetFrob(ctx)
	if err != nil {
		return err
	}
	frob := frobResp.Frob

	authURL := buildAuthURL(client, frob, opts.Perms)
	_, _ = fmt.Fprintln(progress, "Open this URL in your browser to approve the request:")
	_, _ = fmt.Fprintln(progress, "  "+authURL)
	if !opts.NoBrowser {
		if err := openBrowser(authURL); err != nil {
			_, _ = fmt.Fprintf(progress, "  (could not launch browser automatically: %v)\n", err)
		}
	}

	_, _ = fmt.Fprint(progress, "Press Enter once you've clicked \"Allow\"… ")
	if _, err := bufio.NewReader(stdin).ReadString('\n'); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read confirmation: %w", err)
	}

	_, _ = fmt.Fprintln(progress, "Exchanging frob for token…")
	tokenResp, err := client.Auth.GetToken(ctx, rtm.AuthGetTokenParams{Frob: frob})
	if err != nil {
		return err
	}
	token := tokenResp.Auth.Token
	if token == "" {
		return fmt.Errorf("rtm.auth.getToken returned an empty token")
	}

	client.AuthToken = token
	checkResp, err := client.Auth.CheckToken(ctx)
	if err != nil {
		return fmt.Errorf("verify token: %w", err)
	}
	_, _ = fmt.Fprintf(
		progress,
		"Verified: logged in as %s (%s)\n",
		checkResp.Auth.User.Username,
		checkResp.Auth.User.Fullname,
	)

	if err := config.Write(cfgPath, config.Config{
		APIKey:    client.APIKey,
		APISecret: client.APISecret,
		AuthToken: token,
	}); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(progress, "Wrote token to %s\n", cfgPath)
	return nil
}

// buildAuthURL constructs the signed RTM approval URL. RTM's
// signing rule is the same MD5-of-sorted-concatenation the
// client uses for every API request, so it's delegated to
// (*rtm.Client).Sign.
func buildAuthURL(client *rtm.Client, frob, perms string) string {
	params := url.Values{}
	params.Set("api_key", client.APIKey)
	params.Set("perms", perms)
	params.Set("frob", frob)
	params.Set("api_sig", client.Sign(params))
	return authBrowserURL + "?" + params.Encode()
}
