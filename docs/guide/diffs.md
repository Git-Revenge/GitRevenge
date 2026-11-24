# Reading a diff

Click a file in a commit, or in your uncommitted changes, to see its diff. For a file in one of [47 languages](#languages-read-as-code), the diff reads the code: it tells changes apart from formatting, renames and moves, so a tidy-up doesn't bury the one line that matters.

## Layouts

Pick a layout above the diff:

- **Unified:** one column, with removed and added lines interleaved. This is the layout for [staging hunks and lines](changes.md#staging).
- **Split:** the old and new versions side by side.
- **File:** the whole file as of that version. Some files can be shown rendered here; see [Rendered previews](history.md#rendered-previews).

In **Split**, a **ribbon** runs between the sides. The sides aren't padded to line up: each reads as a plain file, the two scroll together, and a curved band joins each change's old lines to its new ones.

- A change is rose fading to green, and lines only added or only removed are a green or rose wedge.
- A rename's band is ochre, and formatting's is dashed.
- A move is drawn over the others in its own colour, so its band can be followed from one end to the other.
- The change you're on is outlined.

To pad the sides instead, turn the ribbon off in [Settings](settings.md).

A row above the diff counts the lines added and removed and the moves; read as code, it also counts the changes, renames and formatting. <kbd>F7</kbd> and <kbd>Shift</kbd>+<kbd>F7</kbd> step through the changes, as the arrows beside the count do, and the change you're on is marked at its left edge.

Code is syntax highlighted. Images (PNG, JPEG, GIF, WebP, AVIF, BMP, ICO and SVG) are shown as pictures and compared side by side, fitted to the view or at actual size. <kbd>Alt</kbd>+<kbd>Z</kbd> turns word wrap on and off. Press <kbd>Escape</kbd> to go back to the graph.

## Lines and words

Every diff starts from lines, matched up as git does: by the `diff.algorithm` in your git config (Myers when it says nothing), unless you pick Myers, Minimal, Patience or Histogram in [Settings](settings.md). Inside a changed line, the words that changed are marked, so a renamed variable doesn't read as a rewritten line.

## Reading the code

A chip after the file's name says **Read as Go** (or whichever it is). Changes are then found token by token:

- **What changed** is marked by token, so an added argument is marked as `, step time.Duration` and nothing else on the line.
- **Formatting** is told apart from changes, and marked with a dotted underline and a note such as "same value". It covers:
  - whitespace and line breaks;
  - a number written another way (`0x30` and `48`);
  - a string written another way (`'a'` and `"a"`, or an escape and the character it stands for);
  - optional punctuation: a trailing comma, a semicolon, or parentheses that change nothing.

  Code indented into another block, such as Python lines into an `if`, isn't formatting: it shows as [moved into](#moved-code) the new structure.
- **Renames.** A name changed the same way in more than one place, where the old name is gone from the file, is a rename. It has "renamed from out" beside it, and the row above the diff says "rename: out → readings, 4 places". Each rename has its own colour, starting with ochre. Click one to light up every place it changed, on both sides; click it again to stop.
- **Reordered code.** When arguments swap places, `send(station, reading)` to `send(reading, station)`, the one that moved across the other is marked as moved, in teal, not as removed and added.

**Hide formatting** beside the layout buttons takes formatting out of the view: the lines read as unchanged, in their new form, and the row above the diff says how many changes it hid, with a link to show them. Settings can make it the starting point.

Where you stage, unstage or discard, nothing is hidden: you see exactly what will be written. Formatting stands in hunks of its own, tagged **formatting only**, so it can be staged or left on its own. A file whose changes are all formatting says **formatting only** in the commit's file list and in your uncommitted changes.

**Code** and **Lines** switch the view between reading the code and comparing whole lines as git does. Settings sets which a diff starts with.

## Moved code

A block of lines that moved shows once, as a move: "moved to line 9" where it left, and "moved from line 3" where it arrived. Click either note to go to the move. The first move is teal, and further ones take plum, sepia and ochre in turn, so two moves side by side can be told apart; a move is never in the colours of added or removed lines.

Moves are found three ways:

- **Lines moved unchanged**, apart from their indentation: "moved from line 3, unchanged", or "indented differently".
- **Lines moved and edited on the way.** A function moved to the end of the file with one line changed is still a move: "moved from line 3, changed". Its note offers **Compare with the original**, which opens the moved lines against the lines they came from, so the edit shows as an edit.
- **Code wrapped in new structure**, when the file is read as code. Statements wrapped in an `if`, a loop or a `try` stay where they were but sit inside something new. Their lines show as a move, "moved into an if statement", and only the new `if` line and its closing brace are added. If one of them was edited as well, the note says "moved into an if statement, changed" and offers **Compare with the original**.

[Settings](settings.md) can turn finding moves off.

## Languages read as code

- **Programming:** Go, JavaScript, TypeScript, Python, C#, Java, C, C++, Objective-C, Rust, PHP, Ruby, Kotlin, Swift, Dart, Scala, Perl, R, Elixir, Haskell, Zig, Groovy and Lua.
- **Scripts:** shell (sh, bash, zsh and ksh) and PowerShell.
- **Markup and styles:** HTML, Vue, Svelte, XML, Markdown, CSS, SCSS and LESS.
- **Data and config:** JSON, YAML, TOML, INI, SQL, GraphQL, Protocol Buffers, HCL (Terraform) and Nix.
- **Build files:** Dockerfile, Makefile, CMake, `go.mod` and git config files.

A script with no file extension is known by its `#!` line.

Each language's own rules apply:

- **Markdown:** a rewrapped paragraph, a realigned table, `*` and `_` for emphasis, and `-` and `*` for bullets are formatting. A hard break, a table column's alignment, and anything inside a code span are not. A fenced code block is read in the language its info string names, so a Go block reads as Go.
- **YAML:** `'a'` and `"a"` are the same, but `yes` and `'yes'` aren't, and indentation is structure.
- **Shell:** quotes and spacing between words are formatting, but spacing inside a string or a heredoc is not.
- **PowerShell** ignores case, so `Get-ChildItem` and `get-childitem` are the same, except in `$env:` variables.
- **Regular expressions** in JavaScript or TypeScript are read token by token, and every space in them counts.

To read a file as another language, set its `linguist-language` attribute in `.gitattributes` (`*.inc linguist-language=Go`). git's own diff drivers count too, so a file with `diff=golang` is read as Go. A file whose `linguist-language` names a language not read here gets the line diff.

## When a file shows lines instead

Some files can't be read as code in good time, and show the line diff with one quiet line saying why:

- a file over 1 MB;
- one that more than a quarter of doesn't parse;
- one too long to compare token by token.

A file that takes longer than 150 ms to read shows its line diff at once and **Reading as Go** in the chip; the view switches to the code when a longer read finishes, within 3 seconds. If that misses too, **Read it as code anyway** gives it up to a minute. A file with a small parse error is still read as code, and a line says part of it didn't parse.

Conflicts read the code too: see [Resolving conflicts](conflicts.md#suggestions).
