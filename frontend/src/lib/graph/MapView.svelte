<script lang="ts">
  // An experimental view with no commits in it: a node for each branch and tag, held around
  // the trunk. Size is commits, distance is how long since it moved. It's a way in to the
  // timeline, not a place to work: a node's card opens the timeline on it.
  import { BranchMap } from '../../../wailsjs/go/app/App.js'
  import { kindInfo } from '../branchKinds'
  import { errorText } from '../errors'
  import Loading from '../ui/Loading.svelte'
  import { laneVar, relativeTime } from './draw'
  import { DEFAULT_SCALE, keyShows, layout, nameWaits, SCALES, zoomBy, type MapNode, type Placed } from './mapLayout'

  let {
    path,
    tick,
    onShow,
    onMenu,
    onSummary,
  }: {
    path: string
    /** Changes when the branches or tags may have: after a reload of the graph. */
    tick: number
    /** Opens the timeline on a node: at its commit, or with `focus` narrowed to its branch. */
    onShow: (node: MapNode, focus: boolean) => void
    /** A right-click on a branch or tag, for its menu. */
    onMenu?: (node: MapNode, x: number, y: number) => void
    /** What the map holds, for the bar: "8 branches · 13 tags". */
    onSummary?: (text: string) => void
  } = $props()

  interface Data {
    trunk: MapNode
    nodes: MapNode[]
    branches: number
    tags: number
    moreBranches: number
  }
  let data = $state<Data | null>(null)
  let error = $state('')
  let width = $state(0)
  let height = $state(0)
  /** The node whose card shows, by kind and name: a branch and a tag can share a name. */
  let picked = $state('')
  const keyOf = (n: Pick<MapNode, 'kind' | 'name'>) => `${n.kind}:${n.name}`

  const plural = (n: number, one: string, many = one + 's') => `${n.toLocaleString()} ${n === 1 ? one : many}`

  let seq = 0
  $effect(() => {
    void tick
    const mine = ++seq
    BranchMap(path)
      .then((res) => {
        if (mine !== seq) return
        data = res as unknown as Data
        error = ''
        onSummary?.(`${plural(data.branches, 'branch', 'branches')} · ${plural(data.tags, 'tag')}`)
        if (picked && ![data.trunk, ...data.nodes].some((n) => keyOf(n) === picked)) picked = ''
      })
      .catch((e: unknown) => {
        if (mine !== seq) return
        error = errorText(e)
      })
  })

  let now = $derived.by(() => {
    void data
    return Date.now() / 1000
  })
  // Zoom: what the rings mean. All the way in, the map is one week and each ring a day; out,
  // the rings become weeks, then months, then years. The level is kept between visits.
  const ZOOM_KEY = 'graph.map.zoom'
  function savedZoom(): number {
    try {
      const n = Number(localStorage.getItem(ZOOM_KEY) ?? DEFAULT_SCALE)
      return Number.isInteger(n) ? zoomBy(n, 0) : DEFAULT_SCALE
    } catch {
      return DEFAULT_SCALE
    }
  }
  let zoom = $state(savedZoom())
  let scale = $derived(SCALES[zoom])
  function zoomTo(level: number) {
    if (level === zoom) return
    zoom = level
    try {
      localStorage.setItem(ZOOM_KEY, String(level))
    } catch {
      // The level just isn't kept.
    }
  }
  /** The wheel zooms a step for each notch: there's nothing here to scroll. */
  let wheeled = 0
  function onWheel(e: WheelEvent) {
    if (!e.deltaY) return
    e.preventDefault()
    wheeled += e.deltaMode === 1 ? e.deltaY * 40 : e.deltaY
    if (Math.abs(wheeled) < 80) return
    zoomTo(zoomBy(zoom, wheeled > 0 ? 1 : -1))
    wheeled = 0
  }
  $effect(() => {
    // Not passive: the wheel is taken.
    stage?.addEventListener('wheel', onWheel, { passive: false })
    return () => stage?.removeEventListener('wheel', onWheel)
  })

  let placed = $derived(data && data.trunk.name && width > 0 && height > 0 ? layout(data.trunk, data.nodes, width, height, now, scale) : null)
  let crowd = $derived(placed ? placed.nodes.length : 0)
  /** Tab order: the trunk, then the rest, newest first. */
  let ordered = $derived(placed ? [...placed.nodes].sort((a, b) => b.updated - a.updated || a.name.localeCompare(b.name)) : [])
  /** Where each line starts: the branch a tag hangs off, or the trunk. */
  const holderOf = (n: Placed) => (n.parent && placed?.nodes.find((p) => p.name === n.parent && (p.kind === 'branch' || p.kind === 'remote'))) || placed?.trunk
  /** The oldest tag has no tag before it to count from. */
  let firstTag = $derived.by(() => {
    if (!data || data.nodes.some((n) => n.kind === 'tags')) return ''
    const tags = data.nodes.filter((n) => n.kind === 'tag')
    return tags.length ? tags.reduce((a, b) => (b.updated < a.updated ? b : a)).name : ''
  })

  /** A branch wears its kind's colour, or else one of the lanes', the same one each time. */
  function colorOf(n: MapNode): string {
    if (n.kind === 'trunk') return laneVar(0)
    if (n.kind === 'tag' || n.kind === 'tags') return 'var(--accent-c)'
    const kind = kindInfo(n.branchKind)
    if (kind) return `var(${kind.token})`
    let h = 0
    for (const c of n.name) h = (h * 31 + c.charCodeAt(0)) >>> 0
    return laneVar(1 + (h % 5))
  }

  function sizeLine(n: MapNode): string {
    if (n.kind === 'tags' || n.kind === 'trunk') return plural(n.commits, 'commit')
    if (n.kind === 'tag') return plural(n.commits, 'commit')
    return n.commits ? plural(n.commits, 'commit') : `all in ${data?.trunk.name ?? 'the trunk'}`
  }

  /** What the card says of a node, and a screen reader hears of it. */
  function about(n: MapNode): string {
    const trunk = data?.trunk.name ?? 'the trunk'
    const when = relativeTime(n.updated, now)
    const moved = `Last moved ${when === 'now' ? 'just now' : when}.`
    if (n.kind === 'trunk') return `${plural(n.commits, 'commit')}. ${moved}`
    if (n.kind === 'tags') return `${plural(n.tags ?? 0, 'tag')} older than the ones shown, with ${plural(n.commits, 'commit')} up to the newest of them.`
    if (n.kind === 'tag') return `${plural(n.commits, 'commit')} ${n.name === firstTag ? 'up to it' : 'since the tag before it'}${n.parent ? `, on ${n.parent}` : ''}. ${moved}`
    const where = n.kind === 'remote' ? 'Only on the remote. ' : ''
    const own = n.commits ? `${plural(n.commits, 'commit')} that ${n.commits === 1 ? "isn't" : "aren't"} in ${trunk}.` : `Nothing ${trunk} doesn't have: it's merged, or hasn't started.`
    return `${where}${own} ${moved}`
  }

  const shapeOf = (n: MapNode) => (n.kind === 'tag' || n.kind === 'tags' ? 'tag' : n.kind === 'remote' ? 'remote' : n.kind === 'branch' && !n.commits ? 'merged' : n.kind)

  /** The card sits beside its node, on whichever side has the room. */
  function cardPlace(n: Placed): string {
    const w = 232
    const left = n.x + n.r + 14 + w < width ? n.x + n.r + 14 : Math.max(8, n.x - n.r - 14 - w)
    const top = Math.min(Math.max(8, n.y - 30), Math.max(8, height - 150))
    return `left: ${left}px; top: ${top}px; width: ${w}px;`
  }

  function onKey(e: KeyboardEvent) {
    // Plus and minus zoom, as they do a map.
    if (e.key === '+' || e.key === '=' || e.key === '-') {
      e.preventDefault()
      zoomTo(zoomBy(zoom, e.key === '-' ? 1 : -1))
      return
    }
    if (e.key !== 'Escape' || !picked) return
    // Only the card closes, not the screen behind it.
    e.stopPropagation()
    const was = picked
    picked = ''
    stage?.querySelector<HTMLButtonElement>(`[data-node="${CSS.escape(was)}"]`)?.focus()
  }
  let stage = $state<HTMLDivElement>()
