<script lang="ts">
  // Find in files (#980), the sidebar's panel: where a text is in the files, now or at a
  // commit, with git grep (#979). It searches as you type; lines arrive as git finds them,
  // grouped by file, and a line opens its file at that line with every match marked.
  import { onMount, tick } from 'svelte'
  import { FindInFiles } from '../../../wailsjs/go/app/App.js'
  import type { app } from '../../../wailsjs/go/models'
  import { ClipboardSetText } from '../../../wailsjs/runtime/runtime.js'
  import { provideCommands } from '../commands.svelte'
  import type { MenuItem } from '../ContextMenu.svelte'
  import type { DiffTarget } from '../diff/DiffView.svelte'
  import { errorText } from '../errors'
  import { onBackend } from '../events'
  import { toasts } from '../toasts.svelte'
  import Empty from '../ui/Empty.svelte'
  import { historyOptions, type HistoryOptions } from '../filters/history'
  import { filePatterns, findRows, findSummary, inView, markedParts, splitPath, type FindOptions, type FindRow, type FoundLine } from './find'
  import { findAsk } from './findAsk.svelte'

  let {
    path,
    current = null,
    onOpen,
    onMenu,
    onHistory,
  }: {
    path: string
    /** What the diff area shows, so the line open in it is marked here. */
    current?: DiffTarget | null
    onOpen: (t: DiffTarget) => void
    onMenu?: (items: MenuItem[], x: number, y: number) => void
    /** Starts a history search on the graph for what was searched (#982): when it was in
     * the history, or when a file gained or lost it. */
    onHistory?: (o: HistoryOptions) => void
  } = $props()

  /** The history search for what was searched here, in the given files. Whole word isn't a
   * thing git log -S can do, so it's left behind. */
  const historyFor = (o: FindOptions, paths: string[]) => historyOptions(o.text, { matchCase: o.matchCase, regex: o.regex, paths })

  /** The rows' height, fixed so only those in view need drawing: a search can find 10,000. */
  const ROW = 26
  const uid = $props.id()

  let text = $state('')
  let matchCase = $state(false)
  let wholeWord = $state(false)
  let regex = $state(false)
  let where = $state<'tree' | 'head' | 'rev'>('tree')
  let rev = $state('')
  let untracked = $state(false)
  let files = $state('')

  let options = $derived<FindOptions>({
    text,
    matchCase,
    wholeWord,
    regex,
    untracked: where === 'tree' && untracked,
    rev: where === 'tree' ? '' : where === 'head' ? 'HEAD' : rev.trim(),
    paths: filePatterns(files),
  })

  // The search under way, or the last: lines come in by event, tagged with its token.
  let token = ''
  let searched = $state<FindOptions | null>(null)
  let lines = $state.raw<FoundLine[]>([])
  let commit = $state('')
  let result = $state<app.FindResult | null>(null)
  let running = $state(false)
  let error = $state('')

  let rows = $derived(findRows(lines))
  let fileCount = $derived(rows.filter((r) => r.type === 'file').length)

  let timer: ReturnType<typeof setTimeout> | undefined
  $effect(() => {
    const o = options
    clearTimeout(timer)
    if (!o.text || (where === 'rev' && !o.rev)) {
      token = ''
      searched = null
      lines = []
      result = null
      running = false
      error = ''
      return
    }
    timer = setTimeout(() => void run(o), 250)
  })

  async function run(o: FindOptions) {
    const t = `${Date.now()}-${Math.random().toString(36).slice(2)}`
    token = t
    searched = o
    lines = []
    result = null
    commit = ''
    error = ''
    running = true
    active = 0
    try {
      const res = await FindInFiles(path, t, o)
      if (token !== t || res.superseded) return
      result = res
      commit = res.commit
    } catch (e) {
      if (token === t) error = errorText(e)
    } finally {
      if (token === t) running = false
    }
  }

  onMount(() =>
    onBackend('find:lines', (b) => {
      if (b.path !== path || b.token !== token) return
      lines = lines.concat(b.lines)
      commit = b.commit
    }),
  )

  // Ctrl+Alt+F and the search box's row bring text to search for.
  let input = $state<HTMLInputElement>()
  let seen = 0
  $effect(() => {
    const a = findAsk.ask
    if (!a || a.path !== path || a.seq === seen) return
    seen = a.seq
    if (a.text) text = a.text
    void tick().then(() => {
      input?.focus()
      input?.select()
    })
  })

  // Only the rows in view are drawn.
  let list = $state<HTMLElement>()
  let scrollTop = $state(0)
  let viewHeight = $state(400)
  let range = $derived({ start: Math.max(0, Math.floor(scrollTop / ROW) - 5), end: Math.min(rows.length, Math.ceil((scrollTop + viewHeight) / ROW) + 5) })

  let active = $state(0)
  const rowId = (i: number) => `${uid}-row-${i}`

  function target(l: FoundLine): DiffTarget {
    return { mode: 'found', file: l.file, commit: commit || undefined, line: l.line, find: searched ?? options }
  }

  /** The line a row stands for: its own, or a file's first. */
  function lineOf(i: number): FoundLine | undefined {
    const r = rows[i]
    if (!r) return undefined
    if (r.type === 'line') return r.found
    const next = rows[i + 1]
    return next?.type === 'line' ? next.found : undefined
  }

  function openRow(i: number) {
    active = i
    const l = lineOf(i)
    if (l) onOpen(target(l))
  }

  function show(i: number) {
    active = Math.max(0, Math.min(rows.length - 1, i))
    const top = active * ROW
    if (!list) return
    if (top < list.scrollTop) list.scrollTop = top
    else if (top + ROW > list.scrollTop + list.clientHeight) list.scrollTop = top + ROW - list.clientHeight
  }

  function listKey(e: KeyboardEvent) {
    // Alt+↓ and the like are commands (the next file found), not moves in the list.
    if (e.altKey || e.ctrlKey || e.metaKey) return
    const page = Math.max(1, Math.floor((list?.clientHeight ?? ROW) / ROW) - 1)
    const to: Record<string, number> = { ArrowDown: active + 1, ArrowUp: active - 1, PageDown: active + page, PageUp: active - page, Home: 0, End: rows.length - 1 }
    if (e.key in to) {
      e.preventDefault()
      show(to[e.key])
    } else if (e.key === 'Enter') {
      e.preventDefault()
      openRow(active)
    }
  }

  const isOpen = (l: FoundLine) => current?.mode === 'found' && current.file === l.file && current.line === l.line && (current.commit ?? '') === commit

  function rowMenu(e: MouseEvent, r: FindRow) {
    if (!onMenu) return
    e.preventDefault()
    const at = commit || undefined
    const items: MenuItem[] =
      r.type === 'file'
        ? [
            { label: 'History', icon: 'history', run: () => onOpen({ mode: 'history', file: r.file }) },
            ...(onHistory && searched
              ? [{ label: `Commits that add or remove “${searched.text}” here`, icon: 'history' as const, run: () => onHistory(historyFor(searched!, [r.file])) }]
              : []),
            { label: 'Blame', icon: 'blame', run: () => onOpen({ mode: 'blame', file: r.file, commit: at }) },
            {
              label: 'Copy the path',
              icon: 'copy',
              separator: true,
              run: () => {
                void ClipboardSetText(r.file)
                toasts.show(`Copied ${r.file}.`)
              },
            },
          ]
        : [{ label: 'Blame at this line', icon: 'blame', run: () => onOpen({ mode: 'blame', file: r.file, commit: at, line: r.found.line }) }]
    onMenu(items, e.clientX, e.clientY)
  }

  // With a found file open, Alt+↓ and Alt+↑ go to the next and previous file's first line.
  provideCommands(() => {
    if (current?.mode !== 'found') return []
    const open = current
    const step = (d: 1 | -1) => () => {
      const heads = rows.map((r, i) => ({ r, i })).filter((x) => x.r.type === 'file')
      const here = heads.findIndex((x) => x.r.file === open.file)
      const next = heads[here + d]
      if (next) {
        show(next.i + 1)
        openRow(next.i + 1)
      }
    }
    return [
      { id: 'diff.nextFile', group: 'View', title: 'Next file found', keywords: 'find in files', shortcut: 'Alt+ArrowDown', run: step(1) },
      { id: 'diff.prevFile', group: 'View', title: 'Previous file found', keywords: 'find in files', shortcut: 'Alt+ArrowUp', run: step(-1) },
    ]
  })

  /** Where it searched, for "isn't in any file of …". */
  let whereSaid = $derived(!searched?.rev ? 'the working tree' : searched.rev === 'HEAD' ? 'HEAD' : commit ? commit.slice(0, 7) : searched.rev)
