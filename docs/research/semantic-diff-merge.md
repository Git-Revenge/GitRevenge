# Semantic diff and merge: research

- Date: 2026-11-23
- Status: decided on 2026-11-23; see [ADR 0015](../adr/0015-diffs-and-merges-read-the-code.md). The open questions near the end now carry their answers.

## The question

What are Git Revenge's realistic options for building structure-aware ("semantic") diffs, like [SemanticDiff](https://semanticdiff.com/), and a three-way merge and conflict tool, like [P4Merge](https://www.perforce.com/products/p4/p4-merge), into the app? The work has to cover plain diffs as well as conflicts from merges, rebases and cherry-picks. It should be fast, preferably native Go, aware of the syntax of most major languages, and drawn in the Ink and Agency design.

## Short answer

1. **Parse in Go with [gotreesitter](https://github.com/odvcencio/gotreesitter).** It is a pure-Go tree-sitter runtime: MIT licence, 206 grammars, no cgo. That keeps [ADR 0002](../adr/0002-system-git-cli.md)'s "There is no cgo" true, and our Windows builds have no C compiler (`.github/workflows/ci.yml`, the comment above `go test`). Every other way of parsing in Go either needs cgo, covers too few languages, or is pre-release (see the parsing table below).
2. **Diff syntax tokens, not whole trees.** Turn each version into a sequence of leaf tokens and normalise them with per-language rules: whitespace isn't a token, `0x10` equals `16`, and optional parentheses and trailing commas don't count. Diff those sequences with a fast Go sequence diff ([znkr.io/diff](https://github.com/znkr/diff), Apache-2.0). Detect moved blocks and group renames afterwards. The plain line diff stays as the fallback. Full tree matching (GumTree, difftastic) costs much more and can wait (see the diff table below).
3. **Draw with what the app already has.** `@codemirror/merge` accepts an externally computed diff through `DiffConfig.override`, added in 6.12.0; we ship 6.12.3. The split view can therefore show the Go result unchanged, and the unified view reads the same result. The conflict tool (#597) already has the P4Merge-like layout of versions above and an editable result below. What it lacks is P4Merge's colouring of where each change came from, and syntax-aware suggestions.
4. **Merging stays git's line merge, with syntax-aware suggestions on top.** For each conflicted place, run a three-way merge over syntax tokens of the base, ours and theirs the backend already reads. Where the two sides changed different tokens, offer the combined result as a suggestion the user accepts. A full structured merge in the style of mergiraf or Spork is a later, larger step. Mergiraf works with the app today if the user has set it up as a git merge driver. We can't bundle it into an Apache-2.0 app without settling its GPLv3 licence first.

**The main risk is the dependency.** gotreesitter is pre-1.0 (v0.58.0, released 2026-11-20). Only 34 of its grammars are certified to produce the same trees as the C runtime. It also makes the app binary bigger: I measured about 10 MB stripped for the runtime and about 15 MB with 22 major grammars. A parse error or a timeout always falls back to the line diff, so a parser bug costs a nicer view, never a wrong one.

## What the app does today

| Piece | Where | What it does |
|---|---|---|
| Line diff | `frontend/src/lib/diff/lineDiff.ts` | Myers O(ND) over lines in TypeScript. Above an edit distance of 4000 it shows everything replaced (`MAX_EDIT_DISTANCE`). It also builds hunks, and `applyChanges` rebuilds a file from a subset of changes for staging, unstaging and discarding lines. |
| Unified view | `frontend/src/lib/diff/UnifiedDiff.svelte` | Custom rendering from `lineDiff.ts`. No highlighting of changes within a line, and no whitespace option. |
| Split view | `frontend/src/lib/diff/SplitDiff.svelte` | `@codemirror/merge` `MergeView`, read-only, `collapseUnchanged: { margin: 3, minSize: 6 }`, with CodeMirror's own diff. |
| Highlighting | `frontend/src/lib/diff/highlight.ts` | Lezer grammars from `@codemirror/language-data`, skipped above 1 MiB (`MAX_HIGHLIGHT`). |
| Conflict data | `internal/git/conflict.go` | `ConflictFile` carries `Base`, `Ours` and `Theirs` (index stages 1–3) and `Merged`. Conflicted places come from `git merge-file -p --diff3` (line 86), whatever `merge.conflictStyle` is set to. |
| Conflict tool | `frontend/src/lib/diff/ConflictView.svelte`, `places.ts`, `placesDeco.ts` | The result is the main editable view, never showing markers. Modes are Result only, Ours and theirs, and Three-way. Each place has a status: undecided, ours, theirs, both or edited. It can also hand off to the user's mergetool (`internal/git/conflictctx.go`). |
| Rebases | `docs/walkthroughs/resolve-a-conflict.md` | A rebase can stop once per conflicting commit. Each stop goes through the same conflict tool. |

Facts that shape the design:

- **Lezer can't be the structural layer.** In `@codemirror/language-data`, C#, Kotlin, Scala, Ruby, Swift, Shell and TOML load `@codemirror/legacy-modes`. Those are CodeMirror 5-style stream parsers (`StreamLanguage`, `node_modules/@codemirror/language/dist/index.d.ts`) that give highlighting tokens but no real syntax tree. Most of the file's 143 languages load legacy modes. Lezer stays the highlighter.
- **The backend already has all three versions of a conflicted file.** A structured merge never needs to rebuild the base from conflict markers. Mergiraf's `solve` mode has to, which is why it breaks with `zdiff3` (see the merge section).
- **Staging works on lines.** `applyChanges` rebuilds files line by line. A diff that hides formatting changes must still let the user stage them, so whatever the semantic view hides must map back to lines (see the open questions).

## Features worth copying

### SemanticDiff

SemanticDiff is closed source, a freemium product from Sysmagine GmbH, shipped as a VS Code extension that runs locally and as a GitHub app ([blog comparison](https://semanticdiff.com/blog/semanticdiff-vs-difftastic)). Nothing about how it is built is public. Its docs say only that it "requires specific rules and logic for each supported programming language" ([what is SemanticDiff](https://semanticdiff.com/docs/what-is-semanticdiff/)). Its fallback "uses the same algorithms as all our other diffs", treating each line as one token ([fallback diff](https://semanticdiff.com/docs/understand-diff/fallback-diff/)). That suggests, but doesn't prove, that its main diff is also a sequence diff over tokens.

Behaviours to copy:

- **Hide changes that don't matter:** whitespace and line breaks, optional commas, and unnecessary parentheses ([what is SemanticDiff](https://semanticdiff.com/docs/what-is-semanticdiff/)).
- **Per-language equivalences** ([invariances](https://semanticdiff.com/docs/invariances/)). For most languages: the base of an integer literal, string escapes, and redundant parentheses. More specific ones: Python keyword-argument order and split or joined strings; JS/TS object-key order and arrow versus anonymous functions; Go type-term order; HTML attribute and class order and whitespace collapsing; Rust `...` versus `..=`.
- **Moved code** ([moved code](https://semanticdiff.com/docs/understand-diff/moved-code/)). A move must cover at least one complete line. Each move gets its own border colour and a line linking the old and new places, and an unchanged move is labelled "Moved without changes". "Compare With Original" shows what changed inside a moved block.
- **Rename grouping** ([refactoring grouping](https://semanticdiff.com/docs/understand-diff/refactoring-grouping/)). Every occurrence of the same rename shares a colour. The docs say this is "currently limited to renames".
- **Colour rules** ([colors](https://semanticdiff.com/docs/understand-diff/colors/)). Whole added or removed lines get a strong fill. Partial changes highlight only the changed part, not the rest of the line. Moved targets get a lighter fill.
- **Visible fallback.** If a file doesn't parse, it shows a plain text diff with a warning, or fails if fallback is off ([limitations](https://semanticdiff.com/docs/limitations/)). There is also an option to hide changes in comments ([docs index](https://semanticdiff.com/docs/)).
- **Language list** ([homepage](https://semanticdiff.com/)): Go, JavaScript, TypeScript, Python, C#, Java, CSS, SCSS, JSON, Rust, Vue, HTML, Lua and XML. The invariances page adds Swift, JSON5 and gettext `.po`.

### P4Merge

From the [P4Merge manual](https://help.perforce.com/helix-core/server-apps/p4merge/current/Content/P4Merge/diff-merge.p4merge.html) and [product page](https://www.perforce.com/products/p4/p4-merge):

- **Three-way layout.** The base sits in the middle of the top half with the two changed versions either side, and the merge result fills the bottom half. Our conflict tool already has versions above and the result below.
- **Every chunk says where it came from.** The legend has entries like "Same in theirs and base.", "Text added to your file. Not in theirs or base.", "Text deleted from their file.", "Same in yours and theirs, different from base." and "Added to both yours and theirs.". Each source has its own colour: purple for theirs, green for yours, yellow for base.
- **Toggle buttons per chunk** ("click once to select the corresponding text, click again to deselect it") and per whole file. The result stays directly editable, with hand edits shown in grey.
- **Toolbar:** next and previous diff, find, and go to line. The [2-way diff page](https://help.perforce.com/helix-core/server-apps/p4merge/current/Content/P4Merge/diff-merge.text_diff.html) offers side-by-side or a single pane, where deleted text is struck through.
- **Also:** image overlay diff, folder diff, ignoring line endings and whitespace, and syntax highlighting ([product page](https://www.perforce.com/products/p4/p4-merge)). It is free.

Two of these are worth adding to what we have: colouring each conflicted place by where its text came from, and showing the base as its own column. We already have a Three-way mode. The other P4Merge features are already in the app.

### Git's own building blocks

These are worth exposing, since we drive the git CLI anyway ([git diff](https://git-scm.com/docs/git-diff), [git config](https://git-scm.com/docs/git-config)):

- `--diff-algorithm=histogram|patience|minimal`
- `--color-moved` with modes such as `zebra` and `dimmed-zebra`, and `--color-moved-ws=allow-indentation-change`
- `--anchored=<text>`
- `merge.conflictStyle=zdiff3`, which "removes matching lines on the two sides from the conflict region"

## Options by layer

### 1. Parsing

| Option | How it runs | Windows build | Languages from our list | Licence | Speed (source) | Maturity | Verdict |
|---|---|---|---|---|---|---|---|
| **[gotreesitter](https://github.com/odvcencio/gotreesitter)** | Pure Go. Runs upstream tree-sitter parse tables, with external scanners ported to Go (119 of 206 grammars have one) | No change | All 22 checked: Go, C, C++, C#, Java, JS, TS, TSX, Python, Rust, Ruby, PHP, Kotlin, Swift, Scala, JSON, YAML, TOML, CSS, HTML, Bash, Markdown ([languages](https://gotreesitter.m31labs.dev/docs/languages)). Also SQL. | MIT. Four copyleft grammars are excluded with `-tags gotreesitter_no_copyleft` | Self-reported: certified grammars 0.3–0.8× C time on small inputs; a frozen 4-file Go run at 4.8× C; v1 target "median at most 3x C" ([performance](https://gotreesitter.m31labs.dev/docs/performance)) | v0.58.0, 2026-11-20, pre-1.0. 34 grammars "C-faithful" by default, the rest on an older engine ([README](https://github.com/odvcencio/gotreesitter)). 586 stars, 5k commits | **Recommended** |
| [tree-sitter/go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter) (official) | cgo over the C runtime. One Go module per grammar. Optional purego loading of shared libraries (Linux and macOS examples only) | Needs a C toolchain on Windows CI and release builds; goes against ADR 0002 | Any upstream grammar | MIT | Native C | Official. You must call `Close` on every object because of "bugs with runtime.SetFinalizer and CGO" | Fallback if gotreesitter fails us |
| [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter) | cgo, grammars bundled | Same as official | About 30 in the repo; no JSON | MIT | Native C | Not archived; no tagged releases | No: cgo, and the official bindings exist |
| [malivvan/tree-sitter](https://github.com/malivvan/tree-sitter) | tree-sitter compiled to WASM, run by wazero | No change | Grammars from smacker | MIT | No numbers | "pre release software", 3 commits | No |
| [bonsai](https://pkg.go.dev/github.com/msuozzo/bonsai) | Grammars compiled to WASM, then to Go by wasm2go | No change | 18 modules; no C#, C++, PHP, Swift, Scala, JSON, TOML, HTML or CSS | MIT | No speed numbers; "+0.7 MB" to "+4.6 MB" per grammar | v0.4.x | No: too few languages |
| [dcosson/treesitter-go](https://github.com/dcosson/treesitter-go) | Pure Go, generated tables | No change | 15; no C#, PHP, Kotlin, Swift, Scala, YAML, TOML, Markdown or SQL | MIT | No numbers | 0.1.0, 5 stars; passes all 1619 upstream corpus tests | No: too few languages |
| [web-tree-sitter](https://github.com/tree-sitter/tree-sitter/tree/master/lib/binding_web) in the frontend | WASM in the webview | No change | Any grammar compiled to WASM | MIT | Native-ish | Official | No: keeps the work in JS, adds WASM to the frontend bundle (budget, #850), and the merge work wants Go |
| Lezer (already shipped) | JS in the webview | — | Real trees only where a Lezer grammar exists; C#, Kotlin, Scala, Ruby, Swift, Shell and TOML are stream modes with no tree (see above) | MIT | Fast | Shipped | Keep for highlighting only |

**Measured on this machine** (Go 1.27.4, windows/amd64, `CGO_ENABLED=0`, `-ldflags "-s -w"`):

- A hello-world binary is 1.66 MB.
- With gotreesitter and only the Go grammar (`-tags 'grammar_subset grammar_subset_go'`, per [build tags](https://github.com/odvcencio/gotreesitter/blob/main/docs/build-tags.md)) it is 11.68 MB.
- With the 22 grammars above as a subset it is 16.36 MB.
- With the full 206-grammar catalog it is 31.79 MB.

So the runtime costs about 10 MB and the major grammars about 5 MB more. For comparison, the v1.1.0 Windows installer is 11.6 MB and the macOS universal DMG is 20.4 MB (`gh release view v1.1.0`). Installers are compressed, so their growth will be smaller than the raw numbers, but I didn't measure it. The `grammar_blobs_external` tag can load grammars from a folder at run time, which allows "download languages on first use" if size matters.

I didn't time parses locally: running the freshly built benchmark binary was blocked by this session's permission checks. The speed column therefore rests on the project's own figures. A parse benchmark on kernel files (`kernel/sched/core.c` is 302 KB) should be the first task of any spike.

### 2. Diff algorithm

| Option | What it gives | Cost | Licence and form | Verdict |
|---|---|---|---|---|
| Line diff (today, TS Myers) | Lines only | O(ND); falls back to replace-all at 4000 edits | Ours | Keep as the fallback; move to Go with the rest |
| **[znkr.io/diff](https://github.com/znkr/diff)** | Generic over any slice (`Edits`, `EditsFunc` with a custom equality, `Hunks`); `textdiff.Unified` with an indent heuristic | Default O(N^1.5 log N), `Minimal()` O(ND), `Fast()` O(N log N); README: usually under 1 ms on Go-repo commits, rare outliers around 100 ms | Apache-2.0, v1.1.x, stable API | **Recommended** engine for both lines and tokens. No histogram or patience mode |
| **Token-sequence diff over syntax leaves** (ours, on top of the two above) | Formatting-insensitive diffs, per-language equivalences through normalised tokens, changes aligned to syntax, moves and renames as extra passes | Linear tokenising plus a sequence diff; parsing dominates | Ours | **Recommended** core |
| [GumTree](https://github.com/GumTreeDiff/gumtree) tree matching | Insert, delete, update and move of tree nodes; a greedy top-down pass matching identical subtrees, a bottom-up pass using the dice similarity of descendants, then an optimal edit script on small subtrees ([Falleri et al., ASE 2014](https://www.labri.fr/perso/xblanc/data/papers/ASE14.pdf), §3) | "worst-case complexity of O(n²)" (§3.3). Mean 20 ms on Jenkins and 74 ms on jQuery file pairs; median 18–30× a text diff, against about 10× for parsing alone (§4.2). The paper used `minHeight = 2`, `minDice = 0.2`, `maxSize = 100` | Java, LGPL-3.0. No credible Go port (pkg.go.dev search) | Later: for moves and renames inside structure, written in Go from the paper |
| [difftastic](https://difftastic.wilfred.me.uk/) as a process | Dijkstra route-finding over two syntax trees ([diffing](https://difftastic.wilfred.me.uk/diffing.html)); about 50 languages | "scales relatively poorly on files with a large number of changes, and can use a lot of memory" ([README](https://github.com/Wilfred/difftastic)). Line diff above 1 MB, above 2M graph vertices, or on any parse error ([options.rs](https://github.com/Wilfred/difftastic/blob/master/src/options.rs)) | MIT, but binary only (no `[lib]` in Cargo.toml). `--display json` with no stated stability; no moves; "does not address" merging | No: an extra binary to ship, an unstable output format, and slower |
| SemanticDiff | — | — | Closed | Not available |

**Why tokens rather than trees first.** A token sequence diff handles most of what people see in SemanticDiff:

- Whitespace never becomes a token.
- A literal like `0x539` normalises to `1337`.
- Optional parentheses and trailing commas can be marked ignorable by node kind.
- A token diff can't produce the "a line with one changed word looks entirely changed" result that line diffs give.

It runs at sequence-diff speed, and it uses the code we need for the fallback anyway. Moves can be found the way SemanticDiff's rules imply: hash unmatched deleted and added runs of at least one full line, and match them. Renames are identifier tokens changed the same way throughout the file. Tree matching (GumTree) earns its cost on structural edits: a block wrapped in an `if`, or arguments reordered. That's the phase after.

### 3. Merge

| Option | What it does | Fit | Licence | Verdict |
|---|---|---|---|---|
| git's line merge (today) | `git merge-file` and the merge machinery | Source of truth, per ADR 0002 | — (we run git, as today) | Keep |
| **Token-level three-way merge inside each conflicted place** (ours) | Diff3 over syntax tokens of base, ours and theirs for just that place. If the two sides changed different tokens, it gives a suggested result; if not, it shows exactly which tokens each side changed | Fits `ConflictView`: a place gains a "suggested" state and token-level marks. Works the same at every rebase stop | Ours | **Recommended first** |
| Structured merge, mergiraf or Spork style, in Go | Parse all three, match with GumTree (base–left, base–right, left–right), merge with parent–child–successor (PCS) triples; commutative parents such as import lists merge by set; on failure, line merge of that node ([mergiraf architecture](https://mergiraf.org/architecture.html), following [Spork](https://arxiv.org/abs/2202.05329)) | Large: needs GumTree in Go first | Ours, if written from the papers | Later |
| [mergiraf](https://mergiraf.org/) as the user's own merge driver | git runs it through `.gitattributes` `merge=mergiraf` during merge, rebase, cherry-pick and revert; it tries a line merge first and only uses the structured merge on conflicts ([usage](https://mergiraf.org/usage.html)). About 30 programming languages plus data formats ([languages](https://mergiraf.org/languages.html)) | Already works with the app: git runs drivers, and our conflict tool shows whatever is left. The app could mention it in the conflict tool if it is installed | GPLv3, Rust ([repo](https://codeberg.org/mergiraf/mergiraf)) | Support, don't bundle. Bundling a GPLv3 binary next to an Apache-2.0 app needs a licence decision; I couldn't fetch the GNU GPL FAQ to quote it |
| `mergiraf solve` on a file with markers | Rebuilds the three versions from diff3 markers; `zdiff3` "can confuse Mergiraf" (unbalanced braces) | We don't need it, since we have the stages | GPLv3 | No |
| rerere | Replays recorded resolutions ([git rerere](https://git-scm.com/docs/git-rerere)) | Separate feature, #938 | — | Separate |

Custom merge drivers in git ([gitattributes](https://git-scm.com/docs/gitattributes)) take `%O %A %B %L %P %S %X %Y` and exit 0 for clean or 1–128 for conflicts. Git Revenge could ship its own Go merge driver later, so syntax-aware merging also happens for merges started in a terminal. It isn't needed for the first version.

### 4. Rendering

**What `@codemirror/merge` can do.** The installed version is 6.12.3; see `node_modules/@codemirror/merge/dist/index.d.ts` and the [reference](https://codemirror.net/docs/ref/#merge).

- `MergeView` is two editors, `a` and `b`, with `collapseUnchanged`, `revertControls`, `gutter` and `diffConfig`.
- `unifiedMergeView` is one editor against an `original` document, with accept and reject per chunk.
- `DiffConfig.override?: (a: string, b: string) => readonly Change[]` swaps out its diff ([CHANGELOG 6.12.0](https://github.com/codemirror/merge/blob/main/CHANGELOG.md)). Its own diff is Myers 1986, with `scanLimit` and `timeout` fallbacks.
- There is no three-way or N-pane mode.

**Plan for the views:**

- **Split view.** Keep `MergeView`. Pass the Go result through `override`. The override is synchronous, so the frontend fetches the Go result first and then builds the view, which is what `SplitDiff.svelte` already does with `languageFor`. Draw moves, renames and hidden changes as decorations: `Decoration.mark` for tokens and `Decoration.line` for moved lines, the same way `placesDeco.ts` builds its decorations.
- **Unified view.** `UnifiedDiff.svelte` renders from the Go result instead of `lineDiff.ts`, adding marks within a line. Staging keeps using line operations.
- **Conflict tool.** Keep its own panes; MergeView can't do three-way. Add origin colouring per place, P4Merge-style ("only ours changed this", "both changed it the same way", "theirs deleted it"), a token-level view of what each side changed against the base, and a "suggested" state with a one-click accept.

**Tokens to reuse** (`frontend/src/style.css`, plus the colours already used in the diff and conflict views):

- **Added and removed:** `--success-wash` and `--danger-wash` for whole lines, as in `UnifiedDiff.svelte`. For changed tokens, `color-mix(in srgb, var(--success|--danger) 35%, transparent)`, as `SplitDiff.svelte` does for `.cm-changedText`.
- **Ours, theirs and base:** `--code-string` (teal) for ours, `--code-function` (plum) for theirs, and an italic `--code-comment` for base, as in `ConflictView.svelte`. Use `--code-number` (ochre) for hand edits, which matches P4Merge's "edited" marking. New "suggested" places could use `--warning-wash`, already in `ConflictView.svelte`, with a dashed border so they don't read as decided.
- **Moves:** give each move a border rather than a fill, as SemanticDiff does, cycling `--accent-a`, `--accent-b`, `--accent-c` and `--ink-ochre`, so moves never use the red or green of add and delete. "Moved without changes" goes in the serif italic already used for collapsed lines (`--font-serif`, as in `SplitDiff.svelte`'s `.cm-collapsedLines`).
- **Hidden formatting changes:** don't hide them silently. Show them in `--ink-faint` or with the gutter marker, with a toggle. Notes such as "Shown as a line diff: the new version doesn't parse" use the existing banner style (`--warning-wash`).
- **Code backgrounds:** everything sits on `.code-ground` like the current diff, blame and conflict bodies, so both the light (Ink Paper) and dark (Ink Parchment) themes come for free.

## Recommended architecture

A new Go package, say `internal/semdiff`, reached through new bound methods on `App`. Note from earlier work: a bound method's result must not implement `error`, or Wails rejects the call.

1. **Inputs** are blob ids where they exist, read through the existing `cat-file --batch` process, plus the working-tree text. Cache results by `(old id, new id, language, engine version)` in an LRU. The working tree is keyed by a content hash.
2. **Language detection.** Use `linguist-language` or a `diff=` driver from `git check-attr` first, then gotreesitter's `grammars.DetectLanguage` by file name. Unknown files go straight to the line diff.
3. **Parsing.** Parse the old and new versions in two goroutines, each with its own parser. I haven't checked whether gotreesitter parsers are safe to share; [bonsai](https://pkg.go.dev/github.com/msuozzo/bonsai) warns its parsers are not. Stop if `context` is cancelled: the tab closed (the #778 lesson) or the budget ran out.
4. **Tokens.** Walk the leaves into a slice of `{kind, text, normalised, span}`. Comments are their own tokens and can be ignored on request. Per-language normalisers start with the SemanticDiff list for our top languages.
5. **Diff.** Use `znkr.io/diff.EditsFunc` over the token slices, comparing normalised text. Then:
   - align change edges to node boundaries;
   - find moves by hashing runs of unmatched tokens that cover at least one full line;
   - group identifier renames that happen the same way throughout the file.
6. **Output.** Byte ranges for each side: changed, moved (with an id), renamed (with an id) and hidden. Also a line map, which the unified view and `MergeView.override` both use. A field records which engine produced it (`structure` or `lines`) and why it fell back.
7. **Conflicts.** For each place from `parsePlaces`, take the base, ours and theirs slices and tokenise them in the context of each whole-file parse. Run a three-way merge over the tokens (diff3 on tokens), and return a suggestion or per-side token changes.

**Budget and fallbacks.** These are proposed numbers; check them on the kernel bench.

| Condition | Action | Precedent |
|---|---|---|
| Either side over 1 MB | Line diff | difftastic `DEFAULT_BYTE_LIMIT = 1_000_000`; our `MAX_HIGHLIGHT` is 1 MiB |
| Any parse error in either version | Line diff, with a note saying why | difftastic `DEFAULT_PARSE_ERROR_LIMIT = 0`; SemanticDiff's fallback warning |
| More than 200,000 tokens per side, or 150 ms wall time | Line diff, with a note | GumTree mean 20–74 ms per file pair; znkr usually under 1 ms |
| Line diff over 4000 edits | Replace-all, as today | `MAX_EDIT_DISTANCE` |
| Many files in a commit | Diff a file when it is opened, not the whole commit at once; prefetch the next file | — |

## Phased plan

1. **Diff service in Go, lines and words** (no parser yet). Move the diff to Go with `znkr.io/diff`. Add highlighting within a line from a generic word tokenizer, an "ignore whitespace" toggle, and moved-line detection. Feed both the unified and split views; the split view uses `override`. This gives the biggest visible change with the least risk. The unified view gets highlighting within lines for the first time, and caching and speed are fixed in one place.
2. **Structure for the top languages.** Add gotreesitter behind a setting and normalise SemanticDiff's equivalences. Start with Go, TypeScript, TSX, JavaScript, Python, Java, C#, Rust, C and C++, then JSON, YAML, CSS and HTML. Show a quiet note whenever a file falls back. Write parity tests on a fixed corpus, including kernel files, so a gotreesitter upgrade can't silently change results.
3. **Conflict tool.** Add P4Merge-style origin colouring and a base column. Add token-level marks for what each side changed. Add three-way token-merge suggestions, which also work at every rebase and cherry-pick stop. Mention mergiraf when it is configured.
4. **Moves and renames in full.** Linked move borders and "Compare with original" for moved blocks; rename grouping with one colour per rename. Then GumTree in Go for structural moves, and commutative merges of import lists and members, Spork-style.

## Open questions, answered 2026-11-23

1. **Binary size.** Is about 15 MB more (uncompressed, 22 grammars) acceptable? Or should we ship a core set and download other grammars on first use (`grammar_blobs_external`)?
   **Answer:** the size is acceptable, as part of the feature. Grammars are built in.
2. **Dependency.** Is it acceptable to depend on gotreesitter, which is pre-1.0 and comes from one main upstream? The alternative is the official cgo bindings, which need a C toolchain on Windows and change ADR 0002.
   **Answer:** yes.
3. **Default behaviour.** Should formatting-only changes be hidden by default, as SemanticDiff does, or dimmed but visible? And should the structural diff be on by default?
   **Answer:** hiding formatting-only changes is an option the user turns on. ADR 0015 makes it off by default.
4. **Staging hidden changes.** When the view hides a formatting change, should staging a nearby line take it along, or should hidden changes become visible whenever the user starts staging?
   **Answer:** staging always shows the end result. Nothing is hidden where changes are staged.
5. **Merge suggestions.** Should token-level merge suggestions always wait for a click, or may a place with no overlap be pre-filled but marked as suggested?
   **Answer:** pre-filled, and marked as suggested.
6. **Languages.** Which languages come first? Matching SemanticDiff's list is the default proposal.
   **Answer:** SemanticDiff's list, for a first pass. gotreesitter v0.58.0 has a grammar for every language on it except gettext `.po`.
7. **mergiraf.** Should the app offer to set it up, or only notice it? Bundling needs a licence decision, since it is GPLv3.
   **Answer:** support it, don't bundle it.

None of the first-pass grammars is copyleft. gotreesitter's copyleft grammars are caddy, disassembly and jq (GPL-3.0) and nim (MPL-2.0) (`docs/licensing.md` in the module). The `gotreesitter_no_copyleft` build tag leaves them out.

## Not verified

- **Parse speed on our files.** Running the benchmark binary was blocked, so speed figures are gotreesitter's own.
- **gotreesitter's concurrency rules, and which grammars are certified.** Only 12 of the 34 certified grammars are named in the README.
- **P4Merge layout.** The manual doesn't say which side shows theirs and which shows yours. The 2-way page says both "purple bands" and "blue and green".
- **CodeMirror reference details** beyond the installed `.d.ts`. The online reference was read as a summary.
- **GPL FAQ.** gnu.org returned HTTP 429 three times, so this note gives no quoted licence guidance on bundling mergiraf.
- **GumTree's later paper.** The ICSE 2024 scalability paper ([DOI](https://doi.org/10.1145/3597503.3639148)) wasn't read; it may change the cost picture for phase 4.

## Sources

- SemanticDiff: [homepage](https://semanticdiff.com/), [what is SemanticDiff](https://semanticdiff.com/docs/what-is-semanticdiff/), [invariances](https://semanticdiff.com/docs/invariances/), [moved code](https://semanticdiff.com/docs/understand-diff/moved-code/), [refactoring grouping](https://semanticdiff.com/docs/understand-diff/refactoring-grouping/), [colors](https://semanticdiff.com/docs/understand-diff/colors/), [fallback diff](https://semanticdiff.com/docs/understand-diff/fallback-diff/), [limitations](https://semanticdiff.com/docs/limitations/), [SemanticDiff vs difftastic](https://semanticdiff.com/blog/semanticdiff-vs-difftastic)
- P4Merge: [product page](https://www.perforce.com/products/p4/p4-merge), [Merging files](https://help.perforce.com/helix-core/server-apps/p4merge/current/Content/P4Merge/diff-merge.p4merge.html), [Diffing text files](https://help.perforce.com/helix-core/server-apps/p4merge/current/Content/P4Merge/diff-merge.text_diff.html)
- Parsers: [gotreesitter README](https://github.com/odvcencio/gotreesitter), [languages](https://gotreesitter.m31labs.dev/docs/languages), [performance](https://gotreesitter.m31labs.dev/docs/performance), [build tags](https://github.com/odvcencio/gotreesitter/blob/main/docs/build-tags.md), [pkg.go.dev](https://pkg.go.dev/github.com/odvcencio/gotreesitter); [tree-sitter](https://tree-sitter.github.io/tree-sitter/); [go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter); [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter); [malivvan/tree-sitter](https://github.com/malivvan/tree-sitter); [bonsai](https://pkg.go.dev/github.com/msuozzo/bonsai); [dcosson/treesitter-go](https://github.com/dcosson/treesitter-go); [web binding](https://github.com/tree-sitter/tree-sitter/tree/master/lib/binding_web)
- Diff: [znkr.io/diff](https://github.com/znkr/diff) ([pkg.go.dev](https://pkg.go.dev/znkr.io/diff)); [sergi/go-diff](https://github.com/sergi/go-diff); [GumTree](https://github.com/GumTreeDiff/gumtree), [GumTree languages](https://github.com/GumTreeDiff/gumtree/wiki/Languages), [Falleri et al. 2014](https://www.labri.fr/perso/xblanc/data/papers/ASE14.pdf); [difftastic manual](https://difftastic.wilfred.me.uk/), [diffing](https://difftastic.wilfred.me.uk/diffing.html), [README](https://github.com/Wilfred/difftastic), [options.rs](https://github.com/Wilfred/difftastic/blob/master/src/options.rs)
- Merge: [mergiraf](https://mergiraf.org/), [architecture](https://mergiraf.org/architecture.html), [usage](https://mergiraf.org/usage.html), [languages](https://mergiraf.org/languages.html), [repo](https://codeberg.org/mergiraf/mergiraf); [Spork](https://arxiv.org/abs/2202.05329); [gitattributes](https://git-scm.com/docs/gitattributes), [git merge-file](https://git-scm.com/docs/git-merge-file), [git config](https://git-scm.com/docs/git-config), [git rerere](https://git-scm.com/docs/git-rerere), [git diff](https://git-scm.com/docs/git-diff)
- Rendering: [@codemirror/merge reference](https://codemirror.net/docs/ref/#merge), [repo and CHANGELOG](https://github.com/codemirror/merge/blob/main/CHANGELOG.md); installed `frontend/node_modules/@codemirror/merge/dist/index.d.ts` (6.12.3), `frontend/node_modules/@codemirror/language-data/dist/index.js`
- This repo: `docs/adr/0002-system-git-cli.md`, `.github/workflows/ci.yml`, `internal/git/conflict.go`, `frontend/src/lib/diff/*`, `frontend/src/style.css`, `docs/walkthroughs/resolve-a-conflict.md`
