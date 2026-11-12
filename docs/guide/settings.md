# Settings and shortcuts

Open **Settings** with <kbd>Ctrl</kbd>+<kbd>,</kbd> or from the command palette.

![The Settings panel](../img/settings-light.webp#gh-light-mode-only)
![The Settings panel](../img/settings-dark.webp#gh-dark-mode-only)

| Setting | What it does |
|---|---|
| **Theme** | System, Light or Dark. Switching cross-fades the window. |
| **Commit graph** | Timeline (a line for each day, and authors beside their commits) or Compact (one commit per line). |
| **Zoom** | Scales the whole app, from 80% to 150%. |
| **Experimental views** | A checkbox each for [Horizontal and Map](experimental-views.md). A view you switch on joins the graph's view menu, and nothing changes until you pick it there. Switching one off takes the graph back to Timeline or Compact. These can change or go away between versions. |
| **Git executable** | Which `git` to use. Leave it empty to find it on your `PATH`. A change takes effect after a restart. |
| **Clone into** | Where new clones go. Empty means the folder of the last repository you opened. |
| **Updates** | Whether to check GitHub for a newer version at startup. This is the only request the app makes on its own. |

With a repository's tab active, a **This repository** section adds settings that belong to it:

| Setting | What it does |
|---|---|
| **Trunk, Release, Environment, Docs, Feature, Bot** | The patterns that decide each [branch kind](branch-care.md#branch-kinds). **Use defaults** puts one back. |
| **Clean up merged branches after each pull** | Deletes local branches whose upstream was deleted and that have nothing unpushed. [More](branch-care.md#cleaning-up-merged-branches). |
| **Sync notes on fetch, pull and push** | Shares git notes with the remote. [More](branch-care.md#syncing-notes). |
| **Hidden branches** | Patterns of branches to leave out of the sidebar and graph. [More](branch-care.md#hiding-branches). |

The settings under **This repository** are kept in the repository's git config, under `gitrevenge` keys, where the [Git Config tab](git-config.md) shows them with the rest; set them in your global config to make them your defaults everywhere. The app's own settings are saved to `settings.json`; see [where your data lives](install.md#where-your-data-lives). Git's own settings, such as your name, email and aliases, are edited in the [Git Config tab](git-config.md): choose **Edit git config…** under Git.

## Links

Smart links turn ticket keys and web addresses into links, wherever they turn up in commit messages, trailers and branch names. A link opens in your browser. Git Revenge only builds the address and never contacts the site.

![Settings › Links, with a link for numbered issues](../img/links-light.webp#gh-light-mode-only)
![Settings › Links, with a link for numbered issues](../img/links-dark.webp#gh-dark-mode-only)

**Web addresses** written out in full (`https://…` and `www.…`) are built in, and have a switch. Everything else is a link you add.

**Add a link…** starts from a preset:

| Preset | What it asks for |
|---|---|
| **Jira** | Your Jira address and, if you like, the project keys to match. |
| **Numbered (#123)** | The address a number goes to, with `{1}` where the number belongs. |
| **GitHub issues**, **GitLab issues** | The repository or project. **Fill in from the remote** reads it from the repository you have open. |
| **Linear** | Your workspace. |
| **Azure Boards** | Your organization and project. |

**My own pattern** takes anything else: a regular expression, and the address it opens, where `{0}` is the whole match and `{1}` the first group. Only `http` and `https` addresses are allowed.

For each link you also choose:

- **Look in:** commit messages, trailers, branch names, or any of the three.
- **Counts as a reference in Insights › Shipped:** [Shipped](insights.md#shipped) then groups items by what this link finds.
- **Save in:** your global git config, for every repository, or this repository's own config.

**Try it** shows the link at work on text you type, and says how many of the repository's recent commits match.

A bare `#123` is never linked on its own, because a number means different things in different places. Add a **Numbered (#123)** link to say where numbers go.

In the list, each link has a switch, arrows to change the order, **Edit** and **Delete**, and a button to copy it between your global config and the repository's. A repository's link replaces a global one with the same name. **Export** and **Import…** move the whole set as a file.

Links are kept in git config, in sections named `[gitrevenge-link "name"]`, so the [Git Config tab](git-config.md) shows them too.

When a branch's name holds a ticket key, the branch's right-click menu starts with the link to it.

## The command palette

<kbd>Ctrl</kbd>+<kbd>K</kbd> opens the command palette: every command in the app, searchable, with its shortcut if it has one. Type a few letters of what you want and press <kbd>Enter</kbd>.

![The command palette, filtered to pull commands](../img/command-palette-light.webp#gh-light-mode-only)
![The command palette, filtered to pull commands](../img/command-palette-dark.webp#gh-dark-mode-only)

## Keyboard shortcuts

On macOS, read <kbd>⌘</kbd> wherever this page says <kbd>Ctrl</kbd>. Press <kbd>Ctrl</kbd>+<kbd>/</kbd> in the app to see this list, including any keys you've changed.

| Action | Keys |
|---|---|
| Command palette | <kbd>Ctrl</kbd>+<kbd>K</kbd> |
| Keyboard shortcuts | <kbd>Ctrl</kbd>+<kbd>/</kbd> |
| Settings | <kbd>Ctrl</kbd>+<kbd>,</kbd> |
| Open repository | <kbd>Ctrl</kbd>+<kbd>O</kbd> |
| New branch | <kbd>Ctrl</kbd>+<kbd>B</kbd> |
| Search commits | <kbd>Ctrl</kbd>+<kbd>F</kbd> |
| Next or previous search match | <kbd>Enter</kbd> / <kbd>Shift</kbd>+<kbd>Enter</kbd> |
| Move the selection | <kbd>↑</kbd> / <kbd>↓</kbd> |
| Move a page | <kbd>Page Up</kbd> / <kbd>Page Down</kbd> |
| First or last commit | <kbd>Home</kbd> / <kbd>End</kbd> |
| The checked-out commit | <kbd>Alt</kbd>+<kbd>Home</kbd> |
| Commit the staged changes | <kbd>Ctrl</kbd>+<kbd>Enter</kbd> |
| Hide or show the right panel | <kbd>Ctrl</kbd>+<kbd>Alt</kbd>+<kbd>B</kbd> |
| Next or previous file, with a diff open | <kbd>Alt</kbd>+<kbd>↓</kbd> and <kbd>Alt</kbd>+<kbd>↑</kbd> |
| Fetch all remotes | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>F</kbd> |
| Pull | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>L</kbd> |
| Push the current branch | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>U</kbd> |
| Undo / redo | <kbd>Ctrl</kbd>+<kbd>Z</kbd> / <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>Z</kbd> |
| Word wrap in diffs | <kbd>Alt</kbd>+<kbd>Z</kbd> |
| Next or previous place, in the conflict tool | <kbd>F7</kbd> / <kbd>Shift</kbd>+<kbd>F7</kbd> |
| Good, bad or can't tell, during a bisect | <kbd>G</kbd> / <kbd>B</kbd> / <kbd>S</kbd> |
| Back to the graph | <kbd>Escape</kbd> |
| Next / previous tab | <kbd>Ctrl</kbd>+<kbd>Tab</kbd> / <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>Tab</kbd> |
| Go to a tab by number | <kbd>Ctrl</kbd>+<kbd>1</kbd> to <kbd>Ctrl</kbd>+<kbd>9</kbd> |
| Close the tab | <kbd>Ctrl</kbd>+<kbd>W</kbd> |
| Start page | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>H</kbd> |
| Filter the sidebar | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>B</kbd> |

In the sidebar's list, the keys are different: see [the commit graph](graph.md#branches-and-tags). The [interactive rebase editor](branches.md#interactive-rebase) has a letter for each choice.

### Changing them

Choose **Customize keyboard shortcuts…** from the command palette or Settings. Every command in the palette is listed, with its keys. Record new keys for any of them, including commands that have none yet. Changes are saved as you make them.
