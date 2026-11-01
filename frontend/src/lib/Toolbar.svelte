<script lang="ts">
  import { onMount } from 'svelte'
  import { CancelNetwork, Fetch, Pull, PushCurrent, Redo, Refs, Undo, UndoState } from '../../wailsjs/go/app/App.js'
  import { CONFIRM_FORCE_PUSH, kindWarning } from './branchKinds'
  import type { Ref } from './graph/rowCache'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import { formatShortcut, isMac, provideCommands, type Command } from './commands.svelte'
  import ContextMenu, { type MenuItem } from './ContextMenu.svelte'
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'

  let { path }: { path: string } = $props()

  /** The running network operation, if any. */
  let running = $state<string | null>(null)
  let progress = $state('')
  let menu = $state<{ x: number; y: number; items: MenuItem[] } | null>(null)

  const labels: Record<string, string> = { fetch: 'Fetching', pull: 'Pulling', push: 'Pushing' }

  /** Labels of the operations Undo and Redo would reverse; "" when unavailable. */
  let undoLabel = $state('')
  let redoLabel = $state('')

  async function refreshUndo() {
    // With no answer, Undo and Redo are just disabled.
    const info = await UndoState(path).catch(() => null)
    undoLabel = info?.undo ?? ''
    redoLabel = info?.redo ?? ''
  }

  // Re-read after anything changes on disk, and whenever the undo history itself changes:
  // an operation can reach disk (and the watcher) before its undo entry is recorded.
  $effect(() => {
    void session.revision[path]
    void session.undoRevision[path]
    void refreshUndo()
  })

  async function undo() {
    if (!undoLabel || running) return
    await session.run(() => Undo(path))
    await refreshUndo()
  }

  async function redo() {
    if (!redoLabel || running) return
    await session.run(() => Redo(path))
    await refreshUndo()
  }

  async function run(op: string, fn: () => Promise<void>) {
    running = op
    progress = ''
    await session.run(fn)
    running = null
    progress = ''
    await refreshUndo()
  }

  const fetchAll = () => run('fetch', () => Fetch(path, ''))
  const pull = (mode: string) => run('pull', () => Pull(path, mode))
  const push = (force: boolean) => run('push', () => PushCurrent(path, force))

  // Force-pushing a Trunk, Release or Environment branch asks first, naming the kind.
  let confirmForce = $state<{ branch: string; warning: string } | null>(null)

  async function forcePush() {
    const refs = ((await session.run(() => Refs(path))) as Ref[] | undefined) ?? []
    const head = refs.find((r) => r.kind === 'local' && r.head)
    const warning = head ? kindWarning(head.short, head.branchKind, CONFIRM_FORCE_PUSH) : ''
    if (warning && head) confirmForce = { branch: head.short, warning }
    else await push(true)
  }

  provideCommands(() => {
    if (running) return []
    const list: Command[] = [
      { id: 'repo.fetch', group: 'Repository', title: 'Fetch all remotes', shortcut: 'Mod+Shift+F', run: fetchAll },
      { id: 'repo.pull', group: 'Repository', title: 'Pull', keywords: 'update download', shortcut: 'Mod+Shift+L', run: () => pull('') },
      { id: 'repo.pull-ff', group: 'Repository', title: 'Pull (fast-forward only)', run: () => pull('ff-only') },
      { id: 'repo.pull-merge', group: 'Repository', title: 'Pull (merge)', run: () => pull('merge') },
      { id: 'repo.pull-rebase', group: 'Repository', title: 'Pull (rebase)', run: () => pull('rebase') },
      { id: 'repo.push', group: 'Repository', title: 'Push the current branch', keywords: 'upload publish', shortcut: 'Mod+Shift+U', run: () => push(false) },
    ]
    // Always bound (and listed on the cheat sheet); the palette shows them only when there's
    // something to undo or redo, and running them otherwise does nothing.
    list.push(
      { id: 'repo.undo', group: 'Repository', title: undoLabel ? `Undo: ${undoLabel}` : 'Undo', shortcut: 'Mod+Z', hidden: !undoLabel, run: undo },
      {
        id: 'repo.redo',
        group: 'Repository',
        title: redoLabel ? `Redo: ${redoLabel}` : 'Redo',
        shortcut: 'Mod+Shift+Z',
        aliases: isMac ? [] : ['Mod+Y'],
        hidden: !redoLabel,
        run: redo,
      },
    )
    return list
  })

  function dropdown(e: MouseEvent, items: MenuItem[]) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    menu = { x: r.left, y: r.bottom + 2, items }
  }

  onMount(() =>
    EventsOn('net:progress', (p: { path: string; phase: string; percent: number; line: string }) => {
      if (p.path !== path) return
      progress = p.phase ? `${p.phase} ${p.percent}%` : p.line
    }),
  )
