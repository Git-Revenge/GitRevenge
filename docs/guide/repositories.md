# Repositories and tabs

## Opening a repository

![The start page: Open, Clone and New repository, recent repositories, and a group](../img/start-page-light.webp#gh-light-mode-only)
![The start page: Open, Clone and New repository, recent repositories, and a group](../img/start-page-dark.webp#gh-dark-mode-only)

The start page has three ways in:

- **Open repository** picks a folder that's already a git repository (<kbd>Ctrl</kbd>+<kbd>O</kbd>).
- **Clone a repository** takes a URL and a folder. Where it clones to by default is set in [Settings](settings.md).
- **New repository** runs `git init` in a folder you choose.

Repositories you open are listed under **Recent**, so you can get back to them in one click.

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

![A group's dashboard](../img/group-dashboard-light.webp#gh-light-mode-only)
![A group's dashboard](../img/group-dashboard-dark.webp#gh-dark-mode-only)

A group's **Dashboard** shows each repository's branch, how many uncommitted changes it has, and when it was last fetched. From there you can fetch or pull them all at once. Pulls here are fast-forward only: a repository whose branch has diverged is reported, never merged. **Open all as tabs** opens every repository in the group. The walkthrough [Look after several repositories](../walkthroughs/several-repositories.md) goes through it step by step.

## Worktrees

A worktree is a second working folder for the same repository, so you can have two branches checked out side by side. **Manage worktrees…** in the command palette lists them, adds new ones, opens one in a tab, and removes linked ones you're done with.

The **Repository** panel of the sidebar lists them, with each one's branch, and a rose dot marks the one this window has open. Click one to open it as a tab, or right-click it to remove it.

## Changes made outside the app

Git Revenge watches each open repository. If you commit, check out or rebase in a terminal or another tool, the app picks it up within about a second, with no refresh button needed. If a merge, rebase, cherry-pick, revert or bisect is in progress, even one you started elsewhere, a banner offers **Continue**, **Abort** and **Skip**.
