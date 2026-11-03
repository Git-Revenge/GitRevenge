# 0009 — Branch kinds are built in, matched by local patterns

- Status: Accepted, amended 2026-11-01 (a sixth kind, Feature, and the sidebar's rail)
- Date: 2026-10-31

## Context

Some branches aren't like the others. `main` is the line everything returns to; `release/1.4` is a release in progress, closer to a tag than to a feature branch; `gh-pages` holds a built site and shares no history with the code; `dependabot/*` branches are opened and deleted by a bot; `production` may be what a deploy pipeline watches. Today the graph and sidebar treat them all alike, and the cleanup, force-push and Insights rules can't tell them apart. The design came from a grilling session on 2026-10-31 (epic #276).

## Decision

- **Six built-in kinds:** Trunk, Release, Environment, Docs, Bot and Feature. Kinds carry behaviour, not just decoration, so they are a fixed set rather than labels people define. Custom labels can come later if a real need appears.
- **Matched by glob against the branch name**, without the remote prefix, so `origin/release/1.4` is a Release branch. The first matching kind wins, in the order above.
- **Built-in defaults** work with no setup: Trunk `main`, `master`, `develop`, `trunk`; Release `release/*`, `releases/*`, `hotfix/*`; Docs `gh-pages`, `gh-docs`, `docs-site`; Bot `dependabot/*`, `renovate/*`; Feature `feature/*`, `features/*`, `feat/*`; Environment none, since `staging` and `production` are too often ordinary branches.
- **Patterns are local.** Overrides live in Git Revenge's settings, keyed by repository path, beside the per-repo hide patterns. Nothing is written into the repository, so the app stays local-only and one person's kinds don't change anyone else's view.
- **A commit's kind is its lane's kind.** A lane takes the kind of the branch tip that starts it and keeps it along that first-parent line. Commits in a merged-in side line with no branch of their own have no kind.
- **What a kind changes:**
  - Look: a chip colour and icon in the graph, a mark in the sidebar's branch tree on the kind's folder (or on the branch, when a folder's branches don't share a kind), and in the timeline the kind's icon in place of author initials (all but Trunk and Feature). Every mark has the kind's name in its tooltip and accessible name.
  - Layout: Trunk lanes are pinned, the first in column 0, the next in column 1.
  - Visibility: Docs lanes draw muted; Docs and Bot each have a visibility toggle, on by default.
  - Safety: cleanup on pull never deletes Trunk, Release, Environment or Docs branches; force push to Trunk, Release or Environment and deleting Trunk or Environment ask first, naming the kind.
  - Insights: Docs and Release branches are left out of Needs attention, conflict checks and Hotspots' open-branch counts, since they are end states expected to drift from the target, not working copies; Bot commits are grouped as dependency updates; and Release branches can bound the releases window.
- **Kinds are an Insights input.** ADR 0008 makes Insights a pure function of repository state and explicit inputs. The kind patterns in use affect results, so they join the inputs and are printed in every export.

## Consequences

- Most repositories get sensible kinds without configuration, and the Docs default fixes the common `gh-pages` case.
- Two people with different patterns can see different Insights numbers for the same repository; the exported inputs show why.
- Kinds don't travel with a clone. A team that wants shared kinds would need a committed file, which this decision defers.
- Pinned Trunk lanes change the incremental layout and have to stay within the kernel benchmark's targets.

## Amendment, 2026-11-01

Two changes, from the sidebar work in epic #332:

- **The sidebar no longer has a section per kind.** An icon rail shows one panel at a time, and a branch tree nests names at each `/`. A section per kind would read `Releases` > `release` > `v0.8`, so a kind marks its folder instead, and Trunk still sorts first after the checked-out branch.
- **Feature is a sixth kind.** Unlike the others, it changes how a branch looks rather than how it's treated: no extra confirmations, no toggle, a normal lane, and it stays in Insights. It keeps author initials in the timeline, because who did the work matters on a working branch. It's last in matching order, so a name another kind claims stays that kind.

## Amendment, 2026-11-03

From #419, on commits that several branches point at:

- **The kind earliest in the matching order wins a shared tip,** whether its branches are local or remote. Before this, a local branch beat a remote one and otherwise the alphabetically last name won, so `release/x` beat `feature/y` by accident of how they sort.
- **A Docs tip with real work on it isn't muted.** If an ordinary or Feature branch points at the same commit, the lane takes that branch's kind, or none. Faded code is worse than a Docs line drawn at full strength. Checking this reads every branch, but only in a repository with a Docs tip.
