# Branches, merges and rebases

## Branches and tags

- **New branch** (<kbd>Ctrl</kbd>+<kbd>B</kbd>, or the **+** at the top of the Branches panel in the sidebar) creates a branch from HEAD and can check it out.
- Right-click any commit for **Create branch here…** or **Create tag here…**. Give a tag a message to make it an annotated tag.
- Double-click a branch in the sidebar to check it out. Checking out a remote branch offers to create a local branch that tracks it.
- Right-click a branch to rename it, delete it, set its upstream, or delete it from the remote. Deleting a branch whose work is already on a Trunk branch or the checked-out branch just asks you to confirm, including when it was squash-merged or rebased. Deleting a branch with unmerged commits says how many and asks you to confirm first.
- **Push** in a branch's right-click menu pushes that one branch to the same place the toolbar's Push would: its upstream, or, for a new branch, the only remote or `origin`, which then becomes its upstream. It works whether or not the branch is checked out.
- **Pull** in the menu of the checked-out branch is the toolbar's Pull. **Pull** on a branch that isn't checked out brings it up to date with its upstream. You stay on your current branch and your uncommitted changes aren't touched. This only fast-forwards: if the branch has diverged from its upstream, check it out to merge or rebase. Undo moves the branch back.
- **Set upstream** suggests the remote branches git knows about, and you can also type any name. Leave it empty to stop tracking.

  ![The upstream dialog for feature/metric-units](../img/set-upstream-light.webp#gh-light-mode-only)
  ![The upstream dialog for feature/metric-units](../img/set-upstream-dark.webp#gh-dark-mode-only)

- Right-click a branch to mark it as a kind (**Mark `name` as…**), **Hide** it, or say what it's stacked on (**Stack `name` on…**). [Branch care](branch-care.md) explains these.
- Force pushing a Trunk, Release or Environment branch, and deleting a Trunk or Environment branch, ask first and name the kind.
- Force pushing any branch asks first when it would lose commits from the remote, and lists them with who wrote them, as far as your last fetch knows. Commits you rebased or amended with the same changes don't count.

## Merging, rebasing and cherry-picking

Right-click a branch or commit, and the menu offers what makes sense for it against the branch you have checked out. In the graph, the branches and tags on a commit each have a [submenu](graph.md#right-click-menus), which is where you'll find merging and rebasing:

- **Merge** *it* **into** *your branch*…
- **Rebase** *your branch* **onto** *it*…
- **Cherry-pick** *it* **onto** *your branch*.
- **Revert** a commit. Reverting a merge asks which parent to keep.

You can also drag. Drop a branch or commit onto the checked-out branch to merge, rebase or cherry-pick it in, or drag the checked-out branch onto another commit. A menu asks which you meant.

### Before a merge or rebase runs

Merge and rebase first show what they would do. Nothing has happened yet: **Cancel** leaves everything as it was, and <kbd>Enter</kbd> goes ahead.

The preview says whether files will conflict, and names them with how many places in each. It works this out in memory, without touching your files, your index or any branch. It's a forecast: the operation itself still has the last word. If the conflicts can't be checked, the preview says so and you can still go ahead.

A merge preview also lists the commits and the number of files coming in, and asks **how** to merge:

- **Fast-forward only** moves your branch forward with no merge commit. It's chosen for you when it's possible, and turned off when your branch has commits of its own.
- **Merge commit** always makes a merge commit.
- **Squash into one** stages everything the other branch changed as one change for you to commit. Nothing is committed, and the other branch isn't recorded as merged. It's turned off when files would conflict.

The button says what it will do, such as **Fast-forward main** or **Merge, and resolve 2 conflicts**.

![A merge preview that names one file that will conflict](../img/merge-preview-light.webp#gh-light-mode-only)
![A merge preview that names one file that will conflict](../img/merge-preview-dark.webp#gh-dark-mode-only)

A rebase preview lists the commits that will be written again, oldest first, and says which commit it would first stop on. Later commits can't be tried until that one is settled, so there may be more. It also offers:

- **Fold the fixups into the commits they name**, when the branch has `fixup!` or `squash!` commits. It's on to begin with.
- **Let me reorder, reword or drop commits first**, which opens the [interactive rebase editor](#interactive-rebase) instead.

If the branch is already pushed, the preview warns that it will need a force push afterwards.

Cherry-pick and revert run straight away: they're one commit, and Undo takes them back.

## Resetting

Right-click a commit and choose **Reset** to move your branch there. You pick the mode:

- **Soft:** keep changes staged.
- **Mixed:** keep changes, unstaged.
- **Hard:** discard all changes.

## Interactive rebase

Right-click a commit and choose **Interactive rebase … from here…**, or tick **Let me reorder, reword or drop commits first** in a rebase's preview. The editor lists every commit after that point, oldest first, the order git applies them in.

Each commit has a menu for what happens to it. The menu says what each choice does, and the letter beside it sets it from the keyboard when the commit has focus:

| Choice | Key | What it does |
|---|---|---|
| **Keep** | <kbd>P</kbd> | Keeps the commit as it is. |
| **Reword** | <kbd>R</kbd> | Keeps the changes and changes the message. The subject is edited in place: <kbd>Enter</kbd> keeps it, <kbd>Esc</kbd> puts the old one back, and <kbd>Ctrl</kbd>+<kbd>Enter</kbd> opens the whole message. |
| **Squash** | <kbd>S</kbd> | Folds the commit into the one above and keeps both messages, which you can edit. |
| **Fixup** | <kbd>F</kbd> | Folds the commit into the one above and discards its message. |
| **Drop** | <kbd>D</kbd> | Throws the commit's changes away. The row says which files go with it. |

A commit that folds into another sits indented under it. Drag a commit by its grip to reorder, or press <kbd>Alt</kbd>+<kbd>↑</kbd> or <kbd>Alt</kbd>+<kbd>↓</kbd> to move the focused one. When the list has `fixup!` or `squash!` commits for others in it, **Fold it in for me** moves each under the commit it names.

A panel beside the plan shows the branch as it will be afterwards: the commits that will exist, marked *folded* or *reworded*, what is dropped and how many lines go with it, and whether this order would stop on a conflict. The forecast follows the plan as you change it. If the branch is already pushed, the panel says it will need a force push.

The button says the outcome, such as **Rewrite as 3 commits**. **Reset the plan** puts every row back, and **Cancel** goes back to the graph with nothing changed. One Undo brings the whole branch back as it was.

![The interactive rebase editor](../img/rebase-editor-light.webp#gh-light-mode-only)
![The interactive rebase editor](../img/rebase-editor-dark.webp#gh-dark-mode-only)

The walkthrough [Tidy a branch before you share it](../walkthroughs/tidy-a-branch.md) shows a full clean-up.

## Conflicts

When a merge, rebase or cherry-pick stops on conflicts, the conflicted files are listed with your uncommitted changes. For each one you can:

- resolve it with **ours** or **theirs** in one click, or
- open the **conflict tool** to decide place by place.

In the conflict tool the **result** is the main view: the file as it will be saved, with a block wherever the two sides disagree. Conflict markers never appear in it and are never saved.

- Each **place** has its own choices: **Ours**, **Theirs** or **Both** (ours first). Until you choose, the place is empty and shows what each side would put there. **Start again** puts a place back to undecided.
- The result is an ordinary editor. Typing inside a place makes it **Edited**, which counts as decided; typing anywhere else just changes the file.
- Above the result, **Ours and theirs** shows both sides of the current place with a few lines around them, each labelled with its branch and the commit that last changed those lines. **Three-way** adds the **Original**, where both sides started. **Result only** hides them. **Take** on a side, or <kbd>1</kbd> and <kbd>2</kbd> (with <kbd>Alt</kbd> while typing), takes that side for the current place.
- **Next** and **Previous**, or <kbd>F7</kbd> and <kbd>Shift</kbd>+<kbd>F7</kbd>, move between places, going to the undecided ones first.
- **Mark resolved** stays off until every place is decided, and its tooltip says how many are left. It saves the result, stages the file, and opens the next conflicted file if there is one.

The panel on the right lists the files of the operation with their progress, and the files that merged without help. For the whole file it offers **Take ours** and **Take theirs**, **Start this file again**, and, when `merge.tool` is set in your git config, **Open in** that tool.

A binary file, or one a side deleted, has no lines to choose between: the tool offers the whole-file choices only.

![The conflict tool: both sides of a place above the result](../img/conflict-tool-light.webp#gh-light-mode-only)
![The conflict tool: both sides of a place above the result](../img/conflict-tool-dark.webp#gh-dark-mode-only)

See [Resolve a merge conflict](../walkthroughs/resolve-a-conflict.md) for the whole thing, step by step.

### While a merge or rebase is in progress

A strip at the top stays until the operation is finished. It names the operation and both sides, such as *Rebasing feature/offline-cache onto main*, and says where things stand: which commit a rebase stopped on and how far along it is, or how many of a merge's conflicted files are resolved.

![The strip for a merge in progress, with one file left to resolve](../img/operation-strip-light.webp#gh-light-mode-only)
![The strip for a merge in progress, with one file left to resolve](../img/operation-strip-dark.webp#gh-dark-mode-only)

- **Resolve N files** opens the first conflicted file in the conflict tool.
- **Continue** is off while anything is conflicted, and its tooltip names what's left. Once everything is resolved it says what it will do: **Continue with 2 more**, **Finish the rebase** or **Commit the merge**. <kbd>Ctrl</kbd>+<kbd>Enter</kbd> presses it.
- **Skip this commit** leaves out the commit a rebase, cherry-pick or revert stopped on.
- **Abort…** asks first, and says what it undoes: where your branch goes back to, and what is thrown away.

During a rebase the graph ticks each commit as it's written again on top of the target, and the commit in hand carries its number.

When the operation finishes, a notice says what happened and offers **Undo**, which puts the branch back where it was before the operation began. After a rebase of a branch that was already pushed, a second notice offers **Force push…**.

The strip appears for an operation started in a terminal too.

Any of these can be undone afterwards; see [Undo and redo](changes.md#undo-and-redo).
