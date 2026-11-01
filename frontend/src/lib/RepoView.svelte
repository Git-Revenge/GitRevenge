<script lang="ts">
  import { CommitRow, WorkingTree } from '../../wailsjs/go/app/App.js'
  import { provideCommands } from './commands.svelte'
  import ContextMenu, { type MenuItem } from './ContextMenu.svelte'
  import Dialog, { type DialogSpec } from './Dialog.svelte'
  import RebaseEditor from './RebaseEditor.svelte'
  import RemotesDialog from './RemotesDialog.svelte'
  import WorktreesDialog from './WorktreesDialog.svelte'
  import Toolbar from './Toolbar.svelte'
  import Icon from './ui/Icon.svelte'
  import { commitMenuItems, branchAtDialog, checkoutRef, dropItems, newBranchDialog, refItems, type MenuContext } from './menus'
  import ReflogDialog from './ReflogDialog.svelte'
  import CommitPanel from './CommitPanel.svelte'
  import type { DiffTarget } from './diff/DiffView.svelte'
  import Lazy from './ui/Lazy.svelte'
  import GraphView from './graph/GraphView.svelte'
  import type { Ref, Row } from './graph/rowCache'
  import { session } from './session.svelte'
  import Sidebar from './Sidebar.svelte'
  import WorkingTreePanel from './WorkingTreePanel.svelte'

  let { path }: { path: string } = $props()

  let graph: GraphView
  let selected = $state<Row | undefined>()
  /** Which panel is on the right: uncommitted changes, or the selected commit. */
  let view = $state<'wip' | 'commit'>('commit')
  let changes = $state(0)
  let menu = $state<{ x: number; y: number; items: MenuItem[] } | null>(null)

  let dialog = $state<DialogSpec | null>(null)
  let remotesOpen = $state(false)
  let worktreesOpen = $state(false)
  /** Commit after which the interactive rebase editor rewrites history, or "" when closed. */
  let rebaseBase = $state('')
  /** The ref whose reflog is open, or "" when closed. */
  let reflogRef = $state('')
  const ctx: MenuContext = {
    get path() {
      return path
    },
    current: () => {
      const head = session.activeTab?.head ?? ''
      return head.startsWith('detached') ? '' : head
    },
    openDialog: (spec) => (dialog = spec),
    openRebase: (base) => {
      diff = null
      rebaseBase = base
    },
    openReflog: (ref) => (reflogRef = ref),
  }

  provideCommands(() => [
    { id: 'branch.new', group: 'Branch', title: 'New branch…', keywords: 'create', shortcut: 'Mod+B', run: () => (dialog = newBranchDialog(ctx)) },
    // The rebase editor is left out: Escape shouldn't throw away a plan being edited.
    ...(diff ? [{ id: 'view.close', group: 'View', title: 'Back to the graph', keywords: 'close diff', shortcut: 'Escape', run: () => (diff = null) }] : []),
    { id: 'repo.remotes', group: 'Repository', title: 'Manage remotes…', keywords: 'origin url', run: () => (remotesOpen = true) },
    { id: 'repo.reflog', group: 'Repository', title: 'Show the reflog…', keywords: 'history head moves recover lost commit', run: () => (reflogRef = 'HEAD') },
    { id: 'repo.worktrees', group: 'Repository', title: 'Manage worktrees…', keywords: 'worktree checkout folder parallel', run: () => (worktreesOpen = true) },
    {
      id: 'view.changes',
      group: 'View',
      title: 'Show uncommitted changes',
      keywords: 'working tree stage commit',
      run: () => {
        view = 'wip'
        if (diff?.mode === 'commit') diff = null
      },
    },
  ])

  function openMenu(items: MenuItem[], x: number, y: number) {
    menu = items.length ? { x, y, items } : null
  }

  /** A file diff shown in place of the graph, or null for the graph. */
  let diff = $state<DiffTarget | null>(null)
  let checkedInitial = false

  async function refreshCount() {
    const wt = await session.run(() => WorkingTree(path))
    if (!wt) return
    changes = wt.status.files.length
    // Open on the changes when there are any.
    if (!checkedInitial) {
      checkedInitial = true
      if (changes > 0) view = 'wip'
    }
  }

  $effect(() => {
    void session.revision[path]
    void refreshCount()
  })

  function onSelect(row: Row | undefined, auto: boolean) {
    selected = row
    if (!auto) diff = null
    // The automatic HEAD selection on load must not pull focus away from the changes.
    if (!auto) view = 'commit'
  }

  /** Shows a commit in the graph (from blame), closing the file view. */
  async function revealCommit(commit: string) {
    const row = await session.run(() => CommitRow(path, commit))
    if (row === undefined) return
    if (row < 0) {
      session.error = 'That commit isn’t in the graph: it isn’t on any branch. Start a branch there to bring it back.'
      return
    }
    diff = null
    graph.reveal(row)
  }

  async function pick(ref: Ref) {
    const row = await session.run(() => CommitRow(path, ref.commit))
    if (row === undefined) return
    if (row < 0) {
      session.error = `${ref.short} points at a commit that isn't in the graph yet.`
      return
    }
    graph.reveal(row)
  }
</script>

