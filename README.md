# rtm-cli-go

Command-line client for the
[Remember The Milk API](https://www.rememberthemilk.com/services/api/).

The RTM API client and cobra command tree under `internal/rtm/`
and `internal/commands/` are produced from RTM's reflection
spec by [rtm-gen-go](https://github.com/morozov/rtm-gen-go) and
are **not** committed — `go generate` writes them on demand. If
your IDE flags unresolved symbols in a fresh checkout, run
`make` (see [Building from source](#building-from-source)) once.

## Building from source

A Go toolchain (1.26+) is all you need at the system level;
everything else (the generator, `goimports`, etc.) is pinned via
the `tool` directive in `go.mod`.

The `Makefile` wraps the build. What you run depends on whether
you already have a `spec.json` at the repo root — it is
gitignored, so fresh clones don't.

### First build (no spec.json yet)

Export RTM API credentials and let the build fetch the
reflection dump for you:

```sh
export RTM_API_KEY=your-key
export RTM_API_SECRET=your-secret
make
./rtm --help
```

This writes `spec.json`, runs `go generate ./...`, and produces
`./rtm`.

### Subsequent builds

Once `spec.json` is on disk, `make` skips the fetch — no
credentials needed:

```sh
make
```

### Refreshing the spec

`spec.json` is not touched by subsequent builds. To pick up any
upstream RTM changes, force a refresh:

```sh
make spec    # rewrites spec.json — needs RTM_API_KEY / RTM_API_SECRET
make         # rebuild against the new spec
```

`spec.json` is gitignored; each developer and CI run maintains
their own. The generated output under `internal/rtm/` and
`internal/commands/` is never committed either — `make generate`
rewrites it on demand.

## Configuration

The CLI reads credentials from three sources, with later sources
overriding earlier ones:

1. Config file at `$XDG_CONFIG_HOME/rtm/config.yaml` (default
   `~/.config/rtm/config.yaml` on Linux/macOS;
   `%AppData%\rtm\config.yaml` on Windows). A `$RTM_CONFIG_FILE`
   env var overrides the path.
2. Environment variables `RTM_API_KEY`, `RTM_API_SECRET`,
   `RTM_AUTH_TOKEN`.
3. Command-line flags `--key=…`, `--secret=…`, `--token=…`.

A missing config file is fine — env and flags still work. A
malformed config file is a fatal error.

### Config file format

```yaml
# ~/.config/rtm/config.yaml
api_key: your-rtm-api-key
api_secret: your-rtm-api-secret
auth_token: your-rtm-auth-token
```

Protect it: `chmod 600 ~/.config/rtm/config.yaml`. The CLI warns
to stderr on every invocation if the file is readable by group
or others. It also warns on unknown keys (likely typos).

### Required vs optional

- `api_key` / `api_secret` — always required.
- `auth_token` — required only for methods that need a
  logged-in user (most writes and some reads).

## First-time login

Before the CLI can hit any user-scoped method, you need an
`auth_token`. RTM issues these via a browser approval flow —
`rtm auth login` drives the whole ceremony end-to-end:

```sh
rtm auth login                     # read-only token (default)
rtm auth login --perms=write       # ask for write access
rtm auth login --perms=delete      # ask for delete-grade access
rtm auth login --no-browser        # just print the URL; don't launch
rtm auth login --force             # replace a working token
```

The command requests a frob, builds the signed approval URL,
opens it in your browser (unless `--no-browser`), waits for you
to click "Allow", exchanges the frob for a token, verifies the
token, and writes it atomically to the config file. If the
config file already holds a working token, it refuses without
`--force`; if the stored token is dead, it proceeds silently.

RTM has no server-side revocation endpoint. Revoke tokens from
the [authorized-apps page](https://www.rememberthemilk.com/app/#settings/apps)
in RTM's web UI.

## Run

```sh
rtm <service> <method> [flags]
```

Assuming credentials are configured (via any of the three
sources above), the CLI mirrors the RTM service hierarchy:

```sh
rtm reflection get-methods
rtm auth check-token
rtm lists get-list
rtm tasks add --name="Ship it" --list-id=123
rtm tasks set-priority --list-id=1 --taskseries-id=2 --task-id=3 --priority=2
rtm tasks set-tags --list-id=1 --taskseries-id=2 --task-id=3 --tags=shipit,work
rtm tasks notes add --list-id=1 --taskseries-id=2 --task-id=3 --note-title="..."
```

Each invocation makes one RTM call, unwraps the `rsp`/`stat`
envelope, and writes a typed response to stdout.

## Output formats

Select with `--output=…` (short form `-o`):

```sh
rtm lists get-list                # json, default
rtm lists get-list --output=yaml  # yaml
rtm lists get-list -o yaml        # short form
```

Both formats emit typed values — integer IDs are numbers, not
strings; booleans are `true`/`false`, not `"0"`/`"1"`. Absent
fields are omitted entirely rather than rendered as `""` or
`null`: empty string attrs, empty slices, and absent timestamps
all drop out of the output. Boolean and integer zero values
(`false`, `0`) are kept — those carry meaning distinct from
absence. Enum fields (`priority`, `perms`, `direction`) render
as their wire values (`"N"`, `"read"`, `"up"`).

## Exit codes

- `0` — success.
- `1` — internal error (bad flag, parse failure, I/O, etc.).
- `2` — the RTM API returned `stat=fail`. The error message
  carries the RTM code and description, e.g.
  `rtm api error 98: Login failed / Invalid auth token`.

## Typed flags and enums

Arguments with a known semantic type are declared as typed cobra
flags — `--list-id=1` accepts an integer, `--archive` accepts a
bool. Mistyped values fail locally with cobra's standard error
before any HTTP call.

Enum arguments (`--priority`, `--direction`) validate against a
closed set and register shell completion:

```sh
rtm tasks set-priority --priority=banana ...
# Error: invalid --priority "banana": expected 1, 2, 3, N
```

Comma-delimited list args (`--tags`) accept both repeated flags
and one comma-joined value:

```sh
rtm tasks set-tags ... --tags=urgent --tags=review
rtm tasks set-tags ... --tags=urgent,review
```

## Programmatic discovery

Tooling and AI agents can enumerate the full CLI surface in a
single call:

```sh
rtm manifest
```

The command walks the cobra tree and emits JSON: every
subcommand, its short/long descriptions, and its flags (name,
type, description, default, `required`, `enum_values` when the
flag is a closed set, plus any `[^N]` reference footnotes from
RTM's docs). Use it instead of crawling `rtm --help` recursively.

Credentials are not required — `manifest` is pure introspection
and never touches RTM.

## Shell completion

Cobra ships a `completion` subcommand that generates scripts for
bash, zsh, fish, and powershell. Install the one for your shell:

```sh
# bash (Linux)
rtm completion bash | sudo tee /etc/bash_completion.d/rtm

# zsh (macOS default, Oh My Zsh users: put it on fpath)
rtm completion zsh > "${fpath[1]}/_rtm"

# fish
rtm completion fish > ~/.config/fish/completions/rtm.fish

# powershell
rtm completion powershell | Out-String | Invoke-Expression
```

`rtm completion <shell> --help` shows per-shell install notes.

## Regenerate

The generator is pinned via the `tool` directive in `go.mod`.
`make generate` runs `go generate ./...`, which rewrites both
`internal/rtm/` and `internal/commands/` from the current
`spec.json`. Use `make spec` to refresh `spec.json` itself; see
[Building from source](#building-from-source) for when to use
which.

See [rtm-gen-go](https://github.com/morozov/rtm-gen-go) for the
underlying generator's flags.

## Distribution

`go install github.com/morozov/rtm-cli-go/cmd/rtm@latest` is
**not** supported — the module source on a proxy carries no
generated code. Distribute pre-built binaries (e.g. GitHub
releases) produced by a build that ran `make` (or `go generate`)
first.
