# Tidy a branch before you share it

Priya's `feature/offline-cache` branch works, but its history tells the story of getting there: a commit called *wip*, a *fixup!* for an earlier commit, and a *debugging* commit that should never have been committed. Before she opens a pull request, she wants two clean commits.

That's what interactive rebase is for: it rewrites a branch's recent commits, letting you squash, reword, reorder and drop them.

![Interactive rebase: dropping a commit, marking a fixup and a squash, then running it](../img/rebase-light.webp#gh-light-mode-only)
![Interactive rebase: dropping a commit, marking a fixup and a squash, then running it](../img/rebase-dark.webp#gh-dark-mode-only)

## Steps

1. **Check out the branch.** In the sidebar, open the `feature` folder and double-click `offline-cache`.

2. **Open the rebase editor.** In the graph, right-click the commit the branch started from (here, `main`'s *Set the alert limit in knots too*) and choose **Interactive rebase feature/offline-cache from here…**. The editor lists every commit after that one, oldest first, the order git applies them in.

   ![The interactive rebase editor listing five commits](../img/rebase-editor-light.webp#gh-light-mode-only)
   ![The interactive rebase editor listing five commits](../img/rebase-editor-dark.webp#gh-dark-mode-only)

3. **Choose what happens to each commit.** Each commit has a menu:

   | Choice | What it does |
   |---|---|
   | **Keep** | Keep the commit as it is. |
   | **Reword** | Keep it, with a new message. The subject becomes a field you type in. |
   | **Squash** | Fold it into the commit above it, and combine both messages. |
   | **Fixup** | Fold it into the commit above it, and keep only that commit's message. |
   | **Drop** | Remove the commit and its changes. |

   For Priya's branch: **Drop** *debugging*, click **Fold it in for me** to put the *fixup!* commit under the commit it names, and **Squash** *wip*. Squash starts the message from both commits, the way git does; **Edit the message** opens it.

   Drag a commit by the grip on its left to reorder, or press <kbd>Alt</kbd>+<kbd>↑</kbd> and <kbd>Alt</kbd>+<kbd>↓</kbd>.

   The panel on the right shows the branch as it will be afterwards, and says whether this order would stop on a conflict.

4. **Run it.** Click **Rewrite as 2 commits**. Git Revenge rewrites the branch, and the graph shows the result: two commits where there were five.

## If it stops partway

A rebase stops if a commit doesn't apply cleanly. A strip at the top says which commit it stopped on, with **Continue**, **Skip this commit** and **Abort…**. Fix the conflict as in [Resolve a merge conflict](resolve-a-conflict.md), then **Continue**. **Abort** puts the branch back exactly as it was.

## Good to know

- **It's undoable.** <kbd>Ctrl</kbd>+<kbd>Z</kbd> puts the branch back the way it was before the rebase.
- **Only rewrite what you haven't shared.** Rebasing gives commits new SHAs. If others have already pulled the branch, they'll have to reconcile their copy with yours, so tidy up before you push, or warn them.
- **Pushing afterwards.** A branch that's already on the remote needs a force push once it's rewritten. Use **Force push (with lease)** from the arrow next to **Push**. *With lease* means it refuses if someone else has pushed to the branch since you last fetched, so you can't overwrite their work by accident.
