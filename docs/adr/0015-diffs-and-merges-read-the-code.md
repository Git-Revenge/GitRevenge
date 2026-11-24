# 0015 — Diffs and merges read the code, in Go

- Status: Accepted
- Date: 2026-11-23

## Context

The app's diffs compare lines. `frontend/src/lib/diff/lineDiff.ts` runs a Myers diff over lines in the webview. The unified view draws that result. The split view lets `@codemirror/merge` work out its own diff. A line with one changed word shows as entirely changed, a reformatted file shows as rewritten, and moved code shows as one deletion and one addition. The conflict tool (#597) only knows the conflicted places that `git merge-file --diff3` hands it, line by line.

Tools like [SemanticDiff](https://semanticdiff.com/) and [P4Merge](https://www.perforce.com/products/p4/p4-merge) show what people want instead. SemanticDiff understands each language, so formatting doesn't count, `0x10` equals `16`, and moves and renames are found. P4Merge's three-way layout says where each change came from.

Three facts constrain how to build it:

- ADR 0002 says there is no cgo, and the Windows builds have no C compiler (`.github/workflows/ci.yml`). The official tree-sitter bindings need cgo.
- The app is Apache-2.0, so GPL code can't be built into it.
- The Lezer grammars the app ships for highlighting don't build syntax trees for C#, Kotlin, Swift, Ruby, Shell and others. They only colour text.

The research, with sources, is in [`docs/research/semantic-diff-merge.md`](../research/semantic-diff-merge.md).

## Decision

- **Diffs are worked out in Go and drawn by the frontend.** A new backend package diffs two versions and returns ranges for each side: changed, moved, renamed, and formatting only. It also returns a line map. The unified view draws from that result, and the split view passes it to `MergeView` through `DiffConfig.override`. Results are cached by blob id. Work stops when its tab closes. The line diff moves to Go too, and stays the fallback for everything.
- **Code is parsed with [gotreesitter](https://github.com/odvcencio/gotreesitter).** It is a tree-sitter runtime written in Go, MIT-licensed, with no cgo, so ADR 0002 stands unchanged. The version is pinned, and a fixed corpus of files, kernel files included, checks every upgrade for changed results.
- **Grammars are built into the app; nothing downloads at run time.** The larger binary is accepted as part of the feature. Builds use `grammar_subset` tags to pick the languages and `gotreesitter_no_copyleft`, so no copyleft grammar ships.
- **The first languages are SemanticDiff's:** Go, JavaScript, TypeScript, Python, C#, Java, CSS, SCSS, JSON, Rust, Vue, HTML, Lua and XML. More come later. A file in any other language gets the line diff. (#965 added YAML, shell, PowerShell, Markdown and regular expressions, which the user asked for, and 29 common languages and file kinds from gotreesitter's permissively licensed grammars. Markdown's paragraphs are read as prose: words, with whitespace counting by its presence.)
- **The diff compares syntax tokens.** Each version becomes a sequence of leaf tokens, normalised by per-language rules:
  - whitespace is not a token
  - number literals compare by value
  - string escapes compare by what they mean
  - optional parentheses and trailing commas can be ignored

  The sequences are diffed with [znkr.io/diff](https://github.com/znkr/diff) (Apache-2.0). Moves and renames are found in passes afterwards. Full tree matching, GumTree style, comes later if it's needed.

  (Added on 2026-11-24 by #945.) The token diff runs inside each block of the line diff, and splits a block where its parts differ in kind: a change, formatting only, or a rename. So blocks stay git's, and staging keeps working on lines. Formatting only means the two sides' tokens are equal and sit in the same syntax, so code indented into a Python block, or a CSS selector losing the space that made it a descendant, is a change.
- **Hiding formatting-only changes is an option the user turns on.** It is off by default, so a diff shows every change until the user asks otherwise.
- **Staging always shows the end result.** Any view that stages, unstages or discards shows every change that would be written, whatever the formatting option says. Formatting-only changes there can be marked as such, but are never hidden. Staging, unstaging and discarding keep working on lines.
- **Merging stays git's.** git's line merge decides what conflicts, at every merge, rebase, cherry-pick and revert stop. Inside each conflicted place, the tool runs a three-way merge over the syntax tokens of base, ours and theirs. Where the two sides changed different tokens, the combined result is **pre-filled** into the place and marked **suggested**. The user can keep it, change it or choose a side. Until they act, it is not counted as decided. P4Merge-style colouring of where each change came from, and a base column, join the existing views.
- **mergiraf is supported, not bundled.** If the user has set up [mergiraf](https://mergiraf.org/) (GPLv3) as a git merge driver, git runs it as it would any driver, and the conflict tool works on whatever is left. The app never ships it.

  How the merge was built (#946): each version of a place is split into its tokens and the whitespace between them, and the three are merged as diff3 merges lines, with base anchors that both sides keep. A place is often half a function and doesn't parse; it is then split into words, runs of whitespace and single other characters, which merge the same way. A string or comment is one token, so two edits inside one string are a clash, never a merge. A file is only given suggestions when both sides read as code within the 3-second read; the forecast and the strip's tally use the same merge on the places `git merge-tree` leaves in diff3 style.
- **When something goes wrong, the app falls back to the line diff and says so.** It falls back when:
  - either version is over 1 MB, which is checked before parsing;
  - a parse stops at its 150 ms timeout or its 256 MB memory budget, both set on the parser;
  - more than a quarter of either version's bytes sit inside ERROR nodes;
  - either version has more than 200,000 tokens.

  A quiet note says which happened. A smaller parse error doesn't fall back: the tokens around it are still diffed, and the note says part of the file didn't parse. A parser bug can cost the nicer view, never a wrong one. (Changed on 2026-11-23 by #943, which measured the budgets. The first draft fell back on any parse error, which would have hit two-thirds of kernel C files.)

  A file that misses the 150 ms limit shows its line diff at once, and is read again in the background with 3 seconds to parse. The view switches to the code when that finishes. If it misses too, **Read it as code anyway** gives it up to a minute, so no file keeps a core busy for good. (Changed on 2026-11-24 by #945. The user ruled that 2 to 3 seconds is acceptable for large files. It rescues the large C# files #943 found missing the limit, at the cost of a view that changes while it's read.)

## Consequences

- The Windows app grows by 10.8 MB (25.95 to 36.76 MB) and its installer by 4.8 MB (11.69 to 16.46 MB), with gotreesitter and the 15 first-pass grammar packages (the list above plus TSX). The full figures are in [`docs/research/gotreesitter-timing.md`](../research/gotreesitter-timing.md).
- gotreesitter is pre-1.0 and has one main upstream. Pinning the version and testing against the fixed corpus contain that risk. If it fails us, the fallback is the official cgo bindings plus a Windows C toolchain, which would mean revisiting ADR 0002.
- **Speed depends on the grammar's engine** (#943).
  - **ccore grammars:** C, TypeScript, Python, CSS, JSON and Rust run on gotreesitter's certified C-faithful engine. They read 6–22 MB a second, and C parses at 2.3 times the speed of the official cgo bindings. `kernel/sched/core.c` (296 KB) parses in 21 ms, and the largest kernel files under 1 MB in about 100 ms.
  - **Older-engine grammars:** Go, JavaScript, TSX, C#, Java, SCSS, Vue, HTML, Lua and XML mostly read 0.7–4 MB a second. Go is about 3 times slower than cgo, and their trees take 0.3–2 KB of memory per source byte while parsing.
  - **Cost:** the timeout keeps the app responsive. Some files in those languages get the line diff instead: 5 of 400 C# files sampled (46–317 KB), and bundled or minified JavaScript. They speed up as upstream certifies more grammars.
- **Kernel C trips the C grammar often.** 67% of kernel C files have an ERROR node, from macros, but in the median such file it covers 0.2% of the bytes. The official runtime gives the same counts.
- **How the parser is used:** trees are walked with `Node.Children()`, two to three times faster on ccore trees than the cursor. Each parse uses a new `Parser`, since a `Parser` isn't safe for concurrent use. A new one costs no more than one from a `ParserPool` (measured in #945), and it can carry its own time limit and a cancellation flag that stops it when its repository closes, which a pool's parsers can't. Trees are released straight after the walk, and only results are cached.
- **Every build that ships uses the tags in `build/tags`** (#945). Without `gotreesitter_no_copyleft`, gotreesitter links the copyleft Nim scanner's code even though no Nim grammar is imported. A test checks the tags list every grammar the app reads.
- `lineDiff.ts` stops being the diff engine. Its job of rebuilding a file from a chosen set of changes, for staging, has to work from the Go result.
- Highlighting stays with Lezer in the frontend, so the app parses some files twice, once for colour and once for structure.
