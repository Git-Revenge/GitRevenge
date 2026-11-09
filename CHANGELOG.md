# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Git tools: everyday git chores without leaving the app.

### Added

- Right-click a changed file or folder to ignore it: the file, every file with its extension, or only on this machine. **Ignore…** picks the pattern and the file it goes in, previews what it hides, and offers to stop tracking matching tracked files. Every rule added can be undone. (#521, #524)
- An **Ignored** toggle lists ignored files with the rule behind each, and right-clicking one makes an exception or removes the rule. (#521, #524)
- An **Ignore rules** tab to read and edit every ignore file that applies: each `.gitignore`, `.git/info/exclude` and your global excludes file. Each rule says what it ignores now, and a checker says which rule decides any path. Open it from the sidebar's Repository panel or the command palette. (#522, #527, #540)
- Smart links: patterns you set in **Settings › Links** turn ticket keys such as `ABC-1234` in commit messages, trailers and branch names into links that open in your browser. Presets cover Jira, numbered issues, GitHub, GitLab, Linear and Azure Boards. Rules live in git config, in this repository's `.git/config` or your global config. A branch's right-click menu opens its ticket first. Git Revenge only makes the link and never contacts the site. (#525, #528)

### Changed

- Insights › Shipped groups by the references your smart links find, when any link counts for Shipped, and shows them as links. The rules in use are listed in every export. Without such a link, `PROJ-123` and `#123` still group as before. (#525, #528)
- A bare `#123` in a trailer no longer links to the remote's issues on its own. Add a **Numbered (#123)** link in Settings to say where numbers go. (#525, #528)
- Text actions that throw something away (**Discard all…**, **Delete**, **Remove**, **Discard mine and reload**) are drawn in the danger colour, so they no longer look like the safe action beside them. (#543)
||||||| parent of f8d0616 (feat: show the graph density as a two-icon segmented control)
- The graph's density switch is a pair of icons, timeline and compact, that shows which is on, instead of a button whose label changed with the state. (#544)
||||||| parent of 1a64be6 (feat: open a repository by clicking its dashboard row)
- A group dashboard's rows open the repository on click, as rows do everywhere else; the **Open** link stays for the keyboard. (#552)
||||||| parent of a09c0e5 (feat: a rose dot on the rail for what needs attention)
- The sidebar rail marks branches that need care, and submodules that need updating, with a rose dot instead of a number, so it no longer looks like the stash count beside it. (#550)
||||||| parent of 7b0f64f (feat: one shape for every empty list)
- Every empty list says so the same way: a handwritten note and one plain sentence, from the sidebar and the working tree to the command palette, LFS, remotes and the reflog. Lists still being read show the loading mark instead. (#549)
||||||| parent of 7f12c5d (feat: segmented controls for the theme and graph settings)
- Settings › Appearance picks the theme (System, Light, Dark) and the commit graph (Timeline, Compact) with segmented controls, the same shape as the diff's Unified / Split, instead of drop-downs. (#561)
||||||| parent of 1a3cfc9 (feat: cross-fade the window when the theme changes)
- Switching between the light and dark themes cross-fades the window instead of flipping it. Reduced motion keeps the flip. (#560)
||||||| parent of 60945ef (feat: ease the graph's selection, hover and dimming; mark the selection with a bar)
- The selected commit is marked by a bar in its lane's colour at the row's left edge, over a lighter wash, and the selection, hover and search dimming ease into place instead of snapping. Reduced motion keeps the instant draw. (#553)

## [0.10.0] - 2026-11-06

Linux becomes a first-class platform, with `.deb` and `.rpm` packages.

### Added

- Linux support for Ubuntu 24.04 and later, Debian 13, RHEL 9 and RHEL 10 (with EPEL), with `.deb` and `.rpm` packages on each release.
- A note when live refresh is off because the system's inotify watch limit is reached.
- A **Kinds** filter in the graph, replacing the Docs and Bots toggles, so any branch kind can be hidden.
- Focus the graph on one branch, or on a whole stack.
- Push or pull a single branch from its right-click menu.
- `.mdx` files preview as Markdown.

### Fixed

- A link in a commit message opened twice per click.
- Branches checked out in other worktrees can be shown, opened and deleted.
- Removing a worktree that's in use finishes the job, and the main tab keeps watching for changes.
- A worktree's tab notices ref changes made elsewhere.
- The graph flickered near 640px wide. The Working tree row now stays pinned left.

## [0.9.1] - 2026-11-04

A round of fixes and commit-panel work from daily use.

### Added

- Pull a branch that isn't checked out. (#408)
- The graph stays readable with many branches: a commit's labels are ranked so the important ones show, a local branch folds with the remote it tracks, the same branch on several remotes becomes one label with a count, and hovering expands every ref. (#412, #420–#423)
- A commit's kind is chosen by priority when several branches share it. (#424)
- The right panel can be resized, and hidden with a shortcut. (#443, #460)
- A commit opens in tabs, with its files always a click away, and its trailers grouped by purpose. (#449, #450)
- Uncommitted changes show as one list of files to tick. The commit button says what happens next, and undo is offered right after committing. (#451, #452, #458)
- Warnings for changes that come from file names differing only by case, and for committing on a detached HEAD. (#453, #454)
- A quiet row above the graph replaces the changes banner. (#455)
- Filter a panel's files, browse them as a tree, and move through them from the keyboard. (#456, #457)
- A trailer helper in the commit form. (#459)
- A stack of three or more branches shows as one group. Manage a stack from its right-click menu: rebase, push and update it as a whole. (#470, #476, #480)
- Delete every merged branch from Needs care in one click. (#472)
- A warning when undo or a force push would leave history behind that others already have. (#479)

### Fixed

- The confirm buttons' labels were hard to read. (#407)
- The row wash overpowered the commit graph. (#431)
- The signature mark touched the date in compact rows. (#442)
- Deleting a squash-merged branch asked to force it. (#445)
- A branch showed twice when its labels expanded. (#447)
- The commit form's options ran onto each other's lines. (#467)
- Stacks merged by squash didn't read as merged, said "parent merged" instead of "merged", and stopped reading as merged once Trunk moved on. (#468, #469, #478)
- Deciding what's safe to delete now reads git, not the graph.

## [0.9.0] - 2026-11-02

A new sidebar, and fixes from using the app every day.

### Added

- A sidebar with an icon rail and panels, a branch tree, the checked-out branch first, and sorting by recent updates. It works from the keyboard and with screen readers. (#334, #336–#338, #342)
- A Repository panel in the sidebar. (#345)
- A Feature branch kind, with a sprout icon. (#347)
- Re-order groups, and their repositories, on the start page. (#349)
- Each ref on a commit gets its own submenu. (#369)
- One loading state across the app. (#364)

### Changed

- Heavy views load on demand, and typing no longer rebuilds ref commands on every key press. (#370, #371)
- Git is asked about upstreams only for branches that have one. (#372)

### Fixed

- The commit menu didn't open on commits with several branches. (#333)
- The graph couldn't scroll back to the very top. (#341)
- Set upstream didn't take a typed branch name. (#348)
- The drop marker drifted when the start page scrolled mid-drag. (#363)
- Some text was smaller than 12px. (#366)
- Failures could look like empty data. (#367)
- Escape and Tab stopped working after a dialog button lost focus. (#375)
- Worktrees showed as clean, or as none, before they had loaded. (#376)
- Repositories under the home folder missed config change events, and a config file was reported before it had finished changing. (#378, #380)
- Trunk lost its kind when another kinded tip sat on its line. (#393)
- The end of a load could undo the move to HEAD. (#394)
- A closed diff reloaded. (#395)
- The rebase editor warned about dropped commits while its plan was still loading. (#396)
- The Repository panel waited for every section before showing any. (#397)
- File counts were claimed before status had loaded. (#398)
- The search box crowded the graph header, labels were clipped past reading, long menu labels were cut off, and blame dates and the Insights scope were clipped. (#399–#402)

## [0.8.0] - 2026-10-31

UI touch, and branch care: kinds, cleanup and stacks.

### Added

- One button family in three sizes, motion tokens, and subtle motion across the app. (#272, #273, #290–#292)
- Branch kinds (Trunk, Release, Environment, Docs, Bot), with per-repository patterns, icons, chips and sidebar sections. (#294, #301)
- Hide branches. (#295)
- Clean up merged branches on pull. (#299)
- Sync notes on fetch, pull and push. (#300)
- Trunk lanes are pinned in the graph. Docs lanes are muted, with toggles and kind icons in the timeline. (#302, #303)
- Confirmation before force pushes and deletes of special branches. (#309)
- Branch kinds in Insights. (#312)
- Stacked branches are detected and shown as a stack. A merged stack parent is flagged, with a one-click restack. (#315, #316)

### Fixed

- Parallel histories didn't interleave by date in the graph. (#288)
- The commit message toggle slid under long subjects. (#287)

## [0.7.0] - 2026-10-30

Insights, rich previews, and a Git Config editor.

### Added

- Git trailers on the selected commit, and search or filter commits by trailer. (#189, #193)
- Markdown in commit messages, with LaTeX math and Mermaid diagrams. (#191, #206, #210)
- A file view mode in the diff viewer. (#194)
- Rendered previews for Markdown, images, CSV and TSV tables, JSON, YAML and TOML trees, and PDFs. (#196–#198, #201, #211)
- Show and edit git notes on a commit. (#203)
- Signature verification badges. (#202)
- A reflog viewer. (#204)
- A settings panel and custom keybindings. (#207, #209)
- Worktree support. (#205)
- A documentation website with walkthroughs, a tour and generated screenshots. (#217, #222)
- A **Git Config** tab for global, local and included config files: a key/value table, a raw text editor, an alias editor, and backups with restore. Saves are validated, and repositories refresh when global config changes. (#231–#233, #235–#239)
- An **Insights** tab: Overview, Shipped, Needs attention and Hotspots for one repository or a group, computed only from repository state, with Copy summary and CSV export. (#255–#261)

### Fixed

- The graph's nodes slid out of view when the history was wider than its column. (#200)
- WebView2's own context menu showed where the app has none. (#214)
- Closing a repository didn't drop its undo and redo history, and the Undo label went stale. (#242, #243)

## [0.6.0] - 2026-10-29

Polish: the Ink and Agency look, a command palette and keyboard shortcuts.

### Added

- Ink and Agency design throughout: tokens, bundled fonts, light and dark themes, the app chrome, menus, dialogs, diff, blame and code views, and an icon system. (#127, #128, #132, #134, #178)
- The merge flag app icon. (#131)
- A timeline commit graph, with a compact mode. (#133)
- A command palette and keyboard shortcuts. (#136, #137)
- A word wrap toggle for diff and blame views. (#179)
- A check for newer versions on GitHub Releases. (#180)
- The commit graph loads progressively, ahead of the viewport. (#182)

### Changed

- The first screen and memory use meet the Linux kernel repository targets. (#187)

### Fixed

- The context menu highlight was hard to see. (#170)
- git-lfs wasn't found when the app was launched from Finder, and its absence wasn't explained. (#175)
- Closing the window on macOS didn't quit the app. (#177)

## [0.5.0] - 2026-10-29

Groups of repositories, LFS and submodules, and file history.

### Added

- Project groups, with a dashboard and batch fetch and pull. (#118)
- Git LFS (patterns, files and locks) and submodules. (#119)
- File history and blame. (#120)
- Clone and init repositories. (#121)

### Fixed

- Closing a repository didn't wait for its background graph load. (#117)

## [0.4.0] - 2026-10-29

Rewriting history, safely.

### Added

- Reset and revert. (#112)
- Undo and redo. (#113)
- A three-pane merge conflict tool. (#114)
- An interactive rebase editor. (#115)

## [0.3.0] - 2026-10-29

Branches and remotes.

### Added

- Branches, upstreams and tags. (#108)
- Fetch, pull and push, remotes, and in-app credential prompts. (#109)
- Merge, rebase and cherry-pick, including by drag and drop. (#110)

## [0.2.0] - 2026-10-28

The working tree.

### Added

- Working tree status, file staging and a commit composer. (#104)
- A diff viewer with hunk and line staging. (#105)
- Stash save, apply, pop and drop, with stashes in the graph. (#106)

## [0.1.0] - 2026-10-28

The foundation: open a repository and read its history fast.

### Added

- Open repositories in tabs, restored on launch, watched for changes made elsewhere.
- Detection of an in-progress merge, rebase, cherry-pick, revert or bisect.
- A canvas commit graph with incremental lane layout and virtualized rows, ref labels, and a read-only sidebar.
- A commit detail panel with changed files and file diffs.
- Search commits by message, author or SHA.
- Works with the `git` you have installed, and checks its version.

[Unreleased]: https://github.com/Git-Revenge/GitRevenge/compare/v0.10.0...HEAD
[0.10.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/Git-Revenge/GitRevenge/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Git-Revenge/GitRevenge/releases/tag/v0.1.0
