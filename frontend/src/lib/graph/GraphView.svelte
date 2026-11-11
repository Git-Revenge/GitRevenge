<script lang="ts">
  import { KINDS, kindFilterRows, type KindFilterRow } from '../branchKinds'
  import { errorText } from '../errors'
  import { onMount, tick, untrack, type Snippet } from 'svelte'
  import { ALONE_STEPS, searchPlaceholder, SQUEEZE, squeezeFor } from '../barSqueeze'
  import { CommitNotes, CommitRow, CommitSignatures, GraphFocus, GraphRows, NotedCommits, Refs, RepoPreferences, SearchGraph, SetGraphFocus, SetKindVisible } from '../../../wailsjs/go/app/App.js'
  import { session } from '../session.svelte'
  import { ClipboardSetText, EventsOn } from '../../../wailsjs/runtime/runtime.js'
  import { theme as colorTheme } from '../theme.svelte'
  import { provideCommands } from '../commands.svelte'
  import Icon from '../ui/Icon.svelte'
  import { DUR, easeInk, fadeUp, labelsFold, labelsUnfold, menuOut, reducedMotion, stagger } from '../ui/motion'
  import KindFilter from './KindFilter.svelte'
  import RefChip from './RefChip.svelte'
  import { bisectRow, drawRow, sealOf, graphWidth, LABEL_COLUMN, LABEL_FONT, labelColumnNeed, LANE_COUNT, laneCapacity, laneVar, laneWindow, laneX, layoutColumns, mainRemote, METRICS, noteMarkSpan, refColumnWidth, refLabels, setLaneColors, type LabelSpan, type Layout, type RefLabel, type Theme } from './draw'
  import { loadLabelWidth, nextLabelWidth, REFIT_AFTER_MS, saveLabelWidth } from './labelColumn'
  import { RowCache, type GraphProgress, type Page, type Ref, type Row } from './rowCache'
  import { scrollMap, topAfterScroll, toScroll, type ScrollMap } from './scrollMap'
  import { Washes, landWash } from './tween'
  import type { BisectMarks } from '../bisect.svelte'

  let {
    path,
    onSelect,
    onContext,
    onDrop,
    onSelectedRefs,
    onNote,
    lead,
    toolsHost,
    squeeze,
    bisect = null,
  }: {
    path: string
    /** A bisect in progress (or a rebase, which marks its commits the same way), to mark in the graph: answers given, the commits still in
     * question, the commit under test and the answer. */
    bisect?: BisectMarks | null
    /** Somewhere else to put the header's tools (count, Kinds, density, search), such as the
     * caller's own toolbar, so the two share one bar. Without one they sit above the graph. */
    toolsHost?: HTMLElement | null
    /** How far the caller's bar has given way to stay on one line (see barSqueeze). Without it
     * the graph's own bar is measured. */
    squeeze?: number
    /** A row of the caller's own above the first commit, such as the working tree. It scrolls
     * with the history, and is told where the first lane and the messages are so it can line
     * up with them. */
    lead?: Snippet<[{ nodeX: number; textX: number }]>
    /** Right-click on a row, with the refs pointing at it and the mouse position. */
    onContext?: (row: Row, refs: Ref[], x: number, y: number) => void
    /** A row dragged onto another row. */
    onDrop?: (source: Row, sourceRefs: Ref[], target: Row, targetRefs: Ref[], x: number, y: number) => void
    /** A click on a commit's note mark, after the commit is selected. */
    onNote?: (row: Row) => void
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
  /** The graph's own width, for squeezing its own bar when no caller says how far. */
  let ownWidth = $state(0)
  let level = $derived(squeeze ?? squeezeFor(ownWidth, ALONE_STEPS))
  /** Whether the search field is showing while the bar only has room for its icon. */
  let searchOpen = $state(false)
  let searchIconOnly = $derived(level >= SQUEEZE.search)

  /** Puts the caret in the search field, opening it first if it's only an icon. */
  async function focusSearch() {
    if (searchIconOnly && !searchOpen) {
      searchOpen = true
      await tick()
    }
    searchInput.focus()
    searchInput.select()
  }

  function closeSearch() {
    query = ''
    void runSearch()
    searchOpen = false
    scroller.focus()
  }
  /** Moves the header into the caller's host while there is one, and back out with the graph.
   * Svelte keeps updating the node wherever it lives; it only needs taking away by hand, since
   * it's no longer under this component's root when the component goes. */
  function hosted(node: HTMLElement, host: HTMLElement | null | undefined) {
    const home = node.parentNode
    const before = node.nextSibling
    const place = (to: HTMLElement | null | undefined) => {
      if (to) to.appendChild(node)
      else if (home && node.parentNode !== home) home.insertBefore(node, before)
      node.classList.toggle('hosted', !!to)
    }
    place(host)
    return { update: place, destroy: () => node.remove() }
  }

  /** Rows above the first commit: the caller's lead row, if it gave one. */
  let leadRows = $derived(lead ? 1 : 0)
  let map = $derived(scrollMap((span + leadRows) * rowH, viewH))
  /** Where commit i's row starts within the history's full height, and the commit at a height
   * (negative on the lead row). */
  const rowY = (i: number) => (i + leadRows) * rowH
  const rowAtY = (y: number) => Math.floor(y / rowH) - leadRows
  /** How far the lead row has scrolled, and how wide the rows are, as of the last paint. */
  let leadShift = $state(0)
  let leadWidth = $state(0)
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
    glide = null
    top = Math.max(0, Math.min(map.maxTop, t))
    placeScroller(map)
    schedule()
  }

  function onScroll() {
    // A scroll that isn't ours is the person's: it ends any glide.
    if (setScroll === null) glide = null
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

  /** The label column's width: what someone dragged it to in this repository, else what the
   * labels in view need, refitted once the graph holds still. */
  let labelChosen = $state<number | null>(loadLabelWidth(untrack(() => path), localStorage))
  let labelW: number | null = null
  /** Where the graph was at the last paint, and when it last moved. */
  let paintedTop = 0
  let movedAt = 0
  let refitTimer: ReturnType<typeof setTimeout> | undefined

  function setLabelWidth(width: number | null) {
    labelChosen = width
    saveLabelWidth(path, width, localStorage)
    schedule()
  }
  // Dragging the column's edge, or the arrow keys on it, sets its width; Home or a double-click
  // goes back to fitting the labels.
  let labelDrag: { x: number; width: number } | null = null
  function startLabelResize(e: PointerEvent) {
    if (!columns) return
    labelDrag = { x: e.clientX, width: columns.refW }
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }
  function labelResize(e: PointerEvent) {
    if (labelDrag) setLabelWidth(labelDrag.width + e.clientX - labelDrag.x)
  }
  function labelResizeKey(e: KeyboardEvent) {
    const step = e.key === 'ArrowLeft' ? -16 : e.key === 'ArrowRight' ? 16 : 0
    if (step && columns) setLabelWidth(columns.refW + step)
    else if (e.key === 'Home') setLabelWidth(null)
    else return
    e.preventDefault()
  }

  /** The row whose labels are all listed because someone asked, with the keyboard or a click on
   * the row's label button. Unlike the hover list, its labels are buttons. */
  let pinned = $state<{ index: number; top: number; labels: RefLabel[]; lane: string } | null>(null)
  let pinnedEl = $state<HTMLElement>()

  function togglePinned(index: number) {
    const row = cache.get(index)
    if (!row || pinned?.index === index) {
      pinned = null
      return
    }
    // The list belongs to the selected row, so it stays while the pointer or the focus is in it.
    select(index)
    const labels = refLabels(cache.refsFor(row.id), remote)
    const height = Math.min(labels.length, EXPANDED_REFS + 1) * CHIP_STEP + EXPAND_PAD * 2
    const place = Math.round(rowY(index) - top + (rowH - CHIP_STEP) / 2 - EXPAND_PAD)
    pinned = { index, top: Math.max(0, Math.min(place, viewH - height)), labels, lane: laneVar(row.color) }
  }
  // Opening the list moves the focus into it, so the next key is on its first label.
  $effect(() => {
    if (pinned) pinnedEl?.querySelector('button')?.focus()
  })

  /** Opens one label's own menu under it: the commit's menu, for that ref alone. */
  function labelMenu(index: number, label: RefLabel, button: HTMLElement) {
    const row = cache.get(index)
    if (!row || !onContext) return
    const r = button.getBoundingClientRect()
    onContext(row, [label.ref], r.left, r.bottom + 2)
  }

  /** Most refs an expanded label shows; the commit panel and right-click menu have them all. */
  const EXPANDED_REFS = 12
  /** The row actions card's height: three 24px buttons with 2px around them. */
  const ACTIONS_H = 28
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
    const i = pointer && rect && !drag?.active ? rowAtY(pointer.y - rect.top + top) : -1
    const span = labelSpans.get(i)
    const x = pointer && rect ? pointer.x - rect.left : -1
    const row = cache.get(i)
    if (!span || !row || x < span[0] || x > span[1]) {
      if (expanded) expanded = null
      return
    }
    const labels = refLabels(cache.refsFor(row.id), remote)
    const height = Math.min(labels.length, EXPANDED_REFS + 1) * CHIP_STEP + EXPAND_PAD * 2
    const place = Math.round(rowY(i) - top + (rowH - CHIP_STEP) / 2 - EXPAND_PAD)
    const fitted = Math.max(0, Math.min(place, viewH - height))
    if (expanded?.index === i && expanded.top === fitted) return
    expanded = { index: i, left: span[0] - EXPAND_PAD, top: fitted, labels, lane: laneVar(row.color) }
  }
  let hovered = -1
  let frame = 0
  /** Whether the grid itself, or its row actions, has the keyboard focus. */
  let gridFocused = $state(false)
  let actionsFocused = $state(false)
  /** The row actions' card, so leaving the grid for it doesn't count as leaving. */
  let actionsEl = $state<HTMLElement>()
  /** Which row shows its actions, and where: the hovered row once the pointer has rested on it
   * a moment, else the selected row while the graph has focus, so the keyboard reaches them too. */
  let actions = $state<{ index: number; top: number; labels: number } | null>(null)
  /** The row whose SHA was just copied, so its button can say so for a moment. */
  let copied = $state(-1)
  let copiedTimer: ReturnType<typeof setTimeout> | undefined
  /** The card waits this long on a hovered row, so a pointer on its way to click a row doesn't
   * find a button under it. */
  const HOVER_DWELL = 200
  let hoverReady = -1
  let hoverPending = -1
  let hoverTimer: ReturnType<typeof setTimeout> | undefined

  function updateActions() {
    let index = -1
    if (drag?.active) {
      // Nothing while dragging.
    } else if (hovered >= 0) {
      if (hovered !== hoverPending) {
        hoverPending = hovered
        hoverReady = -1
        clearTimeout(hoverTimer)
        hoverTimer = setTimeout(() => {
          hoverReady = hoverPending
          updateActions()
        }, HOVER_DWELL)
      }
      if (hovered === hoverReady) index = hovered
    } else {
      hoverPending = -1
      hoverReady = -1
      clearTimeout(hoverTimer)
      if ((gridFocused || actionsFocused) && selected >= 0) index = selected
    }
    const y = rowY(index) - top
    // A pinned list of labels belongs to the row that has the actions: it goes when they move on.
    if (pinned && (pinned.index !== index || Math.abs(top - paintedTop) > 0.5)) pinned = null
    if (index < 0 || !cache.get(index) || y + rowH <= 0 || y >= viewH) {
      if (actions) actions = null
      return
    }
    const h = Math.min(rowH, ACTIONS_H)
    const place = Math.round(y + (rowH - h) / 2)
    // How many labels the row has, for its button that lists them all.
    const labels = refLabels(cache.refsFor(cache.get(index)!.id), remote).length
    if (actions?.index === index && actions.top === place && actions.labels === labels) return
    actions = { index, top: place, labels }
  }

  async function copySha(index: number) {
    const row = cache.get(index)
    if (!row) return
    let ok: unknown
    try {
      ok = await ClipboardSetText(row.id)
    } catch {
      ok = false
    }
    if (ok === false) return
    copied = index
    clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => (copied = -1), 1500)
  }

  /** Opens the row's menu under its "more" button: the same menu as right-click. */
  function more(index: number, button: HTMLElement) {
    const row = cache.get(index)
    if (!row || !onContext) return
    select(index)
    const r = button.getBoundingClientRect()
    onContext(row, cache.refsFor(row.id), r.left, r.bottom + 2)
  }
  /** Each row's selection, hover and search-dimming strength, eased between frames. */
  const washes = new Washes()
  /** The bisect as the paint loop wants it: the range as a set. Repaints when it changes. */
  let bisectView: { terms: Record<string, string>; inQuestion: Set<string> | null; current: string; found: string; currentNote?: string; currentGlyph?: string } | null = null
  $effect(() => {
    bisectView = bisect ? { terms: bisect.terms, inQuestion: bisect.inQuestion ? new Set(bisect.inQuestion) : null, current: bisect.current, found: bisect.found, currentNote: bisect.currentNote, currentGlyph: bisect.currentGlyph } : null
    schedule()
  })
  /** A glide to a commit someone asked for, so the eye can follow it; null between glides. */
  let glide: { from: number; to: number; start: number } | null = null
  /** The row just jumped to, and when: it lands under a wash that then fades. */
  let landing: { index: number; start: number } | null = null
  /** When each commit was first painted, and last painted, so rows that arrive where someone is
   * already looking fade up. Keyed by id: a reload paints the same commits again without the
   * fade. Entries not painted for a minute are forgotten. */
  // eslint-disable-next-line svelte/prefer-svelte-reactivity
  const painted = new Map<string, { start: number; last: number }>()
  let paintedPruned = 0
  const PAINTED_KEEP_MS = 60_000
  /** How far a row that is arriving rises, in pixels, as it fades in. */
  const ARRIVE_RISE = 4
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
      faint: token('--ink-faint'),
      deep: token('--paper-deep'),
      selected: token('--selected'),
      hover: token('--wash-hover'),
      tag: token('--lane-4'),
      good: token('--success'),
      bad: token('--danger'),
      warn: token('--warning'),
      kinds: Object.fromEntries(Object.entries(KINDS).map(([k, info]) => [k, token(info.token)])),
      link: token('--link'),
      teal: token('--accent-a'),
      note: token('--accent-c'),
    }
  }

  // The commits with a git note, in any notes ref: one listing when the repository opens and
  // after every reload, since a notes ref changing reloads the graph. A plain set on purpose:
  // the canvas repaints through schedule(), not reactivity.
  let noted = new Set<string>()
  /** Notes read for the hover card, by commit, until the next listing. */
  // eslint-disable-next-line svelte/prefer-svelte-reactivity
  const noteTexts = new Map<string, { ref: string; text: string }[]>()
  async function loadNoted() {
    // The marks are decoration: without an answer there are just none.
    const ids = await NotedCommits(path).catch(() => [])
    noted = new Set(ids ?? [])
    noteTexts.clear()
    schedule()
  }

  /** The note mark under the pointer, with its commit's notes once they're read. */
  let noteCard = $state<{ index: number; id: string; top: number; right: number; notes: { ref: string; text: string }[] | null } | null>(null)

  /** Whether a point in the graph is on the row's note mark. */
  function onNoteMark(i: number, x: number): boolean {
    const row = cache.get(i)
    if (!row || !columns || !noted.has(row.id)) return false
    const [from, to] = noteMarkSpan(columns)
    return x >= from && x <= to
  }

  /** Shows the notes of the commit whose mark the pointer is on, and hides them otherwise. */
  function updateNoteCard() {
    const rect = scroller?.getBoundingClientRect()
    const i = pointer && rect && !drag?.active ? rowAtY(pointer.y - rect.top + top) : -1
    const x = pointer && rect ? pointer.x - rect.left : -1
    const row = cache.get(i)
    if (!row || !columns || !onNoteMark(i, x)) {
      if (noteCard) noteCard = null
      return
    }
    const place = Math.round(rowY(i) - top + rowH)
    if (noteCard?.id === row.id && noteCard.top === place) return
    const id = row.id
    noteCard = { index: i, id, top: place, right: Math.round(columns.width - noteMarkSpan(columns)[1]), notes: noteTexts.get(id) ?? null }
    if (noteTexts.has(id)) return
    void CommitNotes(path, id)
      .catch(() => [])
      .then((notes) => {
        noteTexts.set(id, notes ?? [])
        if (noteCard?.id === id) noteCard = { ...noteCard, notes: notes ?? [] }
      })
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
    const frameAt = performance.now()
    if (glide) {
      // On the way to a commit: ease top toward it, and keep painting until it's there.
      const p = Math.min(1, (frameAt - glide.start) / DUR.enter)
      top = glide.from + (glide.to - glide.from) * easeInk(p)
      placeScroller(map)
      if (p >= 1) glide = null
      else schedule()
    }
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

    if (Math.abs(top - paintedTop) > 0.5) movedAt = frameAt
    const first = Math.max(0, rowAtY(top))
    const last = Math.min(span, rowAtY(top + h) + 1)
    if (lead) {
      const shift = Math.round(top)
      if (shift !== leadShift) leadShift = shift
      if (w !== leadWidth) leadWidth = w
    }
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
    // The label column fits the labels in view, once the graph has held still for a moment.
    ctx.font = LABEL_FONT
    ctx.letterSpacing = '0px'
    let need = 0
    for (let i = first; i < last; i++) {
      const row = cache.get(i)
      if (row) need = Math.max(need, labelColumnNeed(ctx, cache.refsFor(row.id), remote))
    }
    const settled = !glide && frameAt - movedAt >= REFIT_AFTER_MS
    labelW = nextLabelWidth(labelW, labelChosen, need, settled)
    if (!settled) {
      clearTimeout(refitTimer)
      refitTimer = setTimeout(schedule, REFIT_AFTER_MS)
    }
    const graphW = graphWidth(widest, (w - refColumnWidth(w, labelW)) * MAX_GRAPH_SHARE, m)
    // A history wider than the column scrolls its lanes so the nodes in view stay visible.
    const sel = cache.get(selected)
    const selectedRefs = sel ? cache.refsFor(sel.id) : []
    const selectedKey = sel ? `${sel.id} ${remote} ${selectedRefs.map((r) => r.name).join(' ')}` : ''
    if (selectedKey !== lastSelectedRefs) {
      lastSelectedRefs = selectedKey
      onSelectedRefs?.(refLabels(selectedRefs, remote))
    }
    const firstLane = minCol === Infinity ? 0 : laneWindow(minCol, maxCol, sel && selected >= first && selected < last ? sel.col : null, laneCapacity(graphW, m))
    const lay = layoutColumns(w, graphW, density, firstLane, remote, labelW)
    if (!columns || columns.width !== lay.width || columns.graphW !== lay.graphW || columns.density !== lay.density || columns.refW !== lay.refW) {
      columns = lay
    }
    const th = theme()
    const now = Date.now() / 1000
    labelSpans.clear()
    const still = reducedMotion()
    let arrivals = 0
    for (let i = first; i < last; i++) {
      const row = cache.get(i)
      const y = rowY(i) - top
      if (!row) continue
      wantSignature(row.id)
      // A commit painted for the first time fades up over --dur-move, the first six of a frame
      // 40ms apart, the way every list in the app arrives.
      let seen = painted.get(row.id)
      if (!seen) {
        seen = { start: frameAt + stagger(arrivals++), last: frameAt }
        painted.set(row.id, seen)
      }
      seen.last = frameAt
      const arrive = still ? 1 : Math.min(1, Math.max(0, (frameAt - seen.start) / DUR.move))
      const arriving = arrive < 1
      if (arriving) {
        schedule()
        const e = easeInk(arrive)
        ctx.save()
        ctx.globalAlpha = e
        ctx.translate(0, (1 - e) * ARRIVE_RISE)
      }
      // The drop target is highlighted like a selection while dragging. Each strength eases
      // toward where it should be, so a selection arrives rather than appears.
      const sel = washes.value(`s${i}`, i === selected || i === dropTarget ? 1 : 0, frameAt)
      const hov = washes.value(`h${i}`, i === hovered ? 1 : 0, frameAt)
      // During a bisect a commit wears its answer, and fades once it's out of the running.
      const place = bisectView ? bisectRow(row.id, bisectView, { bad: th.bad }) : null
      const dim = washes.value(`d${i}`, (matchSet && !matchSet.has(i)) || place?.faded ? 1 : 0, frameAt)
      const extra = noted.has(row.id) ? { ...place?.extra, noted: true } : place?.extra
      const span = drawRow(ctx, row, cache.refsFor(row.id), y, lay, th, sel, hov, dim, now, sealOf(sigs.get(row.id)), { prev: cache.get(i - 1), prev2: cache.get(i - 2), next: cache.get(i + 1) }, extra)
      if (arriving) ctx.restore()
      if (span) labelSpans.set(i, span)
    }
    if (landing) {
      // The row just jumped to: a second wash over its selection, held then fading, so the eye
      // finds it and is then left alone.
      const t = (frameAt - landing.start) / (DUR.enter * 2)
      if (t >= 1) landing = null
      else {
        ctx.save()
        ctx.globalAlpha = landWash(t)
        ctx.fillStyle = th.selected
        ctx.fillRect(0, rowY(landing.index) - top, w, rowH)
        ctx.restore()
        schedule()
      }
    }
    if (frameAt - paintedPruned > PAINTED_KEEP_MS) {
      paintedPruned = frameAt
      for (const [id, p] of painted) if (frameAt - p.last > PAINTED_KEEP_MS) painted.delete(id)
    }
    washes.prune((key) => {
      const i = Number(key.slice(1))
      return i >= first - OVERSCAN && i < last + OVERSCAN
    })
    if (washes.busy(frameAt)) schedule()
    updateExpanded()
    updateNoteCard()
    updateActions()
    paintedTop = top
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
    const i = rowAtY(e.clientY - rect.top + top)
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
    // Center it rather than pinning it to an edge. Within three screens the view glides there,
    // so the eye can follow; further, it cuts. Either way the row lands under a wash that fades.
    const target = Math.max(0, Math.min(map.maxTop, rowY(i) - viewH / 2))
    const at = performance.now()
    const still = reducedMotion()
    if (!still && Math.abs(target - top) > 1 && Math.abs(target - top) <= viewH * 3) {
      glide = { from: top, to: target, start: at }
      schedule()
    } else {
      scrollTo(target)
    }
    landing = still ? null : { index: i, start: at }
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
    // Keep the selection in view, unless a glide is already on its way there.
    const y = rowY(selected)
    // The first selection can land before the first draw has measured the view.
    const height = viewH || scroller.clientHeight
    if (glide) {
      // Nothing: the glide centres it.
    } else if (y < top)
      scrollTo(selected === 0 ? 0 : y) // the first commit brings the lead row back with it
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
    void focusSearch()
  }

  function onSearchKey(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      const dir = e.shiftKey ? -1 : 1
      // Enter right after typing waits for that search rather than using the last one.
      if (searchPending) void runSearch().then(() => jump(dir))
      else jump(dir)
    } else if (e.key === 'Escape') {
      closeSearch()
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
      run: () => void focusSearch(),
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
    void loadNoted()
    const off = EventsOn('graph:progress', (p: GraphProgress) => {
      if (p.path !== path) return
      if (p.done) void loadKinds()
      if (p.done) void loadNoted()
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
      clearTimeout(copiedTimer)
      clearTimeout(hoverTimer)
      clearTimeout(refitTimer)
      cancelAnimationFrame(frame)
    }
  })
