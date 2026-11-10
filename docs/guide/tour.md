# A tour of the window

Open a repository and this is what you get. Everything in the screenshots comes from Tidewater, a small made-up project you can [build yourself](../walkthroughs/README.md#a-practice-repository) to follow along.

![The Git Revenge window, with numbered markers on each part](../img/repository-annotated-light.webp#gh-light-mode-only)
![The Git Revenge window, with numbered markers on each part](../img/repository-annotated-dark.webp#gh-dark-mode-only)

1. **Tabs.** One per open repository, each showing the branch it's on. The flag at the far left goes back to the start page.
2. **Sidebar.** An icon rail switches between branches, remotes, tags, stashes and the repository's LFS and submodules. The arrow next to `main` says it's one commit behind `origin/main`. Type in the box at the top to filter the lists. Right-click anything for what you can do with it.
3. **Toolbar.** Undo and redo, then fetch, pull and push. The arrows next to Pull and Push hold the less common variants. The graph's commit count, search and layout switch share the bar, and the button at its right end hides the panel on the right.
4. **Working tree.** The graph's first row, above the newest commit. It says whether you have uncommitted changes. Click it to see them on the right.
5. **The commit graph.** Every commit, newest first, with branch and tag labels and the lines between them. Click a commit to see it on the right. Right-click for everything else: branch, merge, rebase, cherry-pick, revert, reset. [More about the graph](graph.md).
6. **The right-hand panel.** Your uncommitted changes, or the commit you selected: its message, author, notes, signature and the files it changed. Click a file to see its diff.
7. **Commit message.** Where you write the summary and description, then commit with <kbd>Ctrl</kbd>+<kbd>Enter</kbd>.
8. **Command palette.** Every command, searchable. <kbd>Ctrl</kbd>+<kbd>K</kbd> opens it from anywhere. The gear next to it opens Settings, and the moon or sun switches between dark and light.

The strip along the bottom shows the git and git-lfs versions in use, and which version of Git Revenge you're running. Click the version, or the heart beside it, for the About dialog: the project's website, how to sponsor it, and the open source projects it's built with.

## The sidebar

The rail down the left edge switches the sidebar between its panels, and the filter stays on as you move between them, with a count on each panel that has a match. A rose dot on a panel means something in it needs attention.

![Switching between the sidebar panels, filtering across them, and showing the branches that need care](../img/sidebar-panels-light.webp#gh-light-mode-only)
![Switching between the sidebar panels, filtering across them, and showing the branches that need care](../img/sidebar-panels-dark.webp#gh-dark-mode-only)

[The commit graph](graph.md#branches-and-tags) covers the sidebar in full.

## Getting around without the mouse

The command palette is the quickest way to anything. Open it, type a few letters of what you want, and press <kbd>Enter</kbd>. Each command shows its keyboard shortcut, if it has one, so you pick those up as you go.

![Searching the command palette for "pull reb", then running Fetch](../img/palette-light.webp#gh-light-mode-only)
![Searching the command palette for "pull reb", then running Fetch](../img/palette-dark.webp#gh-dark-mode-only)

<kbd>Ctrl</kbd>+<kbd>/</kbd> lists every shortcut. On macOS, read <kbd>⌘</kbd> wherever these docs say <kbd>Ctrl</kbd>.

## Next

The [walkthroughs](../walkthroughs/README.md) take you through common jobs from start to finish. If you'd rather explore, the rest of the guide covers each part of the app in turn, starting with [repositories and tabs](repositories.md).
