<script lang="ts">
  import { untrack } from 'svelte'
  import { CommitRow, WorkingTree } from '../../wailsjs/go/app/App.js'
  import { provideCommands } from './commands.svelte'
  import ContextMenu, { type MenuItem } from './ContextMenu.svelte'
  import Dialog, { type DialogSpec } from './Dialog.svelte'
  import RebaseEditor from './RebaseEditor.svelte'
  import RemotesDialog from './RemotesDialog.svelte'
  import WorktreesDialog from './WorktreesDialog.svelte'
  import Toolbar from './Toolbar.svelte'
  import Icon from './ui/Icon.svelte'
  import { DEFAULT_WIDTH, fitWidth, loadRightPanel, maxWidth, MIN_WIDTH, saveRightPanel } from './rightPanel'
  import { commitMenuItems, branchAtDialog, checkoutRef, dropItems, newBranchDialog, refItems, stackItems, type MenuContext } from './menus'
  import ReflogDialog from './ReflogDialog.svelte'
  import CommitPanel from './CommitPanel.svelte'
  import type { DiffTarget } from './diff/DiffView.svelte'
  import Lazy from './ui/Lazy.svelte'
  import GraphView from './graph/GraphView.svelte'
  import type { RefLabel } from './graph/draw'
  import type { Ref, Row } from './graph/rowCache'
  import { session } from './session.svelte'
  import Sidebar from './Sidebar.svelte'
  import { linkStore } from './links/links.svelte'
  import WorkingTreePanel from './WorkingTreePanel.svelte'

  let { path }: { path: string } = $props()

  const storage = (): Pick<Storage, 'getItem' | 'setItem'> => localStorage
  let right = $state(loadRightPanel(untrack(() => path), storage()))
  $effect(() => saveRightPanel(path, $state.snapshot(right), storage()))

  // Smart links come from git config: read again whenever the repository's config changes.
  $effect(() => {
    void session.revision[path]
    void linkStore.load(path)
  })
  let windowWidth = $state(0)
  /** The panel's width now. Before the first measure there's nothing to fit it to. */
  let rightWidth = $derived(windowWidth ? fitWidth(right.width, windowWidth) : right.width)

  // Dragging the panel's edge, or the arrow keys on it, resizes the panel.
  let rightDrag: { x: number; width: number } | null = null
  function startRightResize(e: PointerEvent) {
    rightDrag = { x: e.clientX, width: rightWidth }
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }
  function rightResize(e: PointerEvent) {
    if (rightDrag) right.width = fitWidth(rightDrag.width + rightDrag.x - e.clientX, windowWidth)
  }
  function rightResizeKey(e: KeyboardEvent) {
    const step = e.key === 'ArrowLeft' ? 16 : e.key === 'ArrowRight' ? -16 : 0
    if (step) right.width = fitWidth(rightWidth + step, windowWidth)
    else if (e.key === 'Home') right.width = DEFAULT_WIDTH
    else return
    e.preventDefault()
  }

  let graph: GraphView
  /** Where the graph puts its header tools: in the bar, beside the repository's actions. */
  let toolsHost = $state<HTMLElement>()
  let selected = $state<Row | undefined>()
  /** Which panel is on the right: uncommitted changes, or the selected commit. */
  let view = $state<'wip' | 'commit'>('commit')
  /** Files with uncommitted changes; null until the first answer, so nothing claims "clean". */
  let changes = $state<number | null>(null)
  let menu = $state<{ x: number; y: number; items: MenuItem[] } | null>(null)
  /** Where the graph's columns are, so the row of changes above it lines up with them. */

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
      id: 'view.rightPanel',
      group: 'View',
      title: right.hidden ? 'Show the right panel' : 'Hide the right panel',
      keywords: 'details commit changes collapse graph width',
      shortcut: 'Mod+Alt+B',
      run: () => (right.hidden = !right.hidden),
    },
    {
      id: 'view.changes',
      group: 'View',
      title: 'Show uncommitted changes',
      keywords: 'working tree stage commit',
      run: () => {
        view = 'wip'
        right.hidden = false
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

  /** Labels for the branches and tags on the selected commit. */
  let selectedRefs = $state<RefLabel[]>([])

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
      session.error = graph.focused() ? 'That commit isn’t part of what the graph is focused on. Stop focusing to see it.' : 'That commit isn’t in the graph: it isn’t on any branch. Start a branch there to bring it back.'
      return
    }
    diff = null
    graph.reveal(row)
  }

  async function pick(ref: Ref) {
    const row = await session.run(() => CommitRow(path, ref.commit))
    if (row === undefined) return
    if (row < 0) {
      session.error = graph.focused() ? `${ref.short} isn't part of what the graph is focused on. Stop focusing to see it.` : `${ref.short} points at a commit that isn't in the graph yet.`
      return
    }
    graph.reveal(row)
  }
