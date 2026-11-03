# Stage and commit

You've changed a file, and want to commit some of it now and keep the rest for later. Here, Maya has reworded the wind alert in `src/alerts.js` and added a small helper in `src/units.js`, which is already staged.

![Opening a changed file, staging a hunk, and committing it](../img/stage-and-commit-light.webp#gh-light-mode-only)
![Opening a changed file, staging a hunk, and committing it](../img/stage-and-commit-dark.webp#gh-dark-mode-only)

## Steps

1. **See what's changed.** Click the row at the top of the graph that says how many files changed. The panel on the right lists your changes. A tick means a file is going in the next commit, and a dash means part of it is.

2. **Open the file.** Click `src/alerts.js`. Its diff opens in the middle of the window, in the **Unified** layout.

   ![The diff of src/alerts.js, with Stage hunk and Discard hunk buttons](../img/diff-unified-light.webp#gh-light-mode-only)
   ![The diff of src/alerts.js, with Stage hunk and Discard hunk buttons](../img/diff-unified-dark.webp#gh-dark-mode-only)

3. **Stage what you want.** Each block of changes (a *hunk*) has a **Stage hunk** button. To stage only some lines of a hunk, select them first. Anything you don't stage stays in your working tree, untouched.

   To stage a whole file, use the **+** next to it in the panel. **Stage all** stages everything.

4. **Go back to the graph.** Press <kbd>Escape</kbd>.

5. **Write the message.** Type a summary in **Summary**. Keep it under about 72 characters; the counter next to it changes colour when it's longer. Add a **Description** if the change needs explaining.

6. **Commit.** Press <kbd>Ctrl</kbd>+<kbd>Enter</kbd>, or click **Commit**. The new commit appears at the top of the graph.

## Good to know

- **Changed your mind?** <kbd>Ctrl</kbd>+<kbd>Z</kbd> undoes the commit and leaves its changes staged, so nothing is lost. See [Undo a mistake](undo-a-mistake.md).
- **Forgot something?** Stage it, tick **Amend last commit**, and commit again. The last commit is replaced, message included, so you can fix that too.
- **Discarding is permanent.** **Discard hunk** and the discard buttons throw changes away, so each asks you to confirm first.
- **Signed commits.** If your git config signs commits, so does Git Revenge. Nothing to set up.
