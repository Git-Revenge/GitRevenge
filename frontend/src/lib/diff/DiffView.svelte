<script lang="ts" module>
  /** What to compare. Opened from the commit panel or the working tree panel. */
  export interface DiffTarget {
    /** 'conflict' opens the conflict tool instead of a diff. */
    mode: 'commit' | 'staged' | 'unstaged' | 'conflict' | 'history' | 'blame'
    file: string
    oldPath?: string
    commit?: string
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

  async function load() {
    const t = target as DiffTarget | null
    // A write that finishes after the diff has closed lands here with nothing left to show.
    if (!t) return
    const res = await session.run(() => FileDiff(path, t.mode, t.commit ?? '', t.file, t.oldPath ?? '', force))
    if (res && t === target) loaded = res
  }

  // Reload when the target changes, when the repo changes on disk (for work tree diffs), and
  // when the diff settings change.
  $effect(() => {
    void target
    void force
    void prefs.diffAlgorithm
    void prefs.diffMoves
    if (target.mode !== 'commit') void session.revision[path]
    void load()
  })

  // The changes to step through. Opening another file starts again from none.
  let stops = $derived(diff ? stopsOf(diff) : [])
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

  const modeLabel = { commit: '', staged: 'Staged', unstaged: 'Unstaged', conflict: 'Conflict', history: '', blame: '' }
</script>

<div class="diffview">
  <header class="view-head">
    <button class="btn quiet" onclick={onClose} title="Back to the graph"><Icon name="back" />Back</button>
    <span class="view-file" title={target.file}
      >{target.oldPath && target.oldPath !== target.file ? `${target.oldPath} → ` : ''}{target.file}</span
    >
    {#if modeLabel[target.mode]}<span class="view-badge">{modeLabel[target.mode]}</span>{/if}
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

  {#if contents?.kind === 'text' && diff && layout !== 'file'}
    <div class="summary" role="group" aria-label="What changed">
      {#if diff.added}<span class="sum"><span class="swatch add" aria-hidden="true"></span>{count(diff.added, 'line added', 'lines added')}</span>{/if}
      {#if diff.removed}<span class="sum"><span class="swatch del" aria-hidden="true"></span>{count(diff.removed, 'line removed', 'lines removed')}</span>{/if}
      {#if diff.moves?.length}<span class="sum"><span class="swatch move" aria-hidden="true"></span>{count(diff.moves.length, 'move', 'moves')}</span>{/if}
      <span class="spacer"></span>
      {#if stops.length}
        <span class="where" title="Lines matched by {diff.algorithm}">{current >= 0 ? `${current + 1} of ${stops.length}` : count(stops.length, 'change', 'changes')}</span>
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
      <SplitDiff path={target.file} oldText={contents.old.text} newText={contents.new.text} {diff} ribbon={prefs.diffRibbon} {stops} {current} {jump} {labels} onStep={step} />
      {#if target.mode !== 'commit'}
        <p class="muted hint">Switch to Unified to stage or discard individual hunks and lines.</p>
      {/if}
    {:else if diff}
      <UnifiedDiff path={target.file} oldText={contents.old.text} newText={contents.new.text} {diff} {actions} {stops} {current} {jump} />
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
