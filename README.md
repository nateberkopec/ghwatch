# ghwatch

<p align="center">
![watcher-glitch](https://github.com/user-attachments/assets/766f41e7-b767-46d0-bdf8-8ee2b79f5fd6)
</p>

Bubble Tea TUI for keeping an eye on GitHub Actions runs. Paste a PR, commit, or
workflow-run URL at the bottom of the screen and the run instantly appears in the
table with live status updates.

<img width="2022" height="1179" alt="Screenshot 2025-11-12 at 10 35 54" src="https://github.com/user-attachments/assets/682d8a8e-f511-47cd-a004-f19b36ecb42d" />

## Requirements

- macOS, Linux, or Windows
- Optional GitHub PAT for higher rate limits (`repo` + `workflow` scopes)
- Go 1.25+ (only required if building from source)

## Quick Start

### Install from Release

Download the latest binary for your platform from the [releases page](https://github.com/nateberkopec/ghwatch/releases):

```bash
# macOS (Apple Silicon)
curl -L https://github.com/nateberkopec/ghwatch/releases/latest/download/ghwatch_darwin_arm64.tar.gz | tar xz
sudo mv ghwatch /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/nateberkopec/ghwatch/releases/latest/download/ghwatch_darwin_amd64.tar.gz | tar xz
sudo mv ghwatch /usr/local/bin/

# Linux (x86_64)
curl -L https://github.com/nateberkopec/ghwatch/releases/latest/download/ghwatch_linux_amd64.tar.gz | tar xz
sudo mv ghwatch /usr/local/bin/

# Run it
ghwatch
```

### Install from Source

If you have Go 1.25+ installed:

```bash
go install github.com/nateberkopec/ghwatch/cmd/ghwatch@latest
```

Or build from the repository:

```bash
git clone https://github.com/nateberkopec/ghwatch.git
cd ghwatch
go build -o ghwatch ./cmd/ghwatch
./ghwatch
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

Unit/snapshot tests:

```bash
go test ./...
```

Integration tests (hit api.github.com):

```bash
go test -tags=integration ./integration
```

Integration tests automatically use `GITHUB_TOKEN`, `GH_TOKEN`, or `GH_PAT` environment
variables if available. In CI, the tests run on every push using GitHub's auto-generated
`GITHUB_TOKEN`.

Snapshots live under `internal/app/__snapshots__` via
[go-snaps](https://github.com/gkampitakis/go-snaps).

## Need Architecture Details?

See [`docs/architecture.md`](docs/architecture.md) for module layout, data flow,
and contributor guidance (including notes for LLM agents).
