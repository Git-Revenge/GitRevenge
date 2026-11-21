# 0013 — Trailers are read once and kept, outside the repository

- Status: Accepted
- Date: 2026-11-21

## Context

Smart filters (#825) turn the trailers already on commits into filters for the graph. Every part of it asks the same two things: which commits carry this key and value, and which keys and values there are. Until now a trailer search ran `git log --grep` over the history and parsed every match, each time it was asked.

On the Linux kernel, the repository the app is measured against, git takes about 20 seconds to hand over every commit's message from a warm disk and over three minutes from a cold one. That is too long to pay per question, and it would be paid again on every launch.

A commit never changes, so neither do its trailers. What was read once is right for good.

## Decision

- **Every commit's trailers are read once, in the background, and kept.** Reading starts when a repository's graph has loaded what it wants for now, so the first screen of commits never waits for it. It covers everything reachable from a branch, a remote-tracking branch or a tag, newest first.
- **After the first read, only new commits are read.** The index records the tips a finished read started from. The next read asks git for what the current tips reach and those don't, by id, so with nothing new it reads nothing.
- **A read that's cut short keeps its place.** Closing a tab or quitting stops it. The next one walks the same commits in the same order and skips the ones already taken, which git does in about a second for the kernel. If a commit it started from has since been deleted, its place means nothing: the read starts over, and leaves out what it already holds.
- **The app's own parser decides what a trailer is,** not git's `%(trailers)`. A filter must agree with a commit's Trailers tab, which counts `Closes #12` written without a colon. The parser has a version; when it changes, the index is read again from the history.
- **The index is a file in the operating system's cache folder,** one per repository, named for the repository's shared git directory so that linked worktrees share it. Nothing is written into the repository or its git folder: a tool that reads a repository shouldn't leave files in it, and a cache has no place in what gets backed up or copied.
- **It is a cache, and treated as one.** Deleting the file is always safe. A repository that moves is read again. A file nobody has opened for 90 days is removed. A file that can't be written leaves the index working from memory for the session.
- **The file only grows, a whole frame at a time.** Each frame is written in one append, carries its own strings and has a checksum. A frame cut short by a crash is stepped over, and nothing is ever truncated or rewritten. Two copies of the app with the same repository open each append their own frames; a commit both read is counted once when the file is loaded.
- **Questions are answered from memory, by one pass over what's held.** There are no per-key tables to build or keep right. On the kernel that pass takes tens of milliseconds.
- **Keys and values are kept as written.** Which spellings are the same key, and what kind of thing a value is, is the reading rules' business (#827), and can change without reading the history again.
- **Progress is said, quietly.** While a read is taking a moment the status bar says "Reading trailers", with a bar and how far it has got, for the repository on screen. Anything that asks the index meanwhile gets what has been read so far and is told how far that is.
- **Commit ids are stored as the repository gives them,** 20 or 32 bytes, so a SHA-256 repository works.

## Consequences

- The first time a big repository is opened, git walks its whole history in the background: about 20 seconds for the kernel from a warm disk, minutes from a cold one. Every launch after that takes under a second to load the index and reads nothing.
- The kernel's index is about 96 MB on disk and about 140 MB in memory while its tab is open. An ordinary repository's is a few megabytes or less.
- This is the app's first file outside its settings. It holds commit ids and trailer text from repositories the user has opened, in the user's own cache folder, and nowhere else.
- Insights' Shipped view parses trailers per query today, and can read them from the index instead (#833).
- Rewriting history doesn't hurt: the old commits' trailers stay in the file and match nothing, and the new commits are read like any others.

## Measured

Linux kernel, 1,507,311 commits, 5,084,370 trailer lines under 2,391 keys as written:

| | |
|---|---|
| First read, warm disk | 18 s |
| git alone handing over every message, cold disk | 196 s |
| Opening again, nothing new | 0.9 s, no commits read |
| Every key with its counts | 205 ms |
| The 46,412 values of `Signed-off-by` | 60 ms |
| The 110,350 commits signed off by one person | 24 ms |
| Index file | 96 MB |
| In memory | 141 MB |