</script>

<div class="repo" bind:clientWidth={windowWidth}>
  <Sidebar
    {path}
    selectedCommit={selected?.id}
    onPick={pick}
    onActivate={(ref) => checkoutRef(path, ref)}
    onNewBranch={() => (dialog = newBranchDialog(ctx))}
    onManageRemotes={() => (remotesOpen = true)}
    onContext={(ref, x, y) => openMenu(refItems(ref, ctx), x, y)}
    onStackContext={(members, x, y) => openMenu(stackItems(members, ctx), x, y)}
    onMenu={openMenu}
    onManageWorktrees={() => (worktreesOpen = true)}
    openDialog={ctx.openDialog}
  />
  <div class="main">
    <div class="bar">
      <Toolbar {path} />
      <!-- The graph's own tools join the bar while the graph is showing; a file or the rebase
           editor has its own header, so they step aside for it. -->
      <div class="graph-tools" class:hidden={diff || rebaseBase} bind:this={toolsHost}></div>
      <button class="btn sm quiet icon panel-toggle" aria-pressed={!right.hidden} aria-label={right.hidden ? 'Show the right panel' : 'Hide the right panel'} title={right.hidden ? 'Show the right panel' : 'Hide the right panel'} onclick={() => (right.hidden = !right.hidden)}><Icon name="sidebar" size={14} /></button>
    </div>
    <!-- The graph stays mounted while a diff is open, so scroll position and selection
         survive returning to it. -->
    <div class="graph" class:hidden={diff || rebaseBase}>
      <GraphView bind:this={graph} {path} {toolsHost} {onSelect} onSelectedRefs={(r) => (selectedRefs = r)} onContext={(row, refs, x, y) =>
          openMenu(commitMenuItems(row, refs, ctx), x, y)}
        onDrop={(source, sourceRefs, target, targetRefs, x, y) => {
          const items = dropItems(source, sourceRefs, target, targetRefs, ctx)
          if (items.length) openMenu(items, x, y)
          else
            session.error =
              'Drop onto the checked-out branch to merge, rebase or cherry-pick into it, or drag the checked-out branch onto another commit.'
        }}
      >
        <!-- The working tree is the graph's first row: what isn't a commit yet, above the newest
             one, with its mark on the first lane and its words where the messages start. -->
        {#snippet lead(at: { nodeX: number; textX: number })}
          <button
            class="wip"
            class:active={view === 'wip'}
            aria-pressed={view === 'wip'}
            style:padding-left="{at.textX}px"
            onclick={() => {
              view = 'wip'
              right.hidden = false
              if (diff?.mode === 'commit') diff = null
            }}
          >
            <span class="dot" class:dirty={!!changes} style:left="{at.nodeX - 8}px">{#if changes && changes > 0}<span class="ring"></span>{:else if changes !== null}<Icon name="check" />{/if}</span>
            <span class="wip-title">{changes ? `${changes} ${changes === 1 ? 'file' : 'files'} changed` : 'Working tree'}</span>
            {#if changes && session.activeTab?.head}
              <span class="where">{session.activeTab.head.startsWith('detached') ? session.activeTab.head : `on ${session.activeTab.head}`}</span>
            {:else if changes === 0}
              <span class="meta">clean</span>
            {/if}
            {#if changes}<span class="review">Review</span>{/if}
          </button>
        {/snippet}
      </GraphView>
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
  {#if (view === 'wip' || selected) && !right.hidden}
    <div class="right" style:width="{rightWidth}px">
      <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
      <div
        class="edge"
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize the right panel"
        aria-valuenow={rightWidth}
        aria-valuemin={MIN_WIDTH}
        aria-valuemax={maxWidth(windowWidth)}
        tabindex="0"
        title="Drag to resize. Double-click to reset."
        onpointerdown={startRightResize}
        onpointermove={rightResize}
        onpointerup={() => (rightDrag = null)}
        ondblclick={() => (right.width = DEFAULT_WIDTH)}
        onkeydown={rightResizeKey}
      ></div>
  {#if view === 'wip'}
    <WorkingTreePanel {path} current={diff} openFile={(t) => (diff = t)} onNewBranch={() => (dialog = newBranchDialog(ctx))} onMenu={openMenu} />
  {:else if selected}
    <CommitPanel {path} onMenu={openMenu} commitId={selected.id} refs={selectedRefs} current={diff} openFile={(t) => (diff = t)} onSearch={(q) => graph?.search(q)} onReveal={revealCommit} onActions={(x, y) => selected && openMenu(commitMenuItems(selected, graph.refsOf(selected.id), ctx), x, y)} />
  {/if}
    </div>
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

  /* The panels inside lay out by this width, not the window's: see their container queries. */
  .right {
    position: relative;
    flex: none;
    display: flex;
    min-height: 0;
    container: right-panel / inline-size;
  }

  .edge {
    position: absolute;
    top: 0;
    bottom: 0;
    left: -3px;
    width: 6px;
    z-index: 3;
    cursor: col-resize;
    touch-action: none;
    transition: background-color var(--dur-quick);
  }

  .edge:hover,
  .edge:focus-visible {
    background: var(--link);
    outline: none;
  }

  @media (prefers-reduced-motion: reduce) {
    .edge {
      transition: none;
    }
  }

  .hidden {
    display: none;
  }

  /* The repository's actions, with the right panel's toggle at the end. */
  .bar {
    flex: none;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    border-bottom: 1px solid var(--paper-edge);
  }

  /* Takes the room the actions leave, and a line of its own once that's too little. */
  .graph-tools {
    flex: 1 1 380px;
    min-width: 0;
    display: flex;
    border-left: 1px solid var(--paper-edge);
  }

  .graph-tools.hidden {
    display: none;
  }

  .panel-toggle {
    flex: none;
    margin: 0 8px 0 auto;
    transform: scaleX(-1);
  }

  .wip {
    position: relative;
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 10px;
    height: 100%;
    /* The left padding comes from the graph: where its messages start. */
    padding: 0 14px 0 44px;
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
    background: var(--paper-warm);
  }

  .wip-title {
    font-weight: 500;
    color: var(--ink);
    white-space: nowrap;
  }

  .where {
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .review {
    flex: none;
    margin-left: auto;
    padding: 3px 10px;
    border-radius: var(--r-sm);
    background: color-mix(in srgb, var(--link) 12%, transparent);
    font-size: 14px;
    color: var(--link);
  }

  .ring {
    box-sizing: border-box;
    width: 14px;
    height: 14px;
    border: 2px dashed var(--lane-0);
    border-radius: 50%;
  }

  .dot {
    position: absolute;
    left: 14px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    /* The icon's size, kept while there's no icon yet so the title doesn't shift. */
    min-width: 16px;
    min-height: 16px;
    color: var(--ink-faint);
  }

  .dot.dirty {
    color: var(--warning);
  }
</style>
