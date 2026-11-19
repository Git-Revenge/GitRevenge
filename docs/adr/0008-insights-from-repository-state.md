# 0008 — Insights come only from repository state, and are reproducible

- Status: Accepted, amended 2026-11-19 (bot author patterns are an input)
- Date: 2026-10-30

## Context

The Insights tab (#141) supports sprint reviews: what shipped in a window, and which work is stuck or risky. The project has no accounts, no cloud and no hosting or issue-tracker integrations (see the plan's non-goals), so there are no pull requests, reviews, ticket states or story points to draw on. Teams compare numbers in reviews. If two people looking at the same repository saw different results because their clocks, git config or app state differed, the numbers would be argued over rather than used.

An earlier Insights view (#208) measured from "now" and read through the user's own git config. It was hidden in #212.

## Decision

- **Repository state only.** Insights are computed from git objects and refs, tags and the committed `.mailmap`. Nothing is stored between runs, apart from view preferences such as thresholds, and nothing is fetched: the view never goes to the network on its own.
- **A pure function of (repository state, inputs).** The inputs are the scope, the target branch, the window, the as-of point, the thresholds and the grouping. Anyone with the same repository state who uses the same inputs gets the same results:
  - **No wall clock.** Relative windows ("last 14 days") and ages (stale, long-lived) are measured from an **as-of point**. By default that's the committer date of the target branch tip; it can be set to an explicit date.
  - **Absolute, UTC.** Windows resolve to absolute UTC dates, or to a `tagA..tagB` range. Days are bucketed in UTC by committer date.
  - **Pinned git.** Every command runs with fixed flags and `-c` overrides. Rename detection and merge options are set explicitly, and names come only from `<target>:.mailmap` (`mailmap.blob`, with `mailmap.file` cleared). The user's git config can't change a result.
  - **Deterministic order.** Every sort has a full tie-break: date, then name or SHA. Caps (such as the 50 branches that get signals and conflict checks) choose by most recent tip date, then name.
  - **Inputs travel with results.** Every export (Markdown summary, CSV) starts with the inputs, including each repository's target tip SHA, so a result can be reproduced and checked.
- **Display-only state stays out of results.** Fetch freshness (read from file timestamps) and background progress are shown, but never appear in results. An export either waits for pending conflict checks or lists them as pending.
- **A shipped item** is a first-parent commit on the target branch within the window. A merge commit stands for its branch, and a squash merge is one item. Cycle time is reported only for real merge commits, from the branch's first commit to the merge, and never estimated for squash merges.

## Consequences

- Results can be checked by hand with git: each number's "how is this measured" note gives the query.
- "Last 14 days" means the 14 days up to the target tip, not up to today. A quiet repository's window doesn't drift forward on its own; setting an explicit as-of date covers "as of today".
- Remote branches are only as current as the last fetch. The view says so, and offers Fetch, but two people who fetched at different times have different repository states, and so may see different results.
- Some useful measures are out of reach: review time, ticket status and anything about pull requests. Per-person productivity charts are deliberately left out.
- Results are recomputed rather than read from a store. Window-bounded queries keep this fast on large repositories, and branch work is capped.

## Amendment, 2026-11-19

From #808: which authors are bots is an input.

- Shipped groups an item with the dependency updates when its author is a bot. That used to mean a name ending in `[bot]` and nothing else. A repository can now add patterns for author names and emails, in git config as `gitrevenge.botAuthor`, beside the branch kind patterns (ADR 0009).
- The patterns come from git config, which this decision otherwise keeps out of results. They're treated as the kind patterns are: an explicit input, part of the cache key, and printed with every export as **Bot authors**, the built-in `*[bot]` first. Two people with different patterns can see different groupings, and the exported inputs show why.
- The Map's Authors mode (#816) reads the same patterns, so the app has one answer to who is a bot.
