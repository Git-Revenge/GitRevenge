<script lang="ts">
  import { Refs, SetShowHidden, ShowingHidden } from '../../wailsjs/go/app/App.js'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import { onMount, untrack } from 'svelte'
  import type { DialogSpec } from './Dialog.svelte'
  import type { Ref } from './graph/rowCache'
  import { provideCommands, type Command } from './commands.svelte'
  import RepoExtras from './RepoExtras.svelte'
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'
  import type { IconName } from './ui/Icon.svelte'
  import { kindInfo } from './branchKinds'
  import { clampWidth, DEFAULT_WIDTH, loadSidebar, otherPanelWithMatches, ROW_HEIGHT, saveSidebar, visibleRange, type SidebarPanel } from './sidebarState'
  import { buildPanels, careCount, matchParts, panelMatches, panelRows, refStatus, refTooltip, stackPlace, type FolderRow, type TreeRow } from './sidebarTree'

  let {
    path,
    selectedCommit,
    onPick,
    onActivate,
    onNewBranch,
    onManageRemotes,
    onContext,
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
    openDialog: (spec: DialogSpec) => void
  } = $props()

  const TABS: { key: SidebarPanel; label: string; icon: IconName; empty: string }[] = [
    { key: 'branches', label: 'Branches', icon: 'branch', empty: 'No branches yet.' },
    { key: 'remotes', label: 'Remotes', icon: 'remote', empty: 'No remote branches. Add a remote to get some.' },
    { key: 'tags', label: 'Tags', icon: 'tag', empty: 'No tags yet.' },
    { key: 'stashes', label: 'Stashes', icon: 'stash', empty: 'No stashes.' },
    { key: 'repo', label: 'Repository', icon: 'submodule', empty: 'No submodules or LFS here.' },
  ]

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
  let repoHasContent = $state(true)

  let panels = $derived(buildPanels(refs, { showHidden: showingHidden, sort: ui.sort }))
  let current = $derived(ui.panel === 'repo' ? null : panels[ui.panel])
  let careOn = $derived(care && ui.panel === 'branches')
  let rows = $derived(current ? panelRows(current, { open: ui.open, query: filter, care: careOn }) : [])
  let filtering = $derived(filter.trim().length > 0)
  let needCare = $derived(careCount(panels.branches))
  let matches = $derived(current && filtering ? panelMatches(current, filter) : 0)
  let elsewhere = $derived(rows.length === 0 && filtering ? otherPanelWithMatches(panels, ui.panel, filter) : null)
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

  /** The rail badge's number, and what it means for screen readers. */
  function badge(key: SidebarPanel): { count: number; text: string } {
    if (filtering) {
      const count = key === 'repo' ? 0 : panelMatches(panels[key], filter)
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

  provideCommands(() => {
    const list: Command[] = [
      {
        id: 'view.sidebar',
        group: 'View',
        title: ui.collapsed ? 'Show the sidebar panel' : 'Hide the sidebar panel',
        keywords: 'collapse rail graph width',
        run: () => (ui.collapsed = !ui.collapsed),
      },
    ]
    for (const r of refs) {
      if (r.kind === 'stash') continue
      if (r.kind === 'local' && !r.head && onActivate)
        list.push({ id: `checkout.${r.name}`, group: 'Branch', title: `Check out ${r.short}`, keywords: 'switch branch', run: () => onActivate(r) })
      const what = r.kind === 'tag' ? 'tag' : r.kind === 'remote' ? 'remote branch' : 'branch'
      list.push({ id: `goto.${r.name}`, group: 'Go to', title: `Go to ${r.short}`, keywords: `${what} show reveal graph`, run: () => onPick(r) })
    }
    return list
  })

  async function load() {
    refs = ((await session.run(() => Refs(path))) as Ref[] | undefined) ?? []
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
  <nav class="rail" aria-label="Sidebar panels">
    {#each TABS as t (t.key)}
      {@const b = badge(t.key)}
      <button
        class="tab"
        class:on={t.key === ui.panel && !ui.collapsed}
        aria-pressed={t.key === ui.panel && !ui.collapsed}
        aria-label={b.count ? `${t.label}, ${b.text}` : t.label}
        title={b.count ? `${t.label}: ${b.text}` : t.label}
        onclick={() => pickTab(t.key)}
      >
        <Icon name={t.icon} size={18} />
        {#if b.count}<span class="pip" aria-hidden="true">{b.count > 99 ? '99+' : b.count}</span>{/if}
      </button>
    {/each}
    <button
      class="tab toggle"
      aria-expanded={!ui.collapsed}
      aria-label={ui.collapsed ? 'Show the sidebar panel' : 'Hide the sidebar panel'}
      title={ui.collapsed ? 'Show the sidebar panel' : 'Hide the sidebar panel'}
      onclick={() => (ui.collapsed = !ui.collapsed)}
    >
      <Icon name="sidebar" size={18} />
    </button>
  </nav>

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
      {#if ui.panel !== 'repo'}
        <label class="filter">
          <Icon name="search" size={14} />
          <input type="search" placeholder="Filter all panels" aria-label="Filter branches, remotes, tags and stashes" bind:value={filter} />
          {#if filtering}
            <span class="meta" aria-live="polite">{matches.toLocaleString()}</span>
            <button class="clear" aria-label="Clear the filter" title="Clear the filter" onclick={() => (filter = '')}><Icon name="close" size={12} /></button>
          {/if}
        </label>
      {/if}
      {#if ui.panel === 'branches'}
        <div class="chips" role="group" aria-label="Which branches to show">
          <button class="chip" aria-pressed={!care} onclick={() => (care = false)}>All</button>
          <button class="chip" aria-pressed={care} onclick={() => (care = true)}>Needs care <span class="meta">{needCare.toLocaleString()}</span></button>
        </div>
      {/if}
    </header>

    <div class="scroll" class:narrowed={filtering || careOn} bind:this={list} bind:clientHeight={viewHeight} onscroll={(e) => (scrollTop = e.currentTarget.scrollTop)} hidden={ui.panel === 'repo'}>
      <ul style:height="{rows.length * ROW_HEIGHT}px" style:padding-top="{range.start * ROW_HEIGHT}px">
        {#each rows.slice(range.start, range.end) as row (rowKey(row))}
          <li class:group={row.startsGroup}>
            {#if row.type === 'folder'}
              {@const k = kindInfo(row.kind)}
              <button class="row folder" style:--depth={row.depth} aria-expanded={row.open} title="{row.label}/" onclick={() => toggleFolder(row)}>
                <span class="caret" class:open={row.open}><Icon name="right" size={10} /></span>
                {#if k}
                  <span class="kind-icon" style:color="var({k.token})" title={k.label}><Icon name={k.icon} size={14} /><span class="visually-hidden">{k.label}</span></span>
                {:else}
                  <span class="kind-icon"><Icon name={row.remote ? 'remote' : row.open ? 'folder-open' : 'folder'} size={14} /></span>
                {/if}
                <span class="name">{row.label}</span>
                {#if row.holdsHead && !row.open}<span class="dot" title="The checked-out branch is in here"><span class="visually-hidden">Holds the checked-out branch</span></span>{/if}
                <span class="meta count">{row.count.toLocaleString()}</span>
              </button>
            {:else}
              {@const r = row.ref}
              {@const k = kindInfo(row.mark)}
              {@const status = refStatus(r)}
              {@const parts = matchParts(row.label, filter)}
              {@const isSelected = !!selectedCommit && r.commit === selectedCommit}
              <button
                class="row ref"
                class:head={r.head}
                class:selected={isSelected}
                aria-current={isSelected || undefined}
                class:hidden-ref={r.hidden}
                class:stacked={row.stacked}
                class:context={row.context}
                style:--depth={row.depth}
                onclick={() => onPick(r)}
                ondblclick={() => onActivate?.(r)}
                oncontextmenu={(e) => {
                  if (!onContext) return
                  e.preventDefault()
                  onContext(r, e.clientX, e.clientY)
                }}
                title={refTooltip(r)}
              >
                {#if r.head}<span class="here" aria-label="checked out"></span>{/if}
                {#if k}<span class="kind-icon" style:color="var({k.token})" title={k.label}><Icon name={k.icon} size={13} /><span class="visually-hidden">{k.label}</span></span>{/if}
                <span class="name">{parts.pre}{#if parts.hit}<mark>{parts.hit}</mark>{/if}{parts.post}</span>
                {#if r.hidden}<span class="visually-hidden">, hidden</span>{/if}
                {#if stackPlace(r)}<span class="visually-hidden">, {stackPlace(r)}</span>{/if}
                {#if status}<span class="track {status.tone}"><span aria-hidden="true">{status.text}</span><span class="visually-hidden">, {status.spoken}</span></span>{/if}
              </button>
              {#if onContext}
                <button
                  class="more"
                  aria-label="More for {r.kind === 'stash' ? r.subject || r.short : r.short}"
                  aria-haspopup="menu"
                  title="More"
                  onclick={(e) => {
                    const box = e.currentTarget.getBoundingClientRect()
                    onContext(r, box.left, box.bottom)
                  }}><Icon name="more" size={14} /></button
                >
              {/if}
            {/if}
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
      <RepoExtras
        {path}
        {openDialog}
        onStatus={(s) => {
          repoAttention = s.attention
          repoHasContent = s.hasContent
        }}
      />
      {#if !repoHasContent}<p class="empty">{tab.empty}</p>{/if}
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

  .rail {
    flex: none;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    width: 44px;
    padding: 8px 0;
    box-sizing: border-box;
    background: var(--paper-warm);
    border-right: 1px solid var(--paper-edge);
  }

  .tab {
    position: relative;
    display: grid;
    place-items: center;
    width: 36px;
    height: 36px;
    padding: 0;
    background: none;
    border: none;
    border-radius: 8px;
    color: var(--ink-mute);
    transition:
      background-color var(--dur-quick),
      color var(--dur-quick);
  }

  .tab:hover:not(:disabled) {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .tab.on {
    background: var(--selected);
    color: var(--ink);
  }

  .toggle {
    margin-top: auto;
  }

  .pip {
    position: absolute;
    right: 0;
    top: 0;
    min-width: 15px;
    padding: 0 3px;
    box-sizing: border-box;
    border-radius: 8px;
    background: var(--paper-deep);
    color: var(--ink-soft);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 15px;
    text-align: center;
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

  /* The panel's header already draws the line above the first section. */
  .repo-panel :global(section:first-child) {
    border-top: none;
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

  .row {
    position: relative;
    display: flex;
    align-items: center;
    width: 100%;
    height: 28px;
    box-sizing: border-box;
    background: none;
    border: none;
    border-radius: 0;
    color: var(--ink-soft);
    font-size: 14px;
    text-align: left;
    white-space: nowrap;
  }

  .row:hover:not(:disabled) {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .folder {
    gap: 6px;
    padding: 2px 10px 2px calc(10px + var(--depth, 0) * 14px);
  }

  /* Only the checked-out branch is at full strength; hover, focus and selection bring the
     others up to it. */
  .ref {
    gap: 7px;
    padding: 2px 10px 2px calc(28px + var(--depth, 0) * 14px);
    color: var(--ink-mute);
  }

  .ref:focus-visible,
  li:hover .ref {
    color: var(--ink);
  }

  li:hover .row {
    background: var(--wash-hover);
  }

  .ref.selected {
    background: var(--selected);
    color: var(--ink);
  }

  .ref.head {
    color: var(--ink);
    font-weight: 600;
  }

  /* While narrowed, what matched stands out from the parents kept only for context. */
  .narrowed .ref:not(.context, .head, .selected) {
    color: var(--ink-soft);
  }

  mark {
    background: var(--highlight);
    color: inherit;
  }

  .more {
    position: absolute;
    z-index: 2;
    top: 2px;
    right: 4px;
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    padding: 0;
    background: var(--paper-warm);
    border: none;
    border-radius: 7px;
    color: var(--ink-soft);
    opacity: 0;
    transition: opacity var(--dur-quick);
  }

  .more:hover:not(:disabled) {
    background: var(--paper-deep);
    color: var(--ink);
  }

  li:hover .more,
  li:focus-within .more {
    opacity: 1;
  }

  /* More replaces the status; it stays readable to screen readers. */
  li:hover .track,
  li:focus-within .track {
    opacity: 0;
  }

  .here,
  .dot {
    flex: none;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--link);
  }

  .here {
    margin-left: -15px;
  }

  /* A stacked branch sits indented under its parent, with a thin elbow joining them. */
  .ref.stacked::before {
    content: '';
    position: absolute;
    left: calc(16px + var(--depth, 0) * 14px);
    top: 0;
    width: 7px;
    height: 50%;
    border-left: 1px solid var(--paper-edge);
    border-bottom: 1px solid var(--paper-edge);
    border-bottom-left-radius: 4px;
    pointer-events: none;
  }

  .caret {
    flex: none;
    display: grid;
    place-items: center;
    width: 10px;
    color: var(--ink-mute);
    transition: transform var(--dur-base) var(--ease-ink);
  }

  .caret.open {
    transform: rotate(90deg);
  }

  .kind-icon {
    display: inline-flex;
    flex: none;
    color: var(--ink-mute);
  }

  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .count {
    margin-left: auto;
    flex: none;
  }

  .track {
    margin-left: auto;
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
    font-weight: 400;
    color: var(--link);
  }

  .track.warning {
    color: var(--warning);
  }

  .track.muted {
    color: var(--ink-mute);
  }

  .hidden-ref .name {
    font-style: italic;
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
    .tab,
    .caret,
    .more,
    .edge {
      transition: none;
    }
  }
</style>
