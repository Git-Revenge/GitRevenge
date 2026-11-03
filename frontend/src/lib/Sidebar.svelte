<script lang="ts">
  import { MergedBranches, Refs, SetShowHidden, ShowingHidden } from '../../wailsjs/go/app/App.js'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import { onMount, tick, untrack } from 'svelte'
  import type { DialogSpec } from './Dialog.svelte'
  import type { Ref } from './graph/rowCache'
  import { provideCommands, type Command } from './commands.svelte'
  import type { MenuItem } from './ContextMenu.svelte'
  import RepoPanel from './RepoPanel.svelte'
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'
  import type { IconName } from './ui/Icon.svelte'
  import { typeahead } from './menuKeys'
  import SidebarRail from './SidebarRail.svelte'
  import SidebarRow from './SidebarRow.svelte'
  import { scrollToShow, treeAction, treePositions } from './sidebarKeys'
  import { clampWidth, DEFAULT_WIDTH, loadSidebar, otherPanelWithMatches, ROW_HEIGHT, saveSidebar, visibleRange, type SidebarPanel } from './sidebarState'
  import { buildPanels, careCount, panelMatches, panelRows, type FolderRow, type RefRow, type TreeRow, needsCare } from './sidebarTree'

  let {
    path,
    selectedCommit,
    onPick,
    onActivate,
    onNewBranch,
    onManageRemotes,
    onContext,
    onMenu,
    onManageWorktrees,
    openDialog,
  }: {
    path: string
    /** The commit selected in the graph; refs pointing at it show as selected. */
    selectedCommit?: string
    onPick: (ref: Ref) => void
    /** Double-click: check the ref out. */
    onActivate?: (ref: Ref) => void
    onNewBranch?: () => void
    onManageRemotes?: () => void
    onContext?: (ref: Ref, x: number, y: number) => void
    onMenu?: (items: MenuItem[], x: number, y: number) => void
    onManageWorktrees?: () => void
    openDialog: (spec: DialogSpec) => void
  } = $props()

  const TABS: { key: SidebarPanel; label: string; icon: IconName; empty: string }[] = [
    { key: 'branches', label: 'Branches', icon: 'branch', empty: 'No branches yet.' },
    { key: 'remotes', label: 'Remotes', icon: 'remote', empty: 'No remote branches. Add a remote to get some.' },
    { key: 'tags', label: 'Tags', icon: 'tag', empty: 'No tags yet.' },
    { key: 'stashes', label: 'Stashes', icon: 'stash', empty: 'No stashes.' },
    { key: 'repo', label: 'Repository', icon: 'submodule', empty: '' },
  ]

  const uid = $props.id()

  const storage = (): Pick<Storage, 'getItem' | 'setItem'> => localStorage

  let ui = $state(loadSidebar(untrack(() => path), storage()))
  $effect(() => saveSidebar(path, $state.snapshot(ui), storage()))

  let refs = $state<Ref[]>([])
  let filter = $state('')
  let care = $state(false)
  // Hidden branches (Settings, This repository) leave the list and the graph until shown
  // again for this session.
  let showingHidden = $state(false)
  let hiddenCount = $derived(refs.filter((r) => r.hidden).length)
  let repoAttention = $state(0)
  let repoMatches = $state(0)

  let panels = $derived(buildPanels(refs, { showHidden: showingHidden, sort: ui.sort }))
  let current = $derived(ui.panel === 'repo' ? null : panels[ui.panel])
  let careOn = $derived(care && ui.panel === 'branches')
  let merged = $derived(session.merged[path] ?? {})
  let rows = $derived(current ? panelRows(current, { open: ui.open, query: filter, care: careOn, merged }) : [])
  let filtering = $derived(filter.trim().length > 0)
  let needCare = $derived(careCount(panels.branches))
  let matches = $derived(!filtering ? 0 : current ? panelMatches(current, filter) : repoMatches)
  // An empty panel names one with matches, the Repository panel last.
  let elsewhere = $derived.by(() => {
    if (rows.length > 0 || !filtering) return null
    return otherPanelWithMatches(panels, ui.panel, filter) ?? (repoMatches > 0 ? { panel: 'repo' as const, count: repoMatches } : null)
  })
  let repoElsewhere = $derived(filtering ? otherPanelWithMatches(panels, 'repo', filter) : null)
  let tab = $derived(TABS.find((t) => t.key === ui.panel) ?? TABS[0])

  // Only the rows in view are in the DOM, so a repository with tens of thousands of branches
  // stays quick to scroll.
  let list = $state<HTMLElement>()
  let scrollTop = $state(0)
  let viewHeight = $state(800)
  let range = $derived(visibleRange(scrollTop, viewHeight, rows.length))

  // A new panel or query starts at the top; the old scroll position means nothing there.
  $effect(() => {
    void ui.panel
    void filter
    void careOn
    scrollTop = 0
    if (list) list.scrollTop = 0
  })

  // The tree is one Tab stop. Focus stays on the list and aria-activedescendant names the active
  // row, so rows can come and go with scrolling and filtering without losing focus.
  let filterInput = $state<HTMLInputElement>()
  let tree = $state<HTMLElement>()
  let activeKey = $state('')
  let lastActive = 0
  let positions = $derived(treePositions(rows))
  let activeIndex = $derived.by(() => {
    const found = rows.findIndex((r) => rowKey(r) === activeKey)
    return found >= 0 ? found : Math.min(lastActive, Math.max(0, rows.length - 1))
  })
  $effect(() => {
    lastActive = activeIndex
  })
  const rowId = (index: number) => `${uid}-row-${index}`

  function setActive(index: number) {
    const row = rows[index]
    if (!row) return
    activeKey = rowKey(row)
    showRow(index)
  }

  function showRow(index: number) {
    if (!list) return
    const next = scrollToShow(index, ROW_HEIGHT, list.scrollTop, list.clientHeight)
    if (next !== list.scrollTop) {
      list.scrollTop = next
      scrollTop = next
    }
  }

  function openMenuFor(index: number) {
    const row = rows[index]
    if (row?.type !== 'ref' || !onContext) return
    const box = document.getElementById(rowId(index))?.getBoundingClientRect()
    onContext(row.ref, (box?.left ?? 0) + 32, box?.bottom ?? 0)
  }

  let typed = ''
  let typedTimer: ReturnType<typeof setTimeout> | undefined

  function treeKey(e: KeyboardEvent) {
    // Space continues a name being typed, rather than checking out.
    const typing = e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey && (e.key !== ' ' || typed !== '')
    if (typing) {
      e.preventDefault()
      clearTimeout(typedTimer)
      typed += e.key
      typedTimer = setTimeout(() => (typed = ''), 600)
      const found = typeahead(rows.map((r) => ({ label: r.label })), activeIndex, typed)
      if (found >= 0) setActive(found)
      return
    }
    const action = treeAction({ key: e.key, shiftKey: e.shiftKey }, rows, activeIndex, Math.max(1, Math.floor(viewHeight / ROW_HEIGHT) - 1), filtering || careOn)
    if (!action) return
    e.preventDefault()
    const row = rows[activeIndex]
    if (action.type === 'move') setActive(action.index)
    else if (action.type === 'toggle' && row?.type === 'folder') toggleFolder(row)
    else if (action.type === 'pick' && row?.type === 'ref') onPick(row.ref)
    else if (action.type === 'checkout' && row?.type === 'ref') onActivate?.(row.ref)
    else if (action.type === 'menu') openMenuFor(activeIndex)
  }

  function filterKey(e: KeyboardEvent) {
    if (e.key === 'ArrowDown' && rows.length) {
      e.preventDefault()
      tree?.focus()
    } else if (e.key === 'Escape' && filter) {
      e.preventDefault()
      e.stopPropagation()
      filter = ''
    }
  }

  async function focusFilter() {
    ui.collapsed = false
    await tick()
    filterInput?.focus()
    filterInput?.select()
  }

  /** The rail badge's number, and what it means for screen readers. */
  function badge(key: SidebarPanel): { count: number; text: string } {
    if (filtering) {
      const count = key === 'repo' ? repoMatches : panelMatches(panels[key], filter)
      return { count, text: count === 1 ? '1 match' : `${count} matches` }
    }
    if (key === 'branches') return { count: needCare, text: needCare === 1 ? '1 needs care' : `${needCare} need care` }
    if (key === 'stashes') {
      const count = panels.stashes.root.total
      return { count, text: count === 1 ? '1 stash' : `${count} stashes` }
    }
    if (key === 'repo') return { count: repoAttention, text: repoAttention === 1 ? '1 submodule needs updating' : `${repoAttention} submodules need updating` }
    return { count: 0, text: '' }
  }

  function pickTab(key: SidebarPanel) {
    if (key === ui.panel) ui.collapsed = !ui.collapsed
    else {
      ui.panel = key
      ui.collapsed = false
    }
  }

  function clickFolder(row: FolderRow) {
    activeKey = rowKey(row)
    toggleFolder(row)
  }

  function clickRef(row: RefRow) {
    activeKey = rowKey(row)
    onPick(row.ref)
  }

  function toggleFolder(row: FolderRow) {
    // While filtering, folders are held open to show their matches.
    if (!filtering && !careOn) ui.open[row.key] = !row.open
  }

  async function toggleHidden() {
    const next = !showingHidden
    await session.run(() => SetShowHidden(path, next))
    showingHidden = next
  }

  // Dragging the panel's edge, or the arrow keys on it, resizes the panel.
  let drag: { x: number; width: number } | null = null
  function startResize(e: PointerEvent) {
    drag = { x: e.clientX, width: ui.width }
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }
  function resize(e: PointerEvent) {
    if (drag) ui.width = clampWidth(drag.width + e.clientX - drag.x)
  }
  function resizeKey(e: KeyboardEvent) {
    const step = e.key === 'ArrowLeft' ? -16 : e.key === 'ArrowRight' ? 16 : 0
    if (step) ui.width = clampWidth(ui.width + step)
    else if (e.key === 'Home') ui.width = DEFAULT_WIDTH
    else return
    e.preventDefault()
  }

  const rowKey = (r: TreeRow) => (r.type === 'folder' ? `folder:${r.key}` : r.ref.name)

  provideCommands(() => [
    {
      id: 'view.sidebarFilter',
      group: 'View',
      title: 'Filter the sidebar',
      keywords: 'find branch tag stash search',
      shortcut: 'Mod+Shift+B',
      whenTyping: true,
      run: () => void focusFilter(),
    },
    {
      id: 'view.sidebar',
      group: 'View',
      title: ui.collapsed ? 'Show the sidebar panel' : 'Hide the sidebar panel',
      keywords: 'collapse rail graph width',
      run: () => (ui.collapsed = !ui.collapsed),
    },
  ])

  // One or two per ref, so a key press mustn't build them: that took 61ms with 50,000 branches.
  provideCommands(() => {
    const list: Command[] = []
    for (const r of refs) {
      if (r.kind === 'stash') continue
      if (r.kind === 'local' && !r.head && onActivate)
        list.push({ id: `checkout.${r.name}`, group: 'Branch', title: `Check out ${r.short}`, keywords: 'switch branch', run: () => onActivate(r) })
      const what = r.kind === 'tag' ? 'tag' : r.kind === 'remote' ? 'remote branch' : 'branch'
      list.push({ id: `goto.${r.name}`, group: 'Go to', title: `Go to ${r.short}`, keywords: `${what} show reveal graph`, run: () => onPick(r) })
    }
    return list
  }, { paletteOnly: true })

  // Which branches that need care are already merged. It takes a while, so it fills in after
  // the list has shown; only the latest answer counts.
  let mergedAsked = 0
  async function loadMerged() {
    const asked = ++mergedAsked
    const answer = refs.some(needsCare) ? await MergedBranches(path).catch(() => null) : {}
    if (asked === mergedAsked && answer) session.merged[path] = answer
  }

  async function load() {
    refs = ((await session.run(() => Refs(path))) as Ref[] | undefined) ?? []
    void loadMerged()
    showingHidden = (await session.run(() => ShowingHidden(path))) ?? false
  }

  onMount(() => {
    void load()
    // Refs are re-read with every graph reload.
    return EventsOn('graph:progress', (p: { path: string; done: boolean }) => {
      if (p.path === path && p.done) void load()
    })
  })
