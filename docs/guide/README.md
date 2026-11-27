# User guide

Git Revenge is a free desktop git client for Windows, macOS and Linux. It's built to stay quick on very large repositories, and it does all its work through the `git` you already have installed, so your config, hooks, credential helpers and SSH agent all keep working.

![The Git Revenge window: the sidebar, the commit graph and the working tree's changes](../img/repository-light.webp#gh-light-mode-only)
![The Git Revenge window: the sidebar, the commit graph and the working tree's changes](../img/repository-dark.webp#gh-dark-mode-only)

## What you're getting

- **It's free.** There's no paid tier and no licence to buy, for private repositories or for work. The source is [on GitHub](https://github.com/Git-Revenge/GitRevenge) under Apache-2.0.
- **It's private.** There are no accounts, no telemetry, no cloud features and no AI. The only request the app makes on its own is a check for a newer release at startup, and you can switch that off in [Settings](settings.md). [What leaves your machine](install.md#what-leaves-your-machine) has the whole list.
- **It's fast on big repositories.** The graph loads history just ahead of where you're looking, so a repository with a million commits still opens and scrolls quickly.
- **It's your git.** Anything you do in a terminal shows up in the app within a second, and anything you do in the app is an ordinary git command you could have typed.

What it leaves out: it never talks to the site your repository is hosted on, so there are no pull requests, reviews or tickets in it. A remote on GitHub, GitLab and the like can [open its pages](remotes.md#forges-and-web-pages) in your browser, and [smart links](settings.md#links) can turn a ticket key into a link.

## Where to start

1. [Install it](install.md) and check you have a recent enough git.
2. Take [a tour of the window](tour.md), so you know what's where.
3. Work through a [walkthrough](../walkthroughs/README.md) or two. They use a practice repository you can build yourself, so you can follow along without touching real work.

After that, the guide covers each part of the app in turn:

| Page | What it covers |
|---|---|
| [Repositories and tabs](repositories.md) | Opening, cloning, tabs, groups, the dashboard and worktrees. |
| [Cloning a big repository](big-repositories.md) | Shallow, partial, one-branch and sparse clones: what each costs, This clone, where a shallow history stops, and when a file isn't here yet. |
| [The commit graph](graph.md) | Reading the graph, moving around it, selecting several commits, searching, and the sidebar. |
| [Searching the code](searching-code.md) | Find in files, and the history search: the commits that brought a text in or took it out. |
| [Experimental views](experimental-views.md) | Horizontal, with time running across, and Map, a picture of every branch around the trunk, or of everyone who wrote the commits. |
| [Changes and commits](changes.md) | Staging, committing, stashing, and undo. |
| [Ignoring files](ignoring.md) | Ignoring a file from its menu, and the Ignore rules tab. |
| [Branches, merges and rebases](branches.md) | Merging, rebasing, previews and interactive rebase. |
| [Resolving conflicts](conflicts.md) | The conflict tool, suggestions where code can be combined, resolutions git reuses (rerere), and merge drivers. |
| [Branch care](branch-care.md) | Branch kinds, hiding, cleaning up merged branches, and stacks. |
| [Patches](patches.md) | Saving commits as patches, applying a series with `git am` or a plain diff with `git apply`, previewed first, from a file, a drop or the clipboard, and saving a commit's files as a zip or tar archive. |
| [Two versions of a branch](two-versions.md) | What became of each commit after a rebase, an amend or a reset, with `git range-diff`: changed, the same, folded in, dropped or new, and each changed pair read Marked or Side by side. |
| [Reading a diff](diffs.md) | Layouts and the ribbon, code read token by token, formatting, renames and moved code. |
| [Previews and history](history.md) | Rendered previews, file history and blame, the reflog, bisect, notes and signatures. |
| [Remotes, LFS and submodules](remotes.md) | Fetch, pull and push, where pushes go, the Remotes panel, tidying, pushing to several places, and forges. |
| [Git config](git-config.md) | Editing git's own settings files. |
| [People and the mailmap](people.md) | Who a history names, joining the names that are one person, and the `.mailmap`. |
| [Smart filters](smart-filters.md) | Filtering the graph by the trailers a history already has, pinning a key to rows, and Trailer care. |
| [Insights](insights.md) | What shipped and what's stuck, for sprint reviews. |
| [Settings and shortcuts](settings.md) | The app's settings, smart links, and every keyboard shortcut. |

## The command palette

Almost everything the app can do is in the command palette. Press <kbd>Ctrl</kbd>+<kbd>K</kbd> (<kbd>⌘</kbd>+<kbd>K</kbd> on macOS) and start typing. If you're not sure where something lives, look there first.
