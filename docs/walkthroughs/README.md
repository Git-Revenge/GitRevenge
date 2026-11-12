# Walkthroughs

Each walkthrough takes one everyday job from start to finish, with a short animation of the whole thing and the steps written out underneath.

| Walkthrough | What you'll do |
|---|---|
| [Stage and commit](first-commit.md) | Commit part of a file, and leave the rest for later. |
| [Tidy a branch before you share it](tidy-a-branch.md) | Squash fixups, drop a debugging commit, and reword a "wip". |
| [Resolve a merge conflict](resolve-a-conflict.md) | Merge a branch that changed the same lines as yours. |
| [Undo a mistake](undo-a-mistake.md) | Take back a merge, and find work you thought you'd lost. |
| [Find a commit](find-a-commit.md) | Search by message, author, SHA or trailer. |
| [Find what broke it](find-what-broke.md) | Bisect down to the one commit that introduced a bug. |
| [Look after several repositories](several-repositories.md) | Fetch and pull a whole group at once. |
| [Find your way round a busy repository](busy-repository.md) | See every branch at once on the Map, then focus the timeline on one. |

## A practice repository

The walkthroughs all use Tidewater, a small made-up project: a dashboard for a network of coastal weather stations, with a few months of history by four people. It has everything the walkthroughs need: a branch with a messy history, a branch that conflicts with `main`, a stash, a remote that's one commit ahead, some uncommitted changes, and a bug that crept in after a release, for a bisect to find. For exploring the sidebar, it also has a release folder, a bot's branch, a branch whose upstream is gone, and a second worktree.

You can build the same repository and follow along. You need `git` and a bash shell; on Windows, use Git Bash, which comes with Git for Windows.

```sh
curl -fsSL https://raw.githubusercontent.com/Git-Revenge/GitRevenge/main/docs-site/scripts/demo-repo.sh -o demo-repo.sh
bash demo-repo.sh ~/gitrevenge-practice
```

That makes `~/gitrevenge-practice/tidewater`, along with its remote and a few neighbours. Open it with **Open repository** on the start page. The script deletes and rebuilds the folders it made each time you run it, so run it again whenever you want a clean start.

The practice repository sets its own author name (Maya Okafor) and never signs commits, so nothing you do in it uses your identity or keys.
