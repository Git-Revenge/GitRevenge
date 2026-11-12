<script lang="ts">
  // A GeoJSON file drawn as its shapes, in Web Mercator, over lines of latitude and longitude.
  // There's no map underneath: the app is local-first, and a map would mean fetching tiles
  // from somebody's server for every file looked at. The wheel zooms, a drag pans, and the list
  // beside the map picks a feature and shows its properties.
  import { boundsOf, degreeLabel, gridStep, project, propertyText, readGeoJson, shapeSummary, unproject, type Feature } from './geojson'
  import type { PreviewProps } from './registry'

  let { text }: PreviewProps = $props()

  /** User units per unit of the projection: keeps the numbers in the SVG away from tiny. */
  const S = 1000
  /** The most features the list names; the map draws them all. */
  const LISTED = 500

  let parsed = $derived.by((): { features: Feature[]; error: string } => {
    try {
      return { features: readGeoJson(text), error: '' }
    } catch (e) {
      return { features: [], error: e instanceof Error ? e.message : String(e) }
    }
  })
  let features = $derived(parsed.features)
  let bounds = $derived(boundsOf(features))

  /** The whole of what's drawn, in projected units: where Fit goes back to. */
  let home = $derived.by(() => {
    if (!bounds) return { x: 0, y: 0, w: 1, h: 1 }
    const [x0, y0] = project([bounds.west, bounds.north])
    const [x1, y1] = project([bounds.east, bounds.south])
    return { x: x0, y: y0, w: Math.max(1e-9, x1 - x0), h: Math.max(1e-9, y1 - y0) }
  })
  let width = $state(800)
  let height = $state(480)
  /** The part of the projection in view: its corner and size, kept to the frame's shape. */
  let view = $state({ x: 0, y: 0, w: 1, h: 1 })

  function fit() {
    const aspect = height / Math.max(1, width)
    let { x, y, w, h } = home
    if (h / w > aspect) {
      const nw = h / aspect
      x -= (nw - w) / 2
      w = nw
    } else {
      const nh = w * aspect
      y -= (nh - h) / 2
      h = nh
    }
    view = { x, y, w, h }
  }
  $effect(() => {
    void home
    void width
    void height
    fit()
  })

  /** Pixels on screen per user unit, for things that should stay one size as the map zooms. */
  let px = $derived((view.w * S) / Math.max(1, width))

  const pathOf = (parts: [number, number][][], close: boolean) =>
    parts
      .filter((p) => p.length)
      .map((part) => 'M' + part.map((pos) => project(pos).map((v) => +(v * S).toFixed(3)).join(' ')).join('L') + (close ? 'Z' : ''))
      .join('')

  /** Each feature's drawing, worked out once for the file. */
  let drawn = $derived(
    features.map((f) => ({
      areas: pathOf(f.shapes.filter((s) => s.kind === 'area').flatMap((s) => s.parts), true),
      lines: pathOf(f.shapes.filter((s) => s.kind === 'line').flatMap((s) => s.parts), false),
      points: f.shapes.filter((s) => s.kind === 'point').flatMap((s) => s.parts.map((p) => project(p[0]).map((v) => v * S) as [number, number])),
    })),
  )

  /** The graticule for what's in view: lines and their labels. */
  let grid = $derived.by(() => {
    const [west, north] = unproject([view.x, view.y])
    const [east, south] = unproject([view.x + view.w, view.y + view.h])
    const lonStep = gridStep(east - west)
    const latStep = gridStep(north - south)
    const lons: number[] = []
    const lats: number[] = []
    for (let v = Math.ceil(west / lonStep) * lonStep; v <= east && lons.length < 40; v += lonStep) lons.push(+v.toFixed(6))
    for (let v = Math.ceil(south / latStep) * latStep; v <= north && lats.length < 40; v += latStep) lats.push(+v.toFixed(6))
    return { lons, lats, lonStep, latStep }
  })

  let picked = $state(-1)
  let summary = $derived(shapeSummary(features))

  // Zooming and panning move the view; Fit puts it back.
  let svg = $state<SVGSVGElement>()
  function toUnit(clientX: number, clientY: number): [number, number] {
    const r = svg!.getBoundingClientRect()
    return [view.x + ((clientX - r.left) / r.width) * view.w, view.y + ((clientY - r.top) / r.height) * view.h]
  }
  function zoomAt(factor: number, at: [number, number]) {
    const w = Math.min(4, Math.max(1e-7, view.w * factor))
    const k = w / view.w
    view = { x: at[0] - (at[0] - view.x) * k, y: at[1] - (at[1] - view.y) * k, w, h: view.h * k }
  }
  function onWheel(e: WheelEvent) {
    e.preventDefault()
    zoomAt(Math.exp(e.deltaY * (e.deltaMode === 1 ? 0.05 : 0.0015)), toUnit(e.clientX, e.clientY))
  }
  let drag: { x: number; y: number; view: typeof view } | null = null
  function down(e: PointerEvent) {
    if (e.button !== 0) return
    drag = { x: e.clientX, y: e.clientY, view }
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
  }
  function move(e: PointerEvent) {
    if (!drag || !svg) return
    const r = svg.getBoundingClientRect()
    view = { ...drag.view, x: drag.view.x - ((e.clientX - drag.x) / r.width) * drag.view.w, y: drag.view.y - ((e.clientY - drag.y) / r.height) * drag.view.h }
  }
  $effect(() => {
    const el = svg
    if (!el) return
    // Not passive: the wheel is taken, so the page doesn't scroll as the map zooms.
    el.addEventListener('wheel', onWheel, { passive: false })
    return () => el.removeEventListener('wheel', onWheel)
  })
  function onKey(e: KeyboardEvent) {
    const c: [number, number] = [view.x + view.w / 2, view.y + view.h / 2]
    const pan = (dx: number, dy: number) => (view = { ...view, x: view.x + dx * view.w * 0.15, y: view.y + dy * view.h * 0.15 })
    const keys: Record<string, () => void> = {
      '+': () => zoomAt(0.7, c),
      '=': () => zoomAt(0.7, c),
      '-': () => zoomAt(1 / 0.7, c),
      '0': fit,
      ArrowLeft: () => pan(-1, 0),
      ArrowRight: () => pan(1, 0),
      ArrowUp: () => pan(0, -1),
      ArrowDown: () => pan(0, 1),
    }
    if (!keys[e.key]) return
    e.preventDefault()
    keys[e.key]()
  }

  const fillOf = (f: Feature, k: number) => (k === picked ? 'var(--link)' : (f.fill ?? 'var(--accent-a)'))
  const strokeOf = (f: Feature, k: number) => (k === picked ? 'var(--link)' : (f.stroke ?? 'var(--accent-a)'))
