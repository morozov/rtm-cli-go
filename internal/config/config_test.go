package config

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFlagSet builds a pflag.FlagSet mirroring the persistent
// flags main.go installs on the root cobra command. Flags named
// in `setFlags` are marked Changed via Parse so viper treats them
// as overrides.
func newFlagSet(t *testing.T, setFlags map[string]string) *pflag.FlagSet {
	t.Helper()
	fs := pflag.NewFlagSet("rtm", pflag.ContinueOnError)
	fs.String("key", "", "")
	fs.String("secret", "", "")
	fs.String("token", "", "")
	var args []string
	for name, val := range setFlags {
		args = append(args, "--"+name+"="+val)
	}
	require.NoError(t, fs.Parse(args))
	return fs
}

func clearRTMEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"RTM_API_KEY", "RTM_API_SECRET", "RTM_AUTH_TOKEN", "RTM_CONFIG_FILE"} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

// pointConfigAt redirects Load's file lookup to the given path
// by setting RTM_CONFIG_FILE for the duration of the test.
func pointConfigAt(t *testing.T, path string) {
	t.Helper()
	t.Setenv("RTM_CONFIG_FILE", path)
}

func writeConfig(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestLoadFileOnly(t *testing.T) {
	clearRTMEnv(t)
	dir := t.TempDir()
	path := writeConfig(t, dir, "api_key: k\napi_secret: s\nauth_token: t\n")
	pointConfigAt(t, path)

	var buf bytes.Buffer
	cfg, err := loadWith(newFlagSet(t, nil), &buf)
	require.NoError(t, err)
	assert.Equal(t, Config{APIKey: "k", APISecret: "s", AuthToken: "t"}, cfg)
	assert.Empty(t, buf.String())
}

func TestLoadEnvOnly(t *testing.T) {
	clearRTMEnv(t)
	t.Setenv("RTM_API_KEY", "env-k")
	t.Setenv("RTM_API_SECRET", "env-s")
	t.Setenv("RTM_AUTH_TOKEN", "env-t")
	// Point at a non-existent file so no config-file layer interferes.
	pointConfigAt(t, filepath.Join(t.TempDir(), "missing.yaml"))

	var buf bytes.Buffer
	cfg, err := loadWith(newFlagSet(t, nil), &buf)
	require.NoError(t, err)
	assert.Equal(t, Config{APIKey: "env-k", APISecret: "env-s", AuthToken: "env-t"}, cfg)
}

func TestLoadFlagsOnly(t *testing.T) {
	clearRTMEnv(t)
	pointConfigAt(t, filepath.Join(t.TempDir(), "missing.yaml"))

	var buf bytes.Buffer
	cfg, err := loadWith(newFlagSet(t, map[string]string{
		"key":    "flag-k",
		"secret": "flag-s",
		"token":  "flag-t",
	}), &buf)
	require.NoError(t, err)
	assert.Equal(t, Config{APIKey: "flag-k", APISecret: "flag-s", AuthToken: "flag-t"}, cfg)
}

func TestLoadEnvOverridesFile(t *testing.T) {
	clearRTMEnv(t)
	dir := t.TempDir()
	path := writeConfig(t, dir, "api_key: file-k\napi_secret: file-s\nauth_token: file-t\n")
	pointConfigAt(t, path)
	t.Setenv("RTM_API_KEY", "env-k")

	var buf bytes.Buffer
	cfg, err := loadWith(newFlagSet(t, nil), &buf)
	require.NoError(t, err)
	assert.Equal(t, "env-k", cfg.APIKey, "env MUST override file")
	assert.Equal(t, "file-s", cfg.APISecret, "file value retained when env unset")
	assert.Equal(t, "file-t", cfg.AuthToken)
}

func TestLoadFlagOverridesEverything(t *testing.T) {
	clearRTMEnv(t)
	dir := t.TempDir()
	path := writeConfig(t, dir, "api_key: file-k\napi_secret: file-s\nauth_token: file-t\n")
	pointConfigAt(t, path)
	t.Setenv("RTM_API_KEY", "env-k")

	var buf bytes.Buffer
	cfg, err := loadWith(newFlagSet(t, map[string]string{"key": "flag-k"}), &buf)
	require.NoError(t, err)
	assert.Equal(t, "flag-k", cfg.APIKey, "flag MUST override env and file")
	assert.Equal(t, "file-s", cfg.APISecret, "other fields come from file")
	assert.Equal(t, "file-t", cfg.AuthToken)
}

func TestLoadMissingFileIsOK(t *testing.T) {
	clearRTMEnv(t)
	pointConfigAt(t, filepath.Join(t.TempDir(), "never-exists.yaml"))

	var buf bytes.Buffer
	cfg, err := loadWith(newFlagSet(t, nil), &buf)
	require.NoError(t, err)
	assert.Equal(t, Config{}, cfg)
	assert.Empty(t, buf.String())
}

func TestLoadMalformedFileErrors(t *testing.T) {
	clearRTMEnv(t)
	dir := t.TempDir()
	path := writeConfig(t, dir, "this: is: not: yaml:\n")
	pointConfigAt(t, path)

	var buf bytes.Buffer
	_, err := loadWith(newFlagSet(t, nil), &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path, "error message must name the config file")
}

func TestLoadWarnsOnUnknownKeys(t *testing.T) {
	clearRTMEnv(t)
	dir := t.TempDir()
	path := writeConfig(t, dir, "api_key: k\nfavorite_color: teal\ntimezone: PST\n")
	pointConfigAt(t, path)

	var buf bytes.Buffer
	_, err := loadWith(newFlagSet(t, nil), &buf)
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "favorite_color")
	assert.Contains(t, out, "timezone")
	assert.NotContains(t, out, `"api_key"`, "known keys MUST NOT be reported")
}

func TestLoadWarnsOnWorldReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission warning is Unix-only")
	}
	clearRTMEnv(t)
	dir := t.TempDir()
	path := writeConfig(t, dir, "api_key: k\n")
	require.NoError(t, os.Chmod(path, 0o644))
	pointConfigAt(t, path)

	var buf bytes.Buffer
	_, err := loadWith(newFlagSet(t, nil), &buf)
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, path)
	assert.Contains(t, strings.ToLower(out), "readable")
}

func TestLoadNoWarnOn600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission warning is Unix-only")
	}
	clearRTMEnv(t)
	dir := t.TempDir()
	path := writeConfig(t, dir, "api_key: k\n")
	require.NoError(t, os.Chmod(path, 0o600))
	pointConfigAt(t, path)

	var buf bytes.Buffer
	_, err := loadWith(newFlagSet(t, nil), &buf)
	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "readable")
}

func TestFilePathOverride(t *testing.T) {
	t.Setenv("RTM_CONFIG_FILE", "/custom/rtm.yaml")
	path, err := filePath()
	require.NoError(t, err)
	assert.Equal(t, "/custom/rtm.yaml", path)
}

func TestFilePathXDG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG fallback is Unix-only in this test")
	}
	t.Setenv("RTM_CONFIG_FILE", "")
	_ = os.Unsetenv("RTM_CONFIG_FILE")
	t.Setenv("XDG_CONFIG_HOME", "/xdg/home")
	path, err := filePath()
	require.NoError(t, err)
	assert.Equal(t, "/xdg/home/rtm/config.yaml", path)
}

func TestFilePathHomeFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("home fallback is Unix-only in this test")
	}
	_ = os.Unsetenv("RTM_CONFIG_FILE")
	_ = os.Unsetenv("XDG_CONFIG_HOME")
	t.Setenv("HOME", "/tmp/fake-home")
	path, err := filePath()
	require.NoError(t, err)
	assert.Equal(t, "/tmp/fake-home/.config/rtm/config.yaml", path)
}
