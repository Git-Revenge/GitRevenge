# 0011 — A repository's settings live in git config; the app's stay in the app

- Status: Accepted
- Date: 2026-11-09

## Context

Git Revenge kept everything it remembered in one JSON file of its own (ADR 0005), including settings that describe a repository: branch kind patterns, hidden branches, cleanup on pull, notes syncing, stack parents set by hand, and Insights thresholds. They were keyed by the repository's path, so they didn't follow a repository that moved, couldn't be given defaults once for every repository, and were invisible to anything but the app. Smart links (#525) had already taken the other route and put their rules in git config, where a repository's own config overrides the global one the way git's own settings do. Issue #526 asked whether the rest should follow, and where the line is.

## Decision

- **Settings that describe a repository live in git config.** The repository's `.git/config` where it sets them, the global config otherwise. `.git/config` is never committed, so this changes where they're stored, not who sees them (ADR 0009 is amended to say so).
- **Settings that describe the app stay in the app's file:** theme, graph layout, zoom, keybindings, the clone folder and update checks. They are about this app on this machine. A window zoom in `~/.gitconfig` would follow someone's dotfiles to a machine with a different screen. The git executable's path stays too, since git is needed to read git config, as does all state: open tabs, recent repositories, groups. Thresholds for a *group* in Insights stay with the app, because a group is the app's idea, not a repository.
- **Keys.** Scalars sit in one section, `gitrevenge.cleanupOnPull` and `gitrevenge.syncNotes`. Lists are multi-valued keys, as git's own are: `gitrevenge.hide`, `gitrevenge.hideKind`. Each branch kind's patterns are a subsection, matching smart links: `[gitrevenge-kind "release"] pattern = …`. Insights thresholds are `[gitrevenge-insights] stale, longLived, behind, size`. A stack parent sits beside git's own per-branch keys, `branch.<name>.gitrevengeStackParent`, so git removes it with the branch.
- **The repository wins, per setting, and its list replaces the global list.** A list set in a repository never adds to the global one. One empty value means "none here", which is how a repository turns off a list its user set globally, and how a kind is made to match nothing.
- **Writes go to the repository's config, and only for what changed,** so a value that comes from the global config stays inherited until someone changes it for that repository.
- **The app's file keeps a copy of what applies.** Everything in the app reads the copy: it's there before git has answered and costs nothing to read while a graph loads. The copy is refreshed when a repository opens and when its config or the global config changes (the repository watcher and the global config's watcher, ADR 0007).
- **Moving over.** The first time the app meets a repository whose settings are only in its own file, it writes them to that repository's config, once, and marks the copy as coming from git config. If the repository's config already sets any of these keys, the config is taken as the newer word and nothing is written. Because the copy stays in the old place and shape, a release from before the move still finds the settings it knows. A later release can stop relying on that.
- **Worktrees share them.** Everything here describes the repository, not one checkout, so `config.worktree` isn't used.
- **They stay Insights inputs** (ADR 0008), now named by their keys.

## Consequences

- A person can set their kinds, hidden patterns and thresholds once, globally, and adjust them per repository; dotfiles carry them between machines.
- The settings show up in the Git Config tab like any others, with descriptions, and can be edited there or with `git config`.
- A repository's settings survive the folder being moved or renamed, and are gone with the clone when it's deleted.
- Two sources can disagree for a moment: a change made outside the app shows once the watcher notices. The copy is never written back wholesale, so it can't undo such a change.
- "Use the built-in defaults, ignoring my global patterns" can't be said for one kind in one repository: leaving a kind unset there means inheriting. Setting the patterns explicitly does the job.
- Editing settings for a folder that isn't a repository keeps them in the app alone, as before.
