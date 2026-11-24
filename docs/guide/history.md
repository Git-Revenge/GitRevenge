# Diffs and history

## Diffs

Click a file in a commit, or in your uncommitted changes, to see its diff. Pick a layout:

- **Unified:** one column, with removed and added lines interleaved. This is the layout for [staging hunks and lines](changes.md#staging).
- **Split:** the old and new versions side by side.
- **File:** the whole file as of that version.

Inside a changed line, the words that changed are marked, so a renamed variable doesn't read as a rewritten line. A block of lines that moved shows once, as a move: "moved to line 9" where it left, and "moved from line 3" where it arrived. Click either note to go to the move. The first move is teal, and further ones take plum, sepia and ochre in turn, so two moves side by side can be told apart; a move is never in the colours of added or removed lines. See [Moved code](#moved-code) for moves that changed on the way. A row above the diff counts the lines added and removed and the moves. <kbd>F7</kbd> and <kbd>Shift</kbd>+<kbd>F7</kbd> step through the changes, as the arrows beside the count do, and the change you're on is marked at its left edge.

In **Split**, a **ribbon** runs between the sides. The sides aren't padded to line up: each reads as a plain file, the two scroll together, and a curved band joins each change's old lines to its new ones. A change is rose fading to green, lines only added or only removed are a green or rose wedge, and a move is drawn over the others in its own colour, so its band can be followed from one end to the other. The change you're on is outlined. To pad the sides instead, turn the ribbon off in [Settings](settings.md).

Lines are matched up as git does: by the `diff.algorithm` in your git config (Myers when it says nothing), unless you pick Myers, Minimal, Patience or Histogram in [Settings](settings.md).

### Reading the code

Files in these languages are read as code:

- **Programming:** Go, JavaScript, TypeScript, Python, C#, Java, C, C++, Objective-C, Rust, PHP, Ruby, Kotlin, Swift, Dart, Scala, Perl, R, Elixir, Haskell, Zig, Groovy and Lua.
- **Scripts:** shell (sh, bash, zsh and ksh) and PowerShell.
- **Markup and styles:** HTML, Vue, Svelte, XML, Markdown, CSS, SCSS and LESS.
- **Data and config:** JSON, YAML, TOML, INI, SQL, GraphQL, Protocol Buffers, HCL (Terraform) and Nix.
- **Build files:** Dockerfile, Makefile, CMake, `go.mod` and git config files.

A script with no file extension is known by its `#!` line. A chip after the file's name says **Read as Go** (or whichever it is), and changes are found token by token:

- **What changed** is marked by token, so an added argument is marked as `, step time.Duration` and nothing else on the line.
- **Formatting** is told apart from changes, and marked with a dotted underline and a note such as "same value". It covers whitespace and line breaks, a number written another way (`0x30` and `48`), a string written another way (`'a'` and `"a"`, or an escape and the character it stands for), and optional punctuation: a trailing comma, a semicolon, or parentheses that change nothing. Code indented into another block, such as Python lines into an `if`, isn't formatting: it shows as [moved into](#moved-code) the new structure.
- **Each language's own rules** apply. In Markdown, a rewrapped paragraph, a realigned table, `*` and `_` for emphasis, and `-` and `*` for bullets are formatting. A hard break, a table column's alignment, and anything inside a code span are not. A fenced code block is read in the language its info string names, so a Go block reads as Go. In YAML, `'a'` and `"a"` are the same, but `yes` and `'yes'` aren't, and indentation is structure. In shell, quotes and spacing between words are formatting, but spacing inside a string or a heredoc is not. PowerShell ignores case, so `Get-ChildItem` and `get-childitem` are the same, except in `$env:` variables. A regular expression in JavaScript or TypeScript is read token by token, and every space in it counts.
- **Renames** are told apart too. A name changed the same way in more than one place, where the old name is gone from the file, is a rename. It has "renamed from out" beside it, and the row above the diff says "rename: out → readings, 4 places". Each rename has its own colour, starting with ochre, and clicking one lights up every place it changed, on both sides; click it again to stop. In the split view's ribbon, a rename's band is ochre and formatting's is dashed.
- **Reordered code** is told apart from changed code. When arguments swap places, `send(station, reading)` to `send(reading, station)`, the one that moved across the other is marked as moved, in teal, not as removed and added.

**Hide formatting** beside the layout buttons takes formatting out of the view: the lines read as unchanged, in their new form, and the row above the diff says how many changes it hid, with a link to show them. Settings can make it the starting point. Where you stage, unstage or discard, nothing is hidden: you see exactly what will be written, and formatting stands in hunks of its own, tagged **formatting only**, so it can be staged or left on its own. A file whose changes are all formatting says **formatting only** in the commit's file list and in your uncommitted changes.

**Code** and **Lines** switch the view between reading the code and comparing whole lines as git does. Settings sets which a diff starts with.

A file over 1 MB, one that more than a quarter of doesn't parse, or one too long to compare token by token shows the line diff, with one quiet line saying why. A file that takes longer than 150 ms to read shows its line diff at once and **Reading as Go** in the chip; the view switches to the code when a longer read finishes, within 3 seconds. If that misses too, **Read it as code anyway** gives it up to a minute. A file with a small parse error is still read as code, and a line says part of it didn't parse.

#### Moved code

Moves are found in three ways, whether or not the file is read as code:

- **Lines moved unchanged**, apart from their indentation, as before: "moved from line 3, unchanged".
- **Lines moved and edited on the way.** A function moved to the end of the file with one line changed is still a move: "moved from line 3, changed". Its note offers **Compare with the original**, which opens the moved lines against the lines they came from, so the edit shows as an edit.
- **Code wrapped in new structure**, read as code. Statements wrapped in an `if`, a loop or a `try` stay where they were but sit inside something new. Their lines show as a move, with "moved into an if statement", and only the new `if` line and its closing brace are added. Indenting Python lines into an `if` reads the same way.

[Settings](settings.md) can turn finding moves off.

To read a file as another language, set its `linguist-language` attribute in `.gitattributes` (`*.inc linguist-language=Go`). git's own diff drivers count too, so a file with `diff=golang` is read as Go. A file whose `linguist-language` names a language not read here gets the line diff.

Code is syntax highlighted. Images (PNG, JPEG, GIF, WebP, AVIF, BMP, ICO and SVG) are shown as pictures and compared side by side, fitted to the view or at actual size. <kbd>Alt</kbd>+<kbd>Z</kbd> turns word wrap on and off. Press <kbd>Escape</kbd> to go back to the graph.

## Rendered previews

Some files read better rendered than as text. In the **File** layout, these get a **Rendered** option, and the app remembers whether you left it on:

| Files | Shown as |
|---|---|
| Markdown (`.md`, `.markdown`) | formatted text, including LaTeX math and Mermaid diagrams |
| MDX (`.mdx`) | the same, showing the Markdown it contains. Imports, exports and frontmatter are hidden, a component shows what's inside it, and one with nothing inside shows its name. None of its code runs |
| CSV and TSV (`.csv`, `.tsv`, `.tab`) | a table you can sort by any column |
| JSON, YAML and TOML | a tree you can expand and search |
| LaTeX (`.tex`, `.latex`) | typeset math |
| Mermaid (`.mmd`, `.mermaid`) | the diagram |
| PDF | the document |
| AsciiDoc (`.adoc`, `.asciidoc`, `.asc`) and reStructuredText (`.rst`, `.rest`) | formatted text: titles, sections, lists, links, images, code blocks, quotes, admonitions and tables. Anything else stays as text, and an include names the file it would have brought in |
| Jupyter notebooks (`.ipynb`) | the Markdown cells formatted, and each code cell with the outputs the notebook stored: text, images, errors and tables. Nothing runs |
| HTML (`.html`, `.htm`, `.xhtml`) | the page, in a frame where no script, form or link works. Images from the repository show; anything it would fetch from the web shows as its alt text instead |
| GeoJSON (`.geojson`) | its shapes, drawn over lines of latitude and longitude, with a list of features beside them. Click one for its properties. The wheel zooms and a drag pans. There's no map underneath, since that would mean fetching tiles from someone's server |
| Audio and video (MP3, WAV, Ogg, Opus, FLAC, M4A, AAC, MP4, WebM, MOV and more) | a player, with the length, frame size and file size. It plays only when you press play, and whether a format plays depends on your system |

![A README shown rendered](../img/preview-markdown-light.webp#gh-light-mode-only)
![A README shown rendered](../img/preview-markdown-dark.webp#gh-dark-mode-only)
![A CSV file shown as a table](../img/preview-csv-light.webp#gh-light-mode-only)
![A CSV file shown as a table](../img/preview-csv-dark.webp#gh-dark-mode-only)
![A JSON file shown as a tree you can search](../img/preview-json-light.webp#gh-light-mode-only)
![A JSON file shown as a tree you can search](../img/preview-json-dark.webp#gh-dark-mode-only)
![A Mermaid diagram, drawn](../img/preview-mermaid-light.webp#gh-light-mode-only)
![A Mermaid diagram, drawn](../img/preview-mermaid-dark.webp#gh-dark-mode-only)
![A Jupyter notebook, with its stored outputs](../img/preview-notebook-light.webp#gh-light-mode-only)
![A Jupyter notebook, with its stored outputs](../img/preview-notebook-dark.webp#gh-dark-mode-only)
![A GeoJSON file drawn as its shapes, with a feature's properties](../img/preview-geojson-light.webp#gh-light-mode-only)
![A GeoJSON file drawn as its shapes, with a feature's properties](../img/preview-geojson-dark.webp#gh-dark-mode-only)
![An HTML page in a sandboxed frame](../img/preview-html-light.webp#gh-light-mode-only)
![An HTML page in a sandboxed frame](../img/preview-html-dark.webp#gh-dark-mode-only)

Nothing a preview shows reaches the network. Audio and video of up to 32 MB open straight away, without the "Load anyway" step other large files ask for.

## File history and blame

From a file's diff, **History** lists every commit that changed it, and **Blame** shows who last changed each line and in which commit.

![Blame: each line with the commit and author that last changed it](../img/blame-light.webp#gh-light-mode-only)
![Blame: each line with the commit and author that last changed it](../img/blame-dark.webp#gh-dark-mode-only)


## The reflog

The reflog is git's record of where HEAD and each branch have been. It's how you find a commit you thought you'd lost to a reset or a rebase. Choose **Show the reflog…** in the command palette for HEAD's, or right-click a branch for its own.

![The reflog](../img/reflog-light.webp#gh-light-mode-only)
![The reflog](../img/reflog-dark.webp#gh-dark-mode-only)

[Undo a mistake](../walkthroughs/undo-a-mistake.md) walks through recovering a commit with it.

## Finding the commit that broke something

When something worked once and doesn't now, a bisect finds the commit that changed it. You name one commit where it's broken and one where it worked, and Git Revenge checks out commits in between for you to try, halving the range with each answer.

Right-click the broken commit and choose **Find what broke it, up to here…**, or choose **Find the commit that broke it…** in the command palette to start from the commit you're on. The dialog takes the two ends. It suggests the last tag as the good one, and says how many commits lie between them and about how many steps that is. Your working tree has to be clean enough for git to check out other commits.

While a bisect runs, a strip under the tabs asks about the commit that's checked out. Build it, try the thing that's broken, then answer:

- **Good** (<kbd>G</kbd>) if it works there.
- **Bad** (<kbd>B</kbd>) if it's broken there.
- **Can't tell** (<kbd>S</kbd>) if that commit won't build or can't be tested. Git picks another nearby.

**Undo last** takes back your last answer and checks that commit out again. **Stop…** ends the bisect and returns you to where you started.

The graph shows where things stand. Commits you've answered for carry a tick or a cross, the commit under test has a ring and the note "testing this one", the commits still in question have a teal band, and everything that's been ruled out fades. The panel on the right has the same three answers in words, what the commit changed, and your answers so far.

When the answer is known, the strip says how many steps it took and the graph marks the commit "it broke here". The panel shows what that commit changed and offers to open its changes, start a branch for the fix, or revert it. **Copy the log** copies your answers as text for an issue. **Done** returns you to your branch. If the commits you skipped hide the answer, you're told which commits it could be instead.

![A bisect in progress: the strip asks about the commit under test, and the graph marks the answers so far](../img/bisect-step-light.webp#gh-light-mode-only)
![A bisect in progress: the strip asks about the commit under test, and the graph marks the answers so far](../img/bisect-step-dark.webp#gh-dark-mode-only)

The walkthrough [Find what broke it](../walkthroughs/find-what-broke.md) goes through one from start to finish.

A bisect lives in the repository, not the app: one started with `git bisect` in a terminal shows up here, and one started here carries on in a terminal.

## Notes

Git notes attach extra text to a commit without changing it. The details panel shows a selected commit's notes, and lets you add, edit or remove one. A note is shown as Markdown, the way a commit message is: issue numbers, SHAs and links in it are clickable, and a single line break stays a line break.

In the graph, a commit with a note wears a folded-page mark. Hover over the mark to read the note, or click it to open the commit with its notes in view. Notes in every notes ref count. Type `has:note` in the graph's [search](graph.md#searching) to step through the commits that have one.

git doesn't fetch or push notes on its own. To share them, turn on [syncing notes](branch-care.md#syncing-notes) for the repository.

## Signatures

A signed commit shows a badge in its details:

| Badge | Meaning |
|---|---|
| Verified | The signature is good, from a key you trust. |
| Can't verify | The signature may be fine, but your machine has no trusted key to check it against. |
| Bad signature | The commit doesn't match its signature. |
| Expired | The signature or its key has expired. |
| Revoked | The key has been revoked. |

### Why a signature says "Can't verify"

Git checks signatures with your own keys and settings, so this usually means something is missing locally rather than wrong with the commit.

- **SSH-signed commits** need an allowed-signers file listing whose key is whose. Point git at one with `gpg.ssh.allowedSignersFile`, and add a line for each signer:

  ```sh
  git config --global gpg.ssh.allowedSignersFile ~/.ssh/allowed_signers
  echo "you@example.com $(cat ~/.ssh/id_ed25519.pub)" >> ~/.ssh/allowed_signers
  ```

- **Commits merged on GitHub** are signed by GitHub's own GPG key. Import it, then tell GPG you trust it:

  ```sh
  curl -s https://github.com/web-flow.gpg | gpg --import
  gpg --edit-key noreply@github.com trust
  ```

The commit's details show the result as soon as git can verify it.
