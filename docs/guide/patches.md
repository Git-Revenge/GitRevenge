# Patches

A patch is a commit written out as text: its author, date and message, then its diff. Some projects take changes as patches by mail, and some people pass work between machines or repositories that way. Git Revenge saves commits as patches with `git format-patch`, applies them as commits with `git am`, and applies a plain diff to your files with `git apply`. Each one is shown before anything changes.

The examples use the [practice repository](../walkthroughs/README.md). `feature/reading-times` has three commits by Jessica Nguyen that aren't on `main` yet.

## Saving commits as patches

Commits are saved from where you see them:

- a commit's menu has **Save as patches, up to here…**;
- a branch's menu has **Save feature/reading-times as patches…**, starting where the branch left its base: the branch below it in a [stack](branch-care.md#stacked-branches), or else its remote's default branch;
- [commits selected together](graph.md#selecting-several-commits) have **Save as patches…** on their menu and on the panel beside the graph.

![Save as patches for feature/reading-times: three commits, one file each, numbered, with the subject they make](../img/save-patches-light.webp#gh-light-mode-only)
![Save as patches for feature/reading-times: three commits, one file each, numbered, with the subject they make](../img/save-patches-dark.webp#gh-dark-mode-only)

The dialog lists the commits it will save, oldest first, before anything is written. format-patch leaves merges out, so the dialog says how many it leaves out. Every choice is one of format-patch's, and every default is what format-patch does when you choose nothing:

- **Commits:** from a commit's or a branch's menu, **Since** says where the series starts: where the branch left its base, the last few commits, or another branch, tag or commit.
- **Save as:** one file per commit (`0001-…patch`, `0002-…`), or one mailbox file (`--stdout`).
- **Into folder:** the dialog asks for the folder each time, starting where you saved last. A file already there is replaced only when you say so.
- **Subject:** a prefix such as `RFC PATCH` (`--subject-prefix`), a version such as v2 (`-v`), and numbering (`-n`). The subject they make shows as you type, such as `[RFC PATCH v2 1/3] Keep the time each reading was taken`.
- **A cover letter first** (`--cover-letter`): a message numbered 0 that says what the series is for. Git writes it with placeholders; the app fills in the subject and text you type.
- **Add Signed-off-by** (`-s`), remembered for each repository.
- **Say which commit they apply to** (`--base`): a `base-commit:` line, so whoever applies them knows where you started.

When it's done, **Show in folder** opens the folder.

**Copy as a patch**, on a commit's menu, puts its patch on the clipboard, ready to paste into a mail or a chat. **Copy as patches** does the same for commits selected together.

In a [shallow clone](big-repositories.md#where-a-shallow-history-stops), the oldest commit the clone has can't be saved: its parent isn't there, so its patch would add every file. The dialog says so. In a [partial clone](big-repositories.md#when-a-file-isnt-here-yet), one commit's files download by themselves. A series says how many files it needs, and Save downloads them first, in one download.

## Applying a series

**Apply patches or a diff…**, in the [command palette](README.md#the-command-palette), asks for a file. [Dropping files](#dropping-a-file-and-the-clipboard) on the window, or using the clipboard, opens the same preview. Patches come as format-patch writes them, as a mailbox saved from a mailing list's archive, or as a pull request's `.patch` page.

![A series of three patches previewed against main: the first conflicts in src/feed.js, and the other two wait for it](../img/apply-series-light.webp#gh-light-mode-only)
![A series of three patches previewed against main: the first conflicts in src/feed.js, and the other two wait for it](../img/apply-series-dark.webp#gh-dark-mode-only)

The preview tries each patch, in order, on a throwaway copy of the index. Your files and what you've staged don't change. Each row fills in as its try finishes:

- **applies**: it fits as it is;
- **applies by merging**: it only fits by a three-way merge, which comes out clean;
- **conflicts in** the files named: you'll resolve them when it stops there, and the patches after it say **after you resolve** it;
- **already here as** the commit it came in as: the change is in already, so the row starts unticked;
- **can't apply**: none of those. Hover over it for git's reason.

Untick a patch to leave it out, and the ones after it are tried again without it.

**Threads and versions.** A mailbox saved from a list's archive is a thread. Its replies are left out and counted, numbered patches go in their [n/m] order whatever order the thread has them in, and a cover letter is shown above the patches, not applied. When the thread holds a v1 and a v2, the preview starts on the newest and lists the others to choose from.

**Onto** the branch you're on, or **a new branch from here**, named after the series, so the patches can be tried without touching your branch. **Add Signed-off-by** adds your own. Applying runs `git am --3way` with the ticked patches, keeping each one's author, date and message. `git am` needs nothing staged: with anything staged, the preview says so and waits.

### When a patch stops

A conflict stops `git am` at that patch. The strip over the graph takes over, as it does for a [rebase](branches.md), whether you started from the app or a terminal:

![The strip stopped at patch 1 of 3, Keep the time each reading was taken, by Jessica Nguyen](../img/apply-strip-light.webp#gh-light-mode-only)
![The strip stopped at patch 1 of 3, Keep the time each reading was taken, by Jessica Nguyen](../img/apply-strip-dark.webp#gh-dark-mode-only)

- It says "Applying patches onto main", with a bead for each patch, and names the one it stopped at and who wrote it: "Stopped at patch 1 of 3: Keep the time each reading was taken · Jessica Nguyen".
- **Resolve 1 file** opens the [conflict tool](conflicts.md) on the files. Once they're resolved and staged, **Continue with 2 more** commits the patch with its own author and message and carries on. On the last patch it says **Finish applying**.
- **Skip this patch** leaves it out.
- **Abort…** says first what it undoes: the branch goes back to where it was before the first patch. A branch the preview made for the series goes too, as long as nothing else is on it.

Sometimes a patch can't be merged at all, because a file it changes isn't here, or the versions it was made from aren't. Then nothing is in conflict, and the strip says the patch **didn't apply**, quoting git on which file didn't match. **Show the patch** opens it to read. Skip it, abort, or make the change by hand and stage it, and Continue comes back.

At the end it counts them: "Applied 2 patches onto main; skipped 1."

## Applying a plain diff

A plain diff is a diff with no mail around it: `git diff` output, a `.diff` file, or a stash shown with `-p`. It has no author or message, so it isn't applied as a commit. Its changes land in [Changes](changes.md), to look at, edit and commit as your own. It opens from the same **Apply patches or a diff…**, and is applied with `git apply`.

![A diff of feature/offline-cache previewed: src/cache.js is a new file, and src/readings.js applies](../img/apply-diff-light.webp#gh-light-mode-only)
![A diff of feature/offline-cache previewed: src/cache.js is a new file, and src/readings.js applies](../img/apply-diff-dark.webp#gh-dark-mode-only)

The preview tries each file first and says how it goes, with its + and − counts: **applies**, **applies by merging**, **a new file**, **deleted**, **merges with a conflict**, **in already** or **doesn't fit**. `git apply` takes a diff whole or not at all, so a file that doesn't fit, or is in already, can't be ticked.

Merging needs the versions the diff was made from. `git diff` names them on its `index` lines. A diff from `diff -u` doesn't, so it applies only where it fits exactly, and the preview says so.

- **Apply to** the files, as `git apply` does; the files, and stage them (`--index`); or **staged only**, leaving the files as they are (`--cached`). Staged only can't hold a conflict, so it refuses a file that would merge with one.
- **In reverse** (`-R`) takes a diff back out. Each row then says what that does: deleted, comes back, renamed back, or **not here to take out**.

**Only onto clean files.** A file with changes of yours, staged or not, says **has your changes**, and Apply waits until you commit them, stash them or leave that file out. That's what makes undoing it exact.

**Afterwards** the app says "Applied the diff to 2 files", with **Undo**. Undo puts those files back as they were, which works as long as nothing has changed them since, a commit included. If merging left a conflict, it shows in Changes with the usual conflict view, and Undo reads **Put these files back**.

## Dropping a file, and the clipboard

Drag patches or a diff from your file manager onto the window. While you hold them over it, it says what letting go does: preview them for the repository on screen, on its branch. Nothing changes until you apply. Dropped, they open the same preview as **Apply patches or a diff…**.

- Several files dropped together are one series, in name order (`0001-`, `0002-`…), or one diff.
- A file that's neither patches nor a diff says so, naming it, and nothing opens.
- With no repository on screen, it says to open one, or switch to one's tab.

**Apply the patch on the clipboard**, in the command palette, opens the clipboard's text the same way. It's the pair to **Copy as a patch**, and takes a patch copied from a mail or a web page too.

## What stays on the command line

Git Revenge doesn't send mail. Saving patches makes the files a mailing list wants, and from there the tools built for it take over:

- [`git send-email`](https://git-scm.com/docs/git-send-email) sends the saved files, threaded under the cover letter.
- [b4](https://b4.docs.kernel.org/) fetches a series from a list's archive, as a mailbox to apply here, and prepares and sends your own.

A series fetched with b4, or saved from any archive, opens in the preview like any other mailbox.
