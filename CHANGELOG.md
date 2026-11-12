# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Map**, an experimental view with no commits in it: a node for each branch and tag, held around the trunk. A node's size is its commits (for the trunk all of them, for a branch the ones the trunk doesn't have, for a tag the ones since the tag before it) and its distance is how long since it last moved, with rings for this week and last. Zoom changes what the rings mean: all the way in, the map is one week with a ring for each day; out, the rings become weeks, then months, then years (the wheel, **+** and **-**, or the buttons in the corner). A branch with nothing of its own is a hollow ring, one that's only on the remote has a dashed outline, a tag hangs off the branch that holds it, and tags older than the newest six fold into one node. Click a node for its card, which opens the timeline on it or focused on its branch; right-click a branch or tag for its menu. Switch it on under **Settings › Experimental views**. The working tree is a dashed mark against the checked-out branch that says how many files have changed, a branch checked out in another worktree wears the worktree mark, and a stacked branch hangs off the branch under it. Drag a node to put it where you want it: it stays there for that repository until you double-click it, or its card lets it go. (#652, #661, #662, #672)
- **Horizontal**, an experimental view of the graph with time running across: a column for each commit and a row for each lane, newest on the left. A pinned column on the left names each lane by the branch that started it, even after its tip has scrolled away, and a lane a merge brought in by what the merge says it merged; tags have a row of their own, and lanes scroll up and down when there are more than fit. Subjects lean above their columns, a band across the top names each day, and the selected commit's subject, author and time show in full under the lanes. Left and Right move the selection and the wheel scrolls sideways. It's off until you switch it on under **Settings › Experimental views**; then it's in the graph's view menu. The graph still opens in Timeline. The working tree is the first column, before the newest commit, and the scrollbar is marked as the timeline's ruler is. Bot and Docs lanes fold as they do in the timeline: a lane's commits of a day are one column that says how many, and a click opens it. Drag one commit onto another for the same choices the timeline gives. (#651, #659, #660, #663, #671, #673, #674)

### Changed

- Three touches to the graph's rows. The selected commit's wash thins away across the row, where it was a flat block. A lane line that leaves a commit at full strength now fades into the quiet line below it, where it stopped at a flat edge. And each row has a hairline under it in its lane's colour, from under the commit's node out across the message, thinning to nothing. (#670)
- The graph's two-icon switch between Timeline and Compact is now a menu: one button in the bar shows the current view and opens the list. It takes less room, and it's where experimental views will go. Swapping the two takes one more click than before; the command palette's command still does it in one. (#650)
- Rendered previews use newer libraries: Mermaid 12.3.0 for diagrams, PDF.js 6.5.438 for PDFs and marked 18.1.3 for Markdown. (#643)
- The documentation site no longer loads its fonts from Google, so reading the docs sends nothing to a third party. (#645, #646)

## [0.11.0] - 2026-11-11

Git tools: everyday git chores without leaving the app.

### Added

- A ruler takes the scrollbar's place at the graph's right edge. Its box shows what's in view and can be dragged, with the month beside it as it moves; a press anywhere on it goes there. Ticks mark the checked-out commit, branches, tags, commits with notes and search matches, a faint line marks each month, and a hatched foot is the part of a long history not read yet. Point at a tick to see what's there. (#627, #638)
- A button at the foot of the graph goes back to the checked-out commit whenever that commit is out of view. It names the branch (or the short SHA when detached), points the way, and selects nothing; **Alt+Home** does the same. After a fetch it counts what's new on the upstream. (#619, #633)
- A commit with a git note wears a folded-page mark in the graph, beside its signature seal. Hover the mark to read the note, or click it to open the commit with its notes in view. Notes in every notes ref count. Type `has:note` in the graph's search to step through the commits that have one. (#620, #631)
- An **About** dialog with the version, a link to gitrevenge.com, how to sponsor the project, and credits for Go, Svelte and Wails. Click the version in the status bar, or the heart beside it, or choose **About Git Revenge** in the command palette. (#613)
- Right-click a changed file or folder to ignore it: the file, every file with its extension, or only on this machine. **Ignore…** picks the pattern and the file it goes in, previews what it hides, and offers to stop tracking matching tracked files. Every rule added can be undone. (#521, #524)
- An **Ignored** toggle lists ignored files with the rule behind each, and right-clicking one makes an exception or removes the rule. (#521, #524)
- An **Ignore rules** tab to read and edit every ignore file that applies: each `.gitignore`, `.git/info/exclude` and your global excludes file. Each rule says what it ignores now, and a checker says which rule decides any path. Open it from the sidebar's Repository panel or the command palette. (#522, #527, #540)
- Smart links: patterns you set in **Settings › Links** turn ticket keys such as `ABC-1234` in commit messages, trailers and branch names into links that open in your browser. Presets cover Jira, numbered issues, GitHub, GitLab, Linear and Azure Boards. Rules live in git config, in this repository's `.git/config` or your global config. A branch's right-click menu opens its ticket first. Git Revenge only makes the link and never contacts the site. (#525, #528)
- Bisect: find the commit that broke something. Name a commit where it's broken and one where it worked, then answer **Good**, **Bad** or **Can't tell** as Git Revenge checks out the commits in between. The graph marks your answers, the commit under test and the range still in question, and when the first bad commit is found you can open its changes, start a fix branch or revert it. Start it from a commit's menu or the command palette; a bisect started in a terminal shows up too. (#591, #592)
- Merge and rebase show what they would do before they run. The preview says which files will conflict and in how many places, worked out without touching your files, and lists the commits involved. A merge asks how: fast-forward only, a merge commit, or squash into one, with the ones that can't apply turned off. A rebase says which commit it would stop on, offers to fold `fixup!` commits into the commits they name or to open the rebase editor first, and warns when the branch is already pushed. (#594, #595)
- A merge, rebase, cherry-pick or revert in progress has a strip that says what it is and where it stands: both sides by name, the commit a rebase stopped on with a bead per step, and how many of a merge's conflicted files are resolved. **Resolve** opens the first conflicted file, **Continue** stays off until nothing is conflicted and then says what it will do, and **Abort…** says what it undoes before it does it. The graph ticks a rebase's commits as they're written again. When it's over, a notice offers **Undo**, and **Force push…** after rebasing a pushed branch. (#593)

### Changed

- Bot and Docs lanes fold in the graph: each lane's commits of a day take one line, such as "11 Docs commits", where they used to take a row each. Click the line, or press Right on it, to see the commits; Left closes it. The lane's tip, the selected commit and search matches always show. In the **Kinds** menu, **Fold** beside Bot or Docs chooses between one line a day and every commit; the choice is kept in the repository's git config as `gitrevenge.showKind`. Day counts leave folded commits out. (#624, #637)
- Each day in the timeline has a line of its own, with the day's name and a count of its commits (and of the people, once there's more than one). The line stays at the top of the graph while its day scrolls past, says so when more than a week went by before it ("3 weeks earlier"), and says what just arrived after a pull. Compact rows have no day lines. (#623, #636)
- The timeline gives a commit a second line only when it starts a run: the checked-out commit, a merge, the first commit of a day, a change of author, or a commit made two hours or more after the one before. Commits that carry a run on take one line, with the time at the right, so about twice as many fit on screen. Compact rows are unchanged. (#622, #635)
- The graph follows a branch you move. After a pull, checkout, merge, rebase, reset, cherry-pick or revert, the view goes to the branch's new tip, a third of the way down, and each commit that arrived wears a teal bar at the right edge until you select something else. A commit you had selected stays selected; with nothing chosen, the tip is selected. A fetch, or a change made outside the app, leaves the view where it was. (#619, #633)
- The bar above the graph stays on one line at every width. As it narrows, the commit count moves into the search field ("Search 411 commits"), the search becomes an icon that opens the field over the bar, then Undo and Redo lose their words, then Fetch, Pull and Push. In a very narrow panel the density switch goes too; Settings and the command palette still have it. The timeline no longer has column headings; compact rows keep theirs. (#626, #630)
- The graph's branch and tag column fits the labels in view, between 120 and 240 pixels wide, and you can drag its edge to set a width that's kept for the repository; double-click the edge to fit again. A long name is cut in the middle, so `617-watch…cleanup` and `617-watch…ends` stay apart. A commit shows one label and a count of the rest: hover for the list, or press the row's label button, where each label opens its own menu. The line tying a label to its commit and the band of lane colour behind a commit are gone. (#625, #629)
- Errors are short and say what to do. The banner gives one headline, such as "Push refused: main only takes changes through a pull request", one line on where that leaves you, and up to two next steps: **Push to a new branch…**, **See the rules**, **Pull, then push**, **Force push…** or **Try again**. **Details** shows what git said without its progress lines, and **Copy everything** copies all of it. A push refused by a branch rule, being behind the remote, sign-in, SSH key and connection failures are told this way; any other long message shows its first line with the rest behind **Details**. (#614)
- The interactive rebase editor is rebuilt. Commits are listed oldest first, as git applies them. Each has a menu for what happens to it, which says what every choice does and takes a letter key; a commit that folds into another sits indented under it; a reword edits the subject in place; and you can drag commits or move them with Alt and the arrow keys. **Fold it in for me** places `fixup!` and `squash!` commits under the commits they name. A panel shows the branch as it will be afterwards, what a drop throws away, and whether this order would stop on a conflict. The button says the outcome, such as **Rewrite as 3 commits**. (#596)
- The conflict tool is rebuilt around the result. Each conflicted place is a block in it with its own choices (**Ours**, **Theirs**, **Both**, **Start again**), and conflict markers never show and are never saved. The result is an ordinary editor: typing inside a place makes it **Edited**. Above it you can show both sides of the current place, or all three with the **Original**, each labelled with its branch and the commit that last changed it. F7 and Shift+F7 move between places, and **Mark resolved** stays off until every place is decided. A panel lists the files of the operation with their progress and those that merged cleanly, with whole-file choices and **Open in** your own merge tool when `merge.tool` is set. (#597)
- A repository's own settings (branch kinds, hidden branches, clean up on pull, notes syncing, stack parents set by hand and Insights thresholds) are kept in its git config, under `gitrevenge` keys, instead of the app's settings file. Set them in your global git config to make them your defaults for every repository; a repository's own setting wins, and its list replaces the global one. Existing settings move over the first time each repository opens. The app's own settings, such as theme and zoom, stay where they were. (#526)
- Insights › Shipped groups by the references your smart links find, when any link counts for Shipped, and shows them as links. The rules in use are listed in every export. Without such a link, `PROJ-123` and `#123` still group as before. (#525, #528)
- A bare `#123` in a trailer no longer links to the remote's issues on its own. Add a **Numbered (#123)** link in Settings to say where numbers go. (#525, #528)
- The graph's commit count, Kinds, density and search share one bar with Undo, Redo, Fetch, Pull and Push, so the graph starts a bar higher. They step aside while a file or the rebase editor is open. (#559)
- The working tree is the graph's first row, above the newest commit, with its mark on the first lane and its words where the messages start, instead of a strip above the graph. It still says clean, how many files changed, and offers **Review**. The button that hides the right panel moved to the end of the toolbar. (#556)
- Text actions that throw something away (**Discard all…**, **Delete**, **Remove**, **Discard mine and reload**) are drawn in the danger colour, so they no longer look like the safe action beside them. (#543)
- The graph's density switch is a pair of icons, timeline and compact, that shows which is on, instead of a button whose label changed with the state. (#544)
- A group dashboard's rows open the repository on click, as rows do everywhere else; the **Open** link stays for the keyboard. (#552)
- The sidebar rail marks branches that need care, and submodules that need updating, with a rose dot instead of a number, so it no longer looks like the stash count beside it. (#550)
- Every empty list says so the same way: a handwritten note and one plain sentence, from the sidebar and the working tree to the command palette, LFS, remotes and the reflog. Lists still being read show the loading mark instead. (#549)
- Settings › Appearance picks the theme (System, Light, Dark) and the commit graph (Timeline, Compact) with segmented controls, the same shape as the diff's Unified / Split, instead of drop-downs. (#561)
- Switching between the light and dark themes cross-fades the window instead of flipping it. Reduced motion keeps the flip. (#560)
- Compact rows: the Author column shows a ditto mark for an author carrying on from the row above, and the date column is headed **When**, since it holds relative times. (#557)
- In the timeline, the checked-out commit's row carries a small handwritten "you’re here" after its label when the branch column has room. (#562)
- Jumping to a commit (from a branch, a parent link or a search match) glides there when it's within three screens and cuts when it's further, and the commit lands under a wash that fades. Commits that arrive where you're already looking fade up, a few at a time. Reduced motion keeps everything instant. (#558)
- Resting the pointer on a commit in the graph (or selecting one from the keyboard) shows two small actions at the row's right edge: copy its SHA, and a menu with everything right-click offers. (#555)
- The selected commit is marked by a bar in its lane's colour at the row's left edge, over a lighter wash, and the selection, hover and search dimming ease into place instead of snapping. Reduced motion keeps the instant draw. (#553)
- The timeline's second line stops repeating itself: the first commit of each day names the day (**Today**, **Yesterday**, then the date) under a dashed line, commits below it in the same day show the author and time, and a run of commits by the same author shows only the time, faded. (#554)

### Fixed

- Pressing End in a long history shows the last commit in the commit panel once it has loaded; the panel used to stay empty. (#621, #634)
- Tab from the graph reaches the selected row's actions; the card used to go as the focus left the graph. (#625, #629)
- A link in a commit's subject is rose like the links in its message and trailers, not the browser's blue. (#582)
- An ignored file's menu opens on right-click (or the Menu key) only; a plain click no longer opens it. (#583)
- Opening a repository no longer adds Git LFS's hooks to it. They made every commit, checkout and merge run git-lfs, so a rebase in the app took seconds per commit even in repositories that don't use LFS. Repositories that don't track anything in LFS no longer run git-lfs at all on refresh. Hooks added before stay: in a repository that doesn't use LFS, `git lfs uninstall --local` removes them. (#604, #612)
- On Linux, the first worktree added from a terminal could go unnoticed until something else changed, so its branch didn't say it was checked out there. (#608, #611)

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

[Unreleased]: https://github.com/Git-Revenge/GitRevenge/compare/v0.11.0...HEAD
[0.11.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.10.0...v0.11.0
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
