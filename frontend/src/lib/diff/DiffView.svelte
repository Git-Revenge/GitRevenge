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
  import { FileContents, FileData, FileImage, SetStagedContent, SetWorkTreeContent } from '../../../wailsjs/go/app/App.js'
  import type { git } from '../../../wailsjs/go/models'
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

  provideCommands(() => (contents?.kind === 'text' ? [wrapCommand()] : []))

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

  let contents = $state<git.FileContents | null>(null)
  let force = $state(false)

  async function load() {
    const t = target as DiffTarget | null
    // A write that finishes after the diff has closed lands here with nothing left to show.
    if (!t) return
    const res = await session.run(() =>
      FileContents(path, t.mode, t.commit ?? '', t.file, t.oldPath ?? '', force),
    )
    if (res && t === target) contents = res
  }

  // Reload when the target changes, and when the repo changes on disk (for work tree diffs).
  $effect(() => {
    void target
    void force
    if (target.mode !== 'commit') void session.revision[path]
    void load()
  })

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

  <div class="body code-ground">
    {#if !contents}
      <Loading />
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
    {:else if layout === 'split'}
      <SplitDiff path={target.file} oldText={contents.old.text} newText={contents.new.text} />
      {#if target.mode !== 'commit'}
        <p class="muted hint">Switch to Unified to stage or discard individual hunks and lines.</p>
      {/if}
    {:else}
      <UnifiedDiff path={target.file} oldText={contents.old.text} newText={contents.new.text} {actions} />
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
