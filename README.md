<p align="center">
  <img src="build/social-preview.png" alt="Git Revenge: a fast, local-only desktop git client, built for very large repositories" width="720">
</p>

Git Revenge is a free, private desktop git client for Windows, macOS and Linux that stays fast on huge repositories. It drives the `git` you already have installed, so your config, hooks and credentials work as they do in a terminal.

- **Free.** No paid tier and no licence to buy, for private repositories or for work. Apache-2.0.
- **Private.** No accounts, no telemetry, no cloud features and no AI.
- **Fast.** On the Linux kernel (1.5M commits) the first screen arrives in under half a second.

The user guide, with pictures and walkthroughs, is at [gitrevenge.com](https://gitrevenge.com).

> **Status:** early development. The latest release is [`0.11.0`](https://github.com/Git-Revenge/GitRevenge/releases/tag/v0.11.0).

## What it does

- **A commit graph that copes with big histories.** It loads history just ahead of where you're scrolling instead of all at once. Commits are in date order under a line for each day, with `main` pinned down the left as a spine, bot and docs commits folded away, and a ruler at the edge that marks branches, tags and search matches. You can search, jump to any commit, and filter branches and tags.
- **Branch care.** Branches get kinds (Trunk, Release, Environment, Docs, Bot) that change how they look and behave. Hide the noise, clean up merged branches after a pull (squash merges included), sync git notes, and see stacked branches, with a one-click restack once the branch below merges.
- **Everyday git.** Stage whole files, hunks or single lines. Commit and amend (signed, if your git config signs). Stash, branch, tag, merge, rebase, cherry-pick, reset and revert, with undo and redo.
- **Previews before it happens.** A merge or rebase says which files will conflict before it runs, and a strip shows where an operation in progress stands, with Continue, Skip and Abort.
- **Digging through history.** An interactive rebase editor that shows the branch as it will be, a conflict tool built around the result, bisect, file history, blame, the reflog, and git notes. Signed commits get a badge showing whether the signature checks out.
- **Git's own files.** Ignore a file from its menu, see which rule ignores any path, and edit every ignore file that applies. Edit git config with a line on what each setting does and a backup on every save.
- **Smart links.** Ticket keys in commit messages, trailers and branch names become links to Jira, GitHub, GitLab, Linear, Azure Boards or anywhere else. The app only builds the link.
- **Diffs and previews.** Unified or split diffs with syntax highlighting and image diffs. Files can also be shown rendered: Markdown, AsciiDoc and reStructuredText, SVG, images, CSV tables, JSON/YAML/TOML trees, LaTeX math, Mermaid diagrams, PDFs, Jupyter notebooks with their stored outputs, HTML pages in a sandbox, GeoJSON shapes, and audio and video. None of it reaches the network.
- **Remotes.** Fetch, pull and push through your existing credential helpers and SSH agent. If they can't answer, the app asks you.
- **Lots of repos at once.** Repos open in tabs, worktrees are supported, and a group of local projects gets a dashboard with batch fetch and pull.
- **LFS and submodules.** Track LFS patterns, see which files are in LFS, lock and unlock them. Initialize and update submodules.
- **Insights for sprint reviews.** What shipped in a window, which branches are stuck or would conflict, and where the hotspots are, all computed from the repository alone and reproducible, with Markdown and CSV exports.
- **Comfortable to live in.** Light and dark themes with subtle, reduced-motion-aware animation, a timeline graph with a compact mode and two experimental views (Horizontal, and a Map of every branch), a settings panel, and a command palette (<kbd>Ctrl</kbd>+<kbd>K</kbd>). Common actions have shortcuts, which you can rebind; <kbd>Ctrl</kbd>+<kbd>/</kbd> lists them. On macOS, read <kbd>⌘</kbd> for <kbd>Ctrl</kbd>.

## Privacy

No accounts, no cloud sync, no telemetry. Git Revenge uses your own git config, credential helpers and SSH agent. The only request it makes on its own is a check of GitHub Releases for a newer version at startup, and you can turn that off in Settings. The [privacy policy](docs/privacy.md) has the details, including the website and the download channels.

## Install

Grab the Windows installer or the macOS disk image from the [latest release](https://github.com/Git-Revenge/GitRevenge/releases/latest).

- **macOS:** the disk image is signed and notarized. With Homebrew: `brew install --cask git-revenge/tap/gitrevenge`.
- **Windows:** the installer isn't signed yet, so SmartScreen will warn you before it runs.
- **Linux:** Ubuntu 24.04, Debian 13, RHEL 9, and RHEL 10 with EPEL, or newer. Install with `apt` or `dnf` from the signed repositories at [packages.gitrevenge.com](https://packages.gitrevenge.com); the [install guide](docs/guide/install.md#linux) has the commands. The `.deb` and `.rpm` files are on each release too.

## Requirements

- `git` 2.40 or newer.
- `git-lfs`, if you want the LFS features.
- For very large repositories, a commit-graph file makes the first screen of the graph much faster. Run `git commit-graph write --reachable` once, or set `fetch.writeCommitGraph=true` to keep it up to date.

## Development

You'll need Go 1.27.3+, Node 24+, and the [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation) at the version pinned in `go.mod`:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
```

```sh
wails dev        # run with hot reload
wails build      # build build/bin/gitrevenge(.exe)
```

The Go binary embeds `frontend/dist`, so on a fresh clone build the frontend once before running Go tools directly:

```sh
cd frontend && npm ci && npm run build && cd ..
go test ./...
```

The frontend checks CI runs:

```sh
cd frontend && npm run check && npm run lint && npm test
```

The TypeScript bindings in `frontend/wailsjs` are generated from the Go API by `wails dev`, `wails build` or `wails generate module`. Commit them exactly as generated.

Performance checks run against a large repository of your choice; see [Performance targets](docs/PLAN.md#performance-targets).

## Docs

- [The documentation site](https://gitrevenge.com), with the user guide. Its source is `docs/`, and `docs-site/` builds it: `cd docs-site && npm ci && npm start`.
- [Plan](docs/PLAN.md): scope, architecture, performance targets, milestones and workflow.
- [Architecture decision records](docs/adr/).
- [Changelog](CHANGELOG.md): what changed in each release.
- [Releasing](docs/RELEASING.md).
- [Accessibility](ACCESSIBILITY.md), [contributing](CONTRIBUTING.md), [code of conduct](CODE_OF_CONDUCT.md), [security policy](SECURITY.md) and [privacy policy](docs/privacy.md).
- Work is tracked in [issues](https://github.com/Git-Revenge/GitRevenge/issues), grouped by [milestone](https://github.com/Git-Revenge/GitRevenge/milestones).

## License

[Apache-2.0](LICENSE)
