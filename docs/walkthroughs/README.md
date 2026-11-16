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

The walkthroughs all use Tidewater, a small made-up project: a dashboard for a network of coastal weather stations, with almost two years of history by a team of six: about 1,500 commits, twenty releases, pull requests merged and squashed, and the branches a team leaves behind. It has everything the walkthroughs need: a branch with a messy history, a branch that conflicts with `main`, a stash, a remote that's one commit ahead, some uncommitted changes, and a bug that crept in after a release, for a bisect to find. For exploring the sidebar, it also has a release folder, a bot's branches, a stack of three branches, branches only the remote has, a branch whose upstream is gone, and a second worktree.

You can build the same repository and follow along. You need `git` and a bash shell; on Windows, use Git Bash, which comes with Git for Windows.

```sh
curl -fsSL https://raw.githubusercontent.com/Git-Revenge/GitRevenge/main/docs-site/scripts/demo-repo.sh -o demo-repo.sh
bash demo-repo.sh ~/gitrevenge-practice
```

That makes `~/gitrevenge-practice/tidewater`, along with its remote and a few neighbours, in under a minute. Open it with **Open repository** on the start page. The script deletes and rebuilds the folders it made each time you run it, so run it again whenever you want a clean start.

To try the repository as Git 3.0 will make it, with SHA-256 commit IDs and reftable refs, download `demo-repo-git3.sh` from the same folder and run it the same way. It needs git 2.45 or later and `demo-repo.sh` beside it, and makes `tidewater-git3` next to `tidewater`.

The practice repository sets its own author name (Emily Carter) and never signs commits, so nothing you do in it uses your identity or keys.
