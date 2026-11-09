# Settings and shortcuts

Open **Settings** with <kbd>Ctrl</kbd>+<kbd>,</kbd> or from the command palette.

![The Settings panel](../img/settings-light.webp#gh-light-mode-only)
![The Settings panel](../img/settings-dark.webp#gh-dark-mode-only)

| Setting | What it does |
|---|---|
| **Theme** | System, Light or Dark. |
| **Commit graph** | Timeline (two lines per commit) or Compact (one). |
| **Zoom** | Scales the whole app, from 80% to 150%. |
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

Settings are saved to `settings.json`; see [where your data lives](install.md#where-your-data-lives). Git's own settings, such as your name, email and aliases, are edited in the [Git Config tab](git-config.md): choose **Edit git config…** under Git.

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
| Commit the staged changes | <kbd>Ctrl</kbd>+<kbd>Enter</kbd> |
| Hide or show the right panel | <kbd>Ctrl</kbd>+<kbd>Alt</kbd>+<kbd>B</kbd> |
| Next or previous file, with a diff open | <kbd>Alt</kbd>+<kbd>↓</kbd> and <kbd>Alt</kbd>+<kbd>↑</kbd> |
| Fetch all remotes | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>F</kbd> |
| Pull | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>L</kbd> |
| Push the current branch | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>U</kbd> |
| Undo / redo | <kbd>Ctrl</kbd>+<kbd>Z</kbd> / <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>Z</kbd> |
| Word wrap in diffs | <kbd>Alt</kbd>+<kbd>Z</kbd> |
| Back to the graph | <kbd>Escape</kbd> |
| Next / previous tab | <kbd>Ctrl</kbd>+<kbd>Tab</kbd> / <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>Tab</kbd> |
| Go to a tab by number | <kbd>Ctrl</kbd>+<kbd>1</kbd> to <kbd>Ctrl</kbd>+<kbd>9</kbd> |
| Close the tab | <kbd>Ctrl</kbd>+<kbd>W</kbd> |
| Start page | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>H</kbd> |
| Filter the sidebar | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>B</kbd> |

In the sidebar's list, the keys are different: see [the commit graph](graph.md#branches-and-tags).

### Changing them

Choose **Customize keyboard shortcuts…** from the command palette or Settings. Every command in the palette is listed, with its keys. Record new keys for any of them, including commands that have none yet. Changes are saved as you make them.
