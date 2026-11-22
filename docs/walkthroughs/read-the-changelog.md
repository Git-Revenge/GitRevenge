# Read the changelog down the history

Emily is writing the release notes. Tidewater's pull requests say what kind of change they are in a `Changelog` trailer, and the ones that close an issue say so with `Closes`. She wants every fix that closed an issue, and then to read the history with each commit's `Changelog` beside it.

## Steps

1. **Open the Smart filters panel.** Click **Smart filters** in the sidebar's rail. It lists the keys Tidewater's history uses, most commits first: `Changelog`, `Reviewed-by`, `Closes`, `Co-authored-by`. Under each are its values, with how many commits have each.

2. **Filter to the fixes.** Under `Changelog`, click **fixed**. The graph now holds only the commits whose trailer says `Changelog: fixed`, on one dashed line, and a strip above it says how many: "171 of 1,493 commits".

3. **Find the ones that closed an issue.** Press <kbd>Ctrl</kbd>+<kbd>F</kbd> and type `has:closes`. Search works inside a filter: of the fixes, the 55 that have a `Closes` trailer keep their ink and the rest fade. <kbd>Enter</kbd> steps through them. Each is a merge whose message says which issue it closed.

4. **Read why a commit is there.** With one selected, open its **Trailers** tab. The `Changelog: fixed` line is marked: it's the line the filter picked.

5. **See the fixes in place.** Clear the search, and in the strip switch **Only these** to **Highlight**. Every commit is back. The fixes keep their ink and the rest fade, with a tick for each on the ruler, and a day's line says how many of its commits match.

6. **Clear the filter.** Click **Clear** in the strip.

7. **Pin Changelog.** In the panel, point at `Changelog` and click its pin. Each commit that has the trailer now shows it after its subject, as a chip: `Changelog fixed`, `Changelog none`. Scroll, and the kind of change reads down the history.

8. **Filter from a chip.** Click a chip. The graph filters to the commits with that value, ready for that section of the notes.

## Good to know

- **Values of one key are either-or; different keys must all hold.** Picking `fixed` and `added` keeps commits with either. Picking `fixed` and a reviewer keeps the fixes that person reviewed.
- **In Compact, a pinned key gets a column** with the key in the heading, when the window is wide enough.
- **A pin is a line in the repository's git config**, so it's there next time: `[gitrevenge-trailer "Changelog"]` with `show = rows`. The same pin unpins.
- **Tidewater spells one key two ways.** Eleven commits say `Reviewd-by`. The panel shows **Needs care 1**: click it, and Trailer care asks whether to read `Reviewd-by` as `Reviewed-by`. [Smart filters](../guide/smart-filters.md#trailer-care) covers it.
- **Nothing here changes a commit.** A filter is a way of reading what's already written.
