# 0012 — Names follow the mailmap git reads; Insights keeps its own

- Status: Accepted
- Date: 2026-11-21

## Context

A mailmap says that two names or addresses in a history are one person. Until now the app applied one in two places only. Insights and the Map's Authors mode read the `.mailmap` committed on one branch and nothing else: Insights so that a report can be reproduced (ADR 0008), the Map because it was built on the same queries. Everywhere else, the graph, a commit's details, file history and the lists of commits in previews showed the name each commit was written with. One person under two addresses read as two people, and the app disagreed with `git log` in a terminal, which applies the mailmap by default.

Two readings of "the mailmap" were on the table for those other places: the pinned one Insights uses, or the one git itself reads.

## Decision

- **Where a name is shown and nothing is being reported, it's the name the mailmap git reads gives:** the `.mailmap` in the work tree, plus `mailmap.file` or `mailmap.blob` when the user has set them. The app adds no rules of its own. That covers graph rows, a commit's details, file history, the commits listed in merge and rebase previews, and search by author. Blame already read names this way.
- **A saved `.mailmap` counts at once, committed or not,** because that is what git does. The graph reloads when the file or the `mailmap.*` settings change.
- **The Map's Authors mode goes with the graph.** It's measured from now and nothing in it is meant to be reproduced, so it reads the same mailmap, and what it has counted is dropped when the mailmap changes. Otherwise a `.mailmap` that's saved and not committed would show one person in the graph and two in the Map. After the mailmap, the Map still folds people who share an email or a name.
- **Insights keeps the committed `.mailmap` of its target branch,** with the user's own mailmap file left out. ADR 0008 stands: a report's result can't depend on the working tree or on one person's config.
- **What a commit says is never hidden.** The commit's own name and address stay in what the backend returns, and the commit panel shows them whenever the mailmap reads them as someone else. Copying and signatures use what the commit has.
- **Search finds a person under every name.** A row matches by the mapped name and by the name it was written with. A query with an `@` is an address and matches addresses the same way; other queries leave addresses alone, so "github" doesn't match everyone with a no-reply address there.
- **No commit changes.** A mailmap changes what is shown. Taking a line out puts every name back.
- **A repository needs no mailmap, and the app never creates one unasked.** Everything works without one. Reading never writes. The People view edits the work tree's `.mailmap` only, shows what an edit does before it's saved, and where there's no `.mailmap` it asks before creating the file; the backend refuses to create one unless the call says the user agreed.
- **The app reads mailmap text itself where it has to answer before a save:** the People view's list, its line notes and "Who is this?" follow git's rules in the app's own reader, checked against `git check-mailmap` in the tests. Everything that shows a saved mailmap's names still asks git.

## Consequences

- The app and a terminal agree on who wrote a commit.
- The graph and the Map agree with each other. Insights can show one person differently from both when the working tree's `.mailmap` has edits that aren't committed, or when someone has a private `mailmap.file`. Each says which mailmap it reads.
- The graph holds, for each author, the mapped name and address and the written ones. That's a few strings for each identity, not for each commit: about 45,000 identities for the Linux kernel's 1.5 million commits.
- A repository with no mailmap looks exactly as it did.
- Co-authors in trailers aren't mapped by `git log`; wherever the app shows them as people it has to ask the mailmap itself.
