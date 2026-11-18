# 0005 — Local state and coexisting with external git usage

- Status: Accepted
- Date: 2026-10-28

## Context

The app is local-only, with no accounts and no cloud. Users will keep using the CLI and their IDEs on the same repos while the app is open.

## Decision

- **App state is stored locally.** Settings, project groups, recents and open tabs live in a JSON file under the OS config directory: `%APPDATA%\GitRevenge` on Windows and `~/Library/Application Support/GitRevenge` on macOS. Writes are atomic: write a temp file, then rename it over the original.
- **The repo on screen is watched for external changes.** It gets one native recursive subscription (`ReadDirectoryChangesW` on Windows, FSEvents on macOS, via `github.com/syncthing/notify`) covering the work tree and the git directory. Events are classified (HEAD, refs, index, in-progress state, config, work tree) and debounced: about 150ms of quiet, at most 1s of delay. Work-tree paths are passed through one batched `git check-ignore`, so ignored files never trigger refreshes.
- **A repo off screen isn't watched, and is brought up to date when it's shown.** The repo on screen is the active tab's, or the ones the Ignore rules or Insights tab is showing; the start page and the Git Config tab show none (config files are polled on their own). Switching tabs stops the old watcher and starts the new one before the tab's view opens, so everything the view reads is already covered. The commit graph is the one thing kept from before: it is read again only if HEAD or a ref has moved since it loaded, which two short git calls can tell.
- **Mutating operations are serialized per repo**, while reads run concurrently. If another process holds `index.lock`, the app retries briefly, then reports that another git process is running.
- **In-progress operations are detected**, even if they started outside the app. This covers merge, rebase, cherry-pick, revert and bisect. The app shows a banner with Continue, Abort and Skip.

## Consequences

- The UI stays consistent with changes made in other tools without manual refreshes.
- A tab behind the one on screen costs nothing while it sits there: no subscription, and no git calls when its repository changes. The price is that its label (the branch it's on) can be out of date until it's shown, and that a tab switch waits a few milliseconds for the watcher. If the watcher takes more than 100ms to start (a big tree on Linux, where there is one watch per folder), the switch goes ahead and the view is told to read everything again once it's up.
- Cost does not grow with the number of directories, unlike per-directory watchers such as `fsnotify`, which uses kqueue with one file descriptor per file on macOS.
- `core.fsmonitor`, where enabled, speeds up the `git status` that follows a change. The watcher itself does not depend on it.
- macOS FSEvents needs cgo, which Wails' macOS builds already require.
- *Amended 2026-10-28 (#15): originally specified `fsnotify` plus reliance on fsmonitor for large trees.*
- *Amended 2026-11-18: originally every open tab's repo was watched. With several tabs open, each change in any of them started a round of git calls for a view nobody was looking at, and on Linux each tab held a watch for every folder in its tree.*
