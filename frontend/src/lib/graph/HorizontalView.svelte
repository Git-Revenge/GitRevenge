<script lang="ts">
  // An experimental view of the graph with time running across: a column per commit, lanes as
  // rows, newest on the left. It reads the same rows as the timeline, through the timeline's
  // own cache, and hands selection back to it, so the two stay on the same commit.
  //
  // A pinned column on the left names each lane: the branch whose tip started it, even once
  // that tip has scrolled away. Lanes are reused from one branch to the next, so the names
  // are of the lanes at the view's left edge and change as it scrolls.
  import { onMount, untrack } from 'svelte'
  import { LaneNames } from '../../../wailsjs/go/app/App.js'
  import Icon from '../ui/Icon.svelte'
  import RefChip from './RefChip.svelte'
  import { dayLineLabel, formatTime, initials, laneVar, refLabels, startsRun, type RefLabel } from './draw'
  import {
    BAR_H,
    CHIP_MIN,
    chipRooms,
    clampOffset,
    COL,
    colAt,
    DAY_BAND,
    dayBandText,
    dayCount,
    daySpans,
    LANE,
    lanesAtEdge,
    laneScrollToShow,
    lanesExtent,
    laneY,
    LEAN,
    maxLaneScroll,
    namesWidth,
    offsetForThumb,
    offsetToShow,
    segPath,
    subjectLength,
    TAGS_H,
    thumbFor,
    visibleRange,
    wordsHeight,
  } from './horizontal'
  import type { Ref, Row, RowCache } from './rowCache'
  import { placeTicks, rowToY, tickAt, tickSummary, unreadFrom, type MonthMark } from './ruler'

  let {
    path,
    cache,
    tick,
    count,
    selected,
    headRow,
    remote,
    dimmed,
    ruler,
    wip,
    onSelect,
    onContext,
  }: {
    path: string
    /** The timeline's rows. It isn't reactive: `tick` says when to read it again. */
    cache: RowCache
    /** Changes whenever the rows, the labels or the checked-out commit may have. */
    tick: number
    /** How many commits the history has, or about how many while it's still being read. */
    count: number
    selected: number
    /** The checked-out commit's row, or -1. Asked for afresh at each tick, as the cache is. */
    headRow: () => number
    /** The main remote, whose prefix labels leave off. */
    remote: () => string
    /** Whether a commit fades back, as one that doesn't match the search does. */
    dimmed?: (i: number) => boolean
    /** What the timeline's ruler marks, by row: the scrollbar here carries the same. */
    ruler?: () => { loaded: number; months: MonthMark[]; tips: number[]; tags: number[]; notes: number[]; matches: number[] }
    /** The working tree, which takes the leftmost column, before the newest commit: how many
     * files have changed (null until known), whether its panel is the one showing, and how
     * to show it. Without it the first column is the newest commit. */
    wip?: { changes: number | null; active: boolean; open: () => void }
    onSelect: (i: number) => void
    onContext?: (row: Row, refs: Ref[], x: number, y: number) => void
  } = $props()

  /** The working tree's column, when there is one: every commit sits that much further right. */
  let lead = $derived(wip ? 1 : 0)
  let pad = $derived(lead * COL)

  const id = `graph-across-${Math.random().toString(36).slice(2, 8)}`
  let field = $state<HTMLDivElement>()
  let namesEl = $state<HTMLDivElement>()
  /** The whole view's width, which the pinned column takes a share of, and the plot's own. */
  let bodyW = $state(0)
  let width = $state(0)
  let height = $state(0)
  /** How far the view has scrolled from the newest commit, in px, and how far down the lanes. */
  let offset = $state(0)
  let laneScroll = $state(0)
  let namesW = $derived(namesWidth(bodyW))

  let range = $derived(visibleRange(offset - pad, width, count))
  /** The commits to draw, with what the drawing needs of each. */
  let cols = $derived.by(() => {
    void tick
    const checkedOut = headRow()
    const out: { i: number; row: Row | undefined; refs: Ref[]; starts: boolean; head: boolean }[] = []
    for (let i = range.first; i < range.last; i++) {
      const row = cache.get(i)
      const refs = row ? cache.refsFor(row.id) : []
      const head = i === checkedOut || refs.some((r) => r.head)
      out.push({ i, row, refs, head, starts: !!row && startsRun(row, { prev: cache.get(i - 1), prev2: cache.get(i - 2) }, head) })
    }
    return out
  })
  $effect(() => {
    void tick
    cache.ensure(range.first, range.last + 60)
  })

  let mainRemote = $derived.by(() => {
    void tick
    return remote()
  })

  let lanes = $derived(Math.max(1, ...cols.map((c) => c.row?.width ?? 1)))
  let words = $derived(wordsHeight(height, lanes))
  /** The room the lanes have, and how tall they are all together: past that, they scroll. */
  let lanesView = $derived(Math.max(0, height - DAY_BAND - words - TAGS_H))
  let extent = $derived(lanesExtent(lanes))
  const pitch = LANE.max
  let subjectMax = $derived(subjectLength(words))
  /** The svg's own x for commit i: it starts at the first drawn column. */
  const sx = (i: number) => (i - range.first) * COL + COL / 2

  let days = $derived.by(() => {
    void tick
    const now = Date.now() / 1000
    return daySpans(
      cols.map((c) => c.row),
      range.first,
    ).map((d) => {
      const n = dayCount((i) => cache.get(i), d.start, count)
      return { ...d, label: dayLineLabel(d.time, now), count: n === null ? '' : `${n.toLocaleString()} ${n === 1 ? 'commit' : 'commits'}` }
    })
  })

  // Which branch each lane is. A lane's colour number names it; the backend says which commit
  // started each, and the refs on it. Asked once per lane, until the graph is read afresh.
  interface LaneName {
    commit: string
    refs: Ref[]
  }
  // eslint-disable-next-line svelte/prefer-svelte-reactivity -- namesTick says when it changed
  let laneNames = new Map<number, LaneName>()
  // eslint-disable-next-line svelte/prefer-svelte-reactivity -- bookkeeping, never rendered
  let asked = new Set<number>()
  let namesEpoch = -1
  let namesTick = $state(0)

  /** The lane on each row at the view's left edge, or the first to turn up further right. */
  let edge = $derived(
    lanesAtEdge(
      cols
        // A column half off the left edge still counts as the edge.
        .filter((c) => c.i >= Math.floor((offset - pad) / COL))
        .map((c) => c.row),
    ),
  )
  $effect(() => {
    void tick
    if (cache.epoch !== namesEpoch) {
      namesEpoch = cache.epoch
      laneNames = new Map()
      asked = new Set()
    }
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- local to this run
    const want = new Set<number>()
    for (const c of cols) if (c.row) want.add(c.row.color)
    for (const e of edge.values()) want.add(e.color)
    const need = [...want].filter((c) => !asked.has(c))
    if (!need.length) return
    for (const c of need) asked.add(c)
    const epoch = namesEpoch
    // The names are a convenience: without an answer the column is blank, as for a lane no
    // branch started.
    LaneNames(path, need)
      .then((res) => {
        if (epoch !== namesEpoch) return
        for (const n of res ?? []) {
          if (n.row >= 0) laneNames.set(n.color, { commit: n.commit, refs: (n.refs ?? []) as unknown as Ref[] })
          // A lane whose start isn't read yet is asked about again when more has been.
          else setTimeout(() => asked.delete(n.color), 1500)
        }
        namesTick++
      })
      .catch(() => {})
  })

  /** What the pinned column says, lane by lane from the top. */
  let laneRows = $derived.by(() => {
    void namesTick
    return [...edge]
      .sort((a, b) => a[0] - b[0])
      .map(([lane, e]) => {
        const refs = laneNames.get(e.color)?.refs ?? []
        return { lane, color: e.color, later: e.later, labels: refs.length ? refLabels(refs, mainRemote) : [] }
      })
  })
  let above = $derived(laneRows.filter((r) => laneY(r.lane, pitch) - laneScroll < 6).length)
  let below = $derived(laneRows.filter((r) => laneY(r.lane, pitch) - laneScroll > lanesView - 6).length)

  /** The labels the pinned column doesn't say: tags, and a branch whose commit isn't the start
   * of the lane named on its row. They go in the row over the lanes, above their commit. */
  let marks = $derived.by(() => {
    void namesTick
    const out: { i: number; labels: RefLabel[]; color: number }[] = []
    for (const c of cols) {
      if (!c.row || !c.refs.length) continue
      const named = edge.get(c.row.col)?.color === c.row.color && laneNames.get(c.row.color)?.commit === c.row.id
      if (!named) out.push({ i: c.i, labels: refLabels(c.refs, mainRemote), color: c.row.color })
    }
    const rooms = chipRooms(out.map((m) => m.i))
    return out.map((m) => ({ ...m, room: rooms.get(m.i) ?? 0 }))
  })

  let chosen = $derived.by(() => {
    void tick
    return selected >= 0 ? cache.get(selected) : undefined
  })

  // Keeps the selected commit in view: when it changes, and when the view is first measured.
  // It goes on following it while the history's length or the view's width settles (End lands
  // on an estimate first), until the person scrolls somewhere themselves.
  let shown = -1
  let following = false
  $effect(() => {
    if (selected < 0 || !width) return
    if (selected !== shown) {
      shown = selected
      following = true
    }
    if (!following) return
    // In columns, so the newest commit brings the working tree's column into view with it.
    const next = offsetToShow(
      untrack(() => offset),
      width,
      selected + lead,
      count + lead,
    )
    if (next !== untrack(() => offset)) offset = next
    // Its lane comes into view too, when the lanes scroll.
    const lane = chosen?.col
    if (lane !== undefined && lanesView > 0) {
      const down = laneScrollToShow(
        untrack(() => laneScroll),
        lane,
        lanes,
        lanesView,
      )
      if (down !== untrack(() => laneScroll)) laneScroll = down
    }
  })
  // A history that got shorter, or a narrower view, can leave the offset past the end.
  $effect(() => {
    const next = clampOffset(offset, width, count + lead)
    if (width && next !== offset) offset = next
  })
  $effect(() => {
    const next = Math.min(maxLaneScroll(lanes, lanesView), Math.max(0, laneScroll))
    if (next !== laneScroll) laneScroll = next
  })

  const lines = (e: WheelEvent, d: number) => d * (e.deltaMode === 1 ? COL : 1)
  function scrollLanes(e: WheelEvent) {
    const max = maxLaneScroll(lanes, lanesView)
    if (!max || !e.deltaY) return
    e.preventDefault()
    following = false
    laneScroll = Math.min(max, Math.max(0, laneScroll + lines(e, e.deltaY)))
  }
  function onWheel(e: WheelEvent) {
    // With Shift the wheel scrolls the lanes, when there are more than fit.
    if (e.shiftKey) return scrollLanes(e)
    // Otherwise it scrolls sideways: time runs across.
    const d = Math.abs(e.deltaX) > Math.abs(e.deltaY) ? e.deltaX : e.deltaY
    if (!d) return
    e.preventDefault()
    following = false
    offset = clampOffset(offset + lines(e, d), width, count + lead)
  }

  function onKey(e: KeyboardEvent) {
    const page = Math.max(1, Math.floor(width / COL) - 1)
    const at = Math.max(0, selected)
    // Right is older, as Down is in the timeline.
    const moves: Record<string, number> = { ArrowRight: at + 1, ArrowLeft: at - 1, PageDown: at + page, PageUp: at - page, Home: 0, End: count - 1 }
    if (!(e.key in moves) || !count) return
    e.preventDefault()
    // Left of the newest commit is the working tree; from there, Right comes back.
    if (wip && e.key === 'ArrowLeft' && selected <= 0 && !wip.active) {
      offset = 0
      wip.open()
      return
    }
    if (wip?.active && e.key === 'ArrowRight') return onSelect(at)
    onSelect(Math.min(count - 1, Math.max(0, moves[e.key])))
  }

  /** The column under the pointer: a commit's row, -1 for none, or 'wip' for the working tree. */
  function columnAt(e: MouseEvent): number | 'wip' {
    if (!field) return -1
    const k = colAt(e.clientX - field.getBoundingClientRect().left, offset, count + lead)
    return k < 0 ? -1 : k < lead ? 'wip' : k - lead
  }

  /** The lane the working tree sits on: the checked-out commit's, when that's in view. */
  let wipLane = $derived.by(() => {
    void tick
    const at = headRow()
    return (at >= 0 ? cache.get(at)?.col : undefined) ?? 0
  })
  let wipJoins = $derived.by(() => {
    void tick
    return headRow() === 0
  })
  let wipText = $derived(!wip ? '' : wip.changes ? `${wip.changes} ${wip.changes === 1 ? 'file' : 'files'} changed` : wip.changes === 0 ? 'Working tree, clean' : 'Working tree')

  // The scrollbar: a thumb to drag, and a press anywhere on the track goes there.
  let track = $state<HTMLDivElement>()
  let trackW = $state(0)
  let thumb = $derived(thumbFor(offset, width, count + lead, trackW))
  let drag: { grab: number } | null = null
  // The scrollbar is a ruler, as the timeline's is: its length stands for the whole history,
  // and ticks along it show where things are among the commits read so far.
  let rule = $derived.by(() => {
    void tick
    const r = ruler?.()
    return r ? { ...r, head: headRow() } : null
  })
  let ticks = $derived(rule ? placeTicks({ head: rule.head, tips: rule.tips, tags: rule.tags, notes: rule.notes, matches: rule.matches }, count, trackW) : [])
  let unread = $derived(rule ? unreadFrom(rule.loaded, count, trackW) : null)
  /** What the tick under the pointer is. */
  let barTitle = $state('')
  function barOver(e: PointerEvent) {
    if (drag) return barTo(e.clientX)
    if (!track) return
    const t = tickAt(ticks, e.clientX - track.getBoundingClientRect().left)
    const title = t ? tickSummary(t) : ''
    if (title !== barTitle) barTitle = title
  }
  function barTo(clientX: number) {
    if (!track || !drag) return
    following = false
    offset = offsetForThumb(clientX - track.getBoundingClientRect().left - drag.grab, width, count + lead, trackW)
  }
  function barDown(e: PointerEvent) {
    if (e.button !== 0 || !track) return
    const x = e.clientX - track.getBoundingClientRect().left
    const on = x >= thumb.left && x <= thumb.left + thumb.width
    drag = { grab: on ? x - thumb.left : thumb.width / 2 }
    track.setPointerCapture(e.pointerId)
    barTo(e.clientX)
    e.preventDefault()
  }

  onMount(() => {
    // Not passive: the wheel is taken, so the page behind doesn't scroll instead.
    field?.addEventListener('wheel', onWheel, { passive: false })
    namesEl?.addEventListener('wheel', scrollLanes, { passive: false })
    return () => {
      field?.removeEventListener('wheel', onWheel)
      namesEl?.removeEventListener('wheel', scrollLanes)
    }
  })
