<script lang="ts">
  import { KINDS, kindFilterRows, type KindFilterRow } from '../branchKinds'
  import { errorText } from '../errors'
  import { onMount } from 'svelte'
  import { CommitRow, CommitSignatures, GraphFocus, GraphRows, Refs, RepoPreferences, SearchGraph, SetGraphFocus, SetKindVisible } from '../../../wailsjs/go/app/App.js'
  import { session } from '../session.svelte'
  import { EventsOn } from '../../../wailsjs/runtime/runtime.js'
  import { theme as colorTheme } from '../theme.svelte'
  import { provideCommands } from '../commands.svelte'
  import Icon from '../ui/Icon.svelte'
  import { labelsFold, labelsUnfold } from '../ui/motion'
  import KindFilter from './KindFilter.svelte'
  import RefChip from './RefChip.svelte'
  import { drawRow, sealOf, graphWidth, LANE_COUNT, laneCapacity, laneVar, laneWindow, layoutColumns, mainRemote, METRICS, refColumnWidth, refLabels, setLaneColors, type LabelSpan, type Layout, type RefLabel, type Theme } from './draw'
  import { RowCache, type GraphProgress, type Page, type Ref, type Row } from './rowCache'
  import { scrollMap, topAfterScroll, toScroll, type ScrollMap } from './scrollMap'

  let {
    path,
    onSelect,
    onContext,
    onDrop,
    onSelectedRefs,
  }: {
    path: string
    /** Right-click on a row, with the refs pointing at it and the mouse position. */
    onContext?: (row: Row, refs: Ref[], x: number, y: number) => void
    /** A row dragged onto another row. */
    onDrop?: (source: Row, sourceRefs: Ref[], target: Row, targetRefs: Ref[], x: number, y: number) => void
    /** auto is true for the automatic selection of HEAD when the graph first loads. */
    onSelect?: (row: Row | undefined, auto: boolean) => void
    /** Labels for the selected commit's refs, whenever they change. They can load after the selection. */
    onSelectedRefs?: (labels: RefLabel[]) => void
  } = $props()

  /** The repository's main remote, whose prefix labels leave off. */
  let remote = 'origin'
  /** The Kinds filter's rows (ADR 0009). */
  let kindRows = $state<KindFilterRow[]>([])

  async function loadKinds() {
    const [refs, prefs] = await Promise.all([
      // The filter is optional: without these answers it just doesn't show.
      Refs(path).catch(() => []),
      RepoPreferences(path).catch(() => null),
    ])
    remote = mainRemote(refs as Ref[])
    schedule()
    kindRows = kindFilterRows(refs as Ref[], prefs?.hideKinds ?? [])
  }

  async function setKindVisible(kind: string, visible: boolean) {
    await session.run(() => SetKindVisible(path, kind, visible))
    await loadKinds()
  }

  /** What the graph is focused on, if anything: the opposite of hiding a branch. */
  let focus = $state<{ branch: string; stack: boolean } | null>(null)
  let focusLabel = $derived(focus ? (focus.stack ? `the stack of ${focus.branch}` : focus.branch) : '')

  /** Whether the graph is focused, so a branch outside it can be explained. */
  export function focused(): boolean {
    return !!focus
  }

  /** Reads the focus, reporting whether it's a new one to show from the top. */
  async function loadFocus(): Promise<boolean> {
    const was = focusLabel
    const next = await GraphFocus(path).catch(() => null)
    focus = next?.branch ? next : null
    return !!focus && focusLabel !== was
  }

  function stopFocusing() {
    void session.run(() => SetGraphFocus(path, '', false))
  }

  /** Rows fetched beyond the viewport, so short scrolls never show blanks. */
  const OVERSCAN = 40
  /** The graph column never takes more than this share of the width. */
  const MAX_GRAPH_SHARE = 0.4

  let density = $derived(colorTheme.density)
  let rowH = $derived(METRICS[density].rowH)

  let scroller: HTMLDivElement
  let canvas: HTMLCanvasElement
  let total = $state(0)
  let loading = $state(true)
  let paused = $state(false)
  let estimate = $state(0)
  /** Rows the scroll area spans: the estimated whole history while it's still loading, so
   * the scrollbar is about right and scrolling past the loaded rows reads them in. */
  let span = $derived(loading && estimate > total ? estimate : total)

  let viewH = $state(0)
  let map = $derived(scrollMap(span * rowH, viewH))
  /** The scroll position within the full history's height. scroller.scrollTop is scaled
   * from it when the history is taller than a scroll area can be (see scrollMap). */
  let top = 0
  /** The scrollTop last set from top, so the scroll event that causes isn't read back
   * (that would round top to the scale). Null when the next scroll isn't ours. */
  let setScroll: number | null = null

  /** Moves the scroller to match top. No move means no scroll event, so nothing is marked
   * as ours; a leftover mark would swallow the person's next small scroll. */
  function placeScroller(m: ScrollMap) {
    const next = Math.round(toScroll(top, m))
    if (Math.abs(scroller.scrollTop - next) < 1) return
    setScroll = next
    scroller.scrollTop = next
  }

  function scrollTo(t: number) {
    top = Math.max(0, Math.min(map.maxTop, t))
    placeScroller(map)
    schedule()
  }

  function onScroll() {
    top = topAfterScroll(scroller.scrollTop, setScroll, top, map)
    setScroll = null
    schedule()
  }

  /** When scaled, the wheel moves through rows at its normal speed rather than the scale's. */
  function onWheel(e: WheelEvent) {
    if (map.ratio === 1 || e.ctrlKey) return
    e.preventDefault()
    const unit = e.deltaMode === 1 ? rowH : e.deltaMode === 2 ? viewH : 1
    scrollTo(top + e.deltaY * unit)
  }

  // Keep the scrollbar in step as the history's height changes while loading.
  $effect(() => {
    const m = map
    if (!scroller) return
    placeScroller(m)
  })
  let error = $state('')
  /** Fetching rows failed, separately from reading the history; clears when a fetch works. */
  let rowError = $state('')
  let selected = $state(-1)
  /** The column positions as last drawn, for the header above the canvas. */
  let columns = $state<Layout>()
  /** The most lanes any row in view has used. The graph's width follows it, and only grows, so
   * the message column doesn't move while scrolling. A reload starts it again. */
  let widest = 1

  /** Most refs an expanded label shows; the commit panel and right-click menu have them all. */
  const EXPANDED_REFS = 12
  /** A chip's height and its gap, and the padding around them, for fitting the list in view. */
  const CHIP_STEP = 26
  const EXPAND_PAD = 4
  /** Where each row in view drew its labels, for hovering over them. */
  // eslint-disable-next-line svelte/prefer-svelte-reactivity
  const labelSpans = new Map<number, LabelSpan>()
  /** The last mouse position over the graph; scrolling moves rows under a still pointer. */
  let pointer: { x: number; y: number } | null = null
  /** The labels expanded by hovering over them. */
  let expanded = $state<{ index: number; left: number; top: number; labels: RefLabel[]; lane: string } | null>(null)
  let lastSelectedRefs = ''

  /** Expands the labels under the pointer, if it's over any, and closes them otherwise. */
  function updateExpanded() {
    const rect = scroller?.getBoundingClientRect()
    const i = pointer && rect && !drag?.active ? Math.floor((pointer.y - rect.top + top) / rowH) : -1
    const span = labelSpans.get(i)
    const x = pointer && rect ? pointer.x - rect.left : -1
    const row = cache.get(i)
    if (!span || !row || x < span[0] || x > span[1]) {
      if (expanded) expanded = null
      return
    }
    const labels = refLabels(cache.refsFor(row.id), remote)
    const height = Math.min(labels.length, EXPANDED_REFS + 1) * CHIP_STEP + EXPAND_PAD * 2
    const place = Math.round(i * rowH - top + (rowH - CHIP_STEP) / 2 - EXPAND_PAD)
    const fitted = Math.max(0, Math.min(place, viewH - height))
    if (expanded?.index === i && expanded.top === fitted) return
    expanded = { index: i, left: span[0] - EXPAND_PAD, top: fitted, labels, lane: laneVar(row.color) }
  }
  let hovered = -1
  let frame = 0
  /** Drag state: the row pressed and where; the row under the pointer while dragging. */
  let drag: { from: number; x: number; y: number; active: boolean } | null = null
  let dropTarget = -1

  // Search state. matchSet is only used for dimming when every match fits in the result.
  let searchInput: HTMLInputElement
  let query = $state('')
  let matches = $state<number[]>([])
  let matchTotal = $state(0)
  let matchSet: Set<number> | null = null
  let searchSeq = 0

  const cache = new RowCache(
    // The generated binding types mirror rowCache's; the cast only bridges class vs interface.
    async (start, count) => (await GraphRows(path, start, count)) as unknown as Page,
    () => {
      total = cache.total
      loading = cache.loading
      paused = cache.paused
      estimate = cache.estimate
      selectHeadOnce()
      schedule()
    },
    (e) => (rowError = e === null ? '' : errorText(e)),
  )

  function theme(): Theme {
    const css = getComputedStyle(document.documentElement)
    const token = (name: string) => css.getPropertyValue(name).trim()
    setLaneColors(Array.from({ length: LANE_COUNT }, (_, i) => token(`--lane-${i}`)))
    return {
      bg: token('--paper'),
      fg: token('--ink'),
      soft: token('--ink-soft'),
      muted: token('--ink-mute'),
      deep: token('--paper-deep'),
      selected: token('--selected'),
      hover: token('--wash-hover'),
      tag: token('--lane-4'),
      good: token('--success'),
      bad: token('--danger'),
      warn: token('--warning'),
      kinds: Object.fromEntries(Object.entries(KINDS).map(([k, info]) => [k, token(info.token)])),
    }
  }

  // Signature status of rows that have been on screen, fetched a screen at a time. Plain
  // collections on purpose: the canvas repaints through schedule(), not reactivity.
  // eslint-disable-next-line svelte/prefer-svelte-reactivity
  const sigs = new Map<string, string>()
  // eslint-disable-next-line svelte/prefer-svelte-reactivity
  const sigWanted = new Set<string>()
  let sigTimer: ReturnType<typeof setTimeout> | undefined
  function wantSignature(id: string) {
    if (sigs.has(id) || sigWanted.has(id)) return
    sigWanted.add(id)
    clearTimeout(sigTimer)
    sigTimer = setTimeout(fetchSignatures, 150)
  }
  async function fetchSignatures() {
    const ids = [...sigWanted]
    sigWanted.clear()
    for (const id of ids) sigs.set(id, '') // asked; don't ask again
    // Signature marks are decoration: a failure just means no mark.
    const checks = await CommitSignatures(path, ids).catch(() => [])
    let signed = false
    for (const c of checks) {
      sigs.set(c.id, c.status)
      signed ||= c.status !== 'none'
    }
    if (signed) schedule()
  }

  function schedule() {
    if (!frame) frame = requestAnimationFrame(draw)
  }

  function draw() {
    frame = 0
    if (!canvas || !scroller) return
    const dpr = window.devicePixelRatio || 1
    const w = scroller.clientWidth
    const h = scroller.clientHeight
    // Hidden while a file is open. Coming back resizes it, which draws it again.
    if (!w || !h) return
    if (h !== viewH) viewH = h
    if (canvas.width !== Math.round(w * dpr) || canvas.height !== Math.round(h * dpr)) {
      canvas.width = Math.round(w * dpr)
      canvas.height = Math.round(h * dpr)
      canvas.style.width = `${w}px`
      canvas.style.height = `${h}px`
    }
    const ctx = canvas.getContext('2d')!
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    ctx.clearRect(0, 0, w, h)

    const first = Math.floor(top / rowH)
    const last = Math.min(span, Math.ceil((top + h) / rowH))
    cache.ensure(first - OVERSCAN, last + OVERSCAN)

    let minCol = Infinity
    let maxCol = 0
    for (let i = first; i < last; i++) {
      const row = cache.get(i)
      if (!row) continue
      widest = Math.max(widest, row.width)
      minCol = Math.min(minCol, row.col)
      maxCol = Math.max(maxCol, row.col)
    }
    const m = METRICS[density]
    const graphW = graphWidth(widest, (w - refColumnWidth(w)) * MAX_GRAPH_SHARE, m)
    // A history wider than the column scrolls its lanes so the nodes in view stay visible.
    const sel = cache.get(selected)
    const selectedRefs = sel ? cache.refsFor(sel.id) : []
    const selectedKey = sel ? `${sel.id} ${remote} ${selectedRefs.map((r) => r.name).join(' ')}` : ''
    if (selectedKey !== lastSelectedRefs) {
      lastSelectedRefs = selectedKey
      onSelectedRefs?.(refLabels(selectedRefs, remote))
    }
    const firstLane = minCol === Infinity ? 0 : laneWindow(minCol, maxCol, sel && selected >= first && selected < last ? sel.col : null, laneCapacity(graphW, m))
    const lay = layoutColumns(w, graphW, density, firstLane, remote)
    if (!columns || columns.width !== lay.width || columns.graphW !== lay.graphW || columns.density !== lay.density) {
      columns = lay
    }
    const th = theme()
    const now = Date.now() / 1000
    labelSpans.clear()
    for (let i = first; i < last; i++) {
      const row = cache.get(i)
      const y = i * rowH - top
      // The drop target is highlighted like a selection while dragging.
      if (!row) continue
      wantSignature(row.id)
      const span = drawRow(ctx, row, cache.refsFor(row.id), y, lay, th, i === selected || i === dropTarget, i === hovered, !!matchSet && !matchSet.has(i), now, sealOf(sigs.get(row.id)), { prev: cache.get(i - 1), prev2: cache.get(i - 2), next: cache.get(i + 1) })
      if (span) labelSpans.set(i, span)
    }
    updateExpanded()
    // Lanes scrolled out on the left fade in from the edge, so the cut doesn't look like the end.
    if (firstLane > 0) {
      const fade = ctx.createLinearGradient(lay.refW, 0, lay.refW + m.pad + m.laneW, 0)
      fade.addColorStop(0, th.bg)
      fade.addColorStop(1, 'transparent')
      ctx.fillStyle = fade
      ctx.fillRect(lay.refW, 0, m.pad + m.laneW, h)
    }
  }

  function rowAt(e: MouseEvent): number {
    const rect = scroller.getBoundingClientRect()
    const i = Math.floor((e.clientY - rect.top + top) / rowH)
    return i >= 0 && i < span ? i : -1
  }

  /** The refs on a commit, for its menu. */
  export function refsOf(id: string): Ref[] {
    return cache.refsFor(id)
  }

  /** Searches the graph for q and moves to the first match, e.g. a trailer from the
   * commit panel. */
  export async function search(q: string) {
    query = q
    await runSearch()
    jump(1)
  }

  /** Scrolls to and selects row i (e.g. when a branch is picked in the sidebar). */
  export function reveal(i: number) {
    if (i < 0) return
    // Center it rather than pinning it to an edge.
    scrollTo(i * rowH - viewH / 2)
    select(i)
    scroller.focus()
  }

  /**
   * Initial selection: the top row until the checked-out commit is known, then that commit.
   * Refs load alongside history, so HEAD's label can arrive after the first rows; until the
   * user picks something themselves, keep trying to move to HEAD.
   */
  let autoSelect: 'none' | 'fallback' | 'done' = 'none'

  function selectHeadOnce() {
    if (autoSelect === 'done' || !cache.get(0)) return
    for (let i = 0; i < 200 && cache.get(i); i++) {
      if (cache.refsFor(cache.get(i)!.id).some((r) => r.head)) {
        autoSelect = 'done'
        if (i !== selected) select(i, true)
        return
      }
    }
    if (autoSelect === 'none') {
      autoSelect = 'fallback'
      select(0, true)
    }
  }

  /** The commit selected before a reload, so the selection follows it to its new row. */
  let selectedId = ''

  /** Keeps the selection on its commit after a reload. A new focus starts from its top. */
  async function followSelection() {
    const refocused = await loadFocus()
    if (refocused) scrollTo(0)
    if (selected < 0) return
    const id = selectedId
    const row = refocused ? 0 : id ? await CommitRow(path, id).catch(() => -1) : -1
    // The selection moved while that was being asked, to HEAD or by the person: it stands.
    if (selectedId !== id) return
    // If the commit is gone (e.g. amended), stay on the same position but refresh details.
    const target = row >= 0 ? row : selected
    cache.ensure(target, target + 1)
    // Wait for the row to load, giving up after ~5s (e.g. the graph got shorter).
    let frames = 300
    const wait = () => {
      if (selectedId !== id) return
      if (cache.get(target)) select(target, true)
      else if (frames-- > 0) requestAnimationFrame(wait)
    }
    wait()
  }

  function select(i: number, auto = false) {
    if (!auto) autoSelect = 'done'
    selected = Math.max(0, Math.min(span - 1, i))
    // Keep the selection in view.
    const y = selected * rowH
    // The first selection can land before the first draw has measured the view.
    const height = viewH || scroller.clientHeight
    if (y < top) scrollTo(y)
    else if (y + rowH > top + height) scrollTo(y + rowH - height)
    selectedId = cache.get(selected)?.id ?? ''
    onSelect?.(cache.get(selected), auto)
    schedule()
  }

  // Typing searches once it pauses: a trailer search reads every commit message.
  let searchTimer: ReturnType<typeof setTimeout> | undefined
  let searchPending = false
  function searchSoon() {
    clearTimeout(searchTimer)
    searchPending = true
    searchTimer = setTimeout(() => void runSearch(), 300)
  }

  async function runSearch() {
    clearTimeout(searchTimer)
    searchPending = false
    const seq = ++searchSeq
    const q = query
    if (!q.trim()) {
      matches = []
      matchTotal = 0
      matchSet = null
      schedule()
      return
    }
    const res = await session.run(() => SearchGraph(path, q))
    if (!res || seq !== searchSeq) return
    matches = res.rows
    matchTotal = res.total
    matchSet = res.total === res.rows.length ? new Set(res.rows) : null
    schedule()
  }

  /** Searches for the selected commit's author, which dims everyone else's commits. */
  function searchAuthor() {
    const author = cache.get(selected)?.author
    if (!author) return
    query = author
    void runSearch()
  }

  /** Moves to the next (or previous) match relative to the selection. */
  function jump(dir: 1 | -1) {
    if (matches.length === 0) return
    const next =
      dir === 1
        ? (matches.find((m) => m > selected) ?? matches[0])
        : (matches.findLast((m) => m < selected) ?? matches[matches.length - 1])
    reveal(next)
    searchInput.focus()
  }

  function onSearchKey(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      const dir = e.shiftKey ? -1 : 1
      // Enter right after typing waits for that search rather than using the last one.
      if (searchPending) void runSearch().then(() => jump(dir))
      else jump(dir)
    } else if (e.key === 'Escape') {
      query = ''
      void runSearch()
      scroller.focus()
    }
  }

  provideCommands(() => [
    ...(focus ? [{ id: 'graph.stopFocus', group: 'View', title: `Stop focusing on ${focusLabel}`, run: stopFocusing }] : []),
    {
      id: 'graph.search',
      group: 'View',
      title: 'Search commits',
      keywords: 'find message author sha history',
      shortcut: 'Mod+F',
      whenTyping: true,
      run: () => {
        searchInput.focus()
        searchInput.select()
      },
    },
  ])

  let currentMatch = $derived(matches.indexOf(selected))

  function onKey(e: KeyboardEvent) {
    const page = Math.max(1, Math.floor(scroller.clientHeight / rowH) - 1)
    const moves: Record<string, number> = {
      ArrowDown: 1,
      ArrowUp: -1,
      PageDown: page,
      PageUp: -page,
      Home: -Infinity,
      End: Infinity,
    }
    if (e.key in moves) {
      e.preventDefault()
      const d = moves[e.key]
      select(d === -Infinity ? 0 : d === Infinity ? span - 1 : (selected < 0 ? 0 : selected + d))
    }
  }

  // The canvas holds painted colours, so a theme change needs a repaint.
  $effect(() => {
    void colorTheme.resolved
    schedule()
  })

  // Switching density keeps the same commit at the top of the view.
  let lastRowH = 0
  $effect(() => {
    const h = rowH
    if (scroller && lastRowH && lastRowH !== h) scrollTo((top / lastRowH) * h)
    lastRowH = h
    schedule()
  })

  onMount(() => {
    const ro = new ResizeObserver(schedule)
    ro.observe(scroller)
    // Not passive: a scaled history handles the wheel itself.
    scroller.addEventListener('wheel', onWheel, { passive: false })
    // The canvas paints with the bundled fonts; repaint once they finish loading.
    void document.fonts.ready.then(schedule)
    document.fonts.addEventListener('loadingdone', schedule)
    void loadKinds()
    void loadFocus()
    const off = EventsOn('graph:progress', (p: GraphProgress) => {
      if (p.path !== path) return
      if (p.done) void loadKinds()
      cache.setTotal(p.rows, !p.done, p.paused, p.estimate)
      if (p.refsReady) cache.invalidate() // labels arrived after the first rows
      if (p.reset || p.done) widest = 1
      if (p.reset && !p.done) {
        // A reload swapped in part way through its history: rows are stale.
        cache.invalidate()
        if (query) void runSearch()
        void followSelection()
      }
      if (p.done) {
        // A reload swaps in a new graph; anything cached may be stale.
        cache.invalidate()
        error = p.error
        if (query) void runSearch()
        void followSelection()
      }
      total = p.rows
      loading = !p.done
      paused = p.paused
      if (p.estimate) estimate = p.estimate
      schedule()
    })
    cache.ensure(0, 200)
    return () => {
      ro.disconnect()
      scroller.removeEventListener('wheel', onWheel)
      document.fonts.removeEventListener('loadingdone', schedule)
      off()
      clearTimeout(searchTimer)
      clearTimeout(sigTimer)
      cancelAnimationFrame(frame)
    }
  })
