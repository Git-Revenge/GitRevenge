<script lang="ts">
  import { Refs, RepoState, RunOperationAction, Undo, UndoState } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime.js'
  import { bisect, expectedSteps } from './bisect.svelte'
  import { commands, formatShortcut, provideCommands } from './commands.svelte'
  import { abortEffect, abortedMessage, beads as stepBeads, blockedBy, continueLabel, finishedMessage, OP_NOUN, opNote, opTitle } from './operation'
  import { operations } from './operation.svelte'
  import { toasts } from './toasts.svelte'
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'
  import { reveal } from './ui/motion'

  let { path }: { path: string } = $props()

  let opState = $state<app.RepoState | null>(null)
  let busy = $state(false)

  async function refresh() {
    opState = (await session.run(() => RepoState(path))) ?? null
    operations.state[path] = opState
    void bisect.refresh(path)
  }

  // A merge, rebase, cherry-pick or revert in progress (issue #593).
  let conflicts = $derived(opState?.conflicts ?? [])
  let can = $derived(new Set(opState?.actions ?? []))
  let title = $derived(opState ? opTitle(opState) : null)
  let aborting = $state(false)
  /** The most files in conflict at this stop, so the strip can say how many are resolved. */
  let seen = $state(0)
  let stopKey = ''
  $effect(() => {
    const key = opState?.operation ? `${path} ${opState.operation} ${opState.commit}` : ''
    const n = opState?.conflicts.length ?? 0
    if (key !== stopKey) {
      stopKey = key
      seen = n
      aborting = false
    } else if (n > seen) seen = n
  })

  async function act(action: string) {
    const before = opState
    const failed = session.error
    busy = true
    aborting = false
    await session.run(() => RunOperationAction(path, action))
    busy = false
    await refresh()
    if (before?.operation && !opState?.operation && session.error === failed) void finished(before, action)
  }

  /** Says what happened once an operation is over, with the way back. */
  async function finished(was: app.RepoState, action: string) {
    const repo = path
    if (action === 'abort') {
      toasts.show(abortedMessage(was))
      return
    }
    const label = (await UndoState(repo).catch(() => null))?.undo ?? ''
    toasts.show(
      finishedMessage(was),
      label
        ? {
            label: 'Undo',
            run: async () => {
              const now = await session.run(() => UndoState(repo))
              if (now?.undo !== label) {
                toasts.show('Something else happened since, so this can no longer be undone here. The toolbar Undo steps back one change at a time.')
                return
              }
              await session.run(() => Undo(repo))
            },
          }
        : undefined,
    )
    if (was.operation !== 'rebase') return
    // A rebased branch that was already pushed has left its old commits on the remote.
    const refs = (await Refs(repo).catch(() => null)) ?? []
    const head = refs.find((r) => r.kind === 'local' && r.head)
    if (head?.upstream && head.behind > 0) {
      toasts.show(`${head.upstream} still has the commits from before the rebase.`, { label: 'Force push…', run: () => void commands.all().find((c) => c.id === 'repo.force-push')?.run() })
    }
  }

  function resolveNext() {
    if (conflicts[0]) operations.resolve = { path, file: conflicts[0] }
  }

  function goOn() {
    if (!busy && can.has('continue') && !conflicts.length) void act('continue')
  }

  // Re-fetch when the tab changes or the backend reports a change on disk.
  $effect(() => {
    void session.revision[path]
    void refresh()
  })

  // A bisect has a strip of its own (issue #592): it asks a question at every step.
  let run = $derived(opState?.operation === 'bisect' ? bisect.of(path) : null)
  let done = $derived(!!run && (!!run.firstBad || run.candidates.length > 0))
  let total = $derived(run ? expectedSteps(run) : 0)
  let beads = $derived(Array.from({ length: Math.min(total, 12) }, (_, i) => i))
  let stopping = $state(false)
  let copied = $state(false)
  const short = (id: string) => id.slice(0, 7)

  async function stop() {
    stopping = false
    await bisect.stop(path)
    await refresh()
  }

  async function copyLog() {
    const ok = await ClipboardSetText(bisect.log(path)).catch(() => false)
    if (ok === false) return
    copied = true
    setTimeout(() => (copied = false), 1500)
  }

  // One key each while a commit is under test, so answering never needs the mouse.
  provideCommands(() =>
    !run && opState?.operation
      ? [
          ...(can.has('continue') && !conflicts.length ? [{ id: 'op.continue', group: 'Operation', title: continueLabel(opState), keywords: 'continue finish', shortcut: 'Mod+Enter', run: goOn }] : []),
          ...(conflicts.length ? [{ id: 'op.resolve', group: 'Operation', title: 'Resolve the next conflicted file', keywords: 'conflict', run: resolveNext }] : []),
          ...(can.has('skip') ? [{ id: 'op.skip', group: 'Operation', title: 'Skip this commit', run: () => act('skip') }] : []),
          { id: 'op.abort', group: 'Operation', title: `Abort the ${OP_NOUN[opState.operation] ?? opState.operation}…`, keywords: 'stop cancel', run: () => (aborting = true) },
        ]
      : run && !done
      ? [
          { id: 'bisect.good', group: 'Bisect', title: 'Bisect: this commit works', keywords: 'good', shortcut: 'G', run: () => bisect.mark(path, 'good') },
          { id: 'bisect.bad', group: 'Bisect', title: 'Bisect: this commit is broken', keywords: 'bad', shortcut: 'B', run: () => bisect.mark(path, 'bad') },
          { id: 'bisect.skip', group: 'Bisect', title: "Bisect: can't tell, skip this commit", keywords: 'skip', shortcut: 'S', run: () => bisect.mark(path, 'skip') },
          ...(run.steps > 0 ? [{ id: 'bisect.undo', group: 'Bisect', title: 'Bisect: take back the last answer', keywords: 'undo', run: () => bisect.undo(path) }] : []),
          { id: 'bisect.stop', group: 'Bisect', title: 'Stop the bisect', keywords: 'reset abort', run: () => stop() },
        ]
      : run
        ? [{ id: 'bisect.stop', group: 'Bisect', title: 'Finish the bisect', keywords: 'reset done', run: () => stop() }]
        : [],
  )
