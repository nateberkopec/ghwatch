# GitHub Workflow Watcher

Bubble Tea TUI for keeping an eye on GitHub Actions runs. Paste a PR, commit, or
workflow-run URL at the bottom of the screen and the run instantly appears in the
table with live status updates.

## Requirements

- Go 1.25+
- macOS or Linux (URL opening uses `open` / `xdg-open`)
- Optional GitHub PAT for higher rate limits (`repo` + `workflow` scopes)

## Quick Start

```bash
# install tools (optional): mise use go@latest
go run ./cmd/gogh
```

Alternatively build once and keep the binary:

```bash
go build -o gogh ./cmd/gogh
./gogh
```

Paste any of the following into the bottom input field:

- `https://github.com/<owner>/<repo>/actions/runs/<run-id>`
- `https://github.com/<owner>/<repo>/pull/<number>`
- `https://github.com/<owner>/<repo>/commit/<sha>`

Runs are read-only and fetched directly from the public GitHub REST API.

## Key Bindings

| Key            | Action                                        |
| -------------- | --------------------------------------------- |
| `tab`          | Toggle focus between run list and input       |
| `j` / `down`   | Move selection down                           |
| `k` / `up`     | Move selection up                             |
| `pgdn` / `pgup`| Page through the list                         |
| `enter` / `o`  | Open PR/run URL (`open`/`xdg-open`)           |
| `a`            | Archive (active view) / restore (archive view)|
| `A`            | Toggle active vs archived runs                |
| `b`            | Toggle bell (🔔 vs ❌)                         |
| `q` / `Ctrl+C` | Quit                                          |

Mouse clicks select rows and focus the input, similar to lazygit.

## Environment Variables

The watcher first looks for tokens in:

1. `GITHUB_TOKEN`
2. `GH_TOKEN`
3. `GH_PAT`

Tokens only need read scopes (`repo`, `workflow`) and may be stored in a `.env`
file when using `mise`.

## Testing

Unit/snapshot tests (no network):

```bash
go test ./...
```

Integration tests (hit api.github.com, usually run manually):

```bash
go test -tags=integration ./integration
```

Snapshots live under `internal/app/__snapshots__` via
[go-snaps](https://github.com/gkampitakis/go-snaps).

## Need Architecture Details?

See [`docs/architecture.md`](docs/architecture.md) for module layout, data flow,
and contributor guidance (including notes for LLM agents).
