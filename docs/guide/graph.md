# The commit graph

The graph is the main view of a repository: every commit, the branches and tags that point at it, and the lines connecting them.

![The commit graph of the Tidewater repository, with branches, a stash and uncommitted changes](../img/repository-light.webp#gh-light-mode-only)
![The commit graph of the Tidewater repository, with branches, a stash and uncommitted changes](../img/repository-dark.webp#gh-dark-mode-only)

It doesn't load the whole history when you open a repository. It loads the commits just ahead of where you're looking, and more as you scroll, search or jump. That's what keeps it quick on repositories with a million commits or more.

Commits are in date order, newest at the top, so work that happened side by side, such as a `gh-pages` site and `main`, is interleaved as it happened. Every commit still sits below the commits built on it.

Your **Trunk** branch (`main`, `master` or `develop`) runs straight down the left-hand column in rose, and other lines branch off it to the right. A second Trunk branch, such as `develop` beside `main`, gets the next column.

## Moving around

- Scroll, or use the arrow keys to move the selection one commit at a time.
- <kbd>Page Up</kbd> and <kbd>Page Down</kbd> move a page at a time.
- <kbd>Home</kbd> and <kbd>End</kbd> go to the newest and oldest commits.

Selecting a commit shows its details on the right: author, committer, message, [notes](history.md#notes), [signature](history.md#signatures) and the files it changed. Trailers such as `Co-authored-by` and `Closes` are listed on their own, and `#14` links to the issue on your repository's host.

![A selected commit, with its message, trailers, author, a git note and the files it changed](../img/commit-details-light.webp#gh-light-mode-only)
![A selected commit, with its message, trailers, author, a git note and the files it changed](../img/commit-details-dark.webp#gh-dark-mode-only)


## Searching

Press <kbd>Ctrl</kbd>+<kbd>F</kbd> to search. You can search by commit message, author or SHA. You can also search for a trailer, such as `Co-authored-by: Sam` or `Closes: #12`.

<kbd>Enter</kbd> goes to the next match and <kbd>Shift</kbd>+<kbd>Enter</kbd> to the previous one. Matches further back than what's loaded are found too; the graph loads up to them. [Find a commit](../walkthroughs/find-a-commit.md) shows it in action.

## Branches and tags

The sidebar has an icon rail and one panel at a time: **Branches**, **Remotes**, **Tags**, **Stashes** and **Repository**. Click an icon to switch panels, or click the active one to hide the panel and give the graph the width. Drag the panel's right edge to resize it, and double-click the edge to reset it. Git Revenge remembers all of this for each repository.

Names with a `/` form folders, so `release/v0.8` and `release/v0.7` sit in a `release` folder. **Name** or **Recent**, at the top of the panel, sorts by name or by most recently updated. The checked-out branch always comes first.

Type in **Filter all panels** to narrow the lists. The filter stays on as you switch panels, and the rail shows how many matches each panel has. **Needs care**, in the Branches panel, shows branches whose upstream is gone or whose stack parent was merged. Click a branch or tag to jump to its commit.

Local branches are grouped by [kind](branch-care.md#branch-kinds): Trunk at the top, then ordinary branches, releases, environments, docs and bots. Branches built on other branches sit under them as a [stack](branch-care.md#stacked-branches). A branch's chip in the graph carries its kind's colour and icon.

## Docs and Bots

When a repository has Docs branches (such as `gh-pages`) or Bot branches (such as `dependabot/…`), the graph's header has **Docs** and **Bots** buttons to show or hide them. Docs lines are drawn faded either way. Other branches can be hidden too; see [hiding branches](branch-care.md#hiding-branches).

## Two layouts

The default **timeline** layout gives each commit two lines: the subject, and the author and date. **Compact rows** fits one commit per line, so you see more history at once. Switch between them in [Settings](settings.md).

## Right-click menus

Right-click a commit, branch or tag for what you can do with it: check it out, create a branch or tag there, merge, rebase, cherry-pick, revert, reset, or see its reflog. [Branches, merges and rebases](branches.md) covers these.