<div class="repo">
  <Sidebar
    {path}
    selectedCommit={selected?.id}
    onPick={pick}
    onActivate={(ref) => checkoutRef(path, ref)}
    onNewBranch={() => (dialog = newBranchDialog(ctx))}
    onManageRemotes={() => (remotesOpen = true)}
    onContext={(ref, x, y) => openMenu(refItems(ref, ctx), x, y)}
    onMenu={openMenu}
    onManageWorktrees={() => (worktreesOpen = true)}
    openDialog={ctx.openDialog}
  />
  <div class="main">
    <Toolbar {path} />
    <button
      class="wip"
      class:active={view === 'wip'}
      onclick={() => {
        view = 'wip'
        if (diff?.mode === 'commit') diff = null
      }}
    >
      <span class="dot" class:dirty={changes > 0}><Icon name={changes > 0 ? 'st-modified' : 'check'} /></span>
      <span class="wip-title">{changes > 0 ? 'Uncommitted changes' : 'Working tree'}</span>
      <span class="meta">{changes > 0 ? `${changes} ${changes === 1 ? 'file' : 'files'}` : 'clean'}</span>
    </button>
    <!-- The graph stays mounted while a diff is open, so scroll position and selection
         survive returning to it. -->
    <div class="graph" class:hidden={diff || rebaseBase}>
      <GraphView bind:this={graph} {path} {onSelect} onContext={(row, refs, x, y) =>
          openMenu(commitMenuItems(row, refs, ctx), x, y)}
        onDrop={(source, sourceRefs, target, targetRefs, x, y) => {
          const items = dropItems(source, sourceRefs, target, targetRefs, ctx)
          if (items.length) openMenu(items, x, y)
          else
            session.error =
              'Drop onto the checked-out branch to merge, rebase or cherry-pick into it, or drag the checked-out branch onto another commit.'
        }} />
    </div>
    {#if rebaseBase}
      <div class="graph">
        <RebaseEditor {path} base={rebaseBase} onClose={() => (rebaseBase = '')} />
      </div>
    {:else if diff}
      <div class="graph">
        {#if diff.mode === 'conflict'}
          {#key diff.file}
            <Lazy load={() => import('./diff/ConflictView.svelte')} {path} file={diff.file} onClose={() => (diff = null)} />
          {/key}
        {:else if diff.mode === 'history'}
          {#key diff.file}
            <Lazy load={() => import('./diff/FileHistoryView.svelte')} {path} file={diff.file} onClose={() => (diff = null)} onOpen={(t: DiffTarget) => (diff = t)} />
          {/key}
        {:else if diff.mode === 'blame'}
          {#key diff.file + (diff.commit ?? '')}
            <Lazy
              load={() => import('./diff/BlameView.svelte')}
              {path}
              file={diff.file}
              rev={diff.commit ?? ''}
              onClose={() => (diff = null)}
              onOpen={(t: DiffTarget) => (diff = t)}
              onReveal={revealCommit}
            />
          {/key}
        {:else}
          <Lazy load={() => import('./diff/DiffView.svelte')} {path} target={diff} onClose={() => (diff = null)} onOpen={(t: DiffTarget) => (diff = t)} />
        {/if}
      </div>
    {/if}
  </div>
  {#if view === 'wip'}
    <WorkingTreePanel {path} current={diff} openFile={(t) => (diff = t)} />
  {:else if selected}
    <CommitPanel {path} commitId={selected.id} current={diff} openFile={(t) => (diff = t)} onSearch={(q) => graph?.search(q)} />
  {/if}
</div>

{#if menu}
  <ContextMenu x={menu.x} y={menu.y} items={menu.items} onClose={() => (menu = null)} />
{/if}
{#if dialog}
  <Dialog spec={dialog} onClose={() => (dialog = null)} />
{/if}
{#if reflogRef}
  <ReflogDialog
    {path}
    initialRef={reflogRef}
    onReveal={(commit) => {
      reflogRef = ''
      void revealCommit(commit)
    }}
    onCreateBranch={(commit) => {
      reflogRef = ''
      dialog = branchAtDialog(path, commit)
    }}
    onClose={() => (reflogRef = '')}
  />
{/if}

{#if worktreesOpen}
  <WorktreesDialog {path} onClose={() => (worktreesOpen = false)} />
{/if}

{#if remotesOpen}
  <RemotesDialog {path} onClose={() => (remotesOpen = false)} />
{/if}

<style>
  .repo {
    display: flex;
    height: 100%;
    min-height: 0;
  }

  .main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .graph {
    flex: 1;
    min-height: 0;
  }

  .hidden {
    display: none;
  }

  .wip {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 38px;
    padding: 0 14px;
    background: none;
    border: none;
    border-bottom: 1px dotted var(--paper-edge);
    border-radius: 0;
    text-align: left;
    transition: background-color var(--dur-base);
  }

  .wip:hover:not(:disabled) {
    background: var(--wash-hover);
  }

  .wip.active {
    background: var(--selected);
  }

  .wip-title {
    font-style: italic;
    color: var(--ink-soft);
  }

  .dot {
    display: inline-flex;
    color: var(--ink-faint);
  }

  .dot.dirty {
    color: var(--warning);
  }
</style>
