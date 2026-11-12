# The commit graph

The graph is the main view of a repository: every commit, the branches and tags that point at it, and the lines connecting them.

![The commit graph of the Tidewater repository, with branches, a stash and uncommitted changes](../img/repository-light.webp#gh-light-mode-only)
![The commit graph of the Tidewater repository, with branches, a stash and uncommitted changes](../img/repository-dark.webp#gh-dark-mode-only)

It doesn't load the whole history when you open a repository. It loads the commits just ahead of where you're looking, and more as you scroll, search or jump. That's what keeps it quick on repositories with a million commits or more.

Commits are in date order, newest at the top, so work that happened side by side, such as a `gh-pages` site and `main`, is interleaved as it happened. Every commit still sits below the commits built on it.

Your **Trunk** branch (`main`, `master` or `develop`) runs straight down the left-hand column in rose, and other lines branch off it to the right. A second Trunk branch, such as `develop` beside `main`, gets the next column.

## Reading the graph

A line is drawn thin and faint where it's idle, and thickens around its commits. Trunk is always the left-hand line, and Docs lines are drawn faded, since they're meant to recede.

### Days

In the timeline, each day has a line of its own above its first commit, with the day's name (**Today**, **Yesterday**, then the date) and a count of its commits, and of the people who made them once there's more than one. The line of the day you're looking at stays at the top of the graph while that day scrolls past. When more than a week went by before a day, its line says so, such as "3 weeks earlier", and after a pull it says how many commits just arrived.

A commit takes a second line, with its author and time, only when it starts a run: the checked-out commit, a merge, the first commit of a day, a change of author, or a commit made two hours or more after the one before. The commits that carry a run on take one line, with the time at the right, and their circle shrinks from the author's initials to a dot. About twice as many commits fit on screen that way.

### Branch and tag labels

When there's room, branches and tags have their own column to the left of the graph. The column fits the labels in view. Drag its right edge to set a width, which is kept for the repository, and double-click the edge to fit it again. In a narrow window, labels sit in front of the commit message instead.

A long name is cut in the middle, so `617-watch…cleanup` and `617-watch…ends` stay apart. A commit shows one label and a count of the rest. Hover over them for the full list, or press the label button on the row, where each branch and tag opens its own menu. The same list is in the commit's details on the right.

Labels are ranked so the one you'd most want to see comes first: the checked-out branch, Trunk, Release, tags, Environment, Feature, then the rest.

A cloud marks a name that's on a remote, and the prefix of your main remote is left off. Your main remote is the one the checked-out branch tracks, else `origin`, else the only remote. A local branch shares a label with the remote branch it tracks when both are on the commit. A branch on several remotes takes one label, with a count beside the cloud, and a branch on just one other remote keeps its name, such as `fork/x`.

### Marks on a commit

- The selected commit has a bar in its line's colour at the left edge of its row, and the row is washed in that colour, fading to the right.
- A hairline in the line's colour underlines each row, so you can follow a commit across to its message.
- The checked-out commit's row carries a handwritten "you're here" after its label, when there's room.
- A commit with a [git note](history.md#notes) wears a folded-page mark. Hover over the mark to read the note, or click it to open the commit with its notes in view.
- A signed commit wears a seal; its [signature](history.md#signatures) is in the commit's details.
- After a pull, or anything else that moves your branch, each commit that arrived wears a teal bar at the right edge until you select something else.

![The selected row, washed in its line's colour, and the hairline under every row](../img/graph-row-marks-light.webp#gh-light-mode-only)
![The selected row, washed in its line's colour, and the hairline under every row](../img/graph-row-marks-dark.webp#gh-dark-mode-only)

## Moving around

Jumping to a commit, from a branch in the sidebar, a parent link or a search match, glides there when it's close and cuts when it's far, and the commit lands under a wash that fades so you can find it. If your system asks for reduced motion, everything moves at once instead.

- Scroll, or use the arrow keys to move the selection one commit at a time.
- <kbd>Page Up</kbd> and <kbd>Page Down</kbd> move a page at a time.
- <kbd>Home</kbd> and <kbd>End</kbd> go to the newest and oldest commits.
- <kbd>Alt</kbd>+<kbd>Home</kbd> goes back to the commit you have checked out.

### Back to where you are

Whenever the checked-out commit is out of view, a button at the foot of the graph goes back to it. It names the branch, or the short SHA when you aren't on a branch, and points the way. It selects nothing, so the commit you were reading stays in the panel on the right. After a fetch it also counts what's new on the upstream.

The graph follows a branch you move. After a pull, checkout, merge, rebase, reset, cherry-pick or revert, the view goes to the branch's new tip. A commit you had selected stays selected. A fetch, or a change made outside the app, leaves the view where it was.

### The ruler

A ruler stands at the graph's right edge, where a scrollbar would be. Its box is the part of the history in view: drag it, and the month you're passing shows beside it, or press anywhere on the ruler to go there.

![The ruler at the graph's right edge, with ticks for branches, tags and the checked-out commit](../img/graph-ruler-light.webp#gh-light-mode-only)
![The ruler at the graph's right edge, with ticks for branches, tags and the checked-out commit](../img/graph-ruler-dark.webp#gh-dark-mode-only)

Ticks mark the checked-out commit, branches, tags, commits with notes and search matches, and a faint line marks each month. Point at a tick to see what's there. On a very long history, a hatched foot is the part that hasn't been read yet.

### The panel on the right

Drag the left edge of the panel on the right to resize it, or use the arrow keys on it, and double-click the edge to reset it. Git Revenge remembers the width for each repository. Once it's wide enough, the panel lays out in two columns. To give the graph the room, hide the panel with the button at the right end of the toolbar, or <kbd>Ctrl</kbd>+<kbd>Alt</kbd>+<kbd>B</kbd>.

Selecting a commit shows it on the right. The top of the panel has its subject, author, a button to copy its SHA, a link to its parent, and an **Actions** menu. Tabs under that show the **Message**, the **Files** it changed with their line counts, its **Trailers** such as `Co-authored-by` and `Closes`, grouped by what they're for with long `Cc` lists collapsed (**Grouped** switches to the trailers as written), and **Details** such as [notes](history.md#notes) and its [signature](history.md#signatures). The tab you pick stays picked as you move between commits. Once the panel is wide enough, the files stay beside the other tabs.

Ticket keys and web addresses in a message or trailer are links when a [smart link](settings.md#links) matches them.

![A selected commit, with its message, trailers, author, a git note and the files it changed](../img/commit-details-light.webp#gh-light-mode-only)
![A selected commit, with its message, trailers, author, a git note and the files it changed](../img/commit-details-dark.webp#gh-dark-mode-only)

## Searching

Press <kbd>Ctrl</kbd>+<kbd>F</kbd> to search. You can search by commit message, author or SHA. You can also search for a trailer, such as `Co-authored-by: Sam` or `Closes: #12`, and `has:note` finds the commits that have a git note.

<kbd>Enter</kbd> goes to the next match and <kbd>Shift</kbd>+<kbd>Enter</kbd> to the previous one. Matches further back than what's loaded are found too; the graph loads up to them. Each match has a tick on the [ruler](#the-ruler). [Find a commit](../walkthroughs/find-a-commit.md) shows it in action.

## Branches and tags

The sidebar has an icon rail and one panel at a time: **Branches**, **Remotes**, **Tags**, **Stashes** and **Repository**. Click an icon to switch panels, or click the active one to hide the panel and give the graph the width. Drag the panel's right edge to resize it, and double-click the edge to reset it. Git Revenge remembers all of this for each repository.

![The Branches panel: main first, then feature and release folders, and three other branches](../img/sidebar-branches-light.webp#gh-light-mode-only)
![The Branches panel: main first, then feature and release folders, and three other branches](../img/sidebar-branches-dark.webp#gh-dark-mode-only)

Names with a `/` form folders, so `release/v0.8` and `release/v0.7` sit in a `release` folder. A folder holding one branch is shown as that branch, with its full name. **Name** or **Recent**, at the top of the panel, sorts by name or by most recently updated. The checked-out branch always comes first, marked with a rose dot and in bold, and other branches are muted. A branch shows one status at a time (**parent merged**, **gone**, or how far ahead or behind it is); hover over it for the rest. Hovering or tabbing to a row also shows a **…** button with the same menu as right-click.

Type in **Filter all panels** to narrow the lists. The filter stays on as you switch panels, and the rail shows how many matches each panel has. Without a filter, the rail shows a number for how many stashes you have, and a rose dot on a panel that has something needing attention, such as a branch that needs care.

![The sidebar filtered to "v0", with a count on each panel that has a match](../img/sidebar-filter-light.webp#gh-light-mode-only)
![The sidebar filtered to "v0", with a count on each panel that has a match](../img/sidebar-filter-dark.webp#gh-dark-mode-only)

**Needs care**, in the Branches panel, shows branches whose upstream is gone or whose stack parent was merged. Click a branch or tag to jump to its commit.

![Needs care, showing one branch whose upstream is gone](../img/sidebar-needs-care-light.webp#gh-light-mode-only)
![Needs care, showing one branch whose upstream is gone](../img/sidebar-needs-care-dark.webp#gh-dark-mode-only)

The sidebar works from the keyboard. <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>B</kbd> jumps to the filter, and <kbd>↓</kbd> moves from there into the list. In the list, the arrow keys move and open folders, <kbd>Enter</kbd> shows a branch in the graph, <kbd>Space</kbd> checks it out, the <kbd>Menu</kbd> key or <kbd>Shift</kbd>+<kbd>F10</kbd> opens its menu, and typing jumps to a name. The rail is one <kbd>Tab</kbd> stop, and <kbd>↑</kbd> and <kbd>↓</kbd> move along it.

A [kind](branch-care.md#branch-kinds)'s icon marks a folder, or a branch on its own, and a branch's chip in the graph carries its kind's colour and icon. Branches built on other branches sit under them as a [stack](branch-care.md#stacked-branches).

## Filtering by kind

When a repository has branches of any [kind](branch-care.md#branch-kinds), the bar above the graph has a **Kinds** dropdown with a checkbox for each one. Clear a box to hide every branch of that kind, and the button says how many kinds are hidden. Other branches can be hidden too; see [hiding branches](branch-care.md#hiding-branches).

### Folded lanes

Bot and Docs lines are mostly noise: a dependency bot's branches, or a published site rebuilt on every merge. In the timeline, each such line's commits of a day take one line, such as "11 Docs commits". Click the line, or press <kbd>→</kbd> on it, to see the commits, and <kbd>←</kbd> to close it again. The line's tip, the selected commit and search matches always show.

**Fold**, beside Bot or Docs in the **Kinds** menu, chooses between one line a day and every commit. The choice is kept in the repository's git config, as `gitrevenge.showKind`. The counts on the day lines leave folded commits out.

## Focusing on a branch

Focus is the opposite of [hiding a branch](branch-care.md#hiding-branches): it shows only that branch. Right-click a branch, in the sidebar or the graph, and choose **Focus on …**. A branch in a [stack](branch-care.md#stacked-branches) also offers **Focus on the stack of …**, and a stack's own menu offers **Focus on this stack**, which shows every branch in it.

The graph then shows the branch, its remote-tracking copies, and the history they're built on. Other branches' labels stay on commits that are still showing, so you can see where the branch left them. Stashes and the checked-out branch's other commits leave the view, and the commit panel and working tree still work as usual.

Only one focus exists at a time, and choosing another replaces it. The bar above the graph says what's focused. To go back to everything, click the **×** there, or right-click the branch and choose **Stop focusing**. A focus lasts until you close the tab, or until its branch is deleted.

## Views

The default **Timeline** view is the one described above: a line for each day, and a second line for a commit that starts a run. **Compact** fits one commit per line with no day lines, under column headings, so you see more history at once. In Compact with enough width, the filter button beside **Author** searches for the selected commit's author, an author carrying on from the row above shows as a ditto mark, and the **When** column holds how long ago each commit was made.

Switch between them with the view menu in the bar above the graph. Its button shows the view in use; click it, or press <kbd>Enter</kbd> on it, and pick another. Settings and the command palette switch views too.

Two more views, **Horizontal** and **Map**, are experimental. They're off until you switch them on in [Settings](settings.md), and then join the view menu under a handwritten "experimental" label. [Experimental views](experimental-views.md) covers both.

The graph's commit count, **Kinds**, the view menu and the search share that bar with Undo, Redo, Fetch, Pull and Push, and the bar stays on one line at every width. As it narrows, the commit count moves into the search field ("Search 411 commits") and the view menu shows only its icon, then the search becomes an icon that opens the field over the bar, then Undo and Redo lose their words, then Fetch, Pull and Push. In a very narrow panel the view menu goes too; Settings and the command palette still have it. While a file is open, the bar steps aside for the file's own header.

## Right-click menus

Right-click a commit, branch or tag for what you can do with it: check it out, create a branch or tag there, merge, rebase, cherry-pick, revert, reset, or see its reflog. [Branches, merges and rebases](branches.md) covers these.

You don't have to right-click. Rest the pointer on a commit, or select one from the keyboard, and two small buttons appear at the right of its row: one copies its SHA, the other opens the same menu. <kbd>Tab</kbd> reaches them after the graph.

A commit's menu starts with what you can do with the commit itself. Below that, each branch and tag on it has its own submenu, so what you do to `main` stays apart from what you do to `origin/main`. Hover over one to open it, or use the arrow keys.

![A commit's menu, with the submenu of a branch on it open](../img/commit-menu-light.webp#gh-light-mode-only)
![A commit's menu, with the submenu of a branch on it open](../img/commit-menu-dark.webp#gh-dark-mode-only)
