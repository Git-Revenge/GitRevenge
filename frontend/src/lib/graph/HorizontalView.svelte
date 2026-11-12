<script lang="ts">
  // An experimental view of the graph with time running across: a column per commit, lanes as
  // rows, newest on the left. It reads the same rows as the timeline, through the timeline's
  // own cache, and hands selection back to it, so the two stay on the same commit.
  import { onMount, untrack } from 'svelte'
  import RefChip from './RefChip.svelte'
  import { dayLineLabel, formatTime, initials, laneVar, refLabels, startsRun } from './draw'
  import { BAR_H, clampOffset, COL, colAt, DAY_BAND, dayBandText, dayCount, daySpans, LABEL_ROW, labelPlaces, LABELS_H, lanePitch, laneY, LEAN, offsetForThumb, offsetToShow, segPath, subjectLength, thumbFor, visibleRange, wordsHeight } from './horizontal'
  import type { Ref, Row, RowCache } from './rowCache'

  let {
    cache,
    tick,
    count,
    selected,
    headRow,
    remote,
    dimmed,
    onSelect,
    onContext,
  }: {
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
    onSelect: (i: number) => void
    onContext?: (row: Row, refs: Ref[], x: number, y: number) => void
  } = $props()

  const id = `graph-across-${Math.random().toString(36).slice(2, 8)}`
  let field = $state<HTMLDivElement>()
  let width = $state(0)
  let height = $state(0)
  /** How far the view has scrolled from the newest commit, in px. */
  let offset = $state(0)

  let range = $derived(visibleRange(offset, width, count))
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
  let lanesH = $derived(Math.max(0, height - DAY_BAND - words - LABELS_H))
  let pitch = $derived(lanePitch(lanes, lanesH))
  /** A commit that starts a run shows its author's initials when the lanes leave room. */
  let roomy = $derived(pitch >= 24)
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

  /** Which row each labelled commit's labels sit on, and how wide they can be. */
  let places = $derived(labelPlaces(cols.filter((c) => c.refs.length).map((c) => c.i)))

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
    const next = offsetToShow(
      untrack(() => offset),
      width,
      selected,
      count,
    )
    if (next !== untrack(() => offset)) offset = next
  })
  // A history that got shorter, or a narrower view, can leave the offset past the end.
  $effect(() => {
    const next = clampOffset(offset, width, count)
    if (width && next !== offset) offset = next
  })

  function onWheel(e: WheelEvent) {
    // The wheel scrolls sideways here: there's nowhere to go up or down.
    const d = Math.abs(e.deltaX) > Math.abs(e.deltaY) ? e.deltaX : e.deltaY
    if (!d) return
    e.preventDefault()
    following = false
    offset = clampOffset(offset + d * (e.deltaMode === 1 ? COL : 1), width, count)
  }

  function onKey(e: KeyboardEvent) {
    const page = Math.max(1, Math.floor(width / COL) - 1)
    const at = Math.max(0, selected)
    // Right is older, as Down is in the timeline.
    const moves: Record<string, number> = { ArrowRight: at + 1, ArrowLeft: at - 1, PageDown: at + page, PageUp: at - page, Home: 0, End: count - 1 }
    if (!(e.key in moves) || !count) return
    e.preventDefault()
    onSelect(Math.min(count - 1, Math.max(0, moves[e.key])))
  }

  const columnAt = (e: MouseEvent) => (field ? colAt(e.clientX - field.getBoundingClientRect().left, offset, count) : -1)

  // The scrollbar: a thumb to drag, and a press anywhere on the track goes there.
  let track = $state<HTMLDivElement>()
  let trackW = $state(0)
  let thumb = $derived(thumbFor(offset, width, count, trackW))
  let drag: { grab: number } | null = null
  function barTo(clientX: number) {
    if (!track || !drag) return
    following = false
    offset = offsetForThumb(clientX - track.getBoundingClientRect().left - drag.grab, width, count, trackW)
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
    return () => field?.removeEventListener('wheel', onWheel)
  })
</script>