</script>

<div class="graph">
  <div class="status">
    {#if error || rowError}
      <span class="error">Couldn't read history: {error || rowError}</span>
    {:else if loading && paused}
      <span class="meta" title="More history loads as you scroll, search or jump to a commit"
        >{total.toLocaleString()}{estimate > total ? ` of about ${estimate.toLocaleString()}` : ''} commits loaded</span
      >
    {:else if loading}
      <span class="meta">Loading history… {total.toLocaleString()} commits</span>
    {:else}
      <span class="meta">{total.toLocaleString()} commits</span>
    {/if}
    {#if kindRows.length}<KindFilter rows={kindRows} onToggle={setKindVisible} />{/if}
    <button
      class="btn sm quiet density"
      onclick={() => session.run(() => colorTheme.toggleDensity())}
      title={density === 'compact' ? 'Show the timeline: two-line rows with authors' : 'Show compact single-line rows'}
      aria-pressed={density === 'compact'}
    >
      <Icon name={density === 'compact' ? 'rows' : 'timeline'} size={14} />{density === 'compact' ? 'Compact' : 'Timeline'}
    </button>
    {#if focus}
      <span class="focus" title="Only {focusLabel} and what it's built on are showing">
        <Icon name="focus" size={13} /><span class="focus-name">Focused on {focusLabel}</span>
        <button class="btn sm quiet icon" onclick={stopFocusing} aria-label="Stop focusing" title="Stop focusing, and show every branch again"><Icon name="close" size={13} /></button>
      </span>
    {/if}
    <span class="search">
      <Icon name="search" size={14} />
      <input
        bind:this={searchInput}
        bind:value={query}
        oninput={searchSoon}
        onkeydown={onSearchKey}
        type="search"
        placeholder="Search message, author or SHA"
        title="Search subjects, authors and SHAs, or a trailer such as Co-authored-by: name or Closes: #12"
        aria-label="Search commits"
      />
      {#if query.trim()}
        <span class="count">
          {#if matchTotal === 0}No matches{:else}{currentMatch >= 0 ? `${currentMatch + 1} of ` : ''}{matchTotal.toLocaleString()}{/if}
        </span>
        <button class="btn sm quiet icon step up" onclick={() => jump(-1)} disabled={!matchTotal} aria-label="Previous match" title="Previous match (Shift+Enter)"
          ><Icon name="chevron" size={13} /></button
        >
        <button class="btn sm quiet icon step" onclick={() => jump(1)} disabled={!matchTotal} aria-label="Next match" title="Next match (Enter)"><Icon name="chevron" size={13} /></button>
      {/if}
    </span>
  </div>
  {#if columns?.refW}
    <div class="columns">
      <span style:left="12px">Branch / tag</span>
      {#if columns.graphW >= 56}<span style:left="{columns.refW + columns.m.pad}px">Graph</span>{/if}
      <span style:left="{columns.textX}px">Message</span>
      {#if columns.authorW}
        <span style:left="{columns.authorX}px">
          Author
          <button class="btn sm quiet icon" onclick={searchAuthor} disabled={selected < 0} aria-label="Search for the selected commit's author" title="Search for the selected commit's author"><Icon name="filter" size={13} /></button>
        </span>
      {/if}
      {#if columns.dateW}<span style:left="{columns.dateX}px">Date</span>{/if}
      <span style:left="{columns.shaX}px">SHA</span>
    </div>
  {/if}
  <div class="viewport">
    <canvas bind:this={canvas}></canvas>
    <div
      class="scroller"
      bind:this={scroller}
      tabindex="0"
      role="grid"
      aria-label="Commit history"
      aria-rowcount={span}
      onscroll={onScroll}
      onkeydown={onKey}
      onclick={(e) => {
        const i = rowAt(e)
        if (i >= 0) select(i)
      }}
      onmousedown={(e) => {
        if (e.button !== 0 || !onDrop) return
        const i = rowAt(e)
        if (i >= 0) drag = { from: i, x: e.clientX, y: e.clientY, active: false }
      }}
      onmousemove={(e) => {
        if (drag && !drag.active && Math.hypot(e.clientX - drag.x, e.clientY - drag.y) > 6) drag.active = true
        if (drag?.active) {
          const t = rowAt(e)
          if (t !== dropTarget) {
            dropTarget = t
            schedule()
          }
        }
        const i = rowAt(e)
        if (i !== hovered) {
          hovered = i
          schedule()
        }
        pointer = { x: e.clientX, y: e.clientY }
        updateExpanded()
      }}
      onmouseup={(e) => {
        const d = drag
        const target = dropTarget
        drag = null
        dropTarget = -1
        if (!d?.active) return
        schedule()
        const s = cache.get(d.from)
        const t = target >= 0 ? cache.get(target) : undefined
        if (s && t && t.id !== s.id) onDrop?.(s, cache.refsFor(s.id), t, cache.refsFor(t.id), e.clientX, e.clientY)
      }}
      oncontextmenu={(e) => {
        const i = rowAt(e)
        const row = i >= 0 ? cache.get(i) : undefined
        if (!row || !onContext) return
        e.preventDefault()
        select(i)
        onContext(row, cache.refsFor(row.id), e.clientX, e.clientY)
      }}
      onmouseleave={() => {
        drag = null
        dropTarget = -1
        hovered = -1
        pointer = null
        updateExpanded()
        schedule()
      }}
    >
      <div class="spacer" style:height="{map.spacer}px"></div>
    </div>
    <!-- Hover only, and it never takes the pointer: the commit panel has the same list for the
         keyboard and screen readers, and the row underneath keeps working. -->
    {#each expanded ? [expanded] : [] as open (open.index)}
      <div class="ref-expand" aria-hidden="true" style:left="{open.left}px" style:top="{open.top}px" in:labelsUnfold out:labelsFold>
        {#each open.labels.slice(0, EXPANDED_REFS) as l (l.ref.name)}
          <RefChip label={l} lane={open.lane} />
        {/each}
        {#if open.labels.length > EXPANDED_REFS}
          <span class="more">and {open.labels.length - EXPANDED_REFS} more</span>
        {/if}
      </div>
    {/each}
  </div>
</div>

<style>
  .graph {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }

  .status {
    display: flex;
    align-items: center;
    /* Too narrow even with the search box at its smallest, and the search takes its own line. */
    flex-wrap: wrap;
    gap: 6px 10px;
    min-height: 40px;
    box-sizing: border-box;
    padding: 6px 12px 6px 16px;
    border-bottom: 1px solid var(--paper-edge);
  }

  /* The search box is what gives up width: the count and the buttons keep theirs. */
  .status > :global(.meta),
  .status > :global(.btn) {
    flex: none;
    white-space: nowrap;
  }

  .search {
    flex: 1 1 150px;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 6px;
    color: var(--ink-mute);
  }

  .search :global(svg),
  .search .count,
  .search .step {
    flex: none;
  }

  .search input {
    flex: 1 1 260px;
    width: 0;
    min-width: 90px;
    max-width: 260px;
    height: 28px;
    padding: 0 10px;
    font-size: 13px;
    border-radius: var(--r-sm);
  }

  .search input:focus {
    border-color: var(--link);
    outline: none;
    box-shadow: 0 0 0 3px var(--selection);
  }

  .focus {
    flex: 0 1 auto;
    display: inline-flex;
    align-items: center;
    gap: 5px;
    min-width: 0;
    padding-left: 8px;
    border-radius: var(--r-sm);
    font-size: 13px;
    color: var(--ink-soft);
    background: color-mix(in srgb, var(--accent-a) 14%, transparent);
  }

  .focus-name {
    overflow: hidden;
    max-width: 260px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .step.up {
    transform: rotate(180deg);
  }

  .step.up:hover:not(:disabled) {
    transform: rotate(180deg) translateY(1px);
  }

  .count {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    white-space: nowrap;
  }

  .error {
    color: var(--danger);
  }

  .columns {
    position: relative;
    flex: none;
    height: 28px;
    overflow: hidden;
    border-bottom: 1px solid var(--paper-edge);
    background: var(--paper-warm);
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .columns > span {
    position: absolute;
    top: 0;
    height: 100%;
    display: flex;
    align-items: center;
    gap: 4px;
    white-space: nowrap;
  }

  /* The canvas paints the visible rows; the transparent scroller above it provides the
     native scrollbar and input, with a spacer as tall as all rows. */
  .viewport {
    position: relative;
    flex: 1;
    min-height: 0;
    /* The canvas keeps its size until the next frame. If it overflowed when the column header
       arrived, an outer scrollbar would narrow the graph enough to send the header away again. */
    overflow: hidden;
  }

  canvas {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }

  .scroller {
    position: absolute;
    inset: 0;
    overflow-y: auto;
    overflow-x: hidden;
    outline: none;
  }

  .spacer {
    width: 1px;
  }

  .ref-expand {
    position: absolute;
    z-index: 2;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
    box-sizing: border-box;
    max-width: min(420px, calc(100% - 16px));
    padding: 4px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    background: var(--paper);
    box-shadow: var(--shadow);
    pointer-events: none;
  }

  .more {
    padding: 2px 7px;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }
</style>
