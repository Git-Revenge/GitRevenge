# 0007 — Edit git config through a working copy and `git config --file`

- Status: Accepted
- Date: 2026-10-30

## Context

The Git Config editor (#195) edits global, local and included git config files. It has a key/value table, an alias editor and a raw text editor. Users write these files by hand. They hold comments, layout, `[include]` blocks, multi-valued keys, and spellings like `yes` and `on`, and they can hold secrets. A bad save can break every repo on the machine.

## Decision

- **Git does the reading and writing.** Effective values come from `git config --list --show-origin --show-scope -z`. Table and alias edits run `git config --file <working copy> …` (set, unset, add, and rename and remove a section). The app never parses or serializes INI.
- **Edits go to a working copy.** Opening a scope copies its file to a temp working copy. All three views edit that copy, so switching views never loses an edit.
- **Saves are explicit and validated.** Save runs `git config --file <working copy> --list`. If git can't parse the copy, the save is refused. Otherwise the current file is backed up, then the new one is written atomically (temp file, then rename). Line endings are kept.
- **Backups live in app data**, under `os.UserConfigDir()/GitRevenge/config-backups/<scope-id>/` (see [0005](0005-local-state-and-external-changes.md)):
  - `<scope-id>` is a hash of the normalized path: symlinks resolved, path cleaned, and case folded on Windows and macOS.
  - A `meta.json` in each folder records the original path.
  - Backups are named by colon-free UTC timestamps, such as `20261030T142501Z.gitconfig`.
  - The last 20 backups per file are kept.
  - A restore backs up the current file first.
- **System config is read-only**, because writing it usually needs admin rights. **Worktree config is deferred.**
- **Global and included files are polled**, because the repo watchers don't cover them. One stat per file every 2s costs almost nothing. Watching their folders isn't an option: the global config usually sits in the home folder, and a watch on that folder silences every repository inside it (#377). A change sends an app-wide event, and open repos re-read their config-dependent state.

## Consequences

- Comments, ordering and spelling survive table edits, and git's own rules for quoting and escaping apply.
- Every table edit runs a short-lived `git` process. That's fine for interactive edits, but bulk operations should batch their edits.
- An invalid file can never be saved. Backups make every save and restore reversible on both platforms without writing next to the user's files.
- Backups are exact copies, so they hold the same secrets as the original files. They stay in the user's own app-data folder.
