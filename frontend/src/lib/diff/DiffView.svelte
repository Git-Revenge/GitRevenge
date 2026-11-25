<script lang="ts" module>
  import type { FindOptions } from '../find/find'

  /** What to compare. Opened from the commit panel or the working tree panel. */
  export interface DiffTarget {
    /** 'conflict' opens the conflict tool instead of a diff; 'found' a file Find in files found
     * something in, whole (#980). */
    mode: 'commit' | 'staged' | 'unstaged' | 'conflict' | 'history' | 'blame' | 'found'
    file: string
    oldPath?: string
    commit?: string
    /** The line to open at (1-based): a found line, or a blame's line. */
    line?: number
    /** The search that found it, for its matches to be marked. */
    find?: FindOptions
  }
</script>

<script lang="ts">
  import Loading from '../ui/Loading.svelte'
  import { FileData, FileDiff, FileImage, SetStagedContent, SetWorkTreeContent } from '../../../wailsjs/go/app/App.js'
  import type { app } from '../../../wailsjs/go/models'
  import { prefs } from '../prefs.svelte'
  import { stops as stopsOf } from './blocks'
  import { formatShortcut, provideCommands } from '../commands.svelte'
  import { session } from '../session.svelte'
  import Icon from '../ui/Icon.svelte'
  import FileView from './FileView.svelte'
  import ImageDiff from './ImageDiff.svelte'
  import MoveCompare from './MoveCompare.svelte'
  import { previewerFor } from './previews/registry'
  import SplitDiff from './SplitDiff.svelte'
  import UnifiedDiff, { type Actions } from './UnifiedDiff.svelte'
  import { toggleWrap, wrap, wrapCommand } from './wrap.svelte'

  let {
    path,
    target,
    onClose,
    onOpen,
  }: {
    path: string
    target: DiffTarget
    onClose: () => void
    /** Opens another view of the same file (history or blame). */
    onOpen?: (t: DiffTarget) => void
  } = $props()

  provideCommands(() =>
    contents?.kind === 'text'
      ? [
          wrapCommand(),
          ...(layout !== 'file' && stops.length
            ? [
                { id: 'diff.nextChange', group: 'View', title: 'Next change', keywords: 'diff hunk', shortcut: 'F7', run: () => step(1) },
                { id: 'diff.prevChange', group: 'View', title: 'Previous change', keywords: 'diff hunk', shortcut: 'Shift+F7', run: () => step(-1) },
              ]
            : []),
        ]
      : [],
  )

  type Layout = 'unified' | 'split' | 'file'
  const LAYOUT_KEY = 'gitrevenge.diffLayout'
  function savedLayout(): Layout {
    try {
      const l = localStorage.getItem(LAYOUT_KEY)
      return l === 'split' || l === 'file' ? l : 'unified'
    } catch {
      return 'unified'
    }
  }
  let layout = $state<Layout>(savedLayout())
  function setLayout(l: Layout) {
    layout = l
    try {
      localStorage.setItem(LAYOUT_KEY, l)
    } catch {
      // Storage unavailable; the choice lasts for this session only.
    }
  }

  // In File mode a file with a previewer shows rendered by default; Raw shows its source.
  const RENDER_KEY = 'gitrevenge.renderPreviews'
  let rendered = $state(readRendered())
  function readRendered(): boolean {
    try {
      return localStorage.getItem(RENDER_KEY) !== 'off'
    } catch {
      return true
    }
  }
  function setRendered(on: boolean) {
    rendered = on
    try {
      localStorage.setItem(RENDER_KEY, on ? 'on' : 'off')
    } catch {
      // Storage unavailable; the choice lasts for this session only.
    }
  }
  let previewer = $derived(previewerFor(target.file))

  /** Loads another file in the repository at the version shown, for a rendered preview. */
  const loadImage = (file: string) => FileImage(path, target.mode, target.commit ?? '', file)
  const openRelated = (file: string) => onOpen?.({ mode: target.mode, file, commit: target.commit })
  /** This file's bytes at the version shown (or its last version when deleted). */
  const loadData = () =>
    contents?.new.exists === false && target.mode === 'commit'
      ? Promise.resolve('')
      : FileData(path, target.mode, target.commit ?? '', target.file)

  let loaded = $state<app.FileDiff | null>(null)
  let contents = $derived(loaded?.contents ?? null)
  let diff = $derived(loaded?.diff ?? null)
  let force = $state(false)

  // Code or lines, and whether formatting hides, start as Settings says; the switches here
  // change this view only.
  let readAs = $derived<'code' | 'lines'>(prefs.diffCode ? 'code' : 'lines')
  let hide = $derived(prefs.diffHideFormatting)
  /** "Read it as code anyway": no time limit, for this file. */
  let anyway = $state(false)
  /** A second, patient read is under way, for a file that missed the quick one. */
  let reading = $state(false)
  /** The patient read missed too. */
  let slow = $state(false)
  let generation = 0

  async function load() {
    const t = target as DiffTarget | null
    // A write that finishes after the diff has closed lands here with nothing left to show.
    if (!t) return
    const gen = ++generation
    const read = readAs === 'lines' ? 'lines' : anyway ? 'anyway' : 'code'
    const args = [path, t.mode, t.commit ?? '', t.file, t.oldPath ?? '', force] as const
    const res = await session.run(() => FileDiff(...args, read))
    if (!res || t !== target || gen !== generation) return
    loaded = res
    reading = false
    slow = false
    // Lines first; the code follows when a longer read finishes (ADR 0015).
    if (read === 'code' && res.diff?.engine === 'lines' && res.diff.fallback === 'time') {
      reading = true
      try {
        const better = await FileDiff(...args, 'patient')
        if (gen !== generation || t !== target) return
        if (better.diff?.engine === 'code') loaded = better
        else slow = true
      } catch {
        if (gen === generation) slow = true
      } finally {
        if (gen === generation) reading = false
      }
    }
  }

  // Reload when the target changes, when the repo changes on disk (for work tree diffs), and
  // when the diff settings change.
  $effect(() => {
    void target
    void force
    void readAs
    void anyway
    void prefs.diffAlgorithm
    void prefs.diffMoves
    if (target.mode !== 'commit') void session.revision[path]
    void load()
  })

  /** Formatting hides only where nothing is staged: staging shows what will be written. */
  let stages = $derived(target.mode !== 'commit')
  let hiding = $derived(hide && !stages && diff?.engine === 'code')

  // The changes to step through. Opening another file starts again from none.
  let stops = $derived(diff ? stopsOf(diff, hiding) : [])
  let current = $state(-1)
  let jump = $state(0)
  $effect(() => {
    void target
    current = -1
  })
  $effect(() => {
    // A reload with fewer changes keeps the place in range.
    if (current >= stops.length) current = stops.length - 1
  })
  function step(by: 1 | -1) {
    if (!stops.length) return
    current = current < 0 ? (by > 0 ? 0 : stops.length - 1) : (current + by + stops.length) % stops.length
    jump++
  }

  /** Reads a stop, as clicking a move's note does: it scrolls to it. */
  function pick(i: number) {
    current = i
    jump++
  }

  /** The changed move open against where it came from, by its index, or -1. */
  let comparing = $state(-1)
  $effect(() => {
    void target
    comparing = -1
  })

  /** What the two sides are, for the split view's heads. */
  let labels = $derived.by(() => {
    const short = (target.commit ?? '').slice(0, 7)
    if (target.mode === 'staged') return { before: 'Before · HEAD', after: 'After · staged' }
    if (target.mode === 'unstaged') return { before: 'Before · staged', after: 'After · working tree' }
    return { before: `Before · ${short}^`, after: `After · ${short}` }
  })

  const count = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

  // Reset per-file state when switching files.
  $effect(() => {
    void target
    force = false
    anyway = false
  })

  /** What the summary says, read as code: changes, renames, moves and formatting. */
  let code = $derived.by(() => {
    if (!diff || diff.engine !== 'code') return null
    const all = stopsOf(diff)
    return {
      changes: all.filter((s) => s.kind === 'change' || s.kind === 'add' || s.kind === 'del').length,
      formatting: all.filter((s) => s.kind === 'format').length,
      renames: diff.renames ?? [],
    }
  })

  /** Why a file in a language read as code shows lines, in one quiet line. */
  let fallbackNote = $derived.by(() => {
    if (!diff?.language || readAs === 'lines' || reading) return ''
    const lang = diff.language
    if (diff.engine === 'code') return diff.unparsed ? `Part of this file doesn’t parse as ${lang}; the rest is read as code.` : ''
    switch (diff.fallback) {
      case 'size':
        return 'Shown as lines: the file is over 1 MB.'
      case 'time':
        if (anyway) return `Shown as lines: reading it as ${lang} stopped.`
        return slow ? `Shown as lines: reading it as ${lang} took longer than 3 seconds.` : ''
      case 'memory':
        return `Shown as lines: reading it as ${lang} needed more memory than a diff should.`
      case 'errors':
        return `Shown as lines: more than a quarter of it doesn’t parse as ${lang}.`
      case 'tokens':
        return 'Shown as lines: it’s too long to compare token by token.'
    }
    return ''
  })

  /** The engine chip: read as code, being read, or lines. */
  let engine = $derived.by(() => {
    if (!diff) return null
    if (reading) return { label: `Reading as ${diff.language}`, kind: 'reading', title: 'Shown as lines while it’s read as code' }
    if (diff.engine === 'code') return { label: `Read as ${diff.language}`, kind: 'code', title: `Parsed as ${diff.language}, so changes are counted by token` }
    const title = !diff.language
      ? 'Compared line by line: the app doesn’t read this kind of file as code'
      : readAs === 'lines'
        ? 'Compared line by line, as git does'
        : 'Compared line by line'
    return { label: 'Lines', kind: 'lines', title }
  })

  let actions = $derived.by((): Actions => {
    const file = target.file
    const shownOld = contents?.old.text ?? ''
    const shownNew = contents?.new.text ?? ''
    // After any write, reload so the view reflects the new state immediately.
    const after = async (p: Promise<void>) => {
      await session.run(() => p)
      await load()
    }
    if (target.mode === 'unstaged') {
      return {
        apply: { label: 'Stage', run: (c) => after(SetStagedContent(path, file, shownOld, c)) },
        revert: { label: 'Discard', danger: true, run: (c) => after(SetWorkTreeContent(path, file, shownNew, c)) },
      }
    }
    if (target.mode === 'staged') {
      // Unstaging rebuilds the index from HEAD plus the changes being kept.
      return { revert: { label: 'Unstage', run: (c) => after(SetStagedContent(path, file, shownNew, c)) } }
    }
    return {}
  })

  const modeLabel = { commit: '', staged: 'Staged', unstaged: 'Unstaged', conflict: 'Conflict', history: '', blame: '', found: '' }
