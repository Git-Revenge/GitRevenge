# gotreesitter timings

- Date: 2026-11-23
- Issue: #943, for [ADR 0015](../adr/0015-diffs-and-merges-read-the-code.md)
- Status: measured. The ADR's Consequences and fallback rule now use these numbers.

## The question

Is [gotreesitter](https://github.com/odvcencio/gotreesitter) v0.58.0 fast enough, small enough and correct enough to parse every diff the app shows? And are the budgets in the [research note](semantic-diff-merge.md) right: 1 MB per side, 150 ms, 200,000 tokens, and a line diff on any parse error?

## Short answer

**Go, with four changes to how it's used.**

1. **The speed depends on which of gotreesitter's two engines a grammar uses.** gotreesitter has a new C-faithful engine, `ccore`, and an older one. A grammar runs on `ccore` only after it passes upstream's certification. C, TypeScript, Python, CSS, JSON and Rust are on it. They read 6–22 MB a second, and C parses 2.3 times *faster* than the official cgo bindings on the same machine. Go, JavaScript, TSX, C#, Java, SCSS, Vue, HTML, Lua and XML are still on the older engine. Most of them read 0.7–4 MB a second, and Go parses about 3 times *slower* than cgo overall, and up to 17 times on single files. They also use far more memory while parsing.
2. **The 150 ms limit is enforced by gotreesitter's own timeout, not guessed from file size.** `Parser.SetTimeoutMicros(150_000)` stopped every slow parse within 154 ms. The 1 MB check stays, so huge files aren't read at all. The ccore grammars parse 1 MB in about 100 ms, so for them the size check binds first. On the older engine the timeout binds first. Most C#, JavaScript and Go files still parse in time. The exceptions are a few large C# files (46–317 KB) and bundled or minified JavaScript.
3. **"Any parse error falls back" is the wrong rule.** 67% of kernel C files have at least one ERROR node, nearly all from macros. In the median such file it covers 0.2% of the bytes. A token diff still works around a small error, because the tokens inside it are still tokens. The rule becomes: fall back when more than a quarter of either version's bytes sit inside ERROR nodes. That catches 1.2% of kernel files and 9% of Bootstrap's SCSS.
4. **Walk the tree with `Node.Children()`, use a plain `Parser`, and share nothing but the `Language`.** On ccore trees, `Children()` is 2–3 times faster than the cursor or `Child(i)`. The C grammar's hand-written token source is 5 times slower than the default lexer and produces no fewer errors. A `Parser` isn't safe for concurrent use, but a `ParserPool` is, and 16 goroutines parse 5 times faster than one.

Size: the first-pass grammars add **10.8 MB to the app** and **4.8 MB to the Windows installer**.

## Method

- **Machine:** Intel i9-11900KF (8 cores, 16 threads), 64 GB, Windows 11 Pro. Go 1.27.4, `CGO_ENABLED=0`. Builds used `-tags 'grammar_subset grammar_subset_<lang>… gotreesitter_no_copyleft'`.
- **cgo comparison:** WSL 2 (Debian) on the same machine, Go 1.27.4 linux/amd64, gcc 14.2. It used [go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter) v0.25.0, tree-sitter-c v0.24.2 and tree-sitter-go v0.25.0. gotreesitter was cross-compiled for Linux and run on the same files, so both sides of each comparison come from one OS.
- **Kernel:** `A:/linux` at c339f4010dea (2026-10-28), 37,738 `.c` files. Its size percentiles are p10 1.6 KB, p50 9.1 KB, p90 45 KB, p99 148 KB and max 1.8 MB. The detail files are those percentiles, then `kernel/sched/core.c`, then the six largest files under 1 MB. The sweep is 2,000 files picked by a seeded shuffle (PCG seed 943), the same files for both runtimes.
- **This repo:** all 375 `.go` files at 351a3a71.
- **Other languages:**
  - JavaScript, TypeScript (no `.d.ts`), CSS, JSON and HTML: the app's own `frontend/node_modules`.
  - Python: the CPython 3.12 standard library.
  - Rust: the cargo registry cache.
  - XML and Vue: the Go module cache.
  - C#: JamesNK/Newtonsoft.Json at 52fa3ae.
  - Java: google/gson at d9b5079.
  - Lua: luarocks/luarocks at 85bfe36.
  - SCSS and more HTML: twbs/bootstrap at 03f96ca.

  Each sweep takes up to 400 files with the same seed.
- **Timing:**
  - Each file is parsed once untimed, then 5 times timed, and the median is reported.
  - "Walk" collects every leaf as `{start, end, symbol}` in document order, which is what the token diff needs.
  - Sweep figures are totals over the sample, so the coarse Windows clock doesn't affect them. Times under about 2 ms for a single file aren't reliable on Windows.
- **Memory:** measured on the first parse in a fresh process, before gotreesitter's arena pool warms up. Later parses reuse the pool and allocate almost nothing.
  - "alloc" is bytes allocated.
  - "peak" is the largest Go heap seen during the parse, sampled every 0.2 ms.
  - "tree" is the heap still held while the tree is alive.
  - For cgo, the figure is growth in peak resident memory (`VmHWM`).
- **Size:** `wails build -clean -s -skipbindings` from `main`, then `makensis` on `build/windows/installer/project.nsi`. This was done once as-is, and once with a throwaway file in `package main` importing the 15 grammar packages (the 14 first-pass languages plus TSX) under the subset tags.

The scratch programs aren't committed. #945 should add a kernel parse benchmark to `internal/perf` beside the graph ones, so these numbers can be retaken on every gotreesitter upgrade.

## Results

### C on the kernel, file by file (Windows)

| File | KB | Parse | Walk | Leaves | Peak heap | ERROR nodes |
|---|---:|---:|---:|---:|---:|---:|
| `drivers/gpu/drm/nouveau/nvkm/subdev/fb/gf108.c` (p10) | 1.6 | <1 ms | <1 ms | 98 | — | 0 |
| `drivers/iio/chemical/vz89x.c` (p50) | 9.1 | <1 ms | <1 ms | 1,813 | — | 4 |
| `crypto/ecc.c` (p90) | 45 | 4.0 ms | 3.8 ms | 9,958 | — | 0 |
| `kernel/bpf/helpers.c` (p99) | 148 | 14.6 ms | 7.0 ms | 21,502 | — | 350 |
| `kernel/sched/core.c` | 296 | 20.8 ms | 12.8 ms | 37,176 | 8.2 MB | 225 |
| `net/wireless/nl80211.c` | 668 | 45.8 ms | 39.6 ms | 122,953 | — | 133 |
| `drivers/scsi/lpfc/lpfc_sli.c` | 685 | 43.6 ms | 31.6 ms | 96,611 | — | 144 |
| `kernel/bpf/verifier.c` | 722 | 46.1 ms | 37.6 ms | 118,948 | 17.7 MB | 149 |
| `…/dml2_core/dml2_core_dcn4_calcs.c` | 760 | 40.6 ms | 36.8 ms | 123,491 | — | 41 |
| `fs/nls/nls_cp949.c` | 855 | 81.0 ms | 54.9 ms | 220,407 | 30.2 MB | 4 |
| `drivers/net/wireless/broadcom/brcm80211/brcmsmac/phy/phy_n.c` | 915 | 94.2 ms | 69.7 ms | 265,608 | 36.2 MB | 5 |

Walk times for the last three files are with `Children()`. With the cursor they were 165, 146 and 115 ms.

### gotreesitter against the official cgo bindings (WSL, same files)

| | gotreesitter | cgo tree-sitter |
|---|---:|---:|
| C, 2,000 kernel files (39.0 MB), parse and walk, one goroutine | 4.67 s, **8.4 MB/s** | 10.54 s, 3.7 MB/s |
| C, parse p50 / p99 / max | 0.7 / 12.9 / 46.1 ms | 1.4 / 25.7 / 78.6 ms |
| C, 16 goroutines | 836 ms, 46.6 MB/s | 1.66 s, 23.5 MB/s |
| C, `kernel/sched/core.c` parse | 22.6 ms | 42.4 ms |
| C, `phy_n.c` (915 KB) parse | 100.3 ms | 194.1 ms |
| C, peak memory for `core.c` / `verifier.c` / `phy_n.c` | 8.2 / 17.7 / 36.2 MB | 6.1 / 17.2 / 38.6 MB |
| Go, 375 files of this repo (2.1 MB), parse and walk | 2.03 s, 1.0 MB/s | 0.73 s, **2.9 MB/s** |
| Go, parse p50 / p99 / max | 3.5 / 38.7 / 77.5 ms | 0.9 / 3.5 / 6.5 ms |
| Go, `internal/session/session.go` (28 KB) parse | 76.9 ms | 4.4 ms |
| Go, peak memory for `session.go` | 58 MB | 1.0 MB |

The two runtimes agree closely:

- **C:** the same leaf count on all 11 detail files. Over the 2,000-file sweep there were 6,757,256 leaves against 6,757,242. 1,341 files had an ERROR or MISSING node under both runtimes, and the counts above each error threshold differ by at most two.
- **Go:** 460,828 leaves against 460,827. One file, `internal/git/mailmapfile_test.go`, gets five ERROR nodes from gotreesitter on valid code; the trouble starts at `if (got != id) && len(by) == 0 {`. The same line alone in a small file parses cleanly, so I didn't reduce it further.

The grammar versions aren't identical: gotreesitter embeds its own blob of each grammar.

### Every first-pass language (Windows, one goroutine)

| Language | Engine | Files | MB | MB/s | Parse p99 | Slowest parse | Files with ERROR | Over 25% ERROR |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| C (kernel) | ccore | 2,000 | 39.0 | 8.7 | 12 ms | 45 ms | 67% | 23 (1.2%) |
| TypeScript | ccore | 372 | 2.3 | 6.1 | 8 ms | 12 ms | 0 | 0 |
| Python | ccore | 400 | 6.9 | 6.1 | 25 ms | 31 ms | 1 | 0 |
| CSS | ccore | 39 | 1.1 | 9.1 | 12 ms | 12 ms | 25 (64%), all under 5% of bytes | 0 |
| JSON | ccore | 400 | 4.9 | 22.5 | 7 ms | 10 ms | 0 | 0 |
| Rust | ccore | 400 | 6.1 | 7.9 | 14 ms | 23 ms | 3 | 0 |
| Go | older | 375 | 2.1 | 1.1 | 37 ms | 75 ms | 1 | 0 |
| JavaScript | older | 400 | 3.5 | 0.8 | 95 ms | **2.5 s** (`d3.min.js`, 273 KB) | 2 | 1 |
| C# | older | 400 | 3.4 | 0.7 | **417 ms** | **619 ms** | 27 (6.8%) | 1 |
| Java | older | 268 | 2.1 | 2.0 | 59 ms | 179 ms | 0 | 0 |
| SCSS | older | 142 | 0.5 | 3.0 | 9 ms | 19 ms | **121 (85%)** | **13 (9.2%)** |
| HTML | older | 35 | 0.3 | 8.9 | 5 ms | 5 ms | 7 | 0 |
| Lua | older | 162 | 1.1 | 3.9 | 13 ms | 15 ms | 0 | 0 |
| XML | older | 400 | 2.3 | 1.5 | 28 ms | 48 ms | 0 | 0 |
| Vue | older | 22 | 0.02 | — | — | — | 0 | 0 |

Which engine a grammar uses comes from matching each grammar package's `BlobSHA256` against the module's `testdata/ccore_certificates.json`. TSX is on the older engine, unlike TypeScript. The 22 Vue files were too small to time. Why CSS and SCSS have so many errors wasn't investigated; newer syntax the grammars don't know is the likely cause.

Over 150 ms, without a timeout:

- **C#:** 8 of 400 files, all central Newtonsoft.Json sources of 45–317 KB. `JsonReader.cs` (45 KB) took 306 ms and `DefaultContractResolver.cs` (77 KB) took 561 ms.
- **JavaScript:** 3 of 400, all bundled or minified (`d3.min.js`, two Vite pre-bundles of 274–311 KB).
- **Java:** 1 of 268.
- **Every other language:** none.

### Memory on the older engine

| File | KB | Parse | Allocated | Peak heap | Tree held |
|---|---:|---:|---:|---:|---:|
| Go `internal/trailerindex/reading.go` | 31 | 29 ms | 23 MB | 23 MB | 23 MB |
| Go `internal/session/session.go` | 28 | 69 ms | 62 MB | 58 MB | 58 MB |
| C# `JsonReader.cs` | 45 | 307 ms | 107 MB | 89 MB | 75 MB |
| C# `Linq/JToken.cs` | 112 | 530 ms | 421 MB | 146 MB | 74 MB |
| JavaScript `d3.min.js` | 273 | 2.5 s | 305 MB | 277 MB | 159 MB |
| Python `typing.py` (ccore, for contrast) | 120 | 10 ms | 3.7 MB | 3.6 MB | 3.6 MB |

ccore trees cost about 30–40 bytes per source byte. Older-engine trees cost 0.3–2 KB per source byte. The token diff keeps a token list, not a tree, so each tree can be released straight after its walk. Peak memory is the cost to watch, and the 150 ms timeout bounds it.

### The timeout

With `SetTimeoutMicros(150_000)`, no parse in any sweep took longer than 154 ms. `d3.min.js` and five of the eight slow C# files stopped there and reported `ParseStoppedEarly()`, each with a partial tree: `JToken.cs` gave 780 leaves instead of 9,131.

The other three C# files (`JsonReader.cs`, `JsonConvert.cs`, `ReflectionUtils.cs`) also returned at 151–154 ms, but did *not* report a stop. Their trees were complete, with the same leaf and ERROR counts as an untimed parse. At the deadline gotreesitter seems to finish by a cheaper route when it can. So decide by `ParseStoppedEarly()`, not by the elapsed time.

With the timeout on, the C# sweep's total time fell from 5.1 s to 3.3 s.

### Parse errors

The share of bytes inside outermost ERROR nodes, among kernel files that have any:

- **Spread:** p50 0.20%, p90 1.77%, p99 38%, max 100%.
- **Files over each threshold:** more than 1% of bytes: 212 of 2,000. More than 5%: 65. More than 10%: 43. More than 25%: 23. More than 50%: 10.

cgo tree-sitter's counts matched to within two files at every threshold, so this is the C grammar meeting kernel macros, not a gotreesitter bug.

### Walking the tree

ccore builds node views lazily, so the walk costs 25–45% as much as the parse on ccore grammars:

- **`Node.Children()`:** fetches each node's children once. It is fastest: 69.7 ms on `phy_n.c` and 54.9 ms on `nls_cp949.c`.
- **`TreeCursor` and `Child(i)`:** these re-find each child by its index, which is slow on nodes with thousands of children, such as big array initialisers. They took 111–121 ms and 146–166 ms on the same files.
- **Older-engine trees:** already fully built, so walking is nearly free (1–7 ms).

On the older engine, ccore is 5–10 times slower to parse. `SetAdmissionCandidateRoute(false)` forces C onto it: `phy_n.c` took 1.09 s and 296 MB, against 94 ms and 36 MB on ccore. So the cheap walk never pays for the slower parse.

### The C token source

`grammars.LangEntry.Parse` uses a hand-written token source for C, C++, Java and authzed. On the 2,000 kernel files it was 5 times slower than a plain `Parser` (2.5 MB/s against 8.7). It also left more files over 25% ERROR (34 against 23), and 21 parses stopped early. Use a plain `Parser`.

### Sharing

`Parser` says it "is not safe for concurrent use" (`parser.go`). `NewParserPool` is documented as "a concurrency-safe parser pool", and a `Language` is shared read-only. A pool on 16 goroutines parsed and walked the kernel sample 5.1 times faster than one goroutine on Windows (5.6 times on Linux), and the Go files 5.1 times faster. Parse the two versions of a file on two goroutines from one pool per language.

### Grammar loading

Each grammar's tables decode on first use, taking 7–14 ms (C 1.4 MB of tables, Go 2.1 MB). Load them lazily.

### Size

| Windows amd64 build | App | Installer |
|---|---:|---:|
| `main` at 351a3a71 | 25,950,208 bytes | 11,687,759 bytes |
| With gotreesitter and the 15 first-pass grammar packages | 36,756,992 bytes | 16,457,990 bytes |
| Difference | **+10.8 MB (+42%)** | **+4.8 MB (+41%)** |

That is less than the research note's 15 MB for 22 grammars. The app already links most of the standard library that the runtime needs.

## The budgets, confirmed or changed

| Rule | Before | After |
|---|---|---|
| File size | Either side over 1 MB: line diff | **Kept.** Checked before parsing. ccore grammars parse 1 MB in about 100 ms |
| Time | Over 150 ms: line diff | **Kept, and enforced per parse** with `SetTimeoutMicros(150_000)`. Both versions parse at once, so the wait is the slower of the two. Read `ParseStoppedEarly()` |
| Tokens | Over 200,000 per side: line diff | **Kept.** The leaf counts run from 89 per KB (JSON) to 227 (Go), so only files near 1 MB reach it. It bounds the token diff's work, not the parse |
| Parse errors | Any error in either version: line diff | **Changed:** more than 25% of either version's bytes inside ERROR nodes. A smaller error is shown as a quiet note on the diff |
| Memory | None | **New:** `SetMemoryBudgetBytes(256 << 20)` per parse. gotreesitter's default is 512 MB. A stopped parse falls back like a timeout. Trees are released straight after the walk; only tokens are cached |

## What this means for the first-pass languages

TypeScript, Python, CSS, JSON and Rust get code diffs on effectively every file, as C would. Go, Java, Lua, XML, HTML and SCSS get them on nearly every file. JavaScript loses only bundled or minified files, which are poor material for any diff. **C# loses some of its large files**: 5 of the 400 Newtonsoft.Json files sampled (1.3%, 46–317 KB). They're few, but they're the central ones people edit. As upstream certifies more grammars for ccore, these get faster with no change on our side.

**Open for #945:** when a parse hits the timeout, should the view show the line diff at once and then switch to the code diff once a longer parse finishes in the background, say within 2 s? That would rescue those C# files, at the cost of a view that changes under the reader.

## Not measured

- macOS and Linux desktops. On WSL the C and Go sweep totals were within 10% of Windows, though single files varied more (`kernel/bpf/verifier.c` took 46 ms on Windows and 63 ms on WSL).
- The token diff itself (`znkr.io/diff` over tokens). #944 and #945 time it.
- Lezer's highlighting parse, which stays in the frontend.
