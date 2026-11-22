# 0014 — How a trailer is read: joined spellings, a kind for each value, and what's held back

- Status: Accepted
- Date: 2026-11-21

## Context

The trailer index (ADR 0013) holds what people wrote: a key and a value, for every commit. To filter by them, something has to say which spellings are the same key, which values are the same thing, and what kind of thing a value is.

Git Revenge is used by projects with no shared convention. The Linux kernel writes `Fixes: 1da177e4c3f4 ("…")` and means a commit; a project on GitHub writes `Fixes #12` and means an issue. Some treat Fixes, Closes and Resolves as one word and some don't. A rule that suits one of them is wrong for another.

## Decision

**Keep what was written, join only what is plainly the same spelling, and offer the rest.**

Joined without asking:

- **Letter case of a key.** `Co-Authored-By` and `Co-authored-by` are one key, shown the way it's written most.
- **Word endings of a key,** for four families: Close, Closed, Closes; Fix, Fixed, Fixes; Resolve, Resolved, Resolves; Ref, Refs, Reference, References.
- **Several values on a line.** `Closes #12, #14` is two tickets, when every part is one.
- **Repeats.** A commit counts once for a key and once for a value, however often it says it.
- **A ticket's number and its address,** when one of the repository's smart links sends the number to that address.
- **A person under two names,** by the mailmap git reads, as the graph shows authors (ADR 0012). Read only.

Never joined on their own: **Fixes, Closes and Resolves stay three keys.** Joining them is an offer made per repository (#832).

**The kind belongs to the value, not the key.** It's told from the value's shape, in this order:

| Kind | A value is one when |
|---|---|
| Person | it's a name and an address, with anything after it left off, or an address alone |
| Ticket | a smart link matches the whole of it, or it's `#12`, `owner/repo#12`, or the address of an issue, a pull or merge request, or a bug |
| Commit | it starts with an id that names exactly one commit in the repository |
| Label | it's plain text under a key where a quarter or fewer of the plain values differ, over five uses or more |
| Identifier | it's plain text under a key where four in five or more of the plain values differ, over ten uses or more |
| Text | anything else |

A key's kind is what most of its values are. An address with no name is a person, unless its key has a different one on nearly every commit: a `Message-Id` is written the same way as `Cc: <list@host>` and is nobody.

**Held back** means not offered as a filter. Search still finds a held key.

- **An identifier,** since its values group nothing.
- **A key on 90% or more of commits,** in a history of 50 commits or more, since it filters nothing.
- **A key on fewer than 3 commits,** since it's likely a sentence with a colon in. A key in common use (Signed-off-by, Reviewed-by, Closes and the like) is exempt, and so is one whose values are people, tickets or commits: those are plainly trailers.

**The known keys are defaults for labels and order only.** They no longer decide what counts as a trailer.

**A reading is worked out from the index and the rules in one pass and kept** until the index, the mailmap, the bot patterns, the smart links or the size of the history changes. Changing a rule never reads the history again.

## How the thresholds were set

Against three histories.

| | This project | Tidewater | Linux kernel |
|---|---|---|---|
| Commits | 570 | 1,492 | 1,507,311 |
| Keys, case folded | 5 | 2 | 2,176 |
| On one commit | 2 | 0 | 1,425 |
| On two commits | 0 | 0 | 266 |

- **90% for "nearly every commit".** The kernel's `Signed-off-by` is on 92% of commits. This project's `Co-authored-by` is on 83%, and which co-author is still worth filtering by.
- **Fewer than 3 for "rare".** It holds back `docs` and `Bench` here, each a sentence in a last paragraph, and 1,170 of the kernel's keys. `Reviewed-by` on one commit stays: its value is a person.
- **80% different for an identifier.** The kernel's `Link`, `Message-Id`, `Change-Id` and `Patchwork` are all above 95%, and so are its stray `v2:` change-log lines. `Fixes`, at 70%, is commits and isn't judged this way.
- **25% different for a label.** The kernel's `Assisted-by` is at 9%.

On the kernel a reading offers 910 keys and holds back 1,258: 1,170 rare, 87 identifiers and `Signed-off-by`.

## Consequences

- The kernel's `Closes` and `Link` are held back: their values are mailing-list addresses, a different one on each commit. That is what the rule says, and search still finds them.
- Telling which ids are commits means asking git. For a couple of thousand ids git is asked about each; past that, every commit's id is listed once and the abbreviations are looked up in memory, which took the kernel's 98,353 from 11 seconds to under 4.
- A reading of the kernel takes about 5 seconds the first time, and is kept. An ordinary repository's is immediate.
- Misspelled keys (`Reviewied-by`) are their own keys. Joining them is an offer, not a rule (#832).
