# Branch care

Repositories pile up branches: release branches, a `gh-pages` site, dozens of `dependabot/…` branches, work stacked on other work, and branches whose pull requests merged weeks ago. This page covers the tools for keeping that tidy.

Everything here is set per repository and kept in its git config (`.git/config`), which is never committed, so it never changes what anyone else sees. Set the same keys in your global git config to give every repository your own defaults; a repository's own setting wins, and its list of patterns replaces the global list rather than adding to it. Open **Settings** with a repository's tab active and look under **This repository**.

## Branch kinds

Some branches aren't like the others. Git Revenge knows six **kinds**, and treats each one in its own way:

| Kind | Default patterns | What changes |
|---|---|---|
| **Trunk** | `main`, `master`, `develop`, `trunk` | Pinned to the left of the graph as a straight spine. Its chip is bold. |
| **Release** | `release/*`, `releases/*`, `hotfix/*` | Marks its folder in the sidebar, newest version first. Can bound an Insights releases window. |
| **Environment** | none | For deploy branches such as `staging` or `production`. Asks before a force push or delete. |
| **Docs** | `gh-pages`, `gh-docs`, `docs-site` | Drawn muted in the graph. Left out of Insights. |
| **Feature** | `feature/*`, `features/*`, `feat/*` | Marks its folder in the sidebar with a sprout. Otherwise it's an ordinary working branch: no extra confirmations, and it stays in Insights. |
| **Bot** | `dependabot/*`, `renovate/*` | Marks its folder in the sidebar, which starts closed. Grouped as dependency updates in Insights. |

A pattern matches the branch name without its remote, so `release/*` covers `release/1.4` and `origin/release/1.4` alike. A `*` matches anything, slashes included, so `dependabot/*` catches `dependabot/npm_and_yarn/vite-7.1`. The first kind that matches wins, in the order of the table.

### Changing them

- **One branch:** right-click it and choose **Mark `name` as…**, then pick a kind or **Ordinary branch**. If a wider pattern still claims the branch (`release/2.0` under `release/*`, say), you're told which one to change.
- **Patterns:** in Settings, under **This repository**, each kind has a field. Separate patterns with commas or spaces. **Use defaults** puts a kind back.

### How kinds look

- **Graph chips** take the kind's colour and icon: Trunk in rose and bold, Release in plum, Environment in ochre, Docs as a teal outline, Feature in green, Bot in faint text.
- **The timeline** shows the kind's icon in a commit's circle, instead of the author's initials, for commits on Release, Environment, Docs and Bot lines.
- **Docs lines** are drawn faded, with their commit messages muted, since a published site isn't code. A line that also has an ordinary or Feature branch at its tip isn't faded.
- **When branches of different kinds point at one commit**, each keeps its own label, and the commit's line takes the kind that comes first in the order Trunk, Release, Environment, Docs, Bot, Feature.
- **The sidebar** keeps your local branches in one tree in the **Branches** panel, with the checked-out branch first, or the folder holding it, and Trunk next. A kind's icon marks a folder when every branch in it shares that kind, and otherwise marks each branch. Remote branches do the same in the **Remotes** panel.

The design behind kinds is [ADR 0009](../adr/0009-branch-kinds.md).

## Hiding branches

Hidden branches leave the sidebar and the graph. Commits that only they reach disappear from the graph too, though the branch you have checked out always stays.

- **One branch:** right-click it and choose **Hide**. Hiding `wip/x` hides `origin/wip/x` too.
- **Patterns:** under **This repository** in Settings, list them in **Hidden branches**, such as `wip/*, archive/*`.
- **A whole kind:** the **Kinds** dropdown in the graph's header has a checkbox for each kind the repository has. Clear one to hide every branch of that kind. They all start checked (showing).

When something is hidden, the sidebar says how many, under the branch lists. **Show them** brings them back for now, in italics; **Hide them again** undoes that. To unhide a branch for good, right-click it while it's showing and choose **Unhide**.

## Branches in other worktrees

