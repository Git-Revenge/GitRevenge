# Repositories and tabs

## Opening a repository

![The start page: Open, Clone and New repository, recent repositories, and a group](../img/start-page-light.webp#gh-light-mode-only)
![The start page: Open, Clone and New repository, recent repositories, and a group](../img/start-page-dark.webp#gh-dark-mode-only)

The start page has three ways in:

- **Open repository** picks a folder that's already a git repository (<kbd>Ctrl</kbd>+<kbd>O</kbd>).
- **Clone a repository** takes a URL and a folder. Where it clones to by default is set in [Settings](settings.md).
- **New repository** runs `git init` in a folder you choose.

Repositories you open are listed under **Recent**, so you can get back to them in one click.

### A repository that runs programs of its own

Git takes orders from a repository's own `.git/config` and hooks: a program to run whenever files are checked for changes, a filter for every file, a script on each commit. A clone never brings these along, so in a repository you cloned they are there only if you put them there. A repository that reaches you with its `.git` folder already made, in an archive or on a shared drive, has whatever its maker put there, and opening it in any git tool runs them.

So the first time you open a repository that Git Revenge didn't clone or create, it reads the repository's config and lists its hooks before running anything there. If it finds none of these, the repository opens as usual. If it finds some, it shows each one, with what it does, and asks:

- **Don't open** leaves the repository closed. Nothing in it has been run.
- **Trust and open** opens it, and remembers your answer for that repository.

It asks once, at first contact. A repository you've opened is yours from then on: a hook manager or Git LFS you set up in it later doesn't bring the question back.

What it asks about: settings that name a program (such as `core.fsmonitor`, `core.sshCommand`, `core.hooksPath`, filters, merge and diff drivers, credential helpers), settings that pull in other config files or send fetches and pushes somewhere else (`include.path`, `url.<base>.insteadOf`), and hook scripts. Git LFS's own filter and hooks aren't asked about. Repositories you had already opened before this was added are trusted as they stand.

The repositories you've opened or trusted are listed in `settings.json` under `trustedRepos`; see [where your data lives](install.md#where-your-data-lives). Remove a line to be asked again.

## Tabs

Every repository opens in its own tab, and the tabs come back when you restart the app.

| Action | Keys |
|---|---|
| Next tab | <kbd>Ctrl</kbd>+<kbd>Tab</kbd> |
| Previous tab | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>Tab</kbd> |
| Close the current tab | <kbd>Ctrl</kbd>+<kbd>W</kbd> |
| Back to the start page | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>H</kbd> |

## Groups and the dashboard

If you work across several related repositories, put them in a group. On the start page, choose **New group**, then **Add…** to add repositories to it.

Hover over a group or one of its repositories and drag its grip to change the order. From the keyboard, a group's **…** menu has **Move earlier** and **Move later**, and <kbd>Alt</kbd>+<kbd>↑</kbd> or <kbd>Alt</kbd>+<kbd>↓</kbd> moves the focused repository. A group's order is also the order **Open all** opens its tabs and its dashboard lists its repositories.

![Dragging a repository to the end of its group, then dragging the group past another](../img/reorder-light.webp#gh-light-mode-only)
![Dragging a repository to the end of its group, then dragging the group past another](../img/reorder-dark.webp#gh-dark-mode-only)

![A group's dashboard](../img/group-dashboard-light.webp#gh-light-mode-only)
![A group's dashboard](../img/group-dashboard-dark.webp#gh-dark-mode-only)

A group's **Dashboard** shows each repository's branch, how many uncommitted changes it has, and when it was last fetched. From there you can fetch or pull them all at once. Pulls here are fast-forward only: a repository whose branch has diverged is reported, never merged. Click a repository's row to open it as a tab; **Open all as tabs** opens every repository in the group. The walkthrough [Look after several repositories](../walkthroughs/several-repositories.md) goes through it step by step.

## Worktrees

A worktree is a second working folder for the same repository, so you can have two branches checked out side by side. **Manage worktrees…** in the command palette lists them, adds new ones, opens one in a tab, and removes linked ones you're done with.

The **Repository** panel of the sidebar lists them, with each one's branch, and a rose dot marks the one this window has open. Click one to open it as a tab, or right-click it to remove it.

![The Repository panel, listing two worktrees and their branches](../img/sidebar-repository-light.webp#gh-light-mode-only)
![The Repository panel, listing two worktrees and their branches](../img/sidebar-repository-dark.webp#gh-dark-mode-only)

## Changes made outside the app

Git Revenge watches each open repository. If you commit, check out or rebase in a terminal or another tool, the app picks it up within about a second, with no refresh button needed. If a merge, rebase, cherry-pick, revert or bisect is in progress, even one you started elsewhere, a strip at the top says what it is and where it stands, with **Continue**, **Skip** and **Abort**. See [While a merge or rebase is in progress](conflicts.md#while-a-merge-or-rebase-is-in-progress).
