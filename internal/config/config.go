// Package config resolves RTM credentials for the CLI. It merges
// three sources in precedence order (lowest to highest): a YAML
// config file, RTM_* environment variables, and the root
// command's persistent flags.
package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config is the resolved set of credentials.
type Config struct {
	APIKey    string
	APISecret string
	AuthToken string
}

// knownKeys is every config key the CLI recognises. Used to warn
// on typos in the config file.
var knownKeys = map[string]struct{}{
	"api_key":    {},
	"api_secret": {},
	"auth_token": {},
}

// flagBindings maps viper config keys to the persistent-flag
// names on the root command.
var flagBindings = []struct{ key, flag string }{
	{"api_key", "key"},
	{"api_secret", "secret"},
	{"auth_token", "token"},
}

// Load resolves credentials from the config file (if present),
// RTM_* environment variables, and the persistent flags on the
// supplied FlagSet. Flags win over env; env wins over file.
//
// A missing config file is not an error. A malformed or
// unreadable file is.
func Load(flags *pflag.FlagSet) (Config, error) {
	return loadWith(flags, os.Stderr)
}

func loadWith(flags *pflag.FlagSet, warn io.Writer) (Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	path, err := filePath()
	if err != nil {
		return Config{}, err
	}
	v.SetConfigFile(path)

	v.SetEnvPrefix("RTM")
	v.AutomaticEnv()
	for _, k := range []string{"api_key", "api_secret", "auth_token"} {
		if err := v.BindEnv(k); err != nil {
			return Config{}, fmt.Errorf("bind env %s: %w", k, err)
		}
	}

	for _, b := range flagBindings {
		f := flags.Lookup(b.flag)
		if f == nil {
			continue
		}
		if err := v.BindPFlag(b.key, f); err != nil {
			return Config{}, fmt.Errorf("bind flag %s: %w", b.flag, err)
		}
	}

	warnPerms(path, warn)

	switch err := v.ReadInConfig(); {
	case err == nil:
		warnUnknownKeys(path, v, warn)
	case errors.Is(err, fs.ErrNotExist):
		// Missing config file is fine.
	default:
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	return Config{
		APIKey:    v.GetString("api_key"),
		APISecret: v.GetString("api_secret"),
		AuthToken: v.GetString("auth_token"),
	}, nil
}

// filePath returns where the CLI looks for its config file.
// Override order: $RTM_CONFIG_FILE, $XDG_CONFIG_HOME, $HOME/.config,
// or %AppData% on Windows.
func filePath() (string, error) {
	if override := os.Getenv("RTM_CONFIG_FILE"); override != "" {
		return override, nil
	}
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("AppData"); appdata != "" {
			return filepath.Join(appdata, "rtm", "config.yaml"), nil
		}
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "rtm", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir: %w", err)
	}
	return filepath.Join(home, ".config", "rtm", "config.yaml"), nil
}

func warnPerms(path string, w io.Writer) {
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	mode := info.Mode().Perm()
	if mode&0o044 != 0 {
		fmt.Fprintf(w, "warning: config file %s is readable by group or others (mode %o); consider chmod 600\n", path, mode)
	}
}

func warnUnknownKeys(path string, v *viper.Viper, w io.Writer) {
	keys := v.AllSettings()
	var unknown []string
	for k := range keys {
		if _, ok := knownKeys[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) == 0 {
		return
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		fmt.Fprintf(w, "warning: config file %s has unknown key %q\n", path, k)
	}
}
