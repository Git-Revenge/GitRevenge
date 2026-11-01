# Remotes, LFS and submodules

## Fetch, pull and push

The toolbar has **Fetch**, **Pull** and **Push**.

| Action | Keys |
|---|---|
| Fetch all remotes | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>F</kbd> |
| Pull | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>L</kbd> |
| Push the current branch | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>U</kbd> |

**Pull** does whatever your git config says (`pull.rebase`, `pull.ff`). The arrow next to it, and the command palette, offer **fast-forward only**, **merge** and **rebase** for a one-off.

Fetch and pull both prune remote branches that were deleted on the remote. Two settings under **This repository** add to them: [cleaning up merged branches](branch-care.md#cleaning-up-merged-branches) after a pull, and [syncing notes](branch-care.md#syncing-notes) on fetch, pull and push.

### Signing in

Git Revenge uses your existing credential helpers and SSH agent, so if `git push` works in a terminal, it works here. When git needs something they can't supply, such as a password or a key's passphrase, the app asks you in a dialog. It never stores what you type; saving credentials is left to your credential helper.

The first time you connect to an SSH host your machine doesn't know yet, the app shows the host key's fingerprint and asks before accepting it.

## Managing remotes

**Manage remotes…** (in the command palette, or the gear at the top of the Remotes panel in the sidebar) adds, edits and removes remotes.

## LFS

With [git-lfs](https://git-lfs.com) installed, the **Repository** panel of the sidebar has an **LFS** row with the number of files. Click it for the details:

- the patterns tracked with LFS, and a way to **track** a new one;
- which files are in LFS, and which are only pointers whose content hasn't been downloaded;
- for lockable files, **Check locks** asks the server what's locked, and you can lock and unlock files.

## Submodules

The **Repository** panel of the sidebar lists the repository's submodules, and says in words when one needs attention. Click one to open it as a tab. **Update** initializes and updates all of them.
