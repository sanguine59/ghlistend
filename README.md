# ghlistend

A headless GitHub notifications daemon for Linux. Polls the GitHub
Notifications REST API and dispatches native toasts via D-Bus. No UI, no tray
icon — just a background service you wire into systemd and forget about.

> **Status:** alpha. Useful for personal use; a handful of HIGH-priority bugs
> are tracked in issues. See [Limitations](#limitations).

## Why

GitHub's bell icon only exists in the browser. If you live in a terminal,
notifications about mentions, review requests, and CI failures arrive
whenever you happen to refresh a tab. `ghlistend` brings them into your
desktop notification stack the same way `mail` or Slack would.

## How it works

- **Polls**, not webhooks. Webhooks need a public HTTPS endpoint, which
  doesn't work behind NAT. The Notifications REST API is designed for
  polling: `If-Modified-Since` requests return `304 Not Modified` for free
  (no rate-limit cost), and the server tells the client how often to poll
  via `X-Poll-Interval`.
- **D-Bus** for delivery. We call `org.freedesktop.Notifications.Notify`
  directly, so any notification daemon (dunst, mako, GNOME Shell, KDE) works
  out of the box.
- **SQLite** for state. A tiny local database tracks which threads have been
  notified and the last `Last-Modified` checkpoint so we resume cleanly
  across restarts.
- **Keyring** for the token. The Personal Access Token lives in libsecret
  (via `gnome-keyring` / `kwallet` / equivalent), not a config file.

See [`ghlistend-spec.md`](./ghlistend-spec.md) for the full design notes.

## Install

### From source

Requires Go 1.26+.

```bash
git clone https://github.com/sanguine59/ghlistend.git
cd ghlistend/daemon
go build -o ~/.local/bin/ghlistend .
```

### Pre-built binaries

Not yet — release pipeline lands in `v0.1.0`.

## Usage

### 1. Get a Personal Access Token

Create a fine-grained PAT at https://github.com/settings/personal-access-tokens
with **Notifications: Read** access. No other scopes are required.

### 2. Log in

```bash
ghlistend login
# GitHub PAT: ********
# authenticated as @yourname
```

The token is stored in the OS keyring via libsecret. To script it:

```bash
echo "$GITHUB_PAT" | ghlistend login
```

### 3. Run the daemon

Foreground:

```bash
ghlistend start
```

Background, via systemd user service:

```bash
mkdir -p ~/.config/systemd/user
cp daemon/systemd/ghlistend.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now ghlistend
journalctl --user -u ghlistend -f   # tail logs
```

### Other commands

```bash
ghlistend status     # show authenticated user + last checkpoint
ghlistend logout     # remove the stored token
ghlistend --help     # full reference
```

### Flags

- `start --notify-existing` — fire toasts for notifications already unread
  on first run (default: only notify on activity after the daemon started).
- `--config <path>` — point at a custom config file
  (default: `$XDG_CONFIG_HOME/ghlistend/config.toml`).

## File locations

| Purpose | Path |
|---|---|
| Token | OS keyring (`service=ghlistend`, `username=github-pat`) |
| State DB | `$XDG_STATE_HOME/ghlistend/state.db` (default `~/.local/state/ghlistend/state.db`) |
| Config | `$XDG_CONFIG_HOME/ghlistend/config.toml` (optional) |
| Systemd unit | `~/.config/systemd/user/ghlistend.service` |

## Limitations

Tracked in [GitHub Issues](https://github.com/sanguine59/ghlistend/issues).. The headline items:

- **No pagination yet** — caps at the first 50 unread threads. Power users
  with active orgs will miss notifications past that.
- **No `seen` table pruning** — the local DB grows slowly forever.
- **401 causes a restart loop under systemd** — if your token goes bad,
  expect a re-auth toast every ~10 seconds until you `ghlistend logout` or
  stop the service.
- **D-Bus delivery failures are not retried** — a session blip can drop
  notifications.

Fix all four before treating this as production-grade.

## Development

Repo is a Go workspace:

```
ghlistend/
├── go.work               # workspace root
├── daemon/               # the daemon (this is what you build)
└── shared/               # placeholder for future cross-module code
```

### Run tests

```bash
cd daemon
go test ./...                       # unit + feature tests
go test -tags e2e ./e2e/...         # e2e tests (compile + drive the binary)
go test -race ./...                 # with the race detector
```

See the comments in `internal/poller/poller_test.go`,
`internal/store/store_test.go`, and `e2e/e2e_test.go` for what each layer
covers.

### Cross-compile

```bash
GOOS=linux GOARCH=arm64 go build -o ghlistend-arm64 .
```

Pure-Go SQLite driver means no CGO — the binary is fully static.

## License

Apache License 2.0 — see [LICENSE](./LICENSE). The license includes an
explicit patent grant, so contributions can be incorporated without a
separate CLA.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the development setup,
PR-title convention, and pre-PR checklist.

The HIGH issues in [GitHub Issues](https://github.com/sanguine59/ghlistend/issues). are the
highest-leverage things to pick up — those PRs will be reviewed and merged
faster than drive-by refactors.
