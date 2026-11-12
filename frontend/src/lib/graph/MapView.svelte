<script lang="ts">
  // An experimental view with no commits in it: a node for each branch and tag, held around
  // the trunk. Size is commits, distance is how long since it moved. It's a way in to the
  // timeline, not a place to work: a node's card opens the timeline on it.
  import { BranchMap } from '../../../wailsjs/go/app/App.js'
  import { kindInfo } from '../branchKinds'
  import { errorText } from '../errors'
  import Loading from '../ui/Loading.svelte'
  import { laneVar, relativeTime } from './draw'
  import Icon from '../ui/Icon.svelte'
  import { DEFAULT_SCALE, keyShows, layout, nameWaits, pinKey, SCALES, WIP_R, wipPlace, zoomBy, type MapNode, type Placed } from './mapLayout'

  let {
    path,
    tick,
    onShow,
    onMenu,
    onSummary,
    wip,
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
    /** The working tree: how many files have changed (null until known), whether its panel is
     * showing, and how to show it. It's drawn against the checked-out branch. */
    wip?: { changes: number | null; active: boolean; open: () => void }
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
  const keyOf = pinKey

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

  // Pins: a node dragged somewhere stays there, for this repository, until it's let go. They're
  // kept as shares of the stage, measured from its middle, so they hold when the panel resizes.
  const pinsKey = () => `graph.map.pins:${path}`
  function savedPins(): Record<string, { dx: number; dy: number }> {
    try {
      const got: unknown = JSON.parse(localStorage.getItem(pinsKey()) ?? '{}')
      return got && typeof got === 'object' ? (got as Record<string, { dx: number; dy: number }>) : {}
    } catch {
      return {}
    }
  }
  let pinned = $state(savedPins())
  function savePins() {
    try {
      if (Object.keys(pinned).length) localStorage.setItem(pinsKey(), JSON.stringify(pinned))
      else localStorage.removeItem(pinsKey())
    } catch {
      // The pins just aren't kept.
    }
  }
  let unit = $derived(Math.max(1, Math.min(width, height) / 2))
  let pins = $derived(new Map(Object.entries(pinned).map(([key, p]) => [key, { x: width / 2 + p.dx * unit, y: height / 2 + p.dy * unit }])))
  function unpin(key: string) {
    pinned = Object.fromEntries(Object.entries(pinned).filter(([k]) => k !== key))
    savePins()
  }
  /** A drag in progress: which node, where it was pressed, and whether it has moved enough to
   * be a drag and not a click. */
  let drag: { key: string; x: number; y: number; moved: boolean } | null = null
  /** The node a drag just ended on: the click that follows the release isn't a choice of it. */
  let dragged = ''
  function nodeDown(e: PointerEvent, n: Placed) {
    if (e.button !== 0 || n.kind === 'trunk') return
    drag = { key: pinKey(n), x: e.clientX, y: e.clientY, moved: false }
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }
  function nodeMove(e: PointerEvent) {
    if (!drag || !stage) return
    if (!drag.moved && Math.hypot(e.clientX - drag.x, e.clientY - drag.y) < 5) return
    drag.moved = true
    const box = stage.getBoundingClientRect()
    pinned = { ...pinned, [drag.key]: { dx: (e.clientX - box.left - width / 2) / unit, dy: (e.clientY - box.top - height / 2) / unit } }
  }
  function nodeUp() {
    if (drag?.moved) {
      dragged = drag.key
      savePins()
    }
    drag = null
  }

  let placed = $derived(data && data.trunk.name && width > 0 && height > 0 ? layout(data.trunk, data.nodes, width, height, now, scale, pins) : null)
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

  /** The working tree's mark sits against whatever is checked out; with a detached HEAD, or a
   * checked-out branch that isn't shown, against the trunk. */
  let wipAt = $derived.by(() => {
    if (!wip || !placed) return null
    const head = placed.trunk.head ? placed.trunk : (placed.nodes.find((n) => n.head) ?? placed.trunk)
    return { ...wipPlace(head, placed.trunk, width, height), on: head.name }
  })
  let wipText = $derived(!wip ? '' : wip.changes ? `${plural(wip.changes, 'file')} changed` : wip.changes === 0 ? 'Working tree, clean' : 'Working tree')

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
    const where = (n.kind === 'remote' ? 'Only on the remote. ' : '') + (n.stacked && n.parent ? `Stacked on ${n.parent}. ` : '') + (n.worktree ? `Checked out in the worktree at ${n.worktree}. ` : '')
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
    class:pinned={n.pinned}
    onclick={() => {
      // The release at the end of a drag isn't a click on the node.
      if (dragged === key) return void (dragged = '')
      picked = picked === key ? '' : key
    }}
    onpointerdown={(e) => nodeDown(e, n)}
    onpointermove={nodeMove}
    onpointerup={nodeUp}
    onpointercancel={nodeUp}
    ondblclick={() => n.pinned && unpin(key)}
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
    {#if n.worktree}<span class="badge" title="Checked out in the worktree at {n.worktree}"><Icon name="worktree" size={12} /></span>{/if}
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
        {#if n.pinned}<button class="btn sm text" onclick={() => unpin(key)}>Let it go back to its place</button>{/if}
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
        <line class:tagline={n.kind === 'tag' || n.kind === 'tags'} class:stackline={n.stacked} x1={from.x} y1={from.y} x2={n.x} y2={n.y} stroke={colorOf(n)} />
      {/each}
    </svg>
    <!-- A narrow panel has no room for the rings' names or the key: the nodes come first. -->
    {#if keyShows(width, height)}
    <!-- Each ring's name sits on its ring, along whichever direction is clearest of nodes;
         what's past the last ring is "older". -->
    {@const ray = (placed.nameAngle * Math.PI) / 180}
    {#each placed.rings.marks as ring (ring.days)}
      <span class="ringname" style:left="{placed.rings.cx + Math.cos(ray) * ring.r}px" style:top="{placed.rings.cy + Math.sin(ray) * ring.r}px">{ring.name}</span>
    {/each}
    {@const last = placed.rings.marks[placed.rings.marks.length - 1].r + 30}
    <span class="ringname" style:left="{Math.min(width - 34, Math.max(34, placed.rings.cx + Math.cos(ray) * last))}px" style:top="{Math.min(height - 12, Math.max(12, placed.rings.cy + Math.sin(ray) * last))}px">older</span>
    {/if}

    <div class="nodes" role="group" aria-label="Branches and tags around {data.trunk.name}">
      {@render nodeButton(placed.trunk)}
      {#each ordered as n (keyOf(n))}
        {@render nodeButton(n)}
      {/each}
    </div>

    {#if keyShows(width, height)}
    {#if wip && wipAt}
      <!-- What isn't a commit yet, against the branch it would be committed to. -->
      <button class="wip" class:dirty={!!wip.changes} class:picked={wip.active} aria-pressed={wip.active} aria-label="{wipText}. The working tree, on {wipAt.on}." title="{wipText}, on {wipAt.on}" style:left="{wipAt.x}px" style:top="{wipAt.y}px" style:--size="{WIP_R * 2}px" onclick={() => wip.open()}>
        {#if wip.changes}<span>{wip.changes > 99 ? '99+' : wip.changes}</span>{/if}
      </button>
    {/if}

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

  /* A stack's links are heavier: they're what the stack is. */
  line.stackline {
    stroke-width: 2.5;
    stroke-opacity: 0.7;
  }

  line.tagline {
    stroke-width: 1.25;
    stroke-dasharray: 4 4;
  }

  /* Set on the ring's line with paper behind it, as a contour's height is on a map. */
  .ringname {
    position: absolute;
    padding: 1px 6px;
    border-radius: var(--r-sm);
    background: var(--paper);
    font-family: var(--font-hand);
    font-size: 18px;
    line-height: 1;
    color: var(--ink-faint);
    transform: translate(-50%, -50%) rotate(-2deg);
    pointer-events: none;
    white-space: nowrap;
  }

  /* A node put somewhere by hand: a small pin's head at its edge. */
  .node.pinned::after {
    content: '';
    position: absolute;
    left: calc(50% - var(--reach) * 0.7 - 3px);
    top: calc(50% - var(--reach) * 0.7 - 3px);
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--ink);
    box-shadow: 0 0 0 2px var(--paper);
  }

  .node:not(.is-trunk) {
    touch-action: none;
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

  .badge {
    position: absolute;
    left: calc(50% + var(--reach) * 0.7 - 2px);
    top: calc(50% - var(--reach) * 0.7 - 14px);
    display: inline-flex;
    padding: 2px;
    border-radius: 50%;
    background: var(--paper);
    box-shadow: 0 0 0 1px var(--paper-edge);
    color: var(--ink-soft);
    pointer-events: none;
  }

  /* The working tree: a dashed ring, as in the timeline, with how many files have changed. */
  .wip {
    position: absolute;
    z-index: 2;
    display: flex;
    align-items: center;
    justify-content: center;
    width: var(--size);
    height: var(--size);
    box-sizing: border-box;
    padding: 0;
    border: 1.5px dashed var(--link);
    border-radius: 50%;
    background: var(--paper);
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--link-hover);
    transform: translate(-50%, -50%);
    cursor: pointer;
  }

  /* A bigger press target than the mark. */
  .wip::before {
    content: '';
    position: absolute;
    inset: -5px;
    border-radius: 50%;
  }

  .wip.dirty {
    background: color-mix(in srgb, var(--link) 16%, var(--paper));
  }

  .wip:focus-visible,
  .wip.picked {
    outline: 2px solid var(--link);
    outline-offset: 3px;
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