</script>

<div class="toolbar" role="toolbar" aria-label="Repository actions">
  <button class="btn quiet" disabled={!undoLabel || !!running} onclick={undo} title={undoLabel ? `Undo: ${undoLabel} (${formatShortcut('Mod+Z')})` : 'Nothing to undo'}>
    <Icon name="undo" />Undo
  </button>
  <button class="btn quiet" disabled={!redoLabel || !!running} onclick={redo} title={redoLabel ? `Redo: ${redoLabel} (${formatShortcut('Mod+Shift+Z')})` : 'Nothing to redo'}>
    <Icon name="redo" />Redo
  </button>
  <span class="sep" aria-hidden="true"></span>
  <button class="btn quiet" disabled={!!running} onclick={fetchAll} title="Fetch all remotes ({formatShortcut('Mod+Shift+F')})"><Icon name="fetch" />Fetch</button>
  <span class="split">
    <button class="btn quiet" disabled={!!running} onclick={() => pull('')} title="Pull using your git config (pull.rebase) ({formatShortcut('Mod+Shift+L')})"><Icon name="pull" />Pull</button>
    <button
      class="btn quiet caret"
      disabled={!!running}
      aria-label="Pull options"
      onclick={(e) =>
        dropdown(e, [
          { label: 'Pull (fast-forward only)', icon: 'pull', run: () => pull('ff-only') },
          { label: 'Pull (merge)', icon: 'merge', run: () => pull('merge') },
          { label: 'Pull (rebase)', icon: 'rebase', run: () => pull('rebase') },
        ])}><Icon name="chevron" size={13} /></button
    >
  </span>
  <span class="split">
    <button class="btn quiet" disabled={!!running} onclick={() => push(false)} title="Push the current branch ({formatShortcut('Mod+Shift+U')})"><Icon name="push" />Push</button>
    <button
      class="btn quiet caret"
      disabled={!!running}
      aria-label="Push options"
      onclick={(e) => dropdown(e, [{ label: 'Force push (with lease)', icon: 'push', danger: true, run: forcePush }])}><Icon name="chevron" size={13} /></button
    >
  </span>

  {#if running}
    <span class="progress" role="status">
      <span class="spinner" aria-hidden="true"></span>
      <span class="text">{labels[running]}{progress ? `: ${progress}` : '…'}</span>
      <button class="btn text cancel" onclick={() => CancelNetwork(path)}>Cancel</button>
    </span>
  {/if}
</div>

{#if menu}
  <ContextMenu x={menu.x} y={menu.y} items={menu.items} onClose={() => (menu = null)} />
{/if}

{#if confirmForce}
  {@const c = confirmForce}
  <Modal title="Force push {c.branch}?" eyebrow="careful" onClose={() => (confirmForce = null)}>
    <p class="confirm-text">{c.warning} Force pushing replaces its history on the remote for everyone, as long as nobody has pushed since you last fetched.</p>
    <div class="form-actions">
      <InkButton variant="secondary" onclick={() => (confirmForce = null)}>Cancel</InkButton>
      <InkButton
        variant="danger"
        onclick={() => {
          confirmForce = null
          void push(true)
        }}>Force push anyway</InkButton
      >
    </div>
  </Modal>
{/if}

<style>
  .confirm-text {
    margin: 0;
    font-size: 15px;
    line-height: 1.55;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 2px;
    min-height: 44px;
    padding: 0 12px;
    white-space: nowrap;
    overflow: hidden;
    border-bottom: 1px solid var(--paper-edge);
  }

  .split {
    display: inline-flex;
  }

  /* The caret half of a split button: narrow, tucked against its action. */
  .caret {
    min-width: 0;
    width: 22px;
    padding: 0;
    margin-left: -4px;
  }

  .sep {
    width: 1px;
    height: 20px;
    margin: 0 8px;
    background: var(--paper-edge);
  }

  .progress {
    margin-left: auto;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 8px;
    padding-left: 12px;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    white-space: nowrap;
  }

  /* Only the text shrinks, so Cancel always stays clickable. */
  .progress .text {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .cancel {
    flex-shrink: 0;
  }

  .spinner {
    flex: none;
    width: 11px;
    height: 11px;
    border: 2px solid var(--accent);
    border-top-color: transparent;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
