# Undo a mistake

Emily meant to look at `feature/offline-cache`, and merged it into `main` instead. One keystroke takes it back.

![Merging a branch by mistake, then pressing Ctrl+Z to undo it](../img/undo-light.webp#gh-light-mode-only)
![Merging a branch by mistake, then pressing Ctrl+Z to undo it](../img/undo-dark.webp#gh-dark-mode-only)

## Undo and redo

Press <kbd>Ctrl</kbd>+<kbd>Z</kbd> to undo the last operation, and <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>Z</kbd> to redo it. The Undo button in the toolbar, and the command palette, name what will be undone (*Undo: Merge feature/offline-cache*) so you can check before you press.

You can undo commits, checkouts, merges, cherry-picks, rebases, resets, reverts and pulls, and several in a row.

Undoing a commit keeps its changes staged, so you can fix them and commit again.

### When undo says no

Undo only acts while the repository is exactly where that operation left it. If something has moved since, such as another commit or a checkout in a terminal, it refuses rather than guess, and says so. Nothing is lost when that happens: the reflog still has every earlier state.

## Finding lost work with the reflog

Git keeps a record of everywhere HEAD and each branch have been, called the *reflog*. A commit you reset away, or rewrote in a rebase, is still in there for weeks afterwards.

1. Open the command palette (<kbd>Ctrl</kbd>+<kbd>K</kbd>) and choose **Show the reflog…**. To see one branch's history instead, right-click the branch and choose **Reflog of …**.

   ![The reflog: each move of HEAD, newest first, with Show and New branch buttons](../img/reflog-light.webp#gh-light-mode-only)
   ![The reflog: each move of HEAD, newest first, with Show and New branch buttons](../img/reflog-dark.webp#gh-dark-mode-only)

2. Find the entry from just before things went wrong. Each line says what happened (*commit*, *checkout*, *reset*, *rebase*) and when.

3. **Show** jumps to that commit in the graph. **New branch…** puts a branch there, which brings the work back for good.

## Good to know

- **Discarded changes are different.** Changes you discard from the working tree were never committed, so neither undo nor the reflog can bring them back. That's why discarding asks first.
- **Stashes are safer than discarding.** If you're not sure you'll need some changes, **Stash** them instead. They're listed in the sidebar until you apply or drop them.
