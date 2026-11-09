<script lang="ts">
  import { RepoState, RunOperationAction } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime.js'
  import { bisect, expectedSteps } from './bisect.svelte'
  import { provideCommands } from './commands.svelte'
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'
  import { reveal } from './ui/motion'

  let { path }: { path: string } = $props()

  let opState = $state<app.RepoState | null>(null)
  let busy = $state(false)

  const labels: Record<string, string> = {
    merge: 'Merge',
    rebase: 'Rebase',
    'cherry-pick': 'Cherry-pick',
    revert: 'Revert',
    bisect: 'Bisect',
    am: 'Applying patches',
  }

  async function refresh() {
    opState = (await session.run(() => RepoState(path))) ?? null
    void bisect.refresh(path)
  }

  async function act(action: string) {
    busy = true
    await session.run(() => RunOperationAction(path, action))
    busy = false
    await refresh()
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
    run && !done
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
{:else if opState?.operation}
  <div class="op" role="status" in:reveal out:reveal={{ out: true }}>
    <Icon name="warning" />
    <strong>{labels[opState.operation] ?? opState.operation} in progress</strong>
    {#if opState.total > 0}<span class="meta">Step {opState.step} of {opState.total}</span>{/if}
    {#if opState.conflicts.length > 0}
      <span class="conflicts" title={opState.conflicts.join('\n')}>
        {opState.conflicts.length} conflicted {opState.conflicts.length === 1 ? 'file' : 'files'}
      </span>
    {/if}
    <span class="actions">
      {#each opState.actions as action (action)}
        <button class={['btn sm', action === 'continue' ? 'primary' : 'secondary']} disabled={busy} onclick={() => act(action)}>
          {action === 'abort' && opState.operation === 'bisect' ? 'Stop bisect' : action[0].toUpperCase() + action.slice(1)}
        </button>
      {/each}
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

  .conflicts {
    color: var(--danger);
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
