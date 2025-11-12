# Architecture Guide

This document is aimed at contributors (human or LLM) who need to understand how
the GitHub Workflow Watcher is structured and how to extend it safely.

## High-Level Flow

1. **User input** (PR / commit / run URL) enters via the Bubble Tea model in
   `internal/app`.
2. The URL is parsed into a structured request by `internal/githuburl`.
3. `internal/githubclient` translates that request into GitHub REST calls and
   normalizes the response into a `WorkflowRun`.
4. `internal/watch` keeps a deduplicated set of active and archived runs. It
   detects state changes so the bell can ring.
5. The Bubble Tea model/view renders the table and handles interactions (keyboard
   & mouse) including archiving, opening URLs, and auto-refreshing via a spinner.

```
  [User Input] --> app.Model --> githuburl.Parse
                                  |
                                  v
                     githubclient.Client (REST)
                                  |
                                  v
                   watch.Tracker (active/archived)
                                  |
                                  v
                          app.View (Bubble Tea)
```

## Code Layout

| Path                            | Purpose |
| --------------------------------| ------- |
| `cmd/ghwatch`                      | CLI entry point (`go run ./cmd/ghwatch`) |
| `internal/app`                  | Bubble Tea model/view logic |
| `internal/watch`                | Run tracker (active vs archived, status-change detection) |
| `internal/githuburl`            | URL parsing for commits/PRs/run IDs |
| `internal/githubclient`         | Thin REST wrapper around GitHub Actions endpoints |
| `integration/`                  | Live-integration tests (behind `-tags=integration`) |
| `internal/app/__snapshots__`    | go-snaps snapshot fixtures |
| `docs/architecture.md`          | This document |

## app.Model (Bubble Tea)

- Maintains focus (runs vs input), selection, scroll offsets, and bell state.
- Uses `textinput.Model` for the URL entry field and `spinner.Model` to show
  auto-refresh activity.
- Commands:
  - `fetchRunsCmd` runs when a new URL is submitted.
  - `refreshCmd` polls all active runs at intervals.
  - `openURLCmd` shells out to `open`/`xdg-open`.
- Mouse clicks select rows or focus the input; keyboard is modeled on lazygit.

## watch.Tracker

- `Upsert` deduplicates runs and now *auto-unarchives* a run if it was archived
  earlier and re-added.
- `Archive` and `Unarchive` mutate separate maps and order slices so the UI can
  show runs newest-first without re-sorting.
- Returns flags indicating whether a run is new or its status changed so the UI
  can show status messages and ring the bell.

## githubclient.Client

- Reads tokens from `GITHUB_TOKEN`, `GH_TOKEN`, then `GH_PAT`.
- Implements:
  - `WorkflowRunByID`
  - `RunsByPullRequest` (fetch PR -> head SHA -> runs)
  - `RunsByCommit`
- Normalizes GitHub payloads into a single `WorkflowRun` struct used everywhere
  else. Only GET requests are issued (read-only).

## Testing Strategy

- `go test` at the repo root runs everything (via `all_test.go` delegating to
  `go test ./...`).
- `internal/app/view_test.go` uses go-snaps snapshots; update by running
  `UPDATE_SNAPS=true go test ./internal/app`.
- `integration/githubclient_integration_test.go` has the `integration` build
  tag and hits real GitHub endpoints; it’s skipped unless explicitly requested.
- Snapshot directory is committed to keep deterministic CI runs.

## Contributor Notes

- Use `gofmt` / `go test ./...` before sending changes; CI runs them automatically.
- URLs only open on macOS (`open`) and Linux (`xdg-open`) by design.
- New UI surfaces should be built with Bubble Tea/Bubbles components already in
  use (textinput, spinner, etc.) to keep a consistent feel.
- When touching API behavior, prefer to add coverage in both `watch` (logic) and
  `app` (snapshot) tests if output changes.

## Guidance for LLM Agents

- Prefer `apply_patch` for edits and keep changes focused (instructions from the
  CLI harness still apply).
- Avoid destructive git commands; CI is responsible for enforcing formatting.
- Be mindful of rate limits when running integration tests—only run them when a
  change affects GitHub API interactions.
- Document new commands or key bindings in both `README.md` and the snapshot if
  the UI changes.

Feel free to expand this document with additional diagrams or flows as the
project grows.
