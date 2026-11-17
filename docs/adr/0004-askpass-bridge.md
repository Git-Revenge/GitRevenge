# 0004 — Credential prompts via an askpass bridge

- Status: Accepted
- Date: 2026-10-28

## Context

A GUI has no terminal. If git or ssh needs a password, a passphrase or a host-key confirmation, it would either hang or fail. The maintainer uses both SSH keys and HTTPS with Git Credential Manager (GCM), and no hardware keys.

## Decision

1. **Existing credential helpers come first.** These are GCM, `osxkeychain` and `ssh-agent`. In the common case, no prompt ever reaches the app.
2. **Git never prompts on a terminal.** Every git invocation sets `GIT_TERMINAL_PROMPT=0`.
3. **Remaining prompts go through an askpass bridge.** `GIT_ASKPASS` and `SSH_ASKPASS` (with `SSH_ASKPASS_REQUIRE=force`) point at the `gitrevenge` binary in a helper mode. The helper relays each prompt to the running app over a loopback TCP connection (`127.0.0.1`, random port). The app answers only requests that carry a random 256-bit token, which is passed solely in the git child process's environment. The app shows a dialog and passes the answer back to git, and the prompt times out after 5 minutes. *Amended 2026-10-28 (#46): a loopback port plus token replaces the per-user local socket, because it behaves the same on Windows and macOS.* *Amended 2026-11-17 (GHSA-w5mm-xpwv-w4hw): each fetch, pull, push or clone gets a token of its own, which stops being answered when the operation ends. The dialog says which operation and repository a prompt came from, in the app's own words, and warns when the prompt names a server that isn't one of the repository's remotes, because the prompt's text can be written by the server.*
4. **Unknown SSH host keys get their own confirmation dialog**, which shows the key's fingerprint. They are never accepted silently.
5. **Git Revenge never stores credentials itself.**

The same helper-mode mechanism will later serve as `GIT_SEQUENCE_EDITOR` and `GIT_EDITOR` for interactive rebase.

## Consequences

- Behavior matches the user's command-line setup.
- The helper mode and the relay protocol are tested end to end: git fetches over HTTP Basic auth with its prompts answered through the bridge. The real Windows GUI-subsystem executable has been verified to work as the helper.
