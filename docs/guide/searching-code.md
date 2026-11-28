# Searching the code

The graph's [search](graph.md#searching) finds commits by their messages, authors and trailers. Two more searches look inside the files:

- **Find in files** says where a text is: in the working tree now, or at any commit, line by line. It's `git grep`.
- **The history search** says which commits brought a text in or took it out, and filters the graph to them. It's `git log -S`.

The practice repository has a good example. Tidewater's station map labels the wind `km/h`, but the feed has sent knots since October. [Find what broke it](../walkthroughs/find-what-broke.md) tracks the commit down with bisect. These two searches get there from the text alone.

## Find in files

Press <kbd>Ctrl</kbd>+<kbd>Alt</kbd>+<kbd>F</kbd>, or choose **Find in files** on the sidebar's rail, and type. It searches as you type. Lines arrive as git finds them, grouped by file, with each match marked. Text you've selected, or what's in the box you're typing in, comes with it.

![Find in files: km/h found on two lines, and public/map.js open at line 6 with the match marked](../img/find-in-files-light.webp#gh-light-mode-only)
![Find in files: km/h found on two lines, and public/map.js open at line 6 with the match marked](../img/find-in-files-dark.webp#gh-dark-mode-only)

In the working tree, `km/h` is on two lines: line 6 of `public/map.js`, the map's pin, and line 10 of `notebooks/wind.ipynb`. To open a line, click it, or move to it with <kbd>↑</kbd> and <kbd>↓</kbd> and press <kbd>Enter</kbd>. The whole file opens beside the panel at that line, with every match in it marked. Then:

- <kbd>F7</kbd> and <kbd>Shift</kbd>+<kbd>F7</kbd> step through the file's matches, and the header says which one you're on;
- <kbd>Alt</kbd>+<kbd>↓</kbd> and <kbd>Alt</kbd>+<kbd>↑</kbd> go to the next and previous file found;
- **History** opens the file's history, and **Blame** its blame, at the line you're on.

### What it searches

| Option | What it does |
|---|---|
| **Aa** | Match case: `km/h` finds `km/h` but not `KM/H`. |
| **ab** | Whole word: only where the text isn't part of a longer word. |
| **.\*** | Regular expression, read as git reads one (POSIX extended). If the text isn't one, the box turns red and says why. |
| **In** | The working tree, `HEAD`, or a branch, tag or commit you name. In the working tree, tick **untracked files too** to search files git doesn't track yet, though not those it ignores. |
| **Files** | Patterns between commas: `src/` for a folder, `*.js` for a kind of file, and `!*.min.js` to leave files out. |

The line under the options says what it found and how long it took: "2 lines in 2 files · 46 ms". It stops at the first 10,000 lines, and says so; narrow it with **Files**, **Whole word** or **Match case**. A match is within one line, as git's are. It's quick on big repositories: `git grep` searches the Linux kernel's 97,000 files in under a second.

Right-click a file for its **History**, its **Blame**, **Copy the path**, and the history search for that file alone. Right-click a line for **Blame at this line**.

### From the search box

The graph's search box can send its text here. As you type, the second row under it is **Find "km/h" in the files**, with how many lines that finds. Press <kbd>↓</kbd> to it and <kbd>Enter</kbd>, or click it, and the Find panel opens with the text.

When nothing is found, the panel says where it looked, and offers **When was it in the history?**: the history search for the same text, with the same Match case, Regular expression and Files.

## The history search

Find says where `km/h` is. The history search says how it got there.

Type `km/h` in the graph's search box. The third row under it is **Commits that add or remove "km/h"**: press <kbd>↓</kbd> to it and <kbd>Enter</kbd>. A chip, "adds or removes `km/h`", joins the [filter strip](smart-filters.md#filtering-the-graph) above the graph, and the graph filters to the commits that changed how many times `km/h` is in a file. In Tidewater there are seven, across every branch.

![The history search for km/h as Only these: seven commits, each with what it did to km/h, and Show wind in knots selected](../img/history-search-only-light.webp#gh-light-mode-only)
![The history search for km/h as Only these: seven commits, each with what it did to km/h, and Show wind in knots selected](../img/history-search-only-dark.webp#gh-dark-mode-only)

Each commit it keeps says what it did, at the end of its subject:

- **+1 map.js**: it brought the text into `public/map.js`;
- **−1 feed.js**: it took the text out of `src/feed.js`;
- **±1 checks.md**: it changed a line with the text, which stayed (see [its options](#its-options));
- **+3 in 2 files**: the same, across several files.

The marks are in the colours of added and removed lines. Point at one for each file in full: "Takes km/h out of src/feed.js". In Tidewater, *Show wind in knots* is the commit that took `km/h` out of the feed and left it on the map.

Select a commit, and its **Files** list starts with **Where km/h changed**: the files where it changed the text, each with its mark. Open one, and its diff marks the text in both layouts.

There are two more ways to start one, from [Find in files](#find-in-files): **When was it in the history?**, when Find finds nothing, and a file's menu, which has **Commits that add or remove "km/h" here** for that file alone.

### While it runs

The search starts at the newest commits and works back. A line under the chips says how far it has got, and the commits it finds come into the graph as it goes:

- **Searching… 7 found, back to July 2026.** with **Stop**;
- **Stopped at July 2026: 7 found. Older commits weren't searched.** with **Carry on**, which picks up where it stopped;
- **Searched the whole history · 0.4 s** when it's done.

Further back than two months it says the month, and further back than a year, the part of the year: "late 2024".

![A history search under way: seven commits found, every one since July 2026 searched, and Stop](../img/history-search-running-light.webp#gh-light-mode-only)
![A history search under way: seven commits found, every one since July 2026 searched, and Stop](../img/history-search-running-dark.webp#gh-dark-mode-only)

Tidewater's 1,500 commits take under half a second. A big history takes longer: on the Linux kernel, 1.5 million commits, the whole history takes about six minutes, where `git log -S` in a terminal takes about twenty, because the app searches several stretches of history at once. After five seconds, if the search has more than a minute to go, the line says how long the whole history will take and offers to narrow it. **The last year** searches back a year (on the kernel, under a minute), and **Only in a folder…** opens the options at **Only in**. Under **Only these**, the graph shows what's been found a little behind the search: each time it catches up, it reads back only as far as the search has got, so on the kernel it's ten or twenty seconds behind rather than waiting for the end.

### Its options

Click the chip for its options.

![The history search's options: the text, Match case, Regular expression, the two ways a commit counts with their counts, Only in, and As far back as](../img/history-search-options-light.webp#gh-light-mode-only)
![The history search's options: the text, Match case, Regular expression, the two ways a commit counts with their counts, Only in, and As far back as](../img/history-search-options-dark.webp#gh-dark-mode-only)

| Option | What it does |
|---|---|
| **Text** | What to look for. |
| **Match case** | `km/h` but not `KM/H`. |
| **Regular expression** | Read the text as a regular expression, as git reads one. |
| **A commit counts when** | **It appears or goes away** (`git log -S`), the default: the commit changed how many times the text is in a file. **A line with it changes** (`git log -G`): the commit added or removed a line with the text, even if the text stayed. In Tidewater that adds an eighth commit, *Rewrite the check guide*, which realigned a table row with `km/h` in it. Once a way has run, it shows how many commits it found. |
| **Only in** | Patterns between commas, as in Find's **Files**: `src/`, `*.js`, `!*.min.js`. While the timeline is [focused on a folder or file](graph.md#focusing-on-a-folder-or-file), a new search starts with it here. |
| **As far back as** | The whole history, the last year, or a date. |

**Search again** runs it with what you've changed, and **Remove** takes the chip off. <kbd>Esc</kbd> closes the options.

### With the rest of the filter

The chip is a [smart filter](smart-filters.md#filtering-the-graph), so the strip works as it does for trailers:

- **Only these** and **Highlight** both work, with the dashed line, the ruler's ticks and the arrows that step from match to match. The mode is remembered for each repository.
- With trailer values in the filter too, the graph keeps the commits that pass both: with `Changelog: fixed`, the fixes that brought `km/h` in or took it out.
- **Clear** takes off the search and every value. <kbd>Backspace</kbd> in an empty search box takes off the last chip: the trailer values first, then the search.

![The history search as Highlight: every commit is there, the seven keep their ink and their marks, and the ruler has a tick for each](../img/history-search-highlight-light.webp#gh-light-mode-only)
![The history search as Highlight: every commit is there, the seven keep their ink and their marks, and the ruler has a tick for each](../img/history-search-highlight-dark.webp#gh-dark-mode-only)

### Good to know

- **It searches what the graph shows:** every branch, remote branch and tag, less any branch you've [hidden](branch-care.md#hiding-branches), or only the branch you've [focused](graph.md#focusing-on-a-branch) on.
- **Merges don't count**, as in `git log -S`. A merge's changes are its parents', and they're found there. So is a stash, which git keeps as a merge.
- **A match is within one line**, as in Find.
- **It lasts until you remove it** or close the tab, like any filter.

## Partial clones

A partial clone, made with `git clone --filter=blob:none`, downloads a file's contents only when something needs them. Neither search downloads anything: they search what's on your machine, and say where that runs out.

- **Find** searches the working tree as usual. At a commit whose files were never downloaded, it says so instead of searching.
- **The history search** stops at the first commit whose files were never downloaded, says where, and keeps what it found. That line is information, not an error.

## What leaves your machine

Nothing. Both searches run your own `git` over the repository on your disk, and neither downloads anything, even from a partial clone's remote. See [what leaves your machine](install.md#what-leaves-your-machine).
