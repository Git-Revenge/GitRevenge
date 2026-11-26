# 0002 — Use the system git CLI

- Status: Accepted
- Date: 2026-10-28

## Context

Very large repos are a hard requirement. They include repos with 1M+ commits, 300k+ files, LFS and submodules. The candidate engines were the system `git` CLI, go-git and libgit2 via cgo.

## Decision

Shell out to the **system `git` CLI (>= 2.40)** through a thin Go layer. That layer provides:

- streaming parsers for git's output
- long-lived `git cat-file --batch` processes
- a per-repo operation queue

We do not bundle git.

## Consequences

- We get git's large-repo machinery (`commit-graph`, multi-pack-index, `fsmonitor`, untracked cache, partial clone, sparse checkout) for free.
- The user's config, credential helpers, SSH agent, signing, hooks and LFS behave exactly as they do on the command line.
- There is no cgo, so cross-platform builds stay simple.
- Git must be installed. The app checks the git version at startup and shows a clear error if it is missing or too old.
- *Amended 2026-11-26 (#1001): the minimum is git 2.50 from 1.2.0. Partial clones need 2.44, the first git that can be told not to download (`GIT_NO_LAZY_FETCH`), and downloading what one is missing needs 2.49 (`git backfill`). ADR-0010 says what this means for each Linux.*
- Parsing CLI output must be robust. We run git with `-z` formats, `LC_ALL=C` and `core.quotepath=off`.
- If CLI parsing becomes the bottleneck for the graph, we can add a native reader for the `commit-graph` file later.
