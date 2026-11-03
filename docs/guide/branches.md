# Branches, merges and rebases

## Branches and tags

- **New branch** (<kbd>Ctrl</kbd>+<kbd>B</kbd>, or the **+** at the top of the Branches panel in the sidebar) creates a branch from HEAD and can check it out.
- Right-click any commit for **Create branch here…** or **Create tag here…**. Give a tag a message to make it an annotated tag.
- Double-click a branch in the sidebar to check it out. Checking out a remote branch offers to create a local branch that tracks it.
- Right-click a branch to rename it, delete it, set its upstream, push it, or delete it from the remote. Deleting a branch whose work is already on a Trunk branch or the checked-out branch just asks you to confirm, including when it was squash-merged or rebased. Deleting a branch with unmerged commits says how many and asks you to confirm first.
- **Pull** on a branch that isn't checked out brings it up to date with its upstream. You stay on your current branch and your uncommitted changes aren't touched. This only fast-forwards: if the branch has diverged from its upstream, check it out to merge or rebase. Undo moves the branch back.
- **Set upstream** suggests the remote branches git knows about, and you can also type any name. Leave it empty to stop tracking.

  ![The upstream dialog for feature/metric-units](../img/set-upstream-light.webp#gh-light-mode-only)
  ![The upstream dialog for feature/metric-units](../img/set-upstream-dark.webp#gh-dark-mode-only)

- Right-click a branch to mark it as a kind (**Mark `name` as…**), **Hide** it, or say what it's stacked on (**Stack `name` on…**). [Branch care](branch-care.md) explains these.
- Force pushing a Trunk, Release or Environment branch, and deleting a Trunk or Environment branch, ask first and name the kind.
- Force pushing any branch asks first when it would replace commits on the remote, and lists them with who wrote them, as far as your last fetch knows.

## Merging, rebasing and cherry-picking

Right-click a branch or commit, and the menu offers what makes sense for it against the branch you have checked out. In the graph, the branches and tags on a commit each have a [submenu](graph.md#right-click-menus), which is where you'll find merging and rebasing:

- **Merge** *it* **into** *your branch*, with or without a fast-forward.
- **Rebase** *your branch* **onto** *it*.
- **Cherry-pick** *it* **onto** *your branch*.
- **Revert** a commit. Reverting a merge asks which parent to keep.

You can also drag. Drop a branch or commit onto the checked-out branch to merge, rebase or cherry-pick it in, or drag the checked-out branch onto another commit. A menu asks which you meant.

## Resetting

Right-click a commit and choose **Reset** to move your branch there. You pick the mode:

- **Soft:** keep changes staged.
- **Mixed:** keep changes, unstaged.
- **Hard:** discard all changes.

## Interactive rebase

Right-click a commit and choose **Interactive rebase … from here…**. The editor lists every commit after it, and for each you can **pick**, **reword**, **squash**, **fixup** or **drop** it, and move it up or down. Squashing starts from both messages, as git does. **Start rebase** runs it; **Cancel** goes back to the graph with nothing changed.

![The interactive rebase editor](../img/rebase-editor-light.webp#gh-light-mode-only)
![The interactive rebase editor](../img/rebase-editor-dark.webp#gh-dark-mode-only)

The walkthrough [Tidy a branch before you share it](../walkthroughs/tidy-a-branch.md) shows a full clean-up.

## Conflicts

When a merge, rebase or cherry-pick stops on conflicts, the conflicted files are listed with your uncommitted changes. For each one you can:

- resolve it with **ours** or **theirs** in one click, or
- open the **conflict tool**, which shows ours, theirs and an editable result side by side. Take either side for each conflict, or edit the result by hand, then choose **Mark resolved**.

![The conflict tool: ours and theirs side by side, and the editable result](../img/conflict-tool-light.webp#gh-light-mode-only)
![The conflict tool: ours and theirs side by side, and the editable result](../img/conflict-tool-dark.webp#gh-dark-mode-only)

A banner at the top offers **Continue**, **Abort** and **Skip** until the operation is finished. See [Resolve a merge conflict](../walkthroughs/resolve-a-conflict.md) for the whole thing, step by step.

Any of these can be undone afterwards; see [Undo and redo](changes.md#undo-and-redo).