</script>

<div class="geo">
  {#if parsed.error}
    <p class="problem" role="alert">{parsed.error}</p>
  {:else if !bounds}
    <p class="muted">{features.length ? `${features.length} ${features.length === 1 ? 'feature' : 'features'}, none with anything to draw.` : 'No features in this file.'}</p>
  {:else}
    <p class="facts">
      <span>{features.length.toLocaleString()} {features.length === 1 ? 'feature' : 'features'} · {summary}</span>
      <span class="local">No map underneath: nothing is fetched to draw this</span>
    </p>
    <div class="body">
      <div class="stage" bind:clientWidth={width} bind:clientHeight={height}>
        <!-- The map takes the arrow keys, plus and minus itself, as role="application" says;
             Svelte's checker counts that role as non-interactive. -->
        <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
        <svg
          bind:this={svg}
          viewBox="{view.x * S} {view.y * S} {view.w * S} {view.h * S}"
          preserveAspectRatio="xMidYMid meet"
          role="application"
          aria-roledescription="map"
          aria-label="{features.length} {features.length === 1 ? 'feature' : 'features'}: {summary}. Arrow keys pan, plus and minus zoom, 0 fits."
          tabindex="0"
          onpointerdown={down}
          onpointermove={move}
          onpointerup={() => (drag = null)}
          onpointercancel={() => (drag = null)}
          onkeydown={onKey}
        >
          <g class="grid">
            {#each grid.lons as lon (lon)}
              {@const x = project([lon, 0])[0] * S}
              <line x1={x} x2={x} y1={view.y * S} y2={(view.y + view.h) * S} />
              <text x={x + 4 * px} y={(view.y + view.h) * S - 6 * px} font-size={11 * px}>{degreeLabel(lon, 'lon', grid.lonStep)}</text>
            {/each}
            {#each grid.lats as lat (lat)}
              {@const y = project([0, lat])[1] * S}
              <line x1={view.x * S} x2={(view.x + view.w) * S} y1={y} y2={y} />
              <text x={view.x * S + 6 * px} y={y - 4 * px} font-size={11 * px}>{degreeLabel(lat, 'lat', grid.latStep)}</text>
            {/each}
          </g>
          {#each features as f, k (k)}
            {@const d = drawn[k]}
            <g class="feature" class:picked={k === picked} onclick={() => (picked = k)} role="presentation">
              {#if d.areas}<path class="area" d={d.areas} fill={fillOf(f, k)} stroke={strokeOf(f, k)}><title>{f.name}</title></path>{/if}
              {#if d.lines}<path class="line" d={d.lines} stroke={strokeOf(f, k)}><title>{f.name}</title></path>{/if}
              {#each d.points as [cx, cy], n (n)}<circle {cx} {cy} r={k === picked ? 7 * px : 5 * px} fill={fillOf(f, k)} stroke-width={2 * px}><title>{f.name}</title></circle>{/each}
            </g>
          {/each}
        </svg>
        <div class="tools">
          <button class="btn sm quiet" onclick={() => zoomAt(0.7, [view.x + view.w / 2, view.y + view.h / 2])} aria-label="Zoom in">+</button>
          <button class="btn sm quiet" onclick={() => zoomAt(1 / 0.7, [view.x + view.w / 2, view.y + view.h / 2])} aria-label="Zoom out">−</button>
          <button class="btn sm quiet" onclick={fit}>Fit</button>
        </div>
      </div>
      <aside class="side" aria-label="Features">
        <ul class="list">
          {#each features.slice(0, LISTED) as f, k (k)}
            <li><button aria-pressed={k === picked} onclick={() => (picked = picked === k ? -1 : k)}><i style:--swatch={f.fill ?? f.stroke ?? 'var(--accent-a)'}></i>{f.name}</button></li>
          {/each}
          {#if features.length > LISTED}<li class="more">and {(features.length - LISTED).toLocaleString()} more</li>{/if}
        </ul>
        {#if picked >= 0 && features[picked]}
          {@const props = Object.entries(features[picked].properties)}
          <div class="props" role="group" aria-label="Properties of {features[picked].name}">
            <b>{features[picked].name}</b>
            {#if props.length}
              <table>
                <tbody>{#each props as [key, value] (key)}<tr><th scope="row">{key}</th><td>{propertyText(value)}</td></tr>{/each}</tbody>
              </table>
            {:else}
              <p class="muted">No properties.</p>
            {/if}
          </div>
        {/if}
      </aside>
    </div>
  {/if}
</div>

<style>
  .geo {
    box-sizing: border-box;
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 16px 20px 24px;
    background: var(--paper);
  }

  .facts {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: 4px 16px;
    margin: 0;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .local {
    font-style: italic;
  }

  .body {
    display: flex;
    flex-wrap: wrap;
    gap: 14px;
  }

  .stage {
    position: relative;
    flex: 999 1 420px;
    min-width: 0;
    height: max(360px, calc(100vh - 240px));
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-sm);
    background: var(--paper-warm);
    overflow: hidden;
  }

  /* Out of the flow: an SVG's own size follows its viewBox, which follows the stage's size, so
     in the flow each fit would resize the stage and start another. */
  svg {
    position: absolute;
    inset: 0;
    display: block;
    width: 100%;
    height: 100%;
    cursor: grab;
    touch-action: none;
    outline: none;
  }

  svg:focus-visible {
    outline: 2px solid var(--link);
    outline-offset: -2px;
  }

  .grid line {
    stroke: var(--paper-edge);
    stroke-width: 1;
    vector-effect: non-scaling-stroke;
  }

  .grid text {
    fill: var(--ink-mute);
    font-family: var(--font-mono);
  }

  .area {
    fill-opacity: 0.22;
    fill-rule: evenodd;
    stroke-width: 1.5;
    vector-effect: non-scaling-stroke;
  }

  .line {
    fill: none;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
    vector-effect: non-scaling-stroke;
  }

  circle {
    stroke: var(--paper);
  }

  .feature {
    cursor: pointer;
  }

  .feature.picked .area {
    fill-opacity: 0.35;
    stroke-width: 2.5;
  }

  .feature.picked .line {
    stroke-width: 3.5;
  }

  .tools {
    position: absolute;
    right: 8px;
    top: 8px;
    display: flex;
    gap: 2px;
    padding: 2px;
    border-radius: var(--r-md);
    background: color-mix(in srgb, var(--paper) 90%, transparent);
  }

  .tools .btn {
    min-width: 0;
    padding: 0 10px;
  }

  .side {
    display: flex;
    flex-direction: column;
    gap: 12px;
    flex: 1 1 220px;
    min-width: 0;
    max-height: max(360px, calc(100vh - 240px));
    overflow-y: auto;
  }

  .list {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .list button {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 5px 8px;
    border: none;
    border-radius: var(--r-sm);
    background: none;
    font: inherit;
    font-size: 13.5px;
    color: var(--ink);
    text-align: left;
    cursor: pointer;
  }

  .list button:hover {
    background: var(--wash-hover);
  }

  .list button[aria-pressed='true'] {
    background: var(--selected);
  }

  .list i {
    flex: none;
    width: 10px;
    height: 10px;
    border-radius: 2px;
    background: var(--swatch);
  }

  .more {
    padding: 5px 8px;
    font-size: 13px;
    font-style: italic;
    color: var(--ink-mute);
  }

  .props {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding-top: 10px;
    border-top: 1px dashed var(--hairline);
  }

  .props table {
    border-collapse: collapse;
    font-size: 13px;
  }

  .props th,
  .props td {
    padding: 3px 8px 3px 0;
    border-bottom: 1px solid var(--hairline);
    text-align: left;
    vertical-align: top;
    overflow-wrap: anywhere;
  }

  .props th {
    font-family: var(--font-mono);
    font-size: 12px;
    font-weight: 400;
    color: var(--ink-mute);
  }

  .problem {
    margin: 0;
    color: var(--danger);
  }

  .muted {
    margin: 0;
    color: var(--ink-mute);
  }
</style>
