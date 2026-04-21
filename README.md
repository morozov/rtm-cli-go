# rtm-cli-go

Command-line client for the
[Remember The Milk API](https://www.rememberthemilk.com/services/api/).

Most of this module is hand-written. Two subdirectories are
produced by [rtm-gen-go](https://github.com/morozov/rtm-gen-go)
at build time and are **not** committed:

- `internal/rtm/` — the RTM API client (stdlib-only).
- `internal/commands/` — the cobra command tree, exposed through
  a `Register(root, provider)` function that `cmd/rtm/main.go`
  calls.

Everything else is hand-written: the binary entry point, the
root cobra command, persistent flags, credential sourcing, and
anything else that isn't an RTM API binding. Bugs in generated
code should be filed against the **generator** (`rtm-gen-go`),
not here.

## Building from source

Building requires three things:

1. A Go toolchain (1.26+).
2. A spec source — either a local `spec.json` cache or RTM API
   credentials for live fetch.
3. A local checkout of `rtm-gen-go` as a sibling directory
   (`../rtm-gen-go/`). `go.mod` carries a local `replace` that
   points there until `rtm-gen-go` is published.

### One-time setup

```sh
# Place a cached RTM spec at ./spec.json. It is gitignored.
cp /path/to/your/api.json spec.json
```

Or fetch live:

```sh
# Requires RTM credentials. Live fetch calls the reflection API
# directly; the generator itself does not need a pre-existing
# spec file.
# (See "Regenerate" below — the live-fetch flags go on rtm-gen.)
```

### Build

```sh
go generate ./...       # produces internal/rtm/ and internal/commands/
go build -o rtm ./cmd/rtm
./rtm --help
```

Every fresh clone (and every CI run) repeats these two steps;
the generated output is never committed.

## Configuration

The CLI reads credentials from three sources, with later sources
overriding earlier ones:

1. Config file at `$XDG_CONFIG_HOME/rtm/config.yaml` (default
   `~/.config/rtm/config.yaml` on Linux/macOS;
   `%AppData%\rtm\config.yaml` on Windows). A `$RTM_CONFIG_FILE`
   env var overrides the path.
2. Environment variables `RTM_API_KEY`, `RTM_API_SECRET`,
   `RTM_AUTH_TOKEN`.
3. Command-line flags `--key`, `--secret`, `--token`.

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
rtm tasks add --name "Ship it" --list-id 123
rtm tasks notes add --list-id 1 --taskseries-id 2 --task-id 3 --note-title "..."
```

Each invocation makes one RTM call and writes the raw JSON
response body to stdout.

## Programmatic discovery

Tooling and AI agents can enumerate the full CLI surface in a
single call:

```sh
rtm manifest
```

The command walks the cobra tree and emits JSON: every
subcommand, its short/long descriptions, and its flags (name,
type, description, default, `required`). Use it instead of
crawling `rtm --help` recursively.

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

The generator is pinned via the `tool` directive in `go.mod`:

```sh
go generate ./...
```

That invokes, via `//go:generate` directives in
`internal/rtm/gen.go` and `internal/commands/gen.go`:

```sh
go tool rtm-gen client -spec ./spec.json -out internal/rtm
go tool rtm-gen cli    -spec ./spec.json -out internal/commands \
                       -client-module github.com/morozov/rtm-cli-go/internal/rtm
```

Swap `-spec ./spec.json` for `-key $RTM_API_KEY -secret
$RTM_API_SECRET` in the directives (or run `rtm-gen` manually)
to fetch the spec live instead of reading from a local cache.

## Distribution

`go install github.com/morozov/rtm-cli-go/cmd/rtm@latest` is
**not** supported — the module source on a proxy carries no
generated code. Distribute pre-built binaries (e.g. GitHub
releases) produced by a build that ran `go generate` first.

## Layout

```
rtm-cli-go/
├── cmd/rtm/main.go          (hand-written)
├── internal/
│   ├── rtm/
│   │   ├── gen.go           (hand-written; //go:generate anchor)
│   │   ├── client.go        (generated, gitignored)
│   │   └── <service>.go     (generated, gitignored)
│   └── commands/
│       ├── gen.go           (hand-written; //go:generate anchor)
│       ├── register.go      (generated, gitignored)
│       └── <service>.go     (generated, gitignored)
├── spec.json                (developer-local cache, gitignored)
├── go.mod
├── go.sum
└── README.md
```
