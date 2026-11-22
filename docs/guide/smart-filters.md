# Smart filters

Many projects end a commit message with a few lines that say more about it: who reviewed it, which issue it closes, what kind of change it is. Git calls these **trailers**. Each is a key, a colon and a value, in the last paragraph of the message:

```
Round tide heights to the nearest centimetre

Closes #328

Changelog: fixed
Reviewed-by: Jessica Nguyen <jessica@tidewater.example>
```

**Smart filters** reads the trailers your history already has and turns them into filters for the graph. You can see every commit Jessica reviewed, every fix that closed an issue, or what each commit says under `Changelog`, without opening one.

It's read-only. Smart filters never writes a trailer, changes a commit, or asks you to adopt a convention: it works with whatever keys your project writes. To add a trailer to a commit you're making, use **Trailer** in the [commit form](changes.md).

## The panel

Open **Smart filters** in the sidebar's rail. It lists each key the history uses, most commits first, with what its values are (people, tickets, commits, labels) and how many commits have it. Under each key are its values, each with its count.

![The Smart filters panel: each key with what its values are and how many commits have it, and its values under it](../img/smart-filters-panel-light.webp#gh-light-mode-only)
![The Smart filters panel: each key with what its values are and how many commits have it, and its values under it](../img/smart-filters-panel-dark.webp#gh-dark-mode-only)

- **People** are shown by name, as your [mailmap](people.md) reads them. Someone the mailmap joins is one person under every key, and bots come after everyone else.
- **Tickets** are a reference by number (`#12`, `tide/api#12`), a tracker's key (`TIDE-12`), the address of an issue or pull request, or anything one of your [smart links](settings.md#links) matches.
- **Labels** are a few values used over many commits, such as `Changelog: fixed`.

The box at the top of the sidebar filters the panel as it filters the others: type part of a key or a value.

The first time a repository opens, its history is read for trailers in the background, and the status bar shows how far that has got. Most repositories take a moment. The Linux kernel's 1.5 million commits take a few minutes the first time, as long as git takes to hand over every message. The result is kept beside the app's other caches, so the next launch reads only what's new. Until the read is done, counts are of what's been read so far, and the panel says so.

### What's joined without asking

Histories are inconsistent, and some of that is safe to tidy without a question:

- **Letter case and word endings.** `closes`, `Closes`, `Closed` and `Close` are one key. So are `Fixes` and `Fixed`, `Resolves` and `Resolved`, and `Refs`, `Ref`, `Reference` and `References`.
- **Several on a line.** `Closes: #12, #14` is two tickets.
- **Repeats.** A value said twice on one commit counts once.
- **A number and its address.** Where a smart link says where `#14` leads, `#14` and that address are one ticket.

Anything less certain is a question for you, in [Trailer care](#trailer-care).

### What's held back

Not every key makes a useful filter. The panel leaves some out, and says which and why at its foot:

- **A different value on nearly every commit**, such as a `Change-Id`. It groups nothing.
- **On nearly every commit**, such as `Signed-off-by` where a project requires it. It filters nothing.
- **On only a commit or two.** It's likely a sentence that happens to look like a trailer.

A held key can still be searched for, and you can say otherwise in Trailer care.

## Filtering the graph

Click a value and the graph filters to it. Click it again, or its **×** in the strip above the graph, to take it out.

Several values work together:

- values of **one key** are either-or: `Changelog: fixed` and `Changelog: added` keeps commits with either;
- values of **different keys** must all hold: `Changelog: fixed` and `Reviewed-by: Jessica` keeps the fixes she reviewed.

While a filter is on, a **strip** above the graph says how many commits it keeps ("23 of 1,493 commits"), with a chip for each value, **+ Filter** to pick another, and **Clear**.

A filter has two modes, switched in the strip and remembered for each repository:

- **Only these**, the default. Commits that don't match leave the graph. Those that stay are strung on one dashed line, since they're no longer neighbours. Each day's line says how many it kept of how many the day had: "1 of 3 commits".
- **Highlight.** Nothing leaves. The matches keep their ink and the rest fade, each match has a tick on the [ruler](graph.md#the-ruler), and a day's line adds how many of its commits match. A match inside a [folded lane](graph.md#folded-lanes) shows.

![Only these: the graph holds the 171 commits that say Changelog: fixed, on one dashed line, and each day says how many it kept](../img/smart-filter-only-light.webp#gh-light-mode-only)
![Only these: the graph holds the 171 commits that say Changelog: fixed, on one dashed line, and each day says how many it kept](../img/smart-filter-only-dark.webp#gh-dark-mode-only)

![Highlight: every commit is there, the fixes keep their ink, and the ruler has a tick for each](../img/smart-filter-highlight-light.webp#gh-light-mode-only)
![Highlight: every commit is there, the fixes keep their ink, and the ruler has a tick for each](../img/smart-filter-highlight-dark.webp#gh-dark-mode-only)

The **Horizontal** view always highlights, since taking commits out would break its lanes.

A filter stacks with [focus](graph.md#focusing-on-a-branch) and with [hidden branches](branch-care.md#hiding-branches): the graph shows what passes all of them. It lasts until you clear it or close the tab.

A few things follow from a filter:

- **The commit you had selected stays** in the panel on the right, even when the filter hides its row, and is selected again when its row comes back.
- **A commit's Trailers tab marks the line the filter picked**, so you can see why the commit is there.
- **Where the way back to the checked-out commit would be**, the graph says when that commit isn't in the filter.
- **While trailers are still being read**, the ruler's foot is hatched for the history whose matches aren't known yet.

## Searching by a trailer

The [search box](graph.md#searching) knows trailers too. As you type, a list under it offers the keys and values that contain what you've typed, each with its count:

- typing a word, such as `jess`, offers the values that contain it, under their keys;
- typing a key and a colon, such as `reviewed-by:`, offers that key's values;
- a key's own row writes the key in the box, to choose among its values.

![The search box with reviewed-by: typed, and the reviewers it offers, each with how many commits](../img/search-suggestions-light.webp#gh-light-mode-only)
![The search box with reviewed-by: typed, and the reviewers it offers, each with how many commits](../img/search-suggestions-dark.webp#gh-dark-mode-only)

The first row is always the plain search, and it's where the keys start, so <kbd>Enter</kbd> goes to the next match as it always has. Press <kbd>↓</kbd> to reach a suggestion and <kbd>Enter</kbd> to add it to the filter. <kbd>Tab</kbd> completes it in the box. <kbd>Backspace</kbd> in an empty box takes the last value out of the filter. Counts in the list are within the filter that's already on.

Typed out, a search also understands:

| Search | Finds |
|---|---|
| `Reviewed-by: Jessica` | Commits with that value, by any name of hers the mailmap joins. |
| `Closes: #14` | That ticket, whole: never `#140`. |
| `closed: #14` | The same: a key is found by any of its endings. |
| `Changelog:` | Every commit with the key. |
| `has:reviewed-by` | The same, said the other way. |
| `-has:reviewed-by` | Every commit without the key. |
| `-changelog: none` | Every commit but those with that value. |

A search for a key the history doesn't use stays a search of subjects, so `fix: crash` finds the commit titled that.

## Pinning a key to rows

Pin a key and its values show on the graph itself, so you can read `Changelog: fixed` or `Closes #14` down the history.

Nothing is pinned until you pin it. The pin is beside each key in the panel, shown when you point at the key, and beside each key in a commit's **Trailers** tab. The same pin unpins. From the keyboard, the menu key on a key in the panel offers it. The first time a repository has a label key, the panel offers once to pin it.

- **In the timeline**, a pinned key's values are chips after the subject: the key, then the value in its kind's colour. A row shows two and a count of the rest. Chips take the room the subject doesn't need, and never more than half.
- **In Compact**, a pinned key has a column of its own, left of **Author**, with the key in the heading. Up to two keys get a column, while the message keeps room to be read; a narrower window gives the columns up.

![The timeline with Changelog pinned: a chip after the subject of each commit that has the trailer](../img/smart-filter-pinned-light.webp#gh-light-mode-only)
![The timeline with Changelog pinned: a chip after the subject of each commit that has the trailer](../img/smart-filter-pinned-dark.webp#gh-dark-mode-only)

![Compact rows with Changelog pinned: a column of its own, left of Author](../img/smart-filter-compact-light.webp#gh-light-mode-only)
![Compact rows with Changelog pinned: a column of its own, left of Author](../img/smart-filter-compact-dark.webp#gh-dark-mode-only)

Click a chip, or a value in a column, to filter the graph to it. In Compact, the filter button in a column's heading filters to the selected commit's value. With the keyboard, select the row and press <kbd>Tab</kbd>: its chips are the first of the row's actions.

The Horizontal view and the Map don't show pinned keys.

## Trailer care

Some inconsistencies aren't the app's to settle. **Trailer care** is where you say how this repository's trailers should be read. Open it from **Settings › Smart filters**, from **Needs care** in the panel, which counts the questions waiting, or with **Trailer care** in the command palette.

Nothing in Trailer care changes a commit. An answer is a rule, and a rule only says how to read what's already written. Every one can be undone.

![Trailer care: a key one letter off Reviewed-by to answer, a person written two ways, and the table of keys](../img/trailer-care-light.webp#gh-light-mode-only)
![Trailer care: a key one letter off Reviewed-by to answer, a person written two ways, and the table of keys](../img/trailer-care-dark.webp#gh-dark-mode-only)

### Needs care

- **A key one letter off.** `Reviewd-by` on 2 commits beside `Reviewed-by` on 58 is probably a slip. **Read as Reviewed-by** joins it; **Keep apart** says it's a key of its own. Slips of one key are asked about together, and **Read all** answers them at once. It's a question because one letter can matter: `Nacked-by` isn't `Acked-by`.
- **Two keys holding the same things**, such as `Ticket` and `Refs` naming the same tickets. `Fixes`, `Closes` and `Resolves` are offered to each other where their values are tickets. Joined, the key reads "Closes".
- **One person written two ways.** These are shown, with a way to [People](people.md), where the mailmap joins them. They aren't answered here.

**Keep apart** is remembered, so a question isn't asked twice.

### Joined without asking

A short list of [what was joined automatically](#whats-joined-without-asking) in this repository, so nothing happens out of sight.

### Keys

A table of every key: how else it's written, what its values are, how many values and commits, and where it shows.

- **Kind** is what the key's values are. It says "(a guess)" until you choose: people, tickets, commits, labels, identifiers or text. Saying a key holds tickets makes each of its values one, whatever they look like.
- **Shows** is where the key is offered: **On rows** (pinned), **Panel**, or **Held back**. It's a guess too until you choose, and **Why** says what the guess was based on, or whose rule decides.
- **A join** shows on its key's row, with **Undo**.

The table sorts by any column and filters by a key's name, a way it's written, its kind or where it shows. A long history has a couple of thousand keys; the table shows a hundred at a time.

### Where rules are kept

Rules are kept in git config, beside your [smart links](settings.md#links), so they can be read, written by hand, and copied:

```ini
[gitrevenge-trailer "Reviewed-by"]
	also = Reviewd-by
	kind = people
	show = panel
```

| Line | Means |
|---|---|
| `also = Reviewd-by` | Read that key as this one. A rule can have several. |
| `kind = people` | What the values are: `people`, `tickets`, `commits`, `labels`, `identifiers` or `text`. |
| `show = rows` | Pinned to the graph's rows. `panel` offers the key whatever the guess, `held` holds it back, and `none` unpins, in one repository, a key your global config pins. |
| `apart = Refs` | These two were asked about and stay apart. |

Trailer care writes to the repository's own `.git/config`. A rule in your global config works for every repository, and a repository's rule for a key replaces the global one for that key.

## Smart filters and Insights

[Insights](insights.md)' Shipped view groups work by ticket. It reads a commit's trailers by the same rules, so a ticket counted in the panel is the same ticket there, and a key you say holds tickets, or hold back, counts or doesn't in both.

## Good to know

- **The Map and Insights don't take a filter.** A filter is for the graph.
- **What's kept on disk** is a cache of each repository's trailers, in the app's cache folder. It holds only what's in the commits, never leaves your computer, and is cleared when a repository hasn't been opened for ninety days. See [privacy](../privacy.md).
- **A filter isn't a search.** Search moves between matches and leaves the graph as it is. A filter changes what the graph shows.
