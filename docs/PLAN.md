# Git Revenge — Plan

Git Revenge is a fast, local-only desktop git client. It has a Go backend and is built to handle multiple repos, including very large ones.

This document records the decisions from the initial requirements session (2026-10-28). Architectural decisions with lasting consequences are recorded in [`docs/adr/`](adr/). Progress is tracked in GitHub issues, grouped by milestone.

## Goals

- An easy to use, powerful, fast git client.
- Works well on very large repos (see [Performance targets](#performance-targets)).
- Works across many repos, with user-defined project groups.
- Fully local: no accounts, no cloud, no telemetry.

## Non-goals

- AI features.
- Cloud features: accounts, sync, shared workspaces or hosted patches.
- Hosting-provider or issue-tracker integrations (GitHub/GitLab/Bitbucket/Jira APIs, pull requests).

## Platforms

- **Windows and macOS are first-class.** Linux is a nice-to-have.
- Requires a system `git` **>= 2.40**. `git-lfs` is required for LFS features.

## Architecture

| Concern | Decision | ADR |
|---|---|---|
| App shell | Wails v2 desktop app (WebView2 / WKWebView) | [0001](adr/0001-wails-desktop-shell.md) |
| Git engine | System `git` CLI behind a thin Go layer | [0002](adr/0002-system-git-cli.md) |
| Frontend | Svelte 5 + TypeScript + Vite; canvas commit graph; CodeMirror 6 for diffs | [0003](adr/0003-svelte-canvas-graph.md) |
| Credentials | Existing helpers first; askpass bridge into the UI | [0004](adr/0004-askpass-bridge.md) |
| State | Local JSON settings in the OS config dir | [0005](adr/0005-local-state-and-external-changes.md) |
| Git config editing | Working copy edited through `git config --file`; validated saves with backups | [0007](adr/0007-git-config-editing.md) |
| Insights | Computed only from repository state; a pure function of the repo and explicit inputs | [0008](adr/0008-insights-from-repository-state.md) |
| Branch kinds | Six built-in kinds (Trunk, Release, Environment, Docs, Bot, Feature) matched by local per-repo patterns | [0009](adr/0009-branch-kinds.md) |

### Backend

- **Git calls.** Every call runs with `GIT_TERMINAL_PROMPT=0`. Calls are context-cancellable and stream their output.
- **Persistent helper.** Each open repo keeps a long-lived `git cat-file --batch` process for fast object reads.
- **Per-repo operation queue.** Mutating operations are serialized, and reads run concurrently. If `index.lock` is held by another process, the app retries briefly and then reports it clearly.
- **Commit graph.** The lane layout is computed incrementally in Go while commits stream from `git log`. Only compact row data is sent to the UI.
- **Progressive loading.** The `git log` reader pauses once the graph is a set number of rows (20–50k, tunable) past the furthest row the UI has requested, and resumes as the user scrolls, so memory follows how far someone has scrolled, not repo size. Jump-to-commit, search and ref clicks load up to their target. The scrollbar total comes from `git rev-list --count`.
- **Watching for external changes.** Each repo gets one native recursive file-system subscription (`ReadDirectoryChangesW` on Windows, FSEvents on macOS). Changes are classified and debounced to about 150ms, and ignored files are filtered out. Where `core.fsmonitor` is enabled, it speeds up the `git status` that follows.
- **In-progress operations.** A merge, rebase, cherry-pick, revert or bisect that is in progress is detected even if it started outside the app.

### Frontend

- **Commit graph.** Drawn on a `<canvas>` with virtualized rows, so only the visible rows are drawn.
- **Diff, conflict and blame views.** Built with CodeMirror 6.
- **Visual design.** Until the 0.6 polish milestone, the UI is plain and functional.

## Multi-repo

- **Tabs.** Each open repo gets a tab and keeps warm backend state. Open tabs are restored on launch.
- **Project groups.** A group is a named, local collection of repo paths, and a repo can be in several groups. A group offers a dashboard (branch, ahead/behind, dirty state, last fetch), batch fetch, batch pull (fast-forward only), and an action to open every repo in the group as tabs.
- **Not in v1:** cross-repo branch operations and cross-repo commits.

## Feature scope

**v1 (milestones 0.1 to 0.6):**
- Commit graph with search and filter, and a commit detail panel.
- Staging at file, hunk and line level, a commit composer, amend, and signing (respecting git config).
- Diffs (split and unified, plus image diffs).
- Branches, tags and remotes, including fetch, pull, push and force-with-lease.
- Merge, cherry-pick, and drag-and-drop in the graph.
- A 3-pane conflict tool.
- Stash.
- Interactive rebase, reset, revert, and reflog-based undo/redo.
- LFS and submodules.
- File history and blame.
- Clone and init.
- Command palette, keyboard shortcuts, and light/dark themes.

**Later (1.1+):**
- Worktrees.
- Bisect UI.
- Hooks UI.
- Gitflow.
- An integrated terminal.
- A setup UI for partial clone and sparse checkout. The app still works on repos that already use them.
- Signature verification badges.

## Performance targets

The reference repo is the Linux kernel (about 1.3M commits and 90k files).

| Metric | Target |
|---|---|
| First screen of the graph | < 1s |
| Graph scrolling | 60fps across the full history |
| `status` refresh (warm cache) | < 500ms |
| Switching repo tabs | Instant |
| Memory with the kernel repo open | < 500MB |

The performance check is opt-in and does not run in CI. Run it before tagging each milestone:

```sh
GITREVENGE_BENCH_REPO=/path/to/linux go test ./internal/perf -v -count=1 -timeout 30m
```

It reports time to the first 100 rows, full load, heap in use, window reads, search and refs, and fails if a target is missed.

**Progressive loading.** The reader loads ahead of the viewport instead of the whole history, rather than capping the graph at a fixed size. A fixed cap would leave jump-to-commit, search, the scrollbar total and refs on old commits with nothing to point at.
- `git log` streams into the graph until it is 30,000 rows past the furthest row the UI has asked for (`GITREVENGE_READ_AHEAD` overrides it), then the reader blocks, which blocks git on its output pipe. Asking for later rows resumes it.
- The scrollbar spans `git rev-list --count` over the same revisions while loading. It slightly overcounts stash snapshots, so the status bar calls it approximate.
- Jumping to a commit (a ref, the selection after a reload) reads on until the commit appears. Search reads the rest of the history first, so memory after a search is the full history.
- A background refresh builds off-screen and swaps in once it reaches the same point, then keeps reading on demand.
- `TestReadAhead` in `internal/perf` reports time and heap at the first pause, the estimate, and the jump to the oldest commit.
- The graph's scroll area is capped at 8M px and its position scaled beyond that, since browsers cap element height (Chromium at about 33.5M px) and 1.5M rows at 52 px is 78M px. Scaled, the wheel still moves at its normal speed.

**Kernel run, 2026-10-31** (Windows 11, Linux at c339f4010dea: 1,503,174 commits, 956 refs, with a commit-graph file; date order):

| Measure | Result | Target |
|---|---|---|
| First 100 rows | 283 ms | < 1 s |
| First pause (30,000 rows) | 857 ms, 29 MB heap | |
| Estimate known | 1.26 s | |
| Full load and layout | 22.0 s | |
| Heap, whole history loaded | 469 MB | < 500 MB |
| 100-row window read | avg 3.8 ms, worst 11.0 ms | < 16 ms |
| Search "fix" (whole history) | 156 ms | |
| Jump to the oldest commit (reads all of it) | 19.4 s | |
| Status, warm | 267 ms | < 500 ms |

**Date order (#275).** The graph moved from `--topo-order` to `--date-order` so parallel lines of history (an orphan `gh-pages`, long-running branches) interleave as they happened instead of stacking as blocks. On the kernel that puts thousands of lanes side by side instead of about 800. Snapshotting every 128 rows then took the full-history heap to 641 MB, so snapshots now spread out once the graph is wider than 1,024 lanes (`checkpointGap`), which brought it back to 469 MB. The cost is a slower full load (16.3 s → 22.0 s, since each row steps through more lanes) and slower window reads, both within target. The topo-order run from 2026-10-29 measured 328 ms, 15.5 s, 467 MB and avg 0.74 ms for the same rows.

**Pinned Trunk lanes (#282).** With the default Trunk patterns pinning `master` to column 0, merges converge into the spine instead of opening new columns, and the kernel graph narrows a lot. Same machine, same day, against `main` without pinning: full load 20.0 s → 15.2 s, heap 471 MB → 426 MB, 100-row window avg 3.8 ms → 0.8 ms (worst 10.1 → 4.4 ms). First 100 rows went from 301 ms to 387 ms, the cost of resolving the Trunk tips before `git log` starts. `TestTargets` now pins Trunk lanes as the app does.

Two fixes from this run (#78): `git log` starts only from tags HEAD can't reach, because with all 953 tags as starting points `--topo-order` walked down from each and the first rows took 1.1 s. And checkpoints store each lane as a parent position instead of a 32-byte ID, which took the full-history heap from 857 MB to 467 MB.

**What the measurements so far show:**
- **Commit-graph file.** `git log` (date order) only produces its first commits quickly when the repo has a commit-graph file (`git commit-graph write --reachable`, or `fetch.writeCommitGraph=true`). The check warns when it's missing.
- **Ref short names.** `%(refname:short)` is quadratic in the number of refs, so the app strips ref prefixes itself.
- **Tags as starting points.** Every starting point makes the ordered walk go down from it before printing anything, so old tags are expensive. Tags that HEAD reaches are left out, since their commits are in the history anyway.
- **Loose refs.** A repo with thousands of loose (unpacked) refs reads them slowly on Windows. `git pack-refs --all` fixes this, and git does it during `gc`.

## Milestones

1. **0.1.0 Foundation:** scaffold, git CLI layer, repo open and tabs, read-only graph, commit detail, CI and release.
2. **0.2.0 Working tree:** status, staging, commit composer, diff viewer, stash.
3. **0.3.0 Branching & remotes:** branches, tags, remotes, credential prompts, merge, cherry-pick, drag-and-drop.
4. **0.4.0 History rewriting:** interactive rebase, reset, revert, undo/redo, conflict tool.
5. **0.5.0 Multi-repo & extras:** project groups and dashboard, LFS, submodules, file history and blame, clone and init.
6. **0.6.0 Polish:** visual design, themes, command palette, shortcuts, word wrap, perf pass, update check, signed releases.

Each feature area has an epic issue. Its tasks are tracked as sub-issues, each sized for one PR.

## Workflow

- **Issues to PRs.** Each issue gets its own branch, named `<issue#>-short-slug`, and one PR that says `Closes #N` and is squash-merged into `main`.
- **Commits** follow [Conventional Commits](https://www.conventionalcommits.org/). Release notes are generated from them.
- **Merging.** Chore PRs may be merged once CI is green. Feature PRs are reviewed by the maintainer.
- **Versioning.** Semver, starting at `0.1.0`.
- **Testing.**
  - Go tests run against fixture repos that are built on the fly with real git.
  - Vitest covers frontend logic.
  - CI runs on Windows and macOS.
- **Linting.** `golangci-lint`, `svelte-check` and `eslint`.

## Distribution

- **Builds.** A GitHub Actions workflow is triggered by `v*` tags.
  - Windows: an NSIS installer, unsigned for now.
  - macOS: a universal `.dmg`, signed and notarized with the maintainer's Apple Developer account.
- **Update check.** At startup the app asks the GitHub API for the latest release (no identifiers beyond a User-Agent) and shows a banner with a download link when it's a newer semver version. A dismissed version isn't announced again. The startup check can be turned off from the command palette; "Check for updates" always runs. There is no auto-update.
- **Audience.** First the maintainer, then an internal team, then open source under Apache-2.0.