A branch checked out in another [worktree](repositories.md#worktrees) has a worktree mark beside its name in the sidebar and on its label in the graph. Hover over the mark to see the folder.

Git won't check out or delete a branch while another worktree has it, so its menu changes:

- **Open the worktree of…** replaces **Checkout**, and opens that folder as a tab. Double-clicking the branch does the same.
- **Delete … and its worktree** removes both, after asking. If the worktree has uncommitted changes, or the branch has commits that aren't merged, you tick a box to allow losing them. If something stops the delete, neither is removed.

A branch in the main worktree can't go this way, because the main worktree can't be removed. Open it and switch branches there first.

## Cleaning up merged branches

Turn on **Clean up merged branches after each pull** under **This repository**. After each pull, Git Revenge then deletes local branches that meet all of these:

- the branch's upstream was deleted on the remote, as hosts do after a pull request merges;
- the branch has nothing that wasn't pushed: it sits exactly where its upstream last was;
- it isn't the branch you have checked out, or one checked out in a worktree;
- it isn't a Trunk, Release, Environment or Docs branch;
- no other branch is [stacked](#stacked-branches) on it.

This works with squash merges, which `git branch --merged` can't detect. Git Revenge remembers where each upstream was every time you fetch or pull, so a fetch that clears away a deleted upstream doesn't hide the evidence from the pull that follows. A branch whose upstream disappeared before you opened the app is left alone.

A message in the corner names what was deleted. **Undo** in it, or the toolbar's Undo, brings every one of them back.

You can also do this whenever you like. **Needs care**, in the Branches panel, counts the branches already merged and safe to delete, and **Delete all** deletes them without asking, since Undo brings them back. It leaves the same branches alone as above, along with hidden branches, and looks at each one again first to be sure it's still merged.

## Syncing notes

[Git notes](history.md#notes) aren't fetched or pushed by default. Turn on **Sync notes on fetch, pull and push** under **This repository** to change that:

- **Fetch and pull** bring in the remote's notes, and your notes catch up with them.
- **Push** sends your notes along with the branch. It never forces: if the remote has notes you don't, the push of the notes is refused, and you're told.
- **When both sides wrote notes the other doesn't have**, they've diverged. Git Revenge leaves yours as they are and says so; merge them with `git notes merge`, then push.

A problem with notes never stops the fetch, pull or push itself.

## Stacked branches

A branch is **stacked** when it builds on another branch that hasn't merged yet: you started `feature/b` from `feature/a` to keep going while `feature/a` waits for review.

Git Revenge works this out from history. In the sidebar, a stacked branch sits under the branch it builds on, indented, even when its name would put it in another folder. Hover over it to see what it's stacked on and its place in the stack (**2 of 3**). In the graph, its chip carries a small stack mark.

A stack of three or more becomes a group, named after its bottom branch, with the top of the stack first. Click the group to open or close it. A group whose branches have all merged starts closed and says **merged**.

Right-click a group to work on the whole stack:

- **Check out top of stack** and **New branch on top**. A stack that forks offers each of its tops.
- **Update stack**, for after its lower branches merge. It fetches, moves the branches not yet merged onto the Trunk's upstream, and offers to delete the merged ones. A stack that forks can't move as one.
- **Rebase stack onto…** moves every branch onto another, keeping each in place, and checks out the top. One Undo puts them all back. If it stops on a conflict, fix it and Continue, or Abort to put everything back. Undo isn't offered after a rebase that stopped for conflicts.
- **Push stack** force pushes its unmerged branches with a lease. Each remote takes all of them or none. It asks first only when a push would really lose commits from the remote, naming them, or when a branch is one that asks before a force push. Commits a rebase rewrote with the same changes don't count.
- **Unstack** shows its branches separately again. It doesn't change their history, and the message it leaves has **Undo**.
- **Delete stack** deletes every branch in it. If all of them are merged it goes straight ahead, with **Undo** in the message it leaves. If any aren't, it asks first and names the unmerged work. It leaves alone the checked-out branch and anything checked out in a worktree, hidden, protected by its kind, or built on by a branch that stays.

If the guess is wrong, right-click the branch and choose **Stack `name` on…**: pick the right branch, **Not stacked**, or **Detect automatically** to go back to the guess.

### Restacking after the branch below merges

When a host squash-merges the branch below, the branch on top still carries the old, un-squashed commits. Once a fetch or pull shows the lower branch's upstream is gone, the branch on top is marked **parent merged** in the sidebar.

If the branch on top has no work of its own left to land, it says **merged** instead: there's nothing to restack, and it only needs deleting.

Right-click it and choose **Restack**. Git Revenge checks it out if it isn't already, then moves just its own commits onto the latest Trunk, leaving the merged branch's old commits behind. If that runs into conflicts, you resolve them as in any [rebase](branches.md#conflicts). Undo puts the branch back where it was.

## Safety checks

Some kinds deserve a second thought before something drastic happens to them:

| Action | Asks first for |
|---|---|
| Force push | Trunk, Release and Environment branches |
| Delete a local branch | Trunk and Environment branches name their kind in the confirmation |
| Delete a branch from a remote | Trunk and Environment branches; others go at once |

## In Insights

[Insights](insights.md) uses kinds too. Docs and Release branches are left out of **Needs attention** and its trial merges, since they're expected to drift from the target. Dependency updates from bots are grouped on their own in **Shipped**. Release branches can mark the ends of a releases window. The kind patterns in use are printed with every export, because they change the results.