</script>

<div class="diffview">
  <header class="view-head">
    <button class="btn quiet" onclick={onClose} title="Back to the graph"><Icon name="back" />Back</button>
    <span class="view-file" title={target.file}
      >{target.oldPath && target.oldPath !== target.file ? `${target.oldPath} → ` : ''}{target.file}</span
    >
    {#if modeLabel[target.mode]}<span class="view-badge">{modeLabel[target.mode]}</span>{/if}
    {#if contents?.kind === 'text' && engine && layout !== 'file'}
      <span class="engine {engine.kind}" title={engine.title}
        >{#if engine.kind === 'reading'}<span class="dots" aria-hidden="true"><span></span><span></span><span></span></span>{:else}<span
            class="dot"
            aria-hidden="true"
          ></span>{/if}{engine.label}</span
      >
    {/if}
    <span class="spacer"></span>
    {#if onOpen}
      <button class="btn quiet" onclick={() => onOpen({ mode: 'history', file: target.file })}>History</button>
      <button
        class="btn quiet"
        onclick={() =>
          onOpen({ mode: 'blame', file: target.file, commit: target.mode === 'commit' ? target.commit : undefined })}
        >Blame</button
      >
    {/if}
    {#if contents?.kind === 'text'}
      <button class="btn quiet" aria-pressed={wrap.on} onclick={toggleWrap} title="Wrap long lines ({formatShortcut('Alt+Z')})"
      ><Icon name="wrap" />Wrap</button
    >
      {#if diff?.language && layout !== 'file'}
        <span class="segmented" role="group" aria-label="Compare by">
          <button aria-pressed={readAs === 'code'} onclick={() => (readAs = 'code')} title="Compare the code: tokens, formatting and renames">Code</button>
          <button aria-pressed={readAs === 'lines'} onclick={() => (readAs = 'lines')} title="Compare whole lines, as git does">Lines</button>
        </span>
        <button
          class="switch"
          role="switch"
          aria-checked={hiding}
          disabled={stages || diff.engine !== 'code'}
          onclick={() => (hide = !hide)}
          title={stages
            ? 'Off while you stage: you see exactly what will be written'
            : diff.engine !== 'code'
              ? 'Lines show every change'
              : 'Hide changes that only touch whitespace, line breaks or how a value is written'}
          ><span class="track" aria-hidden="true"><span class="knob"></span></span>Hide formatting{#if stages}<Icon name="lock" size={13} />{/if}</button
        >
      {/if}
      <span class="segmented" role="group" aria-label="Diff layout">
        <button aria-pressed={layout === 'unified'} onclick={() => setLayout('unified')}>Unified</button>
        <button aria-pressed={layout === 'split'} onclick={() => setLayout('split')}>Split</button>
        <button aria-pressed={layout === 'file'} onclick={() => setLayout('file')} title="The whole file as of this version">File</button>
      </span>
      {#if layout === 'file' && previewer}
        <span class="segmented" role="group" aria-label="File view">
          <button aria-pressed={rendered} onclick={() => setRendered(true)}>Rendered</button>
          <button aria-pressed={!rendered} onclick={() => setRendered(false)}>Raw</button>
        </span>
      {/if}
    {/if}
  </header>

  {#if contents?.kind === 'text' && diff && layout !== 'file' && fallbackNote}
    <p class="fallback" role="status">
      <Icon name="info" size={15} />{fallbackNote}
      {#if diff.engine === 'lines' && diff.fallback === 'time' && !anyway}
        <button class="btn sm quiet" onclick={() => (anyway = true)}>Read it as code anyway</button>
      {/if}
    </p>
  {/if}
  {#if contents?.kind === 'text' && diff && layout !== 'file'}
    <div class="summary" role="group" aria-label="What changed">
      {#if code}
        {#if code.changes}<span class="sum"><span class="swatch change" aria-hidden="true"></span>{count(code.changes, 'change', 'changes')}</span>{/if}
        {#each code.renames.slice(0, 2) as r (r.from)}
          <!-- One item in the flex row, so the comma stays with the name before it. -->
          <span class="sum"><span class="swatch rename" aria-hidden="true"></span><span>rename: <code>{r.from}</code> → <code>{r.to}</code>, {count(r.count, 'place', 'places')}</span></span>
        {/each}
        {#if code.renames.length > 2}<span class="sum">{count(code.renames.length - 2, 'more rename', 'more renames')}</span>{/if}
      {:else}
        {#if diff.added}<span class="sum"><span class="swatch add" aria-hidden="true"></span>{count(diff.added, 'line added', 'lines added')}</span>{/if}
        {#if diff.removed}<span class="sum"><span class="swatch del" aria-hidden="true"></span>{count(diff.removed, 'line removed', 'lines removed')}</span>{/if}
      {/if}
      {#if diff.moves?.length}<span class="sum"><span class="swatch move" aria-hidden="true"></span>{count(diff.moves.length, 'move', 'moves')}</span>{/if}
      {#if code?.formatting}
        {#if hiding}
          <span class="sum hidden-note">{count(code.formatting, 'formatting change', 'formatting changes')} hidden</span>
          <button class="link" onclick={() => (hide = false)}>Show {code.formatting === 1 ? 'it' : 'them'}</button>
        {:else}
          <span class="sum"><span class="swatch format" aria-hidden="true"></span>{count(code.formatting, 'formatting change', 'formatting changes')}</span>
        {/if}
      {/if}
      <span class="spacer"></span>
      {#if stops.length}
        <span class="where" title="Lines matched by {diff.algorithm}">{current >= 0 ? `${current + 1} of ${stops.length}` : code ? count(stops.length, 'place', 'places') : count(stops.length, 'change', 'changes')}</span>
        {#if layout === 'unified'}
          <button class="step" aria-label="Previous change" title="Previous change (Shift+F7)" onclick={() => step(-1)}
            ><span class="flip"><Icon name="chevron" size={14} /></span></button
          >
          <button class="step" aria-label="Next change" title="Next change (F7)" onclick={() => step(1)}><Icon name="chevron" size={14} /></button>
        {/if}
      {/if}
    </div>
  {/if}

  <div class="body code-ground" class:fill={layout === 'split' && contents?.kind === 'text'}>
    {#if !contents}
      <Loading />
    {:else if contents.kind === 'large' && previewer?.binary && previewer.large && contents.new.exists}
      {#key target.file + (target.commit ?? '') + target.mode}
        <previewer.component path={target.file} text="" {loadImage} openFile={openRelated} {loadData} />
      {/key}
    {:else if contents.kind === 'large'}
      <div class="notice">
        <p>
          This file is large ({(Math.max(contents.old.size, contents.new.size) / (1 << 20)).toFixed(1)} MB). Showing it may be
          slow.
        </p>
        <button class="btn secondary" onclick={() => (force = true)}>Load anyway</button>
      </div>
    {:else if contents.kind === 'binary' && previewer?.binary && contents.new.exists}
      {#key target.file + (target.commit ?? '') + target.mode}
        <previewer.component path={target.file} text="" {loadImage} openFile={openRelated} {loadData} />
      {/key}
    {:else if contents.kind === 'binary'}
      <p class="muted">Binary file. Sizes: {contents.old.exists ? contents.old.size : '—'} → {contents.new.exists ? contents.new.size : '—'} bytes.</p>
    {:else if contents.kind === 'image'}
      <ImageDiff {contents} />
    {:else if layout === 'file'}
      {#if !contents.new.exists}
        <p class="muted hint">Deleted here. Showing the file as it was before.</p>
      {/if}
      {@const text = contents.new.exists ? contents.new.text : contents.old.text}
      {#if previewer && rendered}
        <previewer.component path={target.file} {text} {loadImage} openFile={openRelated} />
      {:else}
        <FileView path={target.file} {text} />
      {/if}
    {:else if layout === 'split' && diff}
      <SplitDiff
        path={target.file}
        oldText={contents.old.text}
        newText={contents.new.text}
        {diff}
        ribbon={prefs.diffRibbon}
        hideFormatting={hiding}
        {stops}
        {current}
        {jump}
        {labels}
        onStep={step}
        onPick={pick}
        onCompare={(k) => (comparing = k)}
      />
      {#if target.mode !== 'commit'}
        <p class="muted hint">Switch to Unified to stage or discard individual hunks and lines.</p>
      {/if}
    {:else if diff}
      <UnifiedDiff
        path={target.file}
        oldText={contents.old.text}
        newText={contents.new.text}
        {diff}
        {actions}
        hideFormatting={hiding}
        {stops}
        {current}
        {jump}
        onPick={pick}
        onCompare={(k) => (comparing = k)}
      />
    {/if}
    {#if comparing >= 0 && diff?.moves[comparing] && contents?.kind === 'text'}
      <MoveCompare
        path={target.file}
        move={diff.moves[comparing]}
        oldText={contents.old.text}
        newText={contents.new.text}
        language={diff.language}
        onClose={() => (comparing = -1)}
      />
    {/if}
  </div>
</div>

<style>
  .diffview {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }

  .spacer {
    flex: 1;
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }

  /* The split view scrolls its own sides. */
  .body.fill {
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .body.fill > :global(*:first-child) {
    flex: 1;
    min-height: 0;
  }

  .summary {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px 14px;
    padding: 6px 14px;
    border-bottom: 1px solid var(--paper-edge);
    font-size: 13px;
    color: var(--ink-soft);
  }

  .sum {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .swatch {
    width: 14px;
    height: 10px;
    border-radius: 2px;
  }

  .swatch.add {
    background: color-mix(in srgb, var(--success) 45%, transparent);
  }

  .swatch.del {
    background: color-mix(in srgb, var(--danger) 35%, transparent);
  }

  .swatch.move {
    background: color-mix(in srgb, var(--syn-string) 35%, transparent);
  }

  .swatch.change {
    background: linear-gradient(90deg, color-mix(in srgb, var(--danger) 40%, transparent), color-mix(in srgb, var(--success) 45%, transparent));
  }

  .swatch.rename {
    background: color-mix(in srgb, var(--syn-number) 45%, transparent);
  }

  .swatch.format {
    border: 1px dashed var(--ink-faint);
  }

  .sum code {
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .hidden-note {
    font-style: italic;
    color: var(--ink-mute);
  }

  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--link);
    font: inherit;
    text-decoration: underline;
    text-decoration-color: color-mix(in srgb, var(--link) 35%, transparent);
    text-underline-offset: 3px;
    cursor: pointer;
  }

  /* The engine chip: teal when read as code, plain for lines. */
  .engine {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 2px 8px;
    border-radius: 999px;
    font-family: var(--font-mono);
    font-size: 12px;
    white-space: nowrap;
  }

  .engine.code,
  .engine.reading {
    background: color-mix(in srgb, var(--accent-a) 12%, transparent);
    color: var(--accent-a);
  }

  .engine.lines {
    background: var(--paper-warm);
    color: var(--ink-soft);
  }

  .engine .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: currentColor;
  }

  .engine.lines .dot {
    background: var(--ink-faint);
  }

  .dots {
    display: inline-flex;
    gap: 2px;
  }

  .dots span {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: currentColor;
    animation: ink 1.2s cubic-bezier(0.2, 0.6, 0.2, 1) infinite;
  }

  .dots span:nth-child(2) {
    animation-delay: 0.2s;
  }

  .dots span:nth-child(3) {
    animation-delay: 0.4s;
  }

  @keyframes ink {
    0%,
    100% {
      opacity: 0.25;
    }
    50% {
      opacity: 1;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .dots span {
      animation: none;
      opacity: 0.7;
    }
  }

  .switch {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 4px 6px;
    border: 0;
    border-radius: 8px;
    background: none;
    color: var(--ink);
    font: inherit;
    font-size: 13px;
    cursor: pointer;
  }

  .switch:disabled {
    color: var(--ink-mute);
    cursor: default;
  }

  .track {
    position: relative;
    width: 30px;
    height: 18px;
    border-radius: 999px;
    background: var(--paper-edge);
  }

  .knob {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--paper);
    transition: left 0.12s;
  }

  .switch[aria-checked='true'] .track {
    background: var(--accent);
  }

  .switch[aria-checked='true'] .knob {
    left: 14px;
  }

  .fallback {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    margin: 0;
    padding: 6px 14px;
    border-bottom: 1px solid var(--paper-edge);
    background: var(--warning-wash);
    color: var(--ink);
    font-size: 13px;
  }

  .where {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .step {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 26px;
    margin-left: -8px;
    border: 1px solid var(--paper-edge);
    border-radius: 8px;
    background: var(--paper);
    color: var(--ink-soft);
    cursor: pointer;
  }

  .step:first-of-type {
    margin-left: 0;
  }

  .step:hover {
    color: var(--ink);
    border-color: var(--ink-faint);
  }

  .flip {
    display: inline-flex;
    transform: rotate(180deg);
  }

  .notice {
    padding: 16px;
  }

  .muted {
    color: var(--ink-mute);
    padding: 10px 14px;
    margin: 0;
  }

  .hint {
    font-size: 13px;
    font-style: italic;
  }
</style>