</script>

{#if run}
  <div class={['op', 'bisect', done && 'found']} role="status" in:reveal out:reveal={{ out: true }}>
    <Icon name={done ? 'check' : 'search'} />
    <strong>
      {#if run.firstBad}Found it in {run.steps} {run.steps === 1 ? 'step' : 'steps'}{:else if done}It's one of {run.candidates.length} commits{:else}Bisecting{/if}
    </strong>
    <span class="beads" aria-hidden="true">
      {#each beads as i (i)}<i class:done={done || i < run.steps} class:now={!done && i === run.steps}></i>{/each}
    </span>
    <span class="meta">
      {#if run.firstBad}The first bad commit is {short(run.firstBad)}{:else if done}Skipped commits hide which{:else}Step {run.steps + 1} of about {total} · {run.remaining === 1 ? '1 commit' : `${run.remaining} commits`} left after this{/if}
    </span>
    <span class="actions">
      {#if done}
        <button class="btn sm quiet" onclick={copyLog}>{copied ? 'Copied' : 'Copy the log'}</button>
        <button class="btn primary" disabled={bisect.busy} onclick={stop}>Done, back to {run.origin || 'where you were'}</button>
      {:else if stopping}
        <span class="ask">Stop, and go back to {run.origin || 'where you were'}?</span>
        <button class="btn sm danger" disabled={bisect.busy} onclick={stop}>Stop the bisect</button>
        <button class="btn sm secondary" onclick={() => (stopping = false)}>Keep going</button>
      {:else}
        <span class="ask">Is it working at <span class="mono">{short(run.current)}</span>?</span>
        <button class="btn success" disabled={bisect.busy} onclick={() => bisect.mark(path, 'good')} title="It works here (G)">Good <kbd>G</kbd></button>
        <button class="btn danger" disabled={bisect.busy} onclick={() => bisect.mark(path, 'bad')} title="It's broken here (B)">Bad <kbd>B</kbd></button>
        <button class="btn secondary" disabled={bisect.busy} onclick={() => bisect.mark(path, 'skip')} title="This commit can't be tested (S)">Can't tell <kbd class="plain">S</kbd></button>
        <span class="rule" aria-hidden="true"></span>
        <button class="btn sm quiet" disabled={bisect.busy || run.steps === 0} onclick={() => bisect.undo(path)} title="Take back the last answer">Undo last</button>
        <button class="btn sm text danger" onclick={() => (stopping = true)}>Stop…</button>
      {/if}
    </span>
  </div>
{:else if opState?.operation && title}
  <div class={['op', 'run', conflicts.length ? 'stopped' : 'ready']} role="status" in:reveal out:reveal={{ out: true }}>
    <Icon name={conflicts.length ? 'warning' : opState.operation === 'merge' ? 'merge' : 'check'} />
    <div class="what">
      <strong>
        {title.verb}
        {#if title.a}<span class="mono">{title.a}</span>{/if}
        {title.join}
        {#if title.b}<span class="mono">{title.b}</span>{/if}
      </strong>
      <span class="note">{opNote(opState, seen)}</span>
    </div>
    {#if stepBeads(opState).length}
      <span class="beads" role="img" aria-label="{opState.step - 1} of {opState.total} done, {conflicts.length ? 'stopped at' : 'now at'} {opState.step}">
        {#each stepBeads(opState) as b, i (i)}<i class={b}></i>{/each}
      </span>
    {/if}
    <span class="actions">
      {#if aborting}
        <span class="ask wide">{abortEffect(opState, seen)}</span>
        <button class="btn sm danger" disabled={busy} onclick={() => act('abort')}>Abort the {OP_NOUN[opState.operation] ?? opState.operation}</button>
        <button class="btn sm secondary" onclick={() => (aborting = false)}>Keep going</button>
      {:else}
        {#if conflicts.length}
          <button class="btn sm secondary" onclick={resolveNext}>Resolve {conflicts.length} {conflicts.length === 1 ? 'file' : 'files'}</button>
        {/if}
        {#if can.has('continue')}
          <button class={['btn primary', conflicts.length && 'off']} aria-disabled={conflicts.length > 0 || undefined} disabled={busy} title={blockedBy(opState) || undefined} onclick={goOn}>
            {conflicts.length ? 'Continue' : continueLabel(opState)}
            {#if !conflicts.length}<kbd>{formatShortcut('Mod+Enter')}</kbd>{/if}
          </button>
        {/if}
        {#if can.has('skip')}<button class="btn sm quiet" disabled={busy} onclick={() => act('skip')}>Skip this commit</button>{/if}
        {#if can.has('abort')}<button class="btn sm text danger" disabled={busy} onclick={() => (aborting = true)}>Abort…</button>{/if}
      {/if}
    </span>
  </div>
{/if}

<style>
  .op {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 12px;
    align-items: center;
    padding: 6px 8px 6px 14px;
    background: var(--warning-wash);
    border-bottom: 1px solid var(--paper-edge);
  }

  .op > :global(svg) {
    color: var(--warning);
  }

  /* A bisect isn't a warning: it's a question being asked, on the warm paper. */
  .op.bisect {
    min-height: 48px;
    background: var(--paper-warm);
  }

  .op.bisect > :global(svg) {
    color: var(--accent-a);
  }

  .op.bisect.found {
    background: var(--success-wash);
  }

  .op.bisect.found > :global(svg) {
    color: var(--success);
  }

  strong {
    font-weight: 600;
  }

  /* An operation waiting on you: warm paper while it's stopped on conflicts, plain once it's
     ready to go on. The warning is the icon's to give, not the whole bar's. */
  .op.run {
    min-height: 48px;
    background: var(--paper-warm);
  }

  .op.run.ready {
    background: none;
  }

  .op.run.ready > :global(svg) {
    color: var(--success);
  }

  .what {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .note {
    font-size: 13px;
    color: var(--ink-mute);
    overflow-wrap: anywhere;
  }

  .what .mono {
    font-size: 13px;
    font-weight: 500;
  }

  .beads i.bad {
    background: var(--warning);
    box-shadow: 0 0 0 3px var(--warning-wash);
  }

  .ask.wide {
    max-width: 520px;
    color: var(--ink-soft);
  }

  /* Off until nothing is conflicted, but still there to say why. */
  .btn.off {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .actions {
    margin-left: auto;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
  }

  .beads {
    display: inline-flex;
    align-items: center;
    gap: 5px;
  }

  .beads i {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--paper-edge);
  }

  .beads i.done {
    background: var(--success);
  }

  .beads i.now {
    background: var(--accent-a);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent-a) 20%, transparent);
  }

  .ask {
    margin-right: 4px;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .mono {
    font-family: var(--font-mono);
    color: var(--ink-soft);
  }

  kbd {
    padding: 0 6px;
    border-radius: var(--r-sm);
    background: color-mix(in srgb, var(--on-accent) 24%, transparent);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 18px;
  }

  kbd.plain {
    background: var(--paper-deep);
    color: var(--ink-soft);
  }

  .rule {
    width: 1px;
    height: 22px;
    margin: 0 2px;
    background: var(--paper-edge);
  }
</style>
