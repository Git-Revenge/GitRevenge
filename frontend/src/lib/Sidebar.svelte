<script lang="ts">
  import { DeletableBranches, DeleteMergedBranches, MergedBranches, RemotePlaces, Remotes, SetShowHidden, ShowingHidden } from '../../wailsjs/go/app/App.js'
  import type { git } from '../../wailsjs/go/models'
  import { prefs } from './prefs.svelte'
  import { remoteHeader, trackersByRemote, type RemoteHeader, type URLPlace } from './remoteHeader'
  import { repoReads } from './repoReads'
  import { onBackend } from './events'
  import { onMount, tick, untrack } from 'svelte'
  import type { DialogSpec } from './Dialog.svelte'
  import type { Ref } from './graph/rowCache'
  import { provideCommands, type Command } from './commands.svelte'
  import type { MenuItem } from './ContextMenu.svelte'
  import { trailerCare } from './filters/careDialog.svelte'
  import RepoPanel from './RepoPanel.svelte'
  import Lazy from './ui/Lazy.svelte'
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
    onStackContext,
    onMenu,
    onManageWorktrees,
    openDialog,
    filterPicks = [],
    onFilterPick,
    pinnedKeys = [],
    onPin,
    onSearch,
  }: {
    path: string
    /** The values the graph is filtered by, and how to add one or take it out. */
    filterPicks?: { key: string; value: string }[]
    onFilterPick?: (key: string, value: string) => void
    /** The trailer keys pinned to the graph's rows, by ID, and how to pin or unpin one. */
    pinnedKeys?: string[]
    onPin?: (key: string, on: boolean) => void
    /** Searches the graph for text. */
    onSearch?: (query: string) => void
    /** The commit selected in the graph; refs pointing at it show as selected. */
    selectedCommit?: string
    onPick: (ref: Ref) => void
    /** Double-click: check the ref out. */
    onActivate?: (ref: Ref) => void
    onNewBranch?: () => void
    onManageRemotes?: () => void
    onContext?: (ref: Ref, x: number, y: number) => void
    onStackContext?: (members: Ref[], x: number, y: number) => void
    onMenu?: (items: MenuItem[], x: number, y: number) => void
    onManageWorktrees?: () => void
    openDialog: (spec: DialogSpec) => void
  } = $props()

  const TABS: { key: SidebarPanel; label: string; icon: IconName; empty: string }[] = [
    { key: 'branches', label: 'Branches', icon: 'branch', empty: 'No branches yet.' },
    { key: 'remotes', label: 'Remotes', icon: 'remote', empty: 'No remote branches. Add a remote to get some.' },
    { key: 'tags', label: 'Tags', icon: 'tag', empty: 'No tags yet.' },
    { key: 'filters', label: 'Smart filters', icon: 'filter', empty: '' },
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
  let filterMatches = $state(0)
  // The Smart filters panel asks the backend for what it shows, so it's made the first time
  // it's wanted, by showing it or by filtering, and stays for the rail's count after that.
  let filtersWanted = $state(false)
  $effect(() => {
    if (ui.panel === 'filters' || filter.trim()) filtersWanted = true
  })

  // Each remote's details and where its URLs point, for the Remotes panel's folder headers.
  // They're read with the refs, and again when git config changes.
  let remotes = $state<git.Remote[]>([])
  let places = $state<Record<string, URLPlace>>({})
  // The fetch ages move on by themselves.
  let now = $state(Date.now() / 1000)
  onMount(() => {
    const t = setInterval(() => (now = Date.now() / 1000), 30_000)
    return () => clearInterval(t)
  })
  let headers = $derived.by(() => {
    const out: Record<string, RemoteHeader> = {}
    if (ui.panel !== 'remotes') return out
    const trackers = trackersByRemote(refs, remotes.map((r) => r.name))
    for (const r of remotes) {
      const branches = refs.filter((x) => x.kind === 'remote' && x.short.startsWith(`${r.name}/`) && !x.short.endsWith('/HEAD')).length
      out[r.name] = remoteHeader(r, places, now, branches, trackers[r.name] ?? [])
    }
    return out
  })

  async function loadRemotes() {
    const [rs, ps] = await Promise.all([Remotes(path).catch(() => null), RemotePlaces(path).catch(() => null)])
    if (rs) remotes = rs
    if (ps) places = ps as Record<string, URLPlace>
  }

  let panels = $derived(buildPanels(refs, { showHidden: showingHidden, sort: ui.sort }))
  let current = $derived(ui.panel === 'repo' || ui.panel === 'filters' ? null : panels[ui.panel])
  let careOn = $derived(care && ui.panel === 'branches')
  let merged = $derived(session.merged[path] ?? {})
  let deletable = $state<string[]>([])
  let deleting = $state(false)
  let rows = $derived(current ? panelRows(current, { open: ui.open, query: filter, care: careOn, merged }) : [])
  let filtering = $derived(filter.trim().length > 0)
  let needCare = $derived(careCount(panels.branches))
  let matches = $derived(!filtering ? 0 : current ? panelMatches(current, filter) : ui.panel === 'filters' ? filterMatches : repoMatches)
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
    const box = document.getElementById(rowId(index))?.getBoundingClientRect()
    const x = (box?.left ?? 0) + 32
    const y = box?.bottom ?? 0
    if (row?.type === 'ref') onContext?.(row.ref, x, y)
    else if (row?.members) onStackContext?.(row.members, x, y)
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

  /** The rail badge: a count while filtering or for stashes, a dot for things that need
   * attention, and what it means for screen readers. */
  function badge(key: SidebarPanel): { count: number; text: string; dot?: boolean } {
    if (filtering) {
      const count = key === 'repo' ? repoMatches : key === 'filters' ? filterMatches : panelMatches(panels[key], filter)
      return { count, text: count === 1 ? '1 match' : `${count} matches` }
    }
    if (key === 'branches') return { count: needCare, text: needCare === 1 ? '1 needs care' : `${needCare} need care`, dot: true }
    if (key === 'stashes') {
      const count = panels.stashes.root.total
      return { count, text: count === 1 ? '1 stash' : `${count} stashes` }
    }
    if (key === 'repo') return { count: repoAttention, text: repoAttention === 1 ? '1 submodule needs updating' : `${repoAttention} submodules need updating`, dot: true }
    return { count: 0, text: '' }
  }

  /** Shows a panel, opening the sidebar if it's folded: "+ Filter" above the graph asks. */
  export function show(key: SidebarPanel) {
    ui.panel = key
    ui.collapsed = false
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
    if (asked !== mergedAsked || !answer) return
    session.merged[path] = answer
    const names = Object.keys(answer)
    const safe = names.length ? await DeletableBranches(path, names).catch(() => null) : null
    if (asked === mergedAsked) deletable = safe ?? []
  }

  // Undo is one click away in the message this leaves, so it doesn't ask first.
  async function deleteAll() {
    deleting = true
    const gone = await session.run(() => DeleteMergedBranches(path, deletable))
    // The graph takes a moment to reload, and these shouldn't look deletable until it has.
    if (gone) refs = refs.filter((r) => r.kind !== 'local' || !gone.includes(r.short))
    deletable = []
    deleting = false
  }

  async function load() {
    refs = ((await session.run(() => repoReads(path).refs())) as Ref[] | undefined) ?? []
    void loadMerged()
    void loadRemotes()
    showingHidden = (await session.run(() => ShowingHidden(path))) ?? false
  }

  onMount(() => {
    void load()
    // Refs are re-read with every graph reload.
    // A reload of a large history pauses instead of finishing, and reset marks its swap.
    const offGraph = onBackend('graph:progress', (p) => {
      if (p.path === path && (p.done || p.reset)) void load()
    })
    // A remote added, renamed or pointed elsewhere, or a forge rule saved, in this
    // repository's config or the global one.
    const offRepo = onBackend('repo:changed', (c) => {
      if (c.path === path && c.kinds.includes('config')) void loadRemotes()
    })
    const offGlobal = onBackend('gitconfig:changed', () => void loadRemotes())
    return () => {
      offGraph()
      offRepo()
      offGlobal()
    }
  })
  import Empty from './ui/Empty.svelte'
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
        {#if ui.panel === 'filters'}<span class="hand from">from trailers</span>{/if}
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
        {#if careOn && !filter.trim() && deletable.length > 0}
          <div class="care-bar">
            <span>{deletable.length.toLocaleString()} safe to delete</span>
            <button class="btn sm danger" disabled={deleting} onclick={deleteAll}>{deleting ? 'Deleting…' : 'Delete all'}</button>
          </div>
        {/if}
      {/if}
    </header>

    <div class="scroll" class:narrowed={filtering || careOn} bind:this={list} bind:clientHeight={viewHeight} onscroll={(e) => (scrollTop = e.currentTarget.scrollTop)} hidden={ui.panel === 'repo' || ui.panel === 'filters'}>
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
            <SidebarRow {row} {onStackContext} {filter} {selectedCommit} remote={row.type === 'folder' && row.remote && row.depth === 0 ? headers[row.label] : undefined} brand={prefs.forgeColors} mergedInto={row.type === 'ref' && row.ref.kind === 'local' ? merged[row.ref.short] : undefined} onFolderClick={clickFolder} onRefClick={clickRef} {onActivate} {onContext} />
          </li>
        {/each}
      </ul>
      {#if rows.length === 0}
        {#if elsewhere}
          <Empty note="not here" compact>
            Nothing here.
            <button class="btn sm text" onclick={() => elsewhere && (ui.panel = elsewhere.panel)}>{TABS.find((t) => t.key === elsewhere?.panel)?.label} has {elsewhere.count.toLocaleString()}</button>
          </Empty>
        {:else if filtering}
          <Empty note="no match" compact>Nothing matches that.</Empty>
        {:else if careOn}
          <Empty note="all tidy" compact>Nothing needs care.</Empty>
        {:else}
          <Empty note="none yet" compact>{tab.empty}</Empty>
        {/if}
      {/if}
      {#if hiddenCount > 0 && (ui.panel === 'branches' || ui.panel === 'remotes')}
        <p class="hidden-note">
          <span class="meta">{hiddenCount.toLocaleString()} hidden</span>
          <button class="btn sm text" onclick={toggleHidden}>{showingHidden ? 'Hide them again' : 'Show them'}</button>
        </p>
      {/if}
    </div>

    {#if filtersWanted}
      <div class="scroll" hidden={ui.panel !== 'filters'}>
        <Lazy load={() => import('./filters/SmartFiltersPanel.svelte')} {path} {filter} picks={filterPicks} pins={pinnedKeys} {onPin} {onMenu} onCare={() => trailerCare.show(path)} open={ui.open} onPick={(key: string, value: string) => onFilterPick?.(key, value)} onSearch={(q: string) => onSearch?.(q)} onStatus={(s: { matches: number }) => (filterMatches = s.matches)} />
      </div>
    {/if}

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
        <Empty note="not here" compact>
          Nothing here.
          <button class="btn sm text" onclick={() => repoElsewhere && (ui.panel = repoElsewhere.panel)}>{TABS.find((t) => t.key === repoElsewhere?.panel)?.label} has {repoElsewhere.count.toLocaleString()}</button>
        </Empty>
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

  /* Where Smart filters come from, in the margin's hand. */
  .from {
    margin-left: 8px;
    color: var(--accent-a);
    white-space: nowrap;
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
    border-radius: var(--r-sm);
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
    border-radius: var(--r-sm);
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

  .care-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    color: var(--ink-soft);
    font-size: 13px;
  }

  .care-bar .btn {
    flex: none;
  }

  .chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 3px 9px;
    background: none;
    border: none;
    border-radius: 999px;
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