</script>

<aside class="sidebar">
  <SidebarRail
    tabs={TABS.map((t) => ({ key: t.key, label: t.label, icon: t.icon, badge: badge(t.key) }))}
    panel={ui.panel}
    collapsed={ui.collapsed}
    onPick={pickTab}
    onToggle={() => (ui.collapsed = !ui.collapsed)}
  />

  <div class="panel" class:collapsed={ui.collapsed} style:width="{ui.width}px" hidden={ui.collapsed}>
    <header>
      <div class="heading">
        <h2>{tab.label}</h2>
        {#if ui.panel === 'branches' || ui.panel === 'remotes' || ui.panel === 'tags'}
          <button
            class="btn sm quiet sort"
            title={ui.sort === 'name' ? 'Sorted by name. Click to sort by most recently updated.' : 'Sorted by most recently updated. Click to sort by name.'}
            aria-label={ui.sort === 'name' ? 'Sorted by name. Sort by most recently updated.' : 'Sorted by most recently updated. Sort by name.'}
            onclick={() => (ui.sort = ui.sort === 'name' ? 'recent' : 'name')}>{ui.sort === 'name' ? 'Name' : 'Recent'}</button
          >
        {/if}
        {#if ui.panel === 'branches' && onNewBranch}
          <button class="btn sm quiet icon" onclick={onNewBranch} title="New branch from HEAD" aria-label="New branch"><Icon name="plus" size={14} /></button>
        {:else if ui.panel === 'remotes' && onManageRemotes}
          <button class="btn sm quiet icon" onclick={onManageRemotes} title="Add, edit or remove remotes" aria-label="Manage remotes"><Icon name="gear" size={14} /></button>
        {/if}
      </div>
      <label class="filter">
          <Icon name="search" size={14} />
          <input type="search" placeholder="Filter all panels" aria-label="Filter everything in the sidebar" bind:value={filter} bind:this={filterInput} onkeydown={filterKey} />
          {#if filtering}
            <span class="meta" aria-live="polite">{matches.toLocaleString()}</span>
            <button class="clear" aria-label="Clear the filter" title="Clear the filter" onclick={() => (filter = '')}><Icon name="close" size={12} /></button>
          {/if}
        </label>
      {#if ui.panel === 'branches'}
        <div class="chips" role="group" aria-label="Which branches to show">
          <button class="chip" aria-pressed={!care} onclick={() => (care = false)}>All</button>
          <button class="chip" aria-pressed={care} onclick={() => (care = true)}>Needs care <span class="meta">{needCare.toLocaleString()}</span></button>
        </div>
      {/if}
    </header>

    <div class="scroll" class:narrowed={filtering || careOn} bind:this={list} bind:clientHeight={viewHeight} onscroll={(e) => (scrollTop = e.currentTarget.scrollTop)} hidden={ui.panel === 'repo'}>
      <ul
        bind:this={tree}
        role="tree"
        aria-label={tab.label}
        aria-activedescendant={rows.length ? rowId(activeIndex) : undefined}
        tabindex={rows.length ? 0 : -1}
        style:height="{rows.length * ROW_HEIGHT}px"
        style:padding-top="{range.start * ROW_HEIGHT}px"
        onkeydown={treeKey}
        onfocus={() => showRow(activeIndex)}
      >
        {#each rows.slice(range.start, range.end) as row, offset (rowKey(row))}
          {@const index = range.start + offset}
          <li
            id={rowId(index)}
            role="treeitem"
            class:group={row.startsGroup}
            class:active={index === activeIndex}
            aria-level={row.depth + 1}
            aria-posinset={positions[index]?.pos}
            aria-setsize={positions[index]?.size}
            aria-expanded={row.type === 'folder' ? row.open : undefined}
            aria-selected={row.type === 'ref' && !!selectedCommit && row.ref.commit === selectedCommit}
          >
            <SidebarRow {row} {filter} {selectedCommit} mergedInto={row.type === 'ref' && row.ref.kind === 'local' ? merged[row.ref.short] : undefined} onFolderClick={clickFolder} onRefClick={clickRef} {onActivate} {onContext} />
          </li>
        {/each}
      </ul>
      {#if rows.length === 0}
        <p class="empty">
          {#if elsewhere}
            Nothing here.
            <button class="btn sm text" onclick={() => elsewhere && (ui.panel = elsewhere.panel)}>{TABS.find((t) => t.key === elsewhere?.panel)?.label} has {elsewhere.count.toLocaleString()}</button>
          {:else if filtering}
            Nothing matches that.
          {:else if careOn}
            Nothing needs care.
          {:else}
            {tab.empty}
          {/if}
        </p>
      {/if}
      {#if hiddenCount > 0 && (ui.panel === 'branches' || ui.panel === 'remotes')}
        <p class="hidden-note">
          <span class="meta">{hiddenCount.toLocaleString()} hidden</span>
          <button class="btn sm text" onclick={toggleHidden}>{showingHidden ? 'Hide them again' : 'Show them'}</button>
        </p>
      {/if}
    </div>

    <!-- Stays mounted so the rail's badge knows about submodules from the start. -->
    <div class="scroll repo-panel" hidden={ui.panel !== 'repo'}>
      <RepoPanel
        {path}
        {filter}
        {openDialog}
        {onMenu}
        {onManageWorktrees}
        onStatus={(s) => {
          repoAttention = s.attention
          repoMatches = s.matches
        }}
      />
      {#if filtering && repoMatches === 0 && repoElsewhere}
        <p class="empty">
          <button class="btn sm text" onclick={() => repoElsewhere && (ui.panel = repoElsewhere.panel)}>{TABS.find((t) => t.key === repoElsewhere?.panel)?.label} has {repoElsewhere.count.toLocaleString()}</button>
        </p>
      {/if}
    </div>

    <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
    <div
      class="edge"
      role="separator"
      aria-orientation="vertical"
      aria-label="Resize the sidebar panel"
      aria-valuenow={ui.width}
      tabindex="0"
      title="Drag to resize. Double-click to reset."
      onpointerdown={startResize}
      onpointermove={resize}
      onpointerup={() => (drag = null)}
      ondblclick={() => (ui.width = DEFAULT_WIDTH)}
      onkeydown={resizeKey}
    ></div>
  </div>
</aside>

<style>
  .sidebar {
    display: flex;
    flex: none;
    min-height: 0;
  }

  .panel {
    position: relative;
    flex: none;
    display: flex;
    flex-direction: column;
    min-width: 0;
    max-width: 45vw;
    background: var(--paper-warm);
    border-right: 1px solid var(--paper-edge);
  }

  .panel.collapsed {
    display: none;
  }

  header {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px 8px 8px;
    border-bottom: 1px dotted var(--paper-edge);
  }

  .heading {
    display: flex;
    align-items: center;
    gap: 2px;
    min-height: 28px;
    padding-left: 4px;
  }

  h2 {
    flex: 0 1 auto;
    min-width: 0;
    margin: 0 auto 0 0;
    font-size: 19px;
    font-weight: 500;
    letter-spacing: -0.015em;
    line-height: 1.2;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-variation-settings:
      'SOFT' 50,
      'WONK' 1;
  }

  .sort {
    flex: none;
    padding-inline: 8px;
  }

  .filter {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 4px 0 8px;
    background: var(--paper);
    border: 1px solid var(--paper-edge);
    border-radius: 4px;
    color: var(--ink-mute);
  }

  .filter:focus-within {
    outline: 2px solid var(--link);
    outline-offset: 1px;
  }

  .filter input {
    flex: 1;
    min-width: 0;
    height: 28px;
    padding: 0;
    background: none;
    border: none;
    outline: none;
    font-size: 13px;
  }

  /* The clear button replaces the browser's own. */
  .filter input::-webkit-search-cancel-button {
    display: none;
  }

  .clear {
    flex: none;
    display: grid;
    place-items: center;
    width: 22px;
    height: 22px;
    padding: 0;
    background: none;
    border: none;
    border-radius: 5px;
    color: inherit;
  }

  .clear:hover:not(:disabled) {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .chips {
    display: flex;
    gap: 4px;
  }

  .chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 3px 9px;
    background: none;
    border: none;
    border-radius: 6px;
    color: var(--ink-mute);
    font-size: 13px;
  }

  .chip:hover:not(:disabled) {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .chip[aria-pressed='true'] {
    background: var(--selected);
    color: var(--ink);
  }

  /* 12px is the design system's floor, which the app's shared 11px meta style is under. */
  .panel :global(.meta) {
    font-size: 12px;
  }

  .scroll {
    flex: 1;
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
  }

  .scroll[hidden] {
    display: none;
  }

  ul {
    box-sizing: border-box;
    list-style: none;
    margin: 0;
    padding: 0;
  }

  li {
    position: relative;
  }

  /* Drawn over the row, not above it: every row stays one height for virtual scrolling. */
  li.group::before {
    content: '';
    position: absolute;
    z-index: 1;
    top: 0;
    left: 8px;
    right: 8px;
    border-top: 1px dotted var(--paper-edge);
    pointer-events: none;
  }

  [role='tree'] {
    outline: none;
  }

  [role='tree']:focus-visible li.active::after {
    content: '';
    position: absolute;
    z-index: 2;
    inset: 0;
    outline: 2px solid var(--link);
    outline-offset: -2px;
    pointer-events: none;
  }

  .empty {
    margin: 16px 12px;
    color: var(--ink-mute);
    font-style: italic;
  }

  .hidden-note {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 8px 12px;
  }

  .edge {
    position: absolute;
    top: 0;
    bottom: 0;
    right: -3px;
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
</style>
