<p align="center">
  <img src="build/social-preview.png" alt="Git Revenge: a fast, local-only desktop git client, built for very large repositories" width="720">
</p>

Git Revenge is a free, private desktop git client for Windows, macOS and Linux that stays fast on huge repositories. It drives the `git` you already have installed, so your config, hooks and credentials work as they do in a terminal.

- **Free.** No paid tier and no licence to buy, for private repositories or for work. Apache-2.0.
- **Private.** No accounts, no telemetry, no cloud features and no AI.
- **Fast.** On the Linux kernel (1.5M commits) the first screen arrives in under half a second.

The user guide, with pictures and walkthroughs, is at [gitrevenge.com](https://gitrevenge.com).

> **Status:** stable since 1.0.0. The [latest release](https://github.com/Git-Revenge/GitRevenge/releases/latest) is on GitHub, and [what a version promises](docs/PLAN.md#what-a-version-promises) says what 1.x keeps working.

## What it does

- **A commit graph that copes with big histories.** It loads history just ahead of where you're scrolling instead of all at once. Commits are in date order under a line for each day, with `main` pinned down the left as a spine, bot and docs commits folded away, and a ruler at the edge that marks branches, tags and search matches. You can search, jump to any commit, filter branches and tags, and narrow the timeline to one folder or file, as `git log -- <path>` does.
- **Clone a big repository in part.** The clone's options take part of the history, file contents only when something needs them, one branch, or only the folders you choose from the repository's tree, each as git does it and each off unless you choose it. The app then copes: a shallow history says where it stops, a partial clone downloads a file you open by itself and asks before downloading many, and the Repository panel offers the rest when you want it.
- **Search the code and its history.** Find in files searches the working tree or any commit with `git grep` and opens each file at the line. The history search filters the graph to the commits that brought a text in or took it out (`git log -S`), marks what each one did, and on a big history says how far back it has got, with Stop and Carry on. Neither downloads anything, even in a partial clone.
- **Branch care.** Branches get kinds (Trunk, Release, Environment, Docs, Bot) that change how they look and behave. Hide the noise, clean up merged branches after a pull (squash merges included), sync git notes, and see stacked branches, with a one-click restack once the branch below merges.
- **Everyday git.** Stage whole files, hunks or single lines. Commit and amend (signed, if your git config signs). Stash, branch, tag, merge, rebase, cherry-pick, reset and revert, with undo and redo. Select several commits in the graph to compare two, see a line of them as one change, squash it, or cherry-pick or revert them together.
- **Patches.** Save commits as patches with `git format-patch`, with every option it has. Apply a series with `git am` or a plain diff with `git apply`, each file or patch tried first so you see what fits, what merges and what conflicts before anything changes. Patches open from a file, a drop on the window or the clipboard. Sending mail stays with `git send-email` and b4.
- **Two versions of a branch.** After a rebase, an amend or a reset, see what became of each commit with `git range-diff`: which changed, which are the same, and which were folded in, dropped or added. Read a changed commit's two versions marked in one view or side by side. It's one click from the message after a rebase, a branch's menu, the reflog, or beside Force push.
- **Previews before it happens.** A merge or rebase says which files will conflict before it runs, and a strip shows where an operation in progress stands, with Continue, Skip and Abort.
- **Digging through history.** An interactive rebase editor that shows the branch as it will be, a conflict tool built around the result that suggests where both sides' edits combine, bisect, file history, blame, the reflog, and git notes. Signed commits get a badge showing whether the signature checks out.
- **Git's own files.** Ignore a file from its menu, see which rule ignores any path, and edit every ignore file that applies. Edit git config with a line on what each setting does and a backup on every save.
- **One name for each person.** Names follow the repository's mailmap, as `git log` does. The People tab lists everyone a history names, offers the ones that look like the same person, and writes the `.mailmap` line when you say so. It never creates the file without asking.
- **Smart filters.** The trailers a history already has, such as `Reviewed-by`, `Closes` and `Changelog`, become filters for the graph: show only the commits with a value or highlight them, combine several, and pin a key so its values read down the history. It works with whatever keys a project writes, and changes no commit.
- **Smart links.** Ticket keys in commit messages, trailers and branch names become links to Jira, GitHub, GitLab, Linear, Azure Boards or anywhere else. The app only builds the link.
- **Diffs that read the code.** In 47 languages, diffs compare code token by token: formatting, renames and moved code are told apart from changes, and can be hidden. The split view has a ribbon that joins each change across the sides.
- **Diffs and previews.** Unified or split diffs with syntax highlighting and image diffs. Files can also be shown rendered: Markdown, AsciiDoc and reStructuredText, SVG, images, CSV tables, JSON/YAML/TOML trees, LaTeX math, Mermaid diagrams, PDFs, Jupyter notebooks with their stored outputs, HTML pages in a sandbox, GeoJSON shapes, and audio and video. None of it reaches the network.
- **Remotes.** Fetch, pull and push through your existing credential helpers and SSH agent. If they can't answer, the app asks you. Each remote shows its host and forge and when it was last fetched, and opens its branches on the web. The Remotes panel groups by remote, host or branch name, and marks gone, merged and quiet branches for tidying. Push goes where `git push` would, and a remote can push to several places through git's own push URLs. No forge's API is called.
- **Lots of repos at once.** Repos open in tabs, worktrees are supported, and a group of local projects gets a dashboard with batch fetch and pull.
- **LFS and submodules.** Track LFS patterns, see which files are in LFS, lock and unlock them. Initialize and update submodules.
- **Insights for sprint reviews.** What shipped in a window, which branches are stuck or would conflict, and where the hotspots are, all computed from the repository alone and reproducible, with Markdown and CSV exports.
- **Comfortable to live in.** Light and dark themes with subtle, reduced-motion-aware animation, a timeline graph with a compact mode and two experimental views (Horizontal, and a Map of every branch or every author), a settings panel, and a command palette (<kbd>Ctrl</kbd>+<kbd>K</kbd>). Common actions have shortcuts, which you can rebind; <kbd>Ctrl</kbd>+<kbd>/</kbd> lists them. On macOS, read <kbd>⌘</kbd> for <kbd>Ctrl</kbd>.

## Privacy

No accounts, no cloud sync, no telemetry. Git Revenge uses your own git config, credential helpers and SSH agent. The only request it makes on its own is a check of GitHub Releases for a newer version at startup, and you can turn that off in Settings. The [privacy policy](docs/privacy.md) has the details, including the website and the download channels.

## Install

Install with a package manager, or grab the Windows installer or the macOS disk image from the [latest release](https://github.com/Git-Revenge/GitRevenge/releases/latest).

- **Windows:** `winget install GitRevenge.GitRevenge`. The installer on the release is signed too, but SmartScreen may warn you about a new release while its signature builds up a reputation; choose **More info**, then **Run anyway**.
- **macOS:** the disk image is signed and notarized. With Homebrew: `brew install --cask git-revenge/tap/gitrevenge`.
- **Linux:** Ubuntu 24.04 (with the Git PPA), RHEL 9, and RHEL 10 with EPEL, or newer. Install with `apt` or `dnf` from the signed repositories at [packages.gitrevenge.com](https://packages.gitrevenge.com); the [install guide](docs/guide/install.md#linux) has the commands. The `.deb` and `.rpm` files are on each release too.

Installing also gives you a `gitrevenge` command that opens the app [from a terminal](docs/guide/install.md#from-a-terminal).

## Requirements

- `git` 2.50 or newer.
- `git-lfs`, if you want the LFS features.
- For very large repositories, a commit-graph file makes the first screen of the graph much faster. Run `git commit-graph write --reachable` once, or set `fetch.writeCommitGraph=true` to keep it up to date.

## Development

You'll need Go 1.27.4+, Node 24+, and the [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation) at the version pinned in `go.mod`:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
```

```sh
wails dev                            # run with hot reload
wails build -tags "$(cat build/tags)"  # build build/bin/gitrevenge(.exe)
```

`build/tags` names the grammars the app reads code with, and keeps copyleft grammar code out of the build ([ADR 0015](docs/adr/0015-diffs-and-merges-read-the-code.md)). Builds that ship always use it.

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
