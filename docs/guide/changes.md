# Changes and commits

When you have uncommitted changes, a row at the top of the graph says how many files changed and on which branch. Click it (or choose **Show uncommitted changes** in the command palette) to see what's changed in your working tree.

## Staging

Your changes are one list, by filename under their folders, with line counts. A tick means the file goes in the commit.

- Tick a file to stage it, and untick it to unstage it. A dash and **partly staged** mean only some of its changes are going in, and ticking it stages the rest.
- The tick at the top of the list does every file at once. **Discard all** throws away everything unstaged, after asking.
- The line under the heading and the commit button both say what's next: tick a file, describe the commit, then **Commit 3 files to main**. While there are conflicts, the button says how many are left.
- On a filesystem that ignores letter case, such as Windows, files whose names differ only by case show as changed when nobody edited them. The panel says so and marks them **case clash**, and the tick at the top leaves them out.
- If you aren't on a branch, the panel says so and offers **Create a branch here**, since a commit made there is easy to lose.
- Hover over a file for its discard button. Conflicts come first, under **Fix these first**.
- Open a file to stage part of it. In the **Unified** diff, each hunk has a **Stage hunk** button, and you can select individual lines and stage just those. Staged changes can be unstaged the same way.

![A unified diff with Stage hunk and Discard hunk buttons](../img/diff-unified-light.webp#gh-light-mode-only)
![A unified diff with Stage hunk and Discard hunk buttons](../img/diff-unified-dark.webp#gh-dark-mode-only)

Discarding asks you to confirm first, since it throws your changes away. The walkthrough [Stage and commit](../walkthroughs/first-commit.md) goes through it from start to finish.

## Committing

Write a **Summary** (required) and, if you like, a longer **Description**. Keep the summary under about 72 characters; the app warns you when it's longer. Press <kbd>Ctrl</kbd>+<kbd>Enter</kbd> to commit.

You can also amend the last commit.

If your git config signs commits (`commit.gpgsign`), commits made in Git Revenge are signed too, with GPG or SSH as your config says.

## Stashing

**Stash…** sets your uncommitted changes aside, with a message if you want one. Stashes are listed in the **Stashes** panel of the sidebar. Right-click one to **Apply**, **Pop** or **Drop** it.

## Undo and redo

Commits, checkouts, merges, cherry-picks, rebases, resets, reverts and pulls can be undone. Press <kbd>Ctrl</kbd>+<kbd>Z</kbd> to undo and <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>Z</kbd> to redo. The command palette names what the next undo will reverse, such as *Undo: Commit "Fix the login form"*.

Undo is only offered while the repository is exactly where that operation left it. If anything has moved since, whether in the app or in a terminal, it refuses rather than guess. Nothing is lost either way: the [reflog](history.md#the-reflog) keeps every earlier state.
