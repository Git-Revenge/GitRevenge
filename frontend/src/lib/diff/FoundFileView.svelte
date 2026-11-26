<script lang="ts">
  // A file Find in files found something in (#980): the whole file, at the working tree or
  // the commit searched, opened at the line, with every match marked. F7 and Shift+F7 step
  // through the matches; the current one is outlined and kept in view.
  import { FileContents } from '../../../wailsjs/go/app/App.js'
  import type { git } from '../../../wailsjs/go/models'
  import { formatShortcut, provideCommands } from '../commands.svelte'
  import { lineAt, lineStarts, matchAtLine, matchesIn } from '../find/find'
  import { session } from '../session.svelte'
  import Icon from '../ui/Icon.svelte'
  import Loading from '../ui/Loading.svelte'
  import type { DiffTarget } from './DiffView.svelte'
  import FileView from './FileView.svelte'
  import NotDownloaded from './NotDownloaded.svelte'
  import { toggleWrap, wrap } from './wrap.svelte'

  let {
    path,
    target,
    onClose,
    onOpen,
  }: {
    path: string
    target: DiffTarget
    onClose: () => void
    /** Opens the file's history or blame. */
    onOpen: (t: DiffTarget) => void
  } = $props()

  let contents = $state<git.FileContents | null>(null)
  let force = $state(false)
  /** Counts downloads a partial clone finished for this view (#1003), so it loads again. */
  let downloaded = $state(0)

  $effect(() => {
    const t = target
    const f = force
    void downloaded
    contents = null
    // A commit's version of the file, or the working tree's (the unstaged side's new one).
    void session.run(() => FileContents(path, t.commit ? 'commit' : 'unstaged', t.commit ?? '', t.file, '', f)).then((c) => {
      if (t === target && c) contents = c
    })
  })

  let text = $derived(contents?.kind === 'text' ? contents.new.text : '')
  let starts = $derived(lineStarts(text))
  let marks = $derived(target.find ? matchesIn(text, target.find) : [])
  // Where the file opens: the match on the line that was clicked. F7 moves on from there.
  // Raw, so `for` stays the target itself, which is what it's compared with.
  let stepped = $state.raw<{ for: DiffTarget; text: string; at: number } | null>(null)
  let at = $derived(stepped && stepped.for === target && stepped.text === text ? stepped.at : matchAtLine(marks, starts, target.line ?? 1))
  let atLine = $derived(marks[at] ? lineAt(starts, marks[at].from) : (target.line ?? 1))

  function step(d: 1 | -1) {
    if (!marks.length) return
    stepped = { for: target, text, at: (at + d + marks.length) % marks.length }
  }

  provideCommands(() => [
    { id: 'find.nextMatch', group: 'View', title: 'Next match', keywords: 'find in files', shortcut: 'F7', run: () => step(1) },
    { id: 'find.prevMatch', group: 'View', title: 'Previous match', keywords: 'find in files', shortcut: 'Shift+F7', run: () => step(-1) },
  ])
</script>

<div class="found-file">
  <header class="view-head">
    <button class="btn quiet" onclick={onClose} title="Back to the graph"><Icon name="back" />Back</button>
    <span class="view-file" title={target.file}>{target.file}</span>
    <span class="meta where">{target.commit ? `at ${target.commit.slice(0, 7)}` : 'working tree'}</span>
    <span class="spacer"></span>
    {#if marks.length}
      <span class="meta" aria-live="polite">{at + 1} of {marks.length}</span>
      <button class="btn quiet icon" aria-label="Previous match" title="Previous match ({formatShortcut('Shift+F7')})" onclick={() => step(-1)}><Icon name="chevron" size={14} /></button>
      <button class="btn quiet icon down" aria-label="Next match" title="Next match ({formatShortcut('F7')})" onclick={() => step(1)}><Icon name="chevron" size={14} /></button>
    {/if}
    <button class="btn quiet" aria-pressed={wrap.on} onclick={toggleWrap} title="Wrap long lines ({formatShortcut('Alt+Z')})"><Icon name="wrap" />Wrap</button>
    <button class="btn quiet" onclick={() => onOpen({ mode: 'history', file: target.file })}><Icon name="history" />History</button>
    <button class="btn quiet" onclick={() => onOpen({ mode: 'blame', file: target.file, commit: target.commit, line: atLine })}><Icon name="blame" />Blame</button>
  </header>

  <div class="body code-ground">
    {#if !contents}
      <Loading />
    {:else if contents.kind === 'missing'}
      {#key (contents.missing ?? []).join()}
        <NotDownloaded {path} ids={contents.missing ?? []} onDone={() => (downloaded += 1)} />
      {/key}
    {:else if contents.kind === 'large'}
      <div class="notice">
        <p>This file is large ({(contents.new.size / (1 << 20)).toFixed(1)} MB). Showing it may be slow.</p>
        <button class="btn secondary" onclick={() => (force = true)}>Load anyway</button>
      </div>
    {:else if contents.kind !== 'text' || !contents.new.exists}
      <div class="notice"><p>{contents.new.exists ? "This file isn't text, so it can't be shown here." : "This file isn't there any more."}</p></div>
    {:else}
      {#key target.file + (target.commit ?? '')}
        <FileView path={target.file} {text} {marks} active={at} line={target.line ?? 0} />
      {/key}
    {/if}
  </div>
</div>

<style>
  .found-file {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }

  .spacer {
    flex: 1;
  }

  .where {
    white-space: nowrap;
  }

  /* One chevron icon, turned: up for the previous match, down for the next. */
  .icon :global(svg) {
    transform: rotate(180deg);
  }

  .icon.down :global(svg) {
    transform: none;
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }

  .notice {
    padding: 24px;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
    color: var(--ink-soft);
  }
</style>
