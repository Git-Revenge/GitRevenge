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
- **The first languages are SemanticDiff's:** Go, JavaScript, TypeScript, Python, C#, Java, CSS, SCSS, JSON, Rust, Vue, HTML, Lua and XML. More come later. A file in any other language gets the line diff.
- **The diff compares syntax tokens.** Each version becomes a sequence of leaf tokens, normalised by per-language rules:
  - whitespace is not a token
  - number literals compare by value
  - string escapes compare by what they mean
  - optional parentheses and trailing commas can be ignored

  The sequences are diffed with [znkr.io/diff](https://github.com/znkr/diff) (Apache-2.0). Moves and renames are found in passes afterwards. Full tree matching, GumTree style, comes later if it's needed.
- **Hiding formatting-only changes is an option the user turns on.** It is off by default, so a diff shows every change until the user asks otherwise.
- **Staging always shows the end result.** Any view that stages, unstages or discards shows every change that would be written, whatever the formatting option says. Formatting-only changes there can be marked as such, but are never hidden. Staging, unstaging and discarding keep working on lines.
- **Merging stays git's.** git's line merge decides what conflicts, at every merge, rebase, cherry-pick and revert stop. Inside each conflicted place, the tool runs a three-way merge over the syntax tokens of base, ours and theirs. Where the two sides changed different tokens, the combined result is **pre-filled** into the place and marked **suggested**. The user can keep it, change it or choose a side. Until they act, it is not counted as decided. P4Merge-style colouring of where each change came from, and a base column, join the existing views.
- **mergiraf is supported, not bundled.** If the user has set up [mergiraf](https://mergiraf.org/) (GPLv3) as a git merge driver, git runs it as it would any driver, and the conflict tool works on whatever is left. The app never ships it.
- **When something goes wrong, the app falls back to the line diff and says so.** It falls back when a file is over 1 MB, when either version doesn't parse, or when the work runs past its time or size budget. A quiet note says which happened. A parser bug can cost the nicer view, never a wrong one.

## Consequences

- The app binary grows by about 10 MB for the runtime and about 5 MB for 22 major grammars (measured stripped, windows/amd64). The figure for the first-pass list still has to be measured. Installers grow less, since they're compressed.
- gotreesitter is pre-1.0 and has one main upstream. Pinning the version and testing against the fixed corpus contain that risk. If it fails us, the fallback is the official cgo bindings plus a Windows C toolchain, which would mean revisiting ADR 0002.
- Parse speed on our own files hasn't been measured yet. Timing kernel files comes first, before the budgets above are final.
- `lineDiff.ts` stops being the diff engine. Its job of rebuilding a file from a chosen set of changes, for staging, has to work from the Go result.
- Highlighting stays with Lezer in the frontend, so the app parses some files twice, once for colour and once for structure.