</script>

<div class="graph" bind:clientWidth={ownWidth}>
  <div class="status" class:searching={searchIconOnly && (searchOpen || !!query.trim())} use:hosted={toolsHost}>
    {#if error || rowError}
      <span class="error">Couldn't read history: {error || rowError}</span>
    {:else if level >= SQUEEZE.count}
      <!-- The count has left the bar: the search field's placeholder says it. -->
    {:else if loading && paused}
      <span class="meta" title="More history loads as you scroll, search or jump to a commit"
        >{total.toLocaleString()}{estimate > total ? ` of about ${estimate.toLocaleString()}` : ''} commits loaded</span
      >
    {:else if loading}
      <span class="meta">Loading history… {total.toLocaleString()} commits</span>
    {:else}
      <span class="meta">{total.toLocaleString()} commits</span>
    {/if}
    {#if kindRows.length}<KindFilter rows={kindRows} onToggle={setKindVisible} iconOnly={level >= SQUEEZE.network} />{/if}
    {#if level < SQUEEZE.density}
    <span class="segmented" role="group" aria-label="Row density">
      <button class="icon" aria-pressed={density !== 'compact'} aria-label="Timeline" title="Timeline: two lines per commit, with authors" onclick={() => density === 'compact' && session.run(() => colorTheme.toggleDensity())}
        ><Icon name="timeline" size={14} /></button
      >
      <button class="icon" aria-pressed={density === 'compact'} aria-label="Compact" title="Compact: one line per commit" onclick={() => density !== 'compact' && session.run(() => colorTheme.toggleDensity())}
        ><Icon name="rows" size={14} /></button
      >
    </span>
    {/if}
    {#if focus}
      <span class="focus" title="Only {focusLabel} and what it's built on are showing">
        <Icon name="focus" size={13} />{#if !searchIconOnly}<span class="focus-name">Focused on {focusLabel}</span>{/if}
        <button class="btn sm quiet icon" onclick={stopFocusing} aria-label="Stop focusing" title="Stop focusing, and show every branch again"><Icon name="close" size={13} /></button>
      </span>
    {/if}
    {#if searchIconOnly && !searchOpen && !query.trim()}
      <button class="btn sm quiet icon search-open" aria-label="Search commits" title="Search commits" aria-expanded="false" onclick={focusSearch}><Icon name="search" size={14} /></button>
    {/if}
    <span class="search" class:closed={searchIconOnly && !searchOpen && !query.trim()} class:over={searchIconOnly}>
      <Icon name="search" size={14} />
      <input
        bind:this={searchInput}
        bind:value={query}
        oninput={searchSoon}
        onkeydown={onSearchKey}
        onblur={() => {
          if (!query.trim()) searchOpen = false
        }}
        type="search"
        placeholder={searchPlaceholder(level, total, loading)}
        title="Search subjects, authors and SHAs, a trailer such as Co-authored-by: name or Closes: #12, or has:note for commits with a note"
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
      {#if searchIconOnly}
        <button class="btn sm quiet icon" onclick={closeSearch} aria-label="Close search" title="Close search (Escape)"><Icon name="close" size={13} /></button>
      {/if}
    </span>
  </div>
  <!-- Compact is a table, so its columns are named. The timeline has one column of words. -->
  {#if columns?.refW && density === 'compact'}
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
      {#if columns.dateW}<span style:left="{columns.dateX}px">When</span>{/if}
      <span style:left="{columns.shaX}px">SHA</span>
    </div>
  {/if}
  <div class="viewport">
    <canvas bind:this={canvas}></canvas>
    <!-- The caller's row above the first commit. It's real DOM, so it keeps its own semantics and
         focus, placed where row zero is and scrolled with the history. It comes before the grid,
         so Tab reaches it first. It waits for the first layout, so it never sits somewhere else
         for a frame. -->
    {#if lead && columns && leadShift < rowH}
      <div class="lead" style:top="{-leadShift}px" style:height="{rowH}px" style:width="{leadWidth}px" onwheel={(e) => scrollTo(top + e.deltaY)}>
        {@render lead({ nodeX: columns.refW + laneX(-columns.firstLane, columns.m), textX: columns.textX })}
      </div>
    {/if}
    <div
      class="scroller"
      bind:this={scroller}
      tabindex="0"
      role="grid"
      aria-label="Commit history"
      aria-rowcount={span}
      onscroll={onScroll}
      onkeydown={onKey}
      onfocus={() => {
        gridFocused = true
        updateActions()
      }}
      onblur={(e) => {
        gridFocused = false
        // Tab from the graph lands on the row's actions: they have the focus from this moment,
        // or the card would go before the focus arrived.
        if (e.relatedTarget instanceof Node && (actionsEl?.contains(e.relatedTarget) || pinnedEl?.contains(e.relatedTarget))) actionsFocused = true
        updateActions()
      }}
      onclick={(e) => {
        const i = rowAt(e)
        if (i < 0) return
        select(i)
        // A click on the note mark also asks for the note itself.
        const row = cache.get(i)
        if (row && onNoteMark(i, e.clientX - scroller.getBoundingClientRect().left)) onNote?.(row)
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
        updateNoteCard()
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
      onmouseleave={(e) => {
        drag = null
        dropTarget = -1
        pointer = null
        // Moving onto the row's actions keeps the row hovered, so the card stays put.
        if (!(e.relatedTarget instanceof Node && actionsEl?.contains(e.relatedTarget))) hovered = -1
        updateExpanded()
        updateNoteCard()
        schedule()
      }}
    >
      <div class="spacer" style:height="{map.spacer}px"></div>
    </div>
    <!-- The hovered row's actions, or the selected row's while the graph has focus: copy the
         SHA, and the same menu right-click opens. Nothing here changes the repository on one
         click, since the card sits where a click can land. Tab reaches them after the grid. -->
    {#if actions}
      {@const row = cache.get(actions.index)}
      {#if row}
        {@const short = row.id.slice(0, 7)}
        {@const i = actions.index}
        {@const labelCount = actions.labels}
        <div
          class="row-actions"
          role="group"
          aria-label="Actions for {short}"
          bind:this={actionsEl}
          style:top="{actions.top}px"
          in:fadeUp
          out:menuOut
          onfocusin={() => (actionsFocused = true)}
          onfocusout={(e) => {
            actionsFocused = !!(e.relatedTarget instanceof Node && (actionsEl?.contains(e.relatedTarget) || pinnedEl?.contains(e.relatedTarget)))
            updateActions()
          }}
          onmouseleave={(e) => {
            if (e.relatedTarget instanceof Node && scroller.contains(e.relatedTarget)) return
            hovered = -1
            schedule()
          }}
        >
          {#if labelCount > 1}
            <button
              class="btn sm quiet icon"
              aria-label="All {labelCount} branches and tags on {short}"
              title="Every branch and tag on this commit"
              aria-expanded={pinned?.index === i}
              onclick={() => togglePinned(i)}
            >
              <Icon name="tag" size={14} />
            </button>
          {/if}
          <button class="btn sm quiet icon" aria-label={copied === i ? `Copied ${short}` : `Copy ${short}`} title={copied === i ? 'Copied' : 'Copy the full SHA'} onclick={() => copySha(i)}>
            <Icon name={copied === i ? 'check' : 'copy'} size={14} />
          </button>
          <button class="btn sm quiet icon" aria-label="Menu for {short}" title="Everything you can do here, as on right-click" aria-haspopup="menu" onclick={(e) => more(i, e.currentTarget)}>
            <Icon name="more" size={14} />
          </button>
        </div>
      {/if}
    {/if}
    <!-- Hover only, and it never takes the pointer: the commit panel has the same list for the
         keyboard and screen readers, and the row underneath keeps working. -->
    <!-- The notes of the commit whose mark the pointer is on. Hover only, and it never takes the
         pointer: the commit panel has the notes for the keyboard and screen readers, and a click on
         the mark goes there. -->
    {#if noteCard}
      <div class="note-card" aria-hidden="true" style:top="{Math.min(noteCard.top, Math.max(0, viewH - 120))}px" style:right="{noteCard.right}px" in:fadeUp>
        {#if noteCard.notes === null}
          <p class="quiet">Reading the note…</p>
        {:else if noteCard.notes.length === 0}
          <p class="quiet">The note is gone.</p>
        {:else}
          {#each noteCard.notes as n (n.ref)}
            <div class="one">
              {#if n.ref !== 'refs/notes/commits'}<span class="ref">{n.ref.replace(/^refs\/notes\//, '')}</span>{/if}
              <p>{n.text}</p>
            </div>
          {/each}
        {/if}
      </div>
    {/if}
    <!-- The same list, asked for from the row's label button: here each label is a button that
         opens its own menu, and Escape closes the list. Tab reaches it after the row's actions. -->
    {#if pinned}
      {@const at = pinned.index}
      <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
      <div
        class="ref-expand pinned"
        role="group"
        aria-label="Branches and tags on this commit"
        bind:this={pinnedEl}
        style:left="4px"
        style:top="{pinned.top}px"
        in:labelsUnfold
        out:labelsFold
        onfocusin={() => (actionsFocused = true)}
        onfocusout={(e) => {
          actionsFocused = !!(e.relatedTarget instanceof Node && (actionsEl?.contains(e.relatedTarget) || pinnedEl?.contains(e.relatedTarget)))
          updateActions()
        }}
        onkeydown={(e) => {
          if (e.key !== 'Escape') return
          e.stopPropagation()
          pinned = null
          scroller.focus()
        }}
      >
        {#each pinned.labels.slice(0, EXPANDED_REFS) as l (l.ref.name)}
          <button class="label-button" aria-haspopup="menu" aria-label="Menu for {l.text}" onclick={(e) => labelMenu(at, l, e.currentTarget)}><RefChip label={l} lane={pinned.lane} /></button>
        {/each}
        {#if pinned.labels.length > EXPANDED_REFS}
          <span class="more">and {pinned.labels.length - EXPANDED_REFS} more</span>
        {/if}
      </div>
    {/if}
    {#if columns?.refW}
      <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
      <div
        class="label-edge"
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize the branch and tag column"
        aria-valuenow={columns.refW}
        aria-valuemin={LABEL_COLUMN.min}
        aria-valuemax={LABEL_COLUMN.max}
        tabindex="0"
        title="Drag to resize. Double-click to fit the labels again."
        style:left="{columns.refW - 3}px"
        onpointerdown={startLabelResize}
        onpointermove={labelResize}
        onpointerup={() => (labelDrag = null)}
        ondblclick={() => setLabelWidth(null)}
        onkeydown={labelResizeKey}
        onwheel={(e) => scrollTo(top + e.deltaY)}
      ></div>
    {/if}
    {#each expanded && expanded.index !== pinned?.index ? [expanded] : [] as open (open.index)}
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
    /* One line at every width: the bar gives way in steps instead (see barSqueeze). */
    position: relative;
    gap: 6px 10px;
    min-height: 40px;
    box-sizing: border-box;
    padding: 6px 12px 6px 16px;
    border-bottom: 1px solid var(--paper-edge);
  }

  /* In the caller's bar: no rule or room of its own, since the bar has both. */
  .status:global(.hosted) {
    flex: 1;
    min-width: 0;
    min-height: 0;
    padding: 4px 4px 4px 12px;
    border-bottom: none;
    /* An open search covers the caller's whole bar, not only this share of it. */
    position: static;
  }

  /* A long count, such as a history still loading, is cut before it pushes anything out. */
  .status > :global(.meta) {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* The search box is what gives up width: the buttons keep theirs. */
  .status > :global(.btn),
  .status > .segmented {
    flex: none;
    white-space: nowrap;
  }

  /* Only an icon's worth of room: the field waits behind its button, and opens over the bar. */
  .search.closed {
    display: none;
  }

  .search-open {
    margin-left: auto;
  }

  .status.searching > :global(*:not(.search)) {
    visibility: hidden;
  }

  .search.over {
    position: absolute;
    inset: 0;
    z-index: 2;
    padding: 0 8px 0 12px;
    background: var(--paper);
  }

  .search.over input {
    max-width: none;
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

  .lead {
    position: absolute;
    z-index: 1;
    left: 0;
    display: flex;
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

  .ref-expand.pinned {
    pointer-events: auto;
  }

  .note-card {
    position: absolute;
    z-index: 2;
    display: flex;
    flex-direction: column;
    gap: 8px;
    box-sizing: border-box;
    width: max-content;
    max-width: min(360px, calc(100% - 16px));
    padding: 8px 10px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    background: var(--paper);
    box-shadow: var(--shadow);
    pointer-events: none;
    font-size: 13px;
    line-height: 1.45;
  }

  .note-card p {
    margin: 0;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    /* The first lines say what the note is; the commit panel has all of it. */
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 5;
    line-clamp: 5;
    overflow: hidden;
  }

  .note-card .quiet {
    color: var(--ink-mute);
    font-style: italic;
  }

  .note-card .ref {
    display: block;
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .label-button {
    display: flex;
    max-width: 100%;
    padding: 0;
    border: 0;
    border-radius: var(--r-sm);
    background: none;
    cursor: pointer;
  }

  /* The edge between the labels and the graph: drag it to resize the label column. */
  .label-edge {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 6px;
    z-index: 1;
    cursor: col-resize;
    touch-action: none;
    transition: background-color var(--dur-quick);
  }

  .label-edge:hover,
  .label-edge:focus-visible {
    background: var(--link);
    outline: none;
  }

  @media (prefers-reduced-motion: reduce) {
    .label-edge {
      transition: none;
    }
  }

  .more {
    padding: 2px 7px;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  /* On paper over the SHA column, with a hairline, so it reads as a thing on the row rather than
     part of it. Short enough for a compact row. */
  .row-actions {
    position: absolute;
    z-index: 2;
    right: 8px;
    display: flex;
    gap: 2px;
    padding: 2px;
    border-radius: var(--r-md);
    background: var(--paper);
    box-shadow: 0 0 0 1px var(--paper-edge);
  }

  .row-actions .btn {
    width: 24px;
    height: 24px;
  }
</style>
