# Contributing to ghlistend

How to propose changes, run checks locally, and open pull requests.

## License

This project uses the [Apache License 2.0](./LICENSE). By contributing, you
agree your contributions are licensed under the same terms unless stated
otherwise. Apache-2.0 includes an explicit patent grant, so you do not need
to sign a separate CLA — the license itself grants the project the rights it
needs to incorporate your work.

## Where to discuss

- **Issues, bugs, feature ideas:** use [GitHub Issues](https://github.com/sanguine59/ghlistend/issues).
- **Known limitations and review findings:** see [GitHub Issues](https://github.com/sanguine59/ghlistend/issues) — the HIGH items are the highest-leverage things to pick up.

## Development setup

Requires Go 1.26 or newer.

1. Clone the repository.
2. From the repo root, the Go workspace ties the modules together. No setup
   step is needed beyond `go` being on your PATH.
3. Build and run from `daemon/`:

   ```bash
   cd daemon
   go build -o ghlistend .
   ./ghlistend --help
   ```

4. Run tests:

   ```bash
   go test ./...                   # unit + feature tests
   go test -race ./...             # with the race detector
   go test -tags e2e ./e2e/...     # end-to-end (compiles the binary)
   ```

See `internal/poller/poller_test.go`, `internal/store/store_test.go`, and
`e2e/e2e_test.go` for the conventions in each test layer.

## Branch and pull requests

- Branch off `main`. Keep branches short-lived.
- **PR titles follow the conventional-commit format** (see below). The
  convention is what release notes will be generated from once a release
  pipeline lands, so it's worth following from day one even though no
  automation enforces it yet.
- **PR description:** what changed, why, how to verify (commands), and any
  risk or rollback notes.
- Keep diffs focused. One concern per PR makes review fast.

### Pull request titles

Format: `<type>[(scope)][!]: <subject>`

| Type               | Meaning                                                |
| ------------------ | ------------------------------------------------------ |
| `feat`             | New user-facing feature                                |
| `fix`              | Bug fix                                                |
| `perf`             | Performance improvement with no behavior change        |
| `refactor`         | Internal change, no behavior change                    |
| `test`             | Add or fix tests                                       |
| `ci`               | CI/workflow change                                     |
| `build` / `deps`   | Build system or dependency update                      |
| `docs`             | Documentation only                                     |
| `chore` / `revert` | Tooling, version bumps, reverts (excluded from notes)  |

Append `!` to the type or include `BREAKING CHANGE:` in the body to flag a
breaking change. Examples:

```text
feat(poller): paginate notifications past the first page
fix(notifier): skip MarkSeen when D-Bus delivery fails
perf(store): index seen table by notified_at for pruning
chore(deps): bump go-github to v89
ci: add arm64 cross-compile step
feat(cli)!: rename --notify-existing to --backfill
```

Commits within a PR may use any style — only the **merged PR title** is the
authoritative entry, so that's the one the convention applies to.

## Before you open a PR

- [ ] Tests pass: `cd daemon && go test ./...`
- [ ] Vet passes: `go vet ./...`
- [ ] Build passes for amd64 and arm64:
      `go build ./...` and `GOOS=linux GOARCH=arm64 go build ./...`
- [ ] `go mod tidy` is clean — no stray changes to `go.mod` / `go.sum`
- [ ] No secrets, tokens, or machine-specific paths committed
- [ ] Documentation updated if behavior, CLI flags, or file locations change
- [ ] If you touched poller / store / notifier, consider whether a feature
      test in `internal/<pkg>/<pkg>_test.go` would catch the change

## Code review

The maintainer may request changes for correctness, tests, security
(especially around D-Bus content and SQLite queries), or consistency with
the existing patterns. The HIGH issues in are open
invitations — those PRs will be reviewed and merged faster than drive-by
refactors.

## GitHub Actions — Concurrency Convention

Every workflow under `.github/workflows/` must declare a top-level
`concurrency:` block. The current convention:

- **Group key** is `${{ github.workflow }}-${{ github.ref }}` so two PRs
  never collide and two pushes to the same ref serialize correctly.
- **`cancel-in-progress` policy:**

  | Event                                  | `cancel-in-progress` | Why                              |
  | -------------------------------------- | -------------------- | -------------------------------- |
  | `pull_request` CI run                  | `true`               | New push supersedes old run      |
  | `push` to `main`                       | `false`              | Every main commit gets validated |
  | Tag push (release publish)             | `false`              | Never cancel mid-publish         |
  | `workflow_dispatch` (manual run)       | `false`              | Manual runs are intentional      |

When adding a new workflow, copy the concurrency block from
`daemon.yml` and adjust the `cancel-in-progress` value for the event shape.

## AI-assisted contributions

Coding agents are welcome to send PRs. Please:

- Follow any project context files you find (e.g. `CLAUDE.md`, `AGENTS.md`)
  if they exist.
- Avoid drive-by refactors unrelated to the issue you're solving.
- Prefer incremental, test-backed changes — a small change with a test beats
  a large change without one.
- If the agent generated code without running tests locally, say so in the
  PR description so the reviewer knows to lean harder on CI signals.

## Releases

There is no automated release pipeline yet. Versioning will follow
[Semantic Versioning](https://semver.org/) once the first tagged release
(`v0.1.0`) lands.