<div class="across">
  <div
    class="plot"
    bind:this={field}
    bind:clientWidth={width}
    bind:clientHeight={height}
    tabindex="0"
    role="listbox"
    aria-orientation="horizontal"
    aria-label="Commit history, newest first"
    aria-activedescendant={selected >= range.first && selected < range.last ? `${id}-${selected}` : undefined}
    onkeydown={onKey}
    onclick={(e) => {
      const i = columnAt(e)
      if (i >= 0) onSelect(i)
    }}
    oncontextmenu={(e) => {
      const i = columnAt(e)
      const row = i >= 0 ? cache.get(i) : undefined
      if (!row || !onContext) return
      e.preventDefault()
      onSelect(i)
      onContext(row, cache.refsFor(row.id), e.clientX, e.clientY)
    }}
  >
    <div class="days" style:height="{DAY_BAND}px">
      {#each days as d (d.start)}
        {@const left = d.start * COL - offset}
        {@const w = (d.end - d.start) * COL}
        <!-- The day's name stays at the left edge while its day scrolls under it. -->
        {@const seen = Math.max(0, w + Math.min(0, left))}
        {@const says = dayBandText(d.label, d.count, d.time, seen)}
        <div class="day" title={d.count ? `${d.label} · ${d.count}` : d.label} style:left="{Math.max(0, left)}px" style:width="{seen}px">
          <span class="meta">{says.label}</span>{#if says.count}<span class="count">{says.count}</span>{/if}
        </div>
      {/each}
    </div>
    <div class="track" style:top="{DAY_BAND}px" style:transform="translateX({-offset}px)">
      <!-- A faint line where each day ends and the one before it begins. -->
      {#each days as d (d.start)}
        {#if d.start > 0}<span class="daycut" style:left="{d.start * COL}px"></span>{/if}
      {/each}
      {#each cols as c (c.i)}
        {@const labels = c.refs.length ? refLabels(c.refs, mainRemote) : []}
        <div
          class="col"
          class:selected={c.i === selected}
          class:dimmed={c.i !== selected && dimmed?.(c.i)}
          class:waiting={!c.row}
          id="{id}-{c.i}"
          role="option"
          aria-selected={c.i === selected}
          aria-label={c.row ? `${c.row.subject}, ${c.row.author}, ${formatTime(c.row.authorTime)}${labels.length ? `, ${labels.map((l) => l.text).join(', ')}` : ''}` : 'Loading'}
          style:left="{c.i * COL}px"
          style:width="{COL}px"
        >
          {#if c.row}
            <span class="subject" style:top="{words - 22}px" style:max-width="{subjectMax}px" style:transform="rotate(-{LEAN}deg)">{c.row.subject}</span>
            {@const place = places.get(c.i)}
            {#if labels.length && place}
              <span class="labels" style:top="{words + 2 + place.row * LABEL_ROW}px" style:height="{LABEL_ROW}px" style:max-width="{place.room}px">
                <RefChip label={labels[0]} lane={laneVar(c.row.color)} />{#if labels.length > 1}<span class="more" title={labels.slice(1).map((l) => l.text).join(', ')}>+{labels.length - 1}</span>{/if}
              </span>
            {/if}
          {/if}
        </div>
      {/each}
      <svg class="lanes" aria-hidden="true" style:left="{range.first * COL}px" style:top="{words + LABELS_H}px" width={cols.length * COL} height={lanesH}>
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
              {#if c.head}<circle class="halo" {cx} {cy} r={roomy ? 16 : 9} fill={color} />{/if}
              {#if c.row.parents > 1}
                <circle class="cut" {cx} {cy} r="9" />
                <circle {cx} {cy} r="7.5" fill={color} />
                <circle class="hole" {cx} {cy} r="3.5" />
              {:else if c.starts && roomy}
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

  <!-- A pointer's scrollbar: the keyboard moves the selection, and the view follows it. -->
  <div class="bar" bind:this={track} bind:clientWidth={trackW} style:height="{BAR_H}px" aria-hidden="true" onpointerdown={barDown} onpointermove={(e) => barTo(e.clientX)} onpointerup={() => (drag = null)} onpointercancel={() => (drag = null)}>
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

  .plot {
    position: relative;
    flex: 1;
    min-height: 0;
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

  .col.selected {
    background: var(--selected);
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

  .col.dimmed .subject,
  .col.dimmed .labels,
  g.dimmed {
    opacity: 0.3;
  }

  .labels {
    position: absolute;
    left: 3px;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
    white-space: nowrap;
  }

  .labels :global(.chip) {
    min-width: 0;
    overflow: hidden;
  }

  .labels :global(.name) {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Where else a branch is (local, origin) is in its menu and the sidebar: a column is too
     narrow to say it here. */
  .labels :global(.places) {
    display: none;
  }

  .more {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .lanes {
    position: absolute;
    overflow: visible;
    pointer-events: none;
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
