# The commit graph

The graph is the main view of a repository: every commit, the branches and tags that point at it, and the lines connecting them.

![The commit graph of the Tidewater repository, with branches, a stash and uncommitted changes](../img/repository-light.webp#gh-light-mode-only)
![The commit graph of the Tidewater repository, with branches, a stash and uncommitted changes](../img/repository-dark.webp#gh-dark-mode-only)

It doesn't load the whole history when you open a repository. It loads the commits just ahead of where you're looking, and more as you scroll, search or jump. That's what keeps it quick on repositories with a million commits or more.

Commits are in date order, newest at the top, so work that happened side by side, such as a `gh-pages` site and `main`, is interleaved as it happened. Every commit still sits below the commits built on it.

Your **Trunk** branch (`main`, `master` or `develop`) runs straight down the left-hand column in rose, and other lines branch off it to the right. A second Trunk branch, such as `develop` beside `main`, gets the next column.

## Reading the graph

A commit on a branch is tinted with the colour of its line, from the commit to the start of the message, so you can tell which line it's on without tracing it. Trunk and Docs commits have no tint: Trunk is always the left-hand line, and Docs lines are meant to recede. A line is drawn thin and faint where it's idle, and thickens around its commits.

When there's room, branches and tags have their own column to the left of the graph, each joined to its commit by a thin line. A cloud marks a name that's on a remote, and the prefix of your main remote is left off. Your main remote is the one the checked-out branch tracks, else `origin`, else the only remote. A local branch shares a label with the remote branch it tracks when both are on the commit. A branch on several remotes takes one label, with a count beside the cloud, and a branch on just one other remote keeps its name, such as `fork/x`. In a narrow window, labels sit in front of the commit message instead.

Labels are ranked so the ones you'd most want to see come first: the checked-out branch, Trunk, Release, tags, Environment, Feature, then the rest. Whatever doesn't fit becomes **+N**. Hover over a commit's labels to expand them and see every branch and tag on it, in full, with each branch's row saying where it lives, such as `local · origin`. The same list is in the commit's details on the right.

Column headers name what's in each column. In compact rows with enough width, the filter button beside **Author** searches for the selected commit's author.

## Moving around

- Scroll, or use the arrow keys to move the selection one commit at a time.
- <kbd>Page Up</kbd> and <kbd>Page Down</kbd> move a page at a time.
- <kbd>Home</kbd> and <kbd>End</kbd> go to the newest and oldest commits.

Drag the left edge of the panel on the right to resize it, or use the arrow keys on it, and double-click the edge to reset it. Git Revenge remembers the width for each repository. Once it's wide enough, the panel lays out in two columns. To give the graph the room, hide the panel with the button at the right of the row above the graph, or <kbd>Ctrl</kbd>+<kbd>Alt</kbd>+<kbd>B</kbd>.

Selecting a commit shows it on the right. The top of the panel has its subject, author, a button to copy its SHA, a link to its parent, and an **Actions** menu. Tabs under that show the **Message**, the **Files** it changed with their line counts, its **Trailers** such as `Co-authored-by` and `Closes`, grouped by what they're for with long `Cc` lists collapsed (`#14` links to the issue on your repository's host, and **Grouped** switches to the trailers as written), and **Details** such as [notes](history.md#notes) and its [signature](history.md#signatures). The tab you pick stays picked as you move between commits. Once the panel is wide enough, the files stay beside the other tabs.

![A selected commit, with its message, trailers, author, a git note and the files it changed](../img/commit-details-light.webp#gh-light-mode-only)
![A selected commit, with its message, trailers, author, a git note and the files it changed](../img/commit-details-dark.webp#gh-dark-mode-only)


## Searching

Press <kbd>Ctrl</kbd>+<kbd>F</kbd> to search. You can search by commit message, author or SHA. You can also search for a trailer, such as `Co-authored-by: Sam` or `Closes: #12`.

<kbd>Enter</kbd> goes to the next match and <kbd>Shift</kbd>+<kbd>Enter</kbd> to the previous one. Matches further back than what's loaded are found too; the graph loads up to them. [Find a commit](../walkthroughs/find-a-commit.md) shows it in action.

## Branches and tags

The sidebar has an icon rail and one panel at a time: **Branches**, **Remotes**, **Tags**, **Stashes** and **Repository**. Click an icon to switch panels, or click the active one to hide the panel and give the graph the width. Drag the panel's right edge to resize it, and double-click the edge to reset it. Git Revenge remembers all of this for each repository.

![The Branches panel: main first, then feature and release folders, and three other branches](../img/sidebar-branches-light.webp#gh-light-mode-only)
![The Branches panel: main first, then feature and release folders, and three other branches](../img/sidebar-branches-dark.webp#gh-dark-mode-only)

Names with a `/` form folders, so `release/v0.8` and `release/v0.7` sit in a `release` folder. A folder holding one branch is shown as that branch, with its full name. **Name** or **Recent**, at the top of the panel, sorts by name or by most recently updated. The checked-out branch always comes first, marked with a rose dot and in bold, and other branches are muted. A branch shows one status at a time (**parent merged**, **gone**, or how far ahead or behind it is); hover over it for the rest. Hovering or tabbing to a row also shows a **…** button with the same menu as right-click.

Type in **Filter all panels** to narrow the lists. The filter stays on as you switch panels, and the rail shows how many matches each panel has.

![The sidebar filtered to "v0", with a count on each panel that has a match](../img/sidebar-filter-light.webp#gh-light-mode-only)
![The sidebar filtered to "v0", with a count on each panel that has a match](../img/sidebar-filter-dark.webp#gh-dark-mode-only)

**Needs care**, in the Branches panel, shows branches whose upstream is gone or whose stack parent was merged. Click a branch or tag to jump to its commit.

![Needs care, showing one branch whose upstream is gone](../img/sidebar-needs-care-light.webp#gh-light-mode-only)
![Needs care, showing one branch whose upstream is gone](../img/sidebar-needs-care-dark.webp#gh-dark-mode-only)

The sidebar works from the keyboard. <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>B</kbd> jumps to the filter, and <kbd>↓</kbd> moves from there into the list. In the list, the arrow keys move and open folders, <kbd>Enter</kbd> shows a branch in the graph, <kbd>Space</kbd> checks it out, the <kbd>Menu</kbd> key or <kbd>Shift</kbd>+<kbd>F10</kbd> opens its menu, and typing jumps to a name. The rail is one <kbd>Tab</kbd> stop, and <kbd>↑</kbd> and <kbd>↓</kbd> move along it.

A [kind](branch-care.md#branch-kinds)'s icon marks a folder, or a branch on its own, and a branch's chip in the graph carries its kind's colour and icon. Branches built on other branches sit under them as a [stack](branch-care.md#stacked-branches).

## Filtering by kind

When a repository has branches of any [kind](branch-care.md#branch-kinds), the graph's header has a **Kinds** dropdown with a checkbox for each one. Clear a box to hide every branch of that kind, and the button says how many kinds are hidden. Docs lines are drawn faded either way. Other branches can be hidden too; see [hiding branches](branch-care.md#hiding-branches).

## Two layouts

The default **timeline** layout gives each commit two lines: the subject, and the author and date. A commit's circle shows the author's initials, and shrinks to a dot when the same author's previous commit on that line is just above it. **Compact rows** fits one commit per line, so you see more history at once. Switch between them with the button in the graph's header, or in [Settings](settings.md).

## Right-click menus

Right-click a commit, branch or tag for what you can do with it: check it out, create a branch or tag there, merge, rebase, cherry-pick, revert, reset, or see its reflog. [Branches, merges and rebases](branches.md) covers these.

A commit's menu starts with what you can do with the commit itself. Below that, each branch and tag on it has its own submenu, so what you do to `main` stays apart from what you do to `origin/main`. Hover over one to open it, or use the arrow keys.

![A commit's menu, with the submenu of a branch on it open](../img/commit-menu-light.webp#gh-light-mode-only)
![A commit's menu, with the submenu of a branch on it open](../img/commit-menu-dark.webp#gh-dark-mode-only)