</script>

{#snippet nodeButton(n: Placed)}
  {@const key = keyOf(n)}
  <button
    class="node is-{shapeOf(n)}"
    class:picked={picked === key}
    class:quiet={n.kind !== 'trunk' && nameWaits(n, crowd)}
    data-node={key}
    aria-pressed={picked === key}
    aria-label="{n.name}{n.head ? ', checked out' : ''}. {about(n)}"
    style:left="{n.x}px"
    style:top="{n.y}px"
    style:--c={colorOf(n)}
    style:--size="{n.size}px"
    style:--reach="{n.r}px"
    onclick={() => (picked = picked === key ? '' : key)}
    oncontextmenu={(e) => {
      if (!onMenu || n.kind === 'tags') return
      e.preventDefault()
      onMenu(n, e.clientX, e.clientY)
    }}
  >
    <span class="shape">
      {#if n.kind === 'trunk'}
        <b>{n.name}</b><span>{plural(n.commits, 'commit')}</span>{#if n.head}<i>you’re here</i>{/if}
      {/if}
    </span>
    {#if n.kind !== 'trunk'}
      <span class="name" class:above={n.above}><b>{n.name}</b><small>{sizeLine(n)}</small>{#if n.head}<i>you’re here</i>{/if}</span>
    {/if}
  </button>
  {#if picked === key}
    <div class="card" style={cardPlace(n)} role="group" aria-label="About {n.name}">
      <b>{n.name}</b>
      <span>{about(n)}</span>
      <span class="acts">
        <button class="btn sm text" onclick={() => onShow(n, false)}>Show it in the timeline</button>
        {#if n.kind === 'branch'}<button class="btn sm text" onclick={() => onShow(n, true)}>Focus the timeline on it</button>{/if}
      </span>
    </div>
  {/if}
{/snippet}

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="map" bind:this={stage} bind:clientWidth={width} bind:clientHeight={height} onkeydown={onKey}>
  {#if error}
    <p class="note" role="alert">Couldn't read the branches and tags: {error}</p>
  {:else if !data}
    <Loading />
  {:else if !data.trunk.name}
    <p class="note">No branches yet. Make a first commit and the map has something to draw.</p>
  {:else if placed}
    <svg class="under" aria-hidden="true" {width} {height}>
      {#each placed.rings.marks as ring (ring.days)}
        <circle class="ring" cx={placed.rings.cx} cy={placed.rings.cy} r={ring.r} />
      {/each}
      {#each placed.nodes as n (keyOf(n))}
        {@const from = holderOf(n) ?? placed.trunk}
        <line class:tagline={n.kind === 'tag' || n.kind === 'tags'} x1={from.x} y1={from.y} x2={n.x} y2={n.y} stroke={colorOf(n)} />
      {/each}
    </svg>
    <!-- A narrow panel has no room for the rings' names or the key: the nodes come first. -->
    {#if keyShows(width, height)}
    <!-- Each ring's name sits on it, up and to the right; what's past the last is "older". -->
    {#each placed.rings.marks as ring (ring.days)}
      <span class="ringname" style:left="{placed.rings.cx + ring.r * 0.64 + 4}px" style:top="{Math.max(2, placed.rings.cy - ring.r * 0.77 - 20)}px">{ring.name}</span>
    {/each}
    <span class="ringname" style:left="{Math.min(width - 60, placed.rings.cx + placed.rings.marks[placed.rings.marks.length - 1].r + 34)}px" style:top="30px">older</span>
    {/if}

    <div class="nodes" role="group" aria-label="Branches and tags around {data.trunk.name}">
      {@render nodeButton(placed.trunk)}
      {#each ordered as n (keyOf(n))}
        {@render nodeButton(n)}
      {/each}
    </div>

    {#if keyShows(width, height)}
    <div class="zoom" role="group" aria-label="Zoom">
      <button class="btn sm quiet icon" aria-label="Zoom in" title="Zoom in: the rings mark less time (+)" disabled={zoom === 0} onclick={() => zoomTo(zoomBy(zoom, -1))}>+</button>
      <span class="span" aria-live="polite">{scale.name}</span>
      <button class="btn sm quiet icon" aria-label="Zoom out" title="Zoom out: the rings mark more time (-)" disabled={zoom === SCALES.length - 1} onclick={() => zoomTo(zoomBy(zoom, 1))}>−</button>
    </div>

    <div class="legend">
      <div><i class="k-branch"></i>Local branch</div>
      <div><i class="k-remote"></i>Only on the remote</div>
      <div><i class="k-merged"></i>Nothing of its own left</div>
      <div><i class="k-tag"></i>Tag</div>
      {#if data.moreBranches}<div class="more">{plural(data.moreBranches, 'older branch', 'older branches')} not shown</div>{/if}
    </div>
    {/if}
  {/if}
</div>

<style>
  .map {
    position: absolute;
    inset: 0;
    z-index: 3;
    overflow: hidden;
    background: var(--paper);
  }

  .note {
    margin: 0;
    padding: 28px 24px;
    color: var(--ink-mute);
    font-style: italic;
  }

  .under {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }

  .ring {
    fill: none;
    stroke: var(--paper-edge);
    stroke-dasharray: 4 5;
  }

  line {
    stroke-width: 1.75;
    stroke-opacity: 0.5;
  }

  line.tagline {
    stroke-width: 1.25;
    stroke-dasharray: 4 4;
  }

  .ringname {
    position: absolute;
    font-family: var(--font-hand);
    font-size: 19px;
    line-height: 1;
    color: var(--ink-faint);
    transform: rotate(-2deg);
    pointer-events: none;
    white-space: nowrap;
  }

  /* The button is the press target: as big as the shape, and never smaller than 28px. The
     shape and the name are placed from its centre. */
  .node {
    position: absolute;
    width: max(28px, calc(var(--reach) * 2));
    height: max(28px, calc(var(--reach) * 2));
    padding: 0;
    border: none;
    border-radius: 50%;
    background: none;
    color: var(--ink);
    transform: translate(-50%, -50%);
    cursor: pointer;
    outline: none;
  }

  .node:hover,
  .node:focus-visible,
  .node.picked {
    z-index: 2;
  }

  .shape {
    position: absolute;
    left: 50%;
    top: 50%;
    width: var(--size);
    height: var(--size);
    box-sizing: border-box;
    transform: translate(-50%, -50%);
    transition: filter var(--dur-quick);
  }

  .node.is-branch .shape {
    border-radius: 47% 53% 51% 49% / 53% 47% 52% 48%;
    background: color-mix(in srgb, var(--c) 80%, var(--paper));
  }

  .node.is-remote .shape {
    border-radius: 52% 48% 47% 53% / 48% 53% 47% 52%;
    background: color-mix(in srgb, var(--c) 20%, var(--paper));
    border: 1.75px dashed var(--c);
  }

  .node.is-merged .shape {
    border-radius: 50%;
    background: var(--paper);
    box-shadow: inset 0 0 0 2px var(--c);
  }

  .node.is-tag .shape {
    border-radius: 5px;
    background: color-mix(in srgb, var(--c) 72%, var(--paper));
    transform: translate(-50%, -50%) rotate(45deg);
  }

  .node.is-trunk .shape {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1px;
    border-radius: 46% 54% 52% 48% / 53% 47% 53% 47%;
    background: var(--c);
    color: var(--on-accent);
    white-space: nowrap;
  }

  .node.is-trunk .shape b {
    max-width: 90%;
    overflow: hidden;
    text-overflow: ellipsis;
    font-family: var(--font-mono);
    font-size: 14px;
    font-weight: 500;
  }

  .node.is-trunk .shape span {
    font-family: var(--font-serif);
    font-size: 13px;
  }

  .node i {
    font-family: var(--font-hand);
    font-style: normal;
    font-size: 18px;
    line-height: 1;
    transform: rotate(-3deg);
  }

  .name i {
    color: var(--link);
  }

  .node:hover .shape {
    filter: brightness(1.08);
  }

  .node:focus-visible .shape,
  .node.picked .shape {
    outline: 2px solid var(--link);
    outline-offset: 4px;
  }

  .name {
    position: absolute;
    left: 50%;
    top: calc(50% + var(--reach) + 4px);
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 1px 5px;
    border-radius: var(--r-sm);
    /* Paper behind the words, so a line or a ring passing under them doesn't cross them out. */
    background: color-mix(in srgb, var(--paper) 84%, transparent);
    transform: translateX(-50%);
    white-space: nowrap;
    pointer-events: none;
  }

  .name.above {
    top: auto;
    bottom: calc(50% + var(--reach) + 4px);
    flex-direction: column-reverse;
  }

  .name b {
    max-width: 190px;
    overflow: hidden;
    text-overflow: ellipsis;
    font-family: var(--font-mono);
    font-size: 12px;
    font-weight: 400;
    line-height: 1.35;
  }

  .name small {
    font-family: var(--font-serif);
    font-size: 12.5px;
    line-height: 1.3;
    color: var(--ink-mute);
  }

  /* In a crowd the older names wait for the pointer or the focus. */
  .node.quiet .name {
    display: none;
  }

  .node.quiet:hover .name,
  .node.quiet:focus-visible .name,
  .node.quiet.picked .name {
    display: flex;
    background: var(--paper);
    box-shadow: 0 0 0 1px var(--paper-edge);
  }

  .card {
    position: absolute;
    z-index: 4;
    display: flex;
    flex-direction: column;
    gap: 4px;
    box-sizing: border-box;
    padding: 10px 12px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    background: var(--paper);
    box-shadow: var(--shadow);
    font-size: 13px;
    line-height: 1.4;
    color: var(--ink-soft);
    animation: drop-in var(--dur-base) var(--ease-ink);
  }

  .card > b {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: 12.5px;
    font-weight: 500;
    color: var(--ink);
  }

  .acts {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
  }

  .zoom {
    position: absolute;
    right: 12px;
    bottom: 10px;
    z-index: 3;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 2px;
    border-radius: var(--r-md);
    background: color-mix(in srgb, var(--paper-deep) 90%, transparent);
  }

  .zoom .btn {
    font-size: 16px;
    line-height: 1;
  }

  .zoom .span {
    min-width: 120px;
    padding: 0 4px;
    text-align: center;
    font-size: 13px;
    color: var(--ink-soft);
    white-space: nowrap;
  }

  .legend {
    position: absolute;
    left: 16px;
    bottom: 14px;
    display: flex;
    flex-direction: column;
    gap: 7px;
    font-size: 13px;
    color: var(--ink-mute);
    pointer-events: none;
  }

  .legend div {
    display: flex;
    align-items: center;
    gap: 9px;
  }

  .legend i {
    display: inline-block;
    flex: none;
    width: 14px;
    height: 14px;
  }

  .legend .k-branch {
    border-radius: 47% 53% 51% 49% / 53% 47% 52% 48%;
    background: var(--lane-2);
  }

  .legend .k-remote {
    box-sizing: border-box;
    border-radius: 50%;
    border: 1.5px dashed var(--lane-5);
    background: color-mix(in srgb, var(--lane-5) 20%, var(--paper));
  }

  .legend .k-merged {
    border-radius: 50%;
    box-shadow: inset 0 0 0 2px var(--lane-1);
  }

  .legend .k-tag {
    width: 11px;
    height: 11px;
    margin: 0 1.5px;
    border-radius: 2px;
    background: var(--accent-c);
    transform: rotate(45deg);
  }

  .legend .more {
    font-style: italic;
  }

  @media (prefers-reduced-motion: reduce) {
    .card {
      animation: none;
    }

    .shape {
      transition: none;
    }
  }
</style>