</script>

<div class="find">
  <div class="box" class:error={!!error}>
    <input bind:this={input} bind:value={text} aria-label="Text to find" placeholder="Find in the files" spellcheck="false" />
    <button class="opt" aria-pressed={matchCase} title="Match case" aria-label="Match case" onclick={() => (matchCase = !matchCase)}>Aa</button>
    <button class="opt word" aria-pressed={wholeWord} title="Whole word" aria-label="Whole word" onclick={() => (wholeWord = !wholeWord)}>ab</button>
    <button class="opt" aria-pressed={regex} title="Regular expression" aria-label="Regular expression" onclick={() => (regex = !regex)}>.*</button>
  </div>
  {#if error}<p class="message" role="alert">{error}</p>{/if}
  <div class="where">
    <label
      >In
      <select bind:value={where} aria-label="Where to look">
        <option value="tree">The working tree</option>
        <option value="head">HEAD</option>
        <option value="rev">A branch, tag or commit…</option>
      </select></label
    >
    {#if where === 'rev'}
      <input class="mono" bind:value={rev} aria-label="Branch, tag or commit to search" placeholder="main, v1.9.0, 3fbfa29" spellcheck="false" />
    {:else if where === 'tree'}
      <label class="check"><input type="checkbox" bind:checked={untracked} /> untracked files too</label>
    {/if}
  </div>
  <input class="mono files" bind:value={files} aria-label="Only these files" placeholder="Files: src/, *.js, !*.min.js" spellcheck="false" />

  {#if searched}
    <p class="status meta" aria-live="polite">
      {#if running}
        Searching…{#if lines.length}&ensp;{findSummary(lines.length, fileCount)}{/if}
      {:else if result && result.lines > 0}
        {findSummary(result.lines, result.files, result.ms)}
      {/if}
    </p>
  {/if}
  {#if result?.truncated}
    <p class="note">The first {result.lines.toLocaleString()} lines. Narrow it with Files, Whole word or Match case.</p>
  {/if}

  {#if searched && !running && result && result.lines === 0}
    {@const asked = searched}
    <Empty note="not found" compact
      >“{asked.text}” isn't in any file of {whereSaid}.{#if onHistory}
        <button class="btn sm text" onclick={() => onHistory(historyFor(asked, asked.paths))}>When was it in the history?</button>{/if}</Empty
    >
  {:else if !searched}
    <p class="hint">Type to search the files, as <code>git grep</code> does. A line opens its file there.</p>
  {/if}

  <div
    class="results"
    role="listbox"
    aria-label="Lines found"
    tabindex={rows.length ? 0 : -1}
    aria-activedescendant={rows.length ? rowId(active) : undefined}
    bind:this={list}
    bind:clientHeight={viewHeight}
    onscroll={(e) => (scrollTop = e.currentTarget.scrollTop)}
    onkeydown={listKey}
    hidden={!rows.length}
  >
    <div class="rows" style:height="{rows.length * ROW}px" style:padding-top="{range.start * ROW}px">
      {#each rows.slice(range.start, range.end) as r, k (range.start + k)}
        {@const i = range.start + k}
        {#if r.type === 'file'}
          {@const p = splitPath(r.file)}
          <div id={rowId(i)} class="row file" class:active={i === active} role="option" aria-selected={i === active} aria-label="{r.file}, {r.count} {r.count === 1 ? 'line' : 'lines'}" title={r.file} tabindex="-1" onclick={() => openRow(i)} oncontextmenu={(e) => rowMenu(e, r)} onkeydown={() => {}}>
            <span class="name"><span class="folder">{p.folder}</span>{p.name}</span>
            <span class="count meta">{r.count.toLocaleString()}</span>
          </div>
        {:else}
          {@const shown = inView(r.found)}
          <div id={rowId(i)} class="row line" class:active={i === active} class:open={isOpen(r.found)} role="option" aria-selected={i === active} title={r.found.text.trim()} tabindex="-1" onclick={() => openRow(i)} oncontextmenu={(e) => rowMenu(e, r)} onkeydown={() => {}}>
            <span class="num">{r.found.line}</span>
            <span class="text"
              >{#if shown.cutStart}…{/if}{#each markedParts(shown) as part, n (n)}{#if part.hit}<mark>{part.text}</mark>{:else}{part.text}{/if}{/each}{#if shown.cutEnd}…{/if}</span
            >
          </div>
        {/if}
      {/each}
    </div>
  </div>
</div>

<style>
  .find {
    display: flex;
    flex-direction: column;
    gap: 8px;
    height: 100%;
    min-height: 0;
    padding: 2px 10px 0;
    box-sizing: border-box;
  }

  .box {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 3px 4px 3px 8px;
    border: 1px solid var(--paper-edge);
    border-radius: 6px;
    background: var(--paper);
  }

  .box:focus-within {
    border-color: var(--link);
  }

  .box.error {
    border-color: var(--danger);
  }

  .box input {
    flex: 1;
    min-width: 0;
    border: 0;
    outline: none;
    background: transparent;
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--ink);
  }

  .opt {
    border: 0;
    border-radius: 4px;
    padding: 2px 5px;
    background: transparent;
    color: var(--ink-mute);
    font-family: var(--font-mono);
    font-size: 12px;
    cursor: pointer;
  }

  .opt.word {
    text-decoration: underline;
  }

  .opt[aria-pressed='true'] {
    background: var(--accent);
    color: var(--on-accent);
  }

  .message {
    margin: -2px 0 0;
    font-size: 12px;
    line-height: 1.4;
    color: var(--danger);
  }

  .where {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 10px;
    font-size: 13px;
    color: var(--ink-soft);
  }

  .where label {
    display: flex;
    align-items: center;
    gap: 5px;
  }

  .where select,
  .where input.mono,
  .files {
    font-size: 12px;
    border: 1px solid var(--paper-edge);
    border-radius: 4px;
    background: var(--paper);
    color: var(--ink);
    padding: 3px 6px;
    min-width: 0;
  }

  .where input.mono {
    flex: 1;
  }

  .check input {
    accent-color: var(--accent);
  }

  .status {
    margin: 2px 0 0;
    padding-top: 6px;
    border-top: 1px dashed var(--hairline);
  }

  .note,
  .hint {
    margin: 0;
    font-size: 12px;
    line-height: 1.45;
    color: var(--ink-mute);
  }

  .results {
    flex: 1;
    min-height: 0;
    overflow: auto;
    margin: 0 -10px;
    outline: none;
  }

  .results[hidden] {
    display: none;
  }

  .results:focus-visible .row.active {
    outline: 2px solid var(--link);
    outline-offset: -2px;
  }

  .rows {
    box-sizing: border-box;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 26px;
    padding: 0 10px;
    box-sizing: border-box;
    cursor: pointer;
    white-space: nowrap;
  }

  .row:hover {
    background: var(--wash-hover);
  }

  /* The line open in the diff area, and the keys' row while the list has the focus. */
  .row.open,
  .results:focus .row.active {
    background: var(--selection);
  }

  .file .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: 13px;
  }

  .folder {
    color: var(--ink-mute);
  }

  .line {
    padding-left: 18px;
  }

  .num {
    flex: none;
    min-width: 24px;
    text-align: right;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-faint);
  }

  .text {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    font-family: var(--font-mono);
    font-size: 12px;
  }

  mark {
    background: var(--highlight);
    color: inherit;
    border-radius: 2px;
  }
</style>