</script>

<div class="across">
  <div class="body" bind:clientWidth={bodyW}>
    <!-- The pinned column: what each band of the plot is, and which branch each lane is. -->
    <div class="names" style:width="{namesW}px" bind:this={namesEl}>
      <div class="cell top" style:height="{DAY_BAND}px"><span class="meta">Branch</span></div>
      <div class="cell" style:height="{words}px"><span class="hand">newest on the left</span></div>
      <div class="cell" style:height="{TAGS_H}px" title="Tags, and branches that sit on another branch's lane"><span class="meta">Tags</span></div>
      <div class="lane-names" role="list" aria-label="Branches, by lane">
        {#each laneRows as r (r.lane)}
          {@const first = r.labels[0]}
          <div class="lane-name" class:later={r.later} role="listitem" style:top="{laneY(r.lane, pitch) - laneScroll - pitch / 2}px" style:height="{pitch}px">
            {#if first}
              {#if first.ref.stackParent}<span class="stacked" title="Stacked on {first.ref.stackParent}{first.ref.stackSize ? `: ${first.ref.stackDepth} of ${first.ref.stackSize}` : ''}"><Icon name="stack" size={12} /><span class="visually-hidden">Stacked on {first.ref.stackParent}</span></span>{/if}
              <span class="one" title={r.labels.map((l) => l.text).join(', ') + (r.later ? ' (starts further right)' : '')}><RefChip label={first} lane={laneVar(r.color)} /></span>
              {#if r.labels.length > 1}<span class="more">+{r.labels.length - 1}</span>{/if}
              {#if first.ref.head && namesW > 150}<span class="here">you’re here</span>{/if}
            {:else}
              <!-- A lane no branch started: a merged branch whose name is gone. -->
              <span class="nameless" style:--lane={laneVar(r.color)} title="No branch points at this lane's first commit: it was merged in, and its name is gone"></span>
            {/if}
          </div>
        {/each}
        {#if above}<span class="hint up">{above} more above</span>{/if}
        {#if below}<span class="hint down">{below} more below</span>{/if}
      </div>
    </div>

    <div
      class="plot"
      bind:this={field}
      bind:clientWidth={width}
      bind:clientHeight={height}
      tabindex="0"
      role="listbox"
      aria-orientation="horizontal"
      aria-label="Commit history, newest first"
      aria-activedescendant={wip?.active ? `${id}-wip` : selected >= range.first && selected < range.last ? `${id}-${selected}` : undefined}
      onkeydown={onKey}
      onclick={(e) => {
        const i = columnAt(e)
        if (i === 'wip') wip?.open()
        else if (i >= 0) onSelect(i)
      }}
      oncontextmenu={(e) => {
        const i = columnAt(e)
        if (i === 'wip') return
        const row = i >= 0 ? cache.get(i) : undefined
        if (!row || !onContext) return
        e.preventDefault()
        onSelect(i)
        onContext(row, cache.refsFor(row.id), e.clientX, e.clientY)
      }}
    >
      <div class="days" style:height="{DAY_BAND}px">
        {#each days as d (d.start)}
          {@const left = pad + d.start * COL - offset}
          {@const w = (d.end - d.start) * COL}
          {@const seen = Math.max(0, w + Math.min(0, left))}
          {@const says = dayBandText(d.label, d.count, d.time, seen)}
          <!-- The day's name stays at the left edge while its day scrolls under it. -->
          <div class="day" title={d.count ? `${d.label} · ${d.count}` : d.label} style:left="{Math.max(0, left)}px" style:width="{seen}px">
            <span class="meta">{says.label}</span>{#if says.count}<span class="count">{says.count}</span>{/if}
          </div>
        {/each}
      </div>
      <div class="track" style:top="{DAY_BAND}px" style:transform="translateX({-offset}px)">
        <!-- A faint line where each day ends and the one before it begins. -->
        {#each days as d (d.start)}
          {#if d.start > 0}<span class="daycut" style:left="{pad + d.start * COL}px"></span>{/if}
        {/each}
        {#if wip}
          <!-- What isn't a commit yet: always the first column, before the newest commit. -->
          <div class="col wip" class:selected={wip.active} id="{id}-wip" role="option" aria-selected={wip.active} aria-label="{wipText}. The working tree." style:left="0" style:width="{COL}px">
            <span class="subject" style:top="{words - 22}px" style:max-width="{subjectMax}px" style:transform="rotate(-{LEAN}deg)">{wipText}</span>
          </div>
        {/if}
        {#each cols as c (c.i)}
          {@const names = c.refs.length ? refLabels(c.refs, mainRemote) : []}
          <div
            class="col"
            class:selected={c.i === selected}
            class:dimmed={c.i !== selected && dimmed?.(c.i)}
            class:waiting={!c.row}
            id="{id}-{c.i}"
            role="option"
            aria-selected={c.i === selected}
            aria-label={c.row ? `${c.row.subject}, ${c.row.author}, ${formatTime(c.row.authorTime)}${names.length ? `, ${names.map((l) => l.text).join(', ')}` : ''}` : 'Loading'}
            style:left="{pad + c.i * COL}px"
            style:width="{COL}px"
          >
            {#if c.row}
              <span class="subject" style:top="{words - 22}px" style:max-width="{subjectMax}px" style:transform="rotate(-{LEAN}deg)">{c.row.subject}</span>
            {/if}
          </div>
        {/each}
        {#each marks as m (m.i)}
          <span class="mark" class:bare={m.room < CHIP_MIN} style:left="{pad + m.i * COL + 3}px" style:top="{words}px" style:height="{TAGS_H}px" style:max-width="{Math.max(COL - 6, m.room)}px" title={m.labels.map((l) => l.text).join(', ')}>
            {#if m.room < CHIP_MIN}
              <i class:diamond={m.labels[0].ref.kind === 'tag'} style:--lane={laneVar(m.color)}></i>
            {:else}
              <RefChip label={m.labels[0]} lane={laneVar(m.color)} />{#if m.labels.length > 1}<span class="more">+{m.labels.length - 1}</span>{/if}
            {/if}
          </span>
        {/each}
      </div>
      <!-- The lanes scroll both ways, inside their own window: sideways with the commits, and
           up and down when there are more than fit. -->
      <div class="lanebox" style:top="{DAY_BAND + words + TAGS_H}px">
        {#if wip}
          <!-- The working tree's node: dashed, as in the timeline, and joined to the checked-out
               commit when that's the newest. -->
          <span class="wip-node" class:dirty={!!wip.changes} class:joined={wipJoins} style:--lane={laneVar(0)} style:transform="translate({COL / 2 - offset}px, {laneY(wipLane, pitch) - laneScroll}px)"></span>
        {/if}
        <svg class="lanes" aria-hidden="true" style:transform="translate({pad + range.first * COL - offset}px, {-laneScroll}px)" width={cols.length * COL} height={extent}>
          {#each cols as c (c.i)}
            {#if c.row}
              {@const cx = sx(c.i)}
              <g class:muted={c.row.kind === 'docs'} class:dimmed={c.i !== selected && dimmed?.(c.i)}>
                {#each c.row.up as s, n (n)}<path d={segPath(s, cx, 'up', pitch)} stroke={laneVar(s.color)} />{/each}
                {#each c.row.down as s, n (n)}<path d={segPath(s, cx, 'down', pitch)} stroke={laneVar(s.color)} />{/each}
              </g>
            {/if}
          {/each}
          {#each cols as c (c.i)}
            {#if c.row}
              {@const cx = sx(c.i)}
              {@const cy = laneY(c.row.col, pitch)}
              {@const color = laneVar(c.row.color)}
              <g class:muted={c.row.kind === 'docs'} class:dimmed={c.i !== selected && dimmed?.(c.i)}>
                {#if c.head}<circle class="halo" {cx} {cy} r="16" fill={color} />{/if}
                {#if c.row.parents > 1}
                  <circle class="cut" {cx} {cy} r="9" />
                  <circle {cx} {cy} r="7.5" fill={color} />
                  <circle class="hole" {cx} {cy} r="3.5" />
                {:else if c.starts}
                  <circle {cx} {cy} r="12" fill={color} />
                  <circle class="face" {cx} {cy} r="9.75" />
                  <text x={cx} y={cy} dy="0.36em">{initials(c.row.author)}</text>
                {:else}
                  <circle class="cut" {cx} {cy} r="8" />
                  <circle {cx} {cy} r="5" fill={color} />
                {/if}
              </g>
            {/if}
          {/each}
        </svg>
      </div>
    </div>
  </div>

  <!-- A pointer's scrollbar: the keyboard moves the selection, and the view follows it. -->
  <div class="bar" bind:this={track} bind:clientWidth={trackW} style:height="{BAR_H}px" style:margin-left="{namesW}px" aria-hidden="true" title={barTitle || undefined} onpointerdown={barDown} onpointermove={barOver} onpointerup={() => (drag = null)} onpointercancel={() => (drag = null)}>
    <!-- The commits not read yet: a hatched stretch at the far end. Going there reads them. -->
    {#if unread !== null}<span class="unread" style:left="{unread}px"></span>{/if}
    {#each rule?.months ?? [] as m (m.row)}
      {#if m.row}<i class="month" style:left="{Math.round(rowToY(m.row, count, trackW))}px"></i>{/if}
    {/each}
    <!-- Each kind is a different length and place as well as a colour. -->
    {#each ticks as t (t.y)}
      <span class="tick" style:left="{Math.round(t.y)}px">
        {#if t.tips}<i class="tips"></i>{/if}
        {#if t.tags}<i class="tags"></i>{/if}
        {#if t.notes}<i class="notes"></i>{/if}
        {#if t.matches}<i class="matches"></i>{/if}
        {#if t.head}<i class="head"></i>{/if}
      </span>
    {/each}
    <span class="thumb" style:left="{thumb.left}px" style:width="{thumb.width}px"></span>
  </div>

  <div class="detail" aria-live="polite">
    {#if chosen}
      <span class="who" style:--lane={laneVar(chosen.color)}>{initials(chosen.author)}</span>
      <span class="what" title={chosen.subject}>{chosen.subject}</span>
      <span class="meta">{chosen.author} · {formatTime(chosen.authorTime)}</span>
      <span class="sha">{chosen.id.slice(0, 7)}</span>
    {:else}
      <span class="none">Choose a commit to see its subject in full.</span>
    {/if}
  </div>
</div>

<style>
  .across {
    position: absolute;
    inset: 0;
    z-index: 3;
    display: flex;
    flex-direction: column;
    background: var(--paper);
  }

  .body {
    display: flex;
    flex: 1;
    min-height: 0;
  }

  .names {
    display: flex;
    flex-direction: column;
    flex: none;
    box-sizing: border-box;
    border-right: 1px solid var(--paper-edge);
    background: var(--paper);
  }

  .cell {
    display: flex;
    align-items: flex-end;
    flex: none;
    box-sizing: border-box;
    padding: 0 10px 6px;
    overflow: hidden;
    white-space: nowrap;
  }

  .cell.top {
    align-items: center;
    padding-bottom: 0;
    background: var(--paper-warm);
    border-bottom: 1px solid var(--paper-edge);
  }

  .cell .meta {
    color: var(--ink-faint);
  }

  .cell .hand,
  .here {
    font-family: var(--font-hand);
    font-size: 18px;
    line-height: 1;
    color: var(--accent-a);
    transform: rotate(-2deg);
    white-space: nowrap;
  }

  .here {
    flex: none;
    color: var(--link);
  }

  .lane-names {
    position: relative;
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

  .lane-name {
    position: absolute;
    left: 0;
    right: 0;
    display: flex;
    align-items: center;
    gap: 5px;
    box-sizing: border-box;
    padding: 0 8px 0 10px;
    white-space: nowrap;
  }

  /* A lane that only starts further right in the view: its row is empty at the left edge. */
  .lane-name.later {
    opacity: 0.62;
  }

  .one {
    display: inline-flex;
    min-width: 0;
  }

  .one :global(.chip),
  .mark :global(.chip) {
    min-width: 0;
    min-height: 20px;
    padding-top: 1px;
    padding-bottom: 1px;
    overflow: hidden;
  }

  .one :global(.name),
  .mark :global(.name) {
    overflow: hidden;
    text-overflow: ellipsis;
    overflow-wrap: normal;
    white-space: nowrap;
  }

  /* Where else a branch is (local, origin) is in its menu and the sidebar. */
  .one :global(.places),
  .mark :global(.places) {
    display: none;
  }

  .stacked {
    display: inline-flex;
    flex: none;
    color: var(--ink-mute);
  }

  .nameless {
    width: 14px;
    border-top: 1.75px dashed var(--lane);
    opacity: 0.6;
  }

  .more {
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .hint {
    position: absolute;
    left: 0;
    right: 0;
    padding: 2px 10px;
    background: color-mix(in srgb, var(--paper) 88%, transparent);
    font-size: 12.5px;
    font-style: italic;
    color: var(--ink-mute);
  }

  .hint.up {
    top: 0;
  }

  .hint.down {
    bottom: 0;
  }

  .plot {
    position: relative;
    flex: 1;
    min-width: 0;
    overflow: hidden;
    outline: none;
    cursor: default;
  }

  /* The ring is drawn over the bands, which would otherwise cover three of its sides. */
  .plot:focus-visible::after {
    content: '';
    position: absolute;
    inset: 0;
    z-index: 4;
    outline: 2px solid var(--link);
    outline-offset: -2px;
    pointer-events: none;
  }

  .days {
    position: absolute;
    inset: 0 0 auto 0;
    z-index: 2;
    background: var(--paper-warm);
    border-bottom: 1px solid var(--paper-edge);
  }

  .day {
    position: absolute;
    top: 0;
    bottom: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    box-sizing: border-box;
    padding: 0 10px;
    border-right: 1px solid var(--paper-edge);
    overflow: hidden;
    white-space: nowrap;
  }

  .day .meta {
    color: var(--ink-faint);
  }

  .count {
    font-size: 13px;
    color: var(--ink-faint);
  }

  .track {
    position: absolute;
    bottom: 0;
    left: 0;
    will-change: transform;
  }

  .daycut {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 0;
    border-left: 1px dashed var(--hairline);
    pointer-events: none;
  }

  .col {
    position: absolute;
    top: 0;
    bottom: 0;
    box-sizing: border-box;
    border-radius: 2px;
    transition: background-color var(--dur-quick);
  }

  .col:hover {
    background: var(--wash-hover);
  }

  /* Lit from the top, thinning down the column: the same idea as the timeline's row. */
  .col.selected {
    background: linear-gradient(to bottom, var(--selected) 0, var(--selected) 38%, color-mix(in srgb, var(--selected) 34%, transparent) 100%);
  }

  .col.selected::before {
    content: '';
    position: absolute;
    top: 0;
    left: 4px;
    right: 4px;
    height: 3px;
    border-radius: 2px;
    background: var(--accent);
  }

  .subject {
    position: absolute;
    left: 20px;
    z-index: 1;
    transform-origin: 0 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-serif);
    font-size: 13px;
    line-height: 18px;
    color: var(--ink-soft);
    pointer-events: none;
  }

  .selected .subject {
    color: var(--ink);
    font-weight: 500;
  }

  .col.wip .subject {
    font-style: italic;
  }

  /* A dashed ring on the checked-out lane; filled a little when there's something to commit. */
  .wip-node {
    position: absolute;
    left: -9px;
    top: -9px;
    width: 18px;
    height: 18px;
    box-sizing: border-box;
    border: 1.5px dashed var(--lane);
    border-radius: 50%;
    background: var(--paper);
  }

  .wip-node.dirty {
    background: color-mix(in srgb, var(--lane) 22%, var(--paper));
  }

  .wip-node.joined::after {
    content: '';
    position: absolute;
    left: 100%;
    top: 50%;
    width: 16px;
    border-top: 1.5px dashed var(--lane);
  }

  .col.dimmed .subject,
  g.dimmed {
    opacity: 0.3;
  }

  .mark {
    position: absolute;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
    white-space: nowrap;
  }

  /* No room for the name: a mark of its own shape, named on hover. */
  .mark.bare {
    width: 38px;
    justify-content: center;
  }

  .mark i {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--lane);
  }

  .mark i.diamond {
    border-radius: 2px;
    background: var(--lane-4);
    transform: rotate(45deg);
  }

  .lanebox {
    position: absolute;
    inset: 0;
    overflow: hidden;
    pointer-events: none;
  }

  .lanes {
    position: absolute;
    left: 0;
    top: 0;
    overflow: visible;
  }

  .lanes path {
    fill: none;
    stroke-width: 1.75;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .lanes g.muted {
    opacity: 0.4;
  }

  .lanes .halo {
    opacity: 0.3;
  }

  .lanes .cut,
  .lanes .hole {
    fill: var(--paper);
  }

  .lanes .face {
    fill: var(--paper-deep);
  }

  .lanes text {
    text-anchor: middle;
    font-family: var(--font-mono);
    font-size: 12px;
    font-weight: 500;
    letter-spacing: -0.03em;
    fill: var(--ink-soft);
  }

  .bar {
    position: relative;
    flex: none;
    border-top: 1px solid var(--hairline);
    background: var(--paper-warm);
    touch-action: none;
  }

  .unread {
    position: absolute;
    top: 0;
    bottom: 0;
    right: 0;
    opacity: 0.28;
    background: repeating-linear-gradient(-45deg, var(--ink-mute) 0 1px, transparent 1px 5px);
  }

  .month {
    position: absolute;
    top: 1px;
    bottom: 0;
    width: 1px;
    background: var(--ink-mute);
    opacity: 0.22;
  }

  .tick {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 0;
  }

  .tick i {
    position: absolute;
    left: 0;
    width: 2px;
  }

  .tick .tips {
    top: 3px;
    height: 6px;
    background: var(--ink-soft);
  }

  .tick .tags {
    top: 3px;
    height: 9px;
    background: var(--accent-c);
  }

  .tick .notes {
    left: -1px;
    top: 9px;
    width: 4px;
    height: 4px;
    background: var(--accent-a);
  }

  .tick .matches {
    left: -1px;
    top: 5px;
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: var(--link);
  }

  .tick .head {
    left: -1px;
    top: 1px;
    bottom: 0;
    width: 4px;
    background: var(--link);
  }

  .thumb {
    position: absolute;
    top: 1px;
    bottom: 1px;
    box-sizing: border-box;
    border: 1px solid var(--ink-mute);
    border-radius: 3px;
    background: var(--wash-hover);
  }

  .detail {
    display: flex;
    align-items: center;
    gap: 14px;
    flex: none;
    min-height: 44px;
    box-sizing: border-box;
    padding: 6px 14px;
    border-top: 1px solid var(--paper-edge);
    background: var(--paper-warm);
    white-space: nowrap;
  }

  .who {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: none;
    width: 24px;
    height: 24px;
    border-radius: 50%;
    background: var(--paper-deep);
    box-shadow: 0 0 0 2px var(--lane);
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-soft);
  }

  /* When the strip is short of room the subject keeps its place: who and when give way. */
  .what {
    flex: 1 1 0;
    min-width: min(60%, 320px);
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: 15px;
    font-weight: 500;
  }

  .detail .meta,
  .sha {
    flex: none;
    color: var(--ink-mute);
  }

  .detail .meta {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .sha {
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .none {
    font-style: italic;
    color: var(--ink-mute);
  }

  @media (prefers-reduced-motion: reduce) {
    .col {
      transition: none;
    }
  }
</style>
