# Two versions of a branch

A rebase, an amend or a reset gives a branch new commits. Their ids change even when their content doesn't, so comparing the branch's tip before and after shows the changes from everything it was rebased onto as well as yours. **Compare two versions** asks a different question: what became of each commit? It pairs every old commit with the new one it became, with `git range-diff`, and says which changed, which stayed the same, and which were dropped, folded in or added.

It helps before you force-push a branch you've rebased, when you review a series someone has sent again as v2, and when you want to know whether a rebase that ran on its own left anything behind.

The examples use the [practice repository](../walkthroughs/README.md). `feature/offline-cache` has five commits, one of them a `fixup!` for the first. It was rebased onto `main` with the fixup folded in (`git rebase -i --autosquash main`), so it now has four.

## Where it starts

The old version is usually one you've just left, so most ways in fill both versions in for you:

- **What changed?** on the message after a rebase from the app: from a rebase's preview, the interactive rebase editor, the strip once a stopped rebase finishes, and **Restack**.
- **Compare with an earlier version ▸** in a branch's menu. It lists the branch's last tips from before a rebase, an amend or a reset, from its reflog, each named and dated by what moved it ("Before the rebase · 4 min ago"). It also lists the branch's upstream when they've moved apart, and **Pick two versions…**. When there's nothing to compare, the item stays, greyed, and says why.
- **Compare first**, beside **Force push…**, when the remote still has the commits from before a rebase. It compares what the remote has with what you'd push.
- **Compare with now** on a branch's entry in [the reflog](history.md#the-reflog).
- **Compare two versions of a branch…** in the command palette.

![feature/offline-cache's menu at Compare with an earlier version: Before the rebase, just now, and Pick two versions…](../img/versions-menu-light.webp#gh-light-mode-only)
![feature/offline-cache's menu at Compare with an earlier version: Before the rebase, just now, and Pick two versions…](../img/versions-menu-dark.webp#gh-dark-mode-only)

### Where each version starts

A version is a range of commits: where it starts and its tip. Each version starts where its branch left its base, found on its own, since a rebase moves the base too:

- the branch below it, when it's in a [stack](branch-care.md#stacked-branches);
- otherwise the remote's default branch, such as `origin/main`.

When neither can be worked out, the comparison doesn't guess: **Compare two versions** opens with the tip filled in and asks where each version starts.

### Pick two versions

**Pick two versions…** opens the dialog with every field showing. Each version has an **Up to** (its tip) and a **Since** (where it starts), and each takes a branch, a tag or a commit. For the old version, **Up to** offers the branch's earlier tips and its upstream first. Each version says how many commits it has as you type. The git command it will run shows below, and **⇅ Swap** turns the two around.

The versions needn't be one branch's. Two branches that hold the same work, such as a series as it was sent and as it was sent again, compare the same way.

## Reading the list

The comparison opens in the panel on the right. A line at the top says how many commits each version has and what became of them, such as "5 commits then, 4 now: 1 changed, 1 changed only around it, 2 the same, 1 folded in." Each row then pairs an old commit with the new one, with their places in each version (`1 → 1`, or `3 → –` for one only in the old version):

| Row | What it means |
|---|---|
| **changed** | Its change is different in the new version. |
| **message changed** | Its change is the same; its message isn't. |
| **changed only around it** | Its own lines are the same, but lines near them changed, usually because a commit before it changed them. |
| **the same** | Nothing differs but its id. **the same, moved** says it's at another place now ("was 2nd, now 3rd"). |
| **folded into 1** | A `fixup!`, `squash!` or `amend!` commit that went into the commit it names. |
| **dropped** | Only in the old version. |
| **new** | Only in the new version. A row whose subject changed says what it was. |

**Hide the same** hides the rows that are the same and those changed only around them, to leave what you need to read. Clicking a row that changed opens it below the graph; clicking any other shows its commit in the graph. **Copy as text** copies range-diff's own output, and the line at the bottom says which command made the list.

Merges are left out, as range-diff leaves them out, and the top says how many.

![Two versions of feature/offline-cache in the panel: the first commit changed, wip changed only around it, the fixup folded into 1, and two the same](../img/versions-light.webp#gh-light-mode-only)
![Two versions of feature/offline-cache in the panel: the first commit changed, wip changed only around it, the fixup folded into 1, and two the same](../img/versions-dark.webp#gh-dark-mode-only)

## Reading a pair

A pair that changed opens below the graph, with its row picked out in the list. Its header names the old and new commit and says what differs, such as "the message is the same · 1 line differs". A pair changed only around it says which pair changed the lines near it, with a link to that pair. **Previous pair** and **Next pair** step through the pairs that changed, and the close button closes the pair.

**Marked**, the default, shows the commit's change as it is in the new version. Where the two versions differ, the line from the old version is marked **gone** and struck through, the line now is marked **new**, and the words that differ within it are picked out. Everything else is the change both versions share.

![The changed pair of feature/offline-cache, Marked: the new version's change, with the line the fixup added marked](../img/version-pair-light.webp#gh-light-mode-only)
![The changed pair of feature/offline-cache, Marked: the new version's change, with the line the fixup added marked](../img/version-pair-dark.webp#gh-dark-mode-only)

**Side by side** shows the old version's change on the left and the new one's on the right, line for line. The app remembers which you chose.

![The same pair, Side by side: the old change on the left, the new one on the right](../img/version-pair-side-light.webp#gh-light-mode-only)
![The same pair, Side by side: the old change on the left, the new one on the right](../img/version-pair-side-dark.webp#gh-dark-mode-only)

## When it can't say at once

- **A commit reworked a lot.** Git pairs two commits when their changes are close enough. A commit rewritten more than that shows as one dropped and one new, and the panel says so: "1 dropped and 1 new, which may be one commit reworked a lot." **Match more loosely** compares them again with `--creation-factor=100`, which pairs commits further apart; **Match as git does** goes back.
- **Nothing changed.** When every commit is the same and only the ids differ, the top says so: "All 4 commits are the same in both versions: only their ids changed."
- **A big version.** Above 500 commits in either version, the panel asks before it compares: "The old version has 1,240 commits. Compare them anyway?"
- **A [partial clone](big-repositories.md).** range-diff reads every version of every file the commits change. When this clone hasn't got some of them, the panel says how many and **Download them** fetches them in one download, with **Stop**, before comparing.
- **A [shallow clone](big-repositories.md).** When a version starts where the clone's history stops, its oldest commit has no parent here, so its change shows every file as added. The panel says so and offers **Fetch 1,000 commits more**.
- **A tip that's gone.** Git prunes commits nothing points to after a while. An old tip it has pruned says "2f75eb1 isn't in this repository any more, so there's nothing to compare it with."
