<script lang="ts">
  // The split view: old and new side by side, drawn from the Go diff. With the ribbon, the
  // sides aren't padded to line up: each scrolls as a plain file, the two move together, and
  // curved bands in the gap join each change's old lines to its new ones (after RibbonDiff).
  // Without it, MergeView pads them so changed lines sit level.
  import { Change, MergeView } from '@codemirror/merge'
  import { EditorState, type Extension } from '@codemirror/state'
  import { Decoration, EditorView, lineNumbers, MatchDecorator, ViewPlugin, type DecorationSet, type ViewUpdate } from '@codemirror/view'
  import { findPattern, type FindOptions } from '../find/find'
  import { syntaxHighlighting } from '@codemirror/language'
  import { classHighlighter } from '@lezer/highlight'
  import { bandPath, edgePath, mapLine, moveTone, type Result, type Stop } from './blocks'
  import { languageFor } from './highlight'
  import { setCurrent, setHot, sideMarks, sideModel, type SideActions } from './sideMarks'
  import { splitLines } from './lineDiff'
  import { wrap } from './wrap.svelte'
  import Icon from '../ui/Icon.svelte'

  let {
    path,
    oldText,
    newText,
    diff,
    ribbon = true,
    hideFormatting = false,
    stops = [],
    current = -1,
    jump = 0,
    labels,
    onStep,
    onCompare,
    onPick,
    find,
  }: {
    path: string
    oldText: string
    newText: string
    diff: Result
    /** A search whose text is marked on both sides: a history search's, in a commit's diff
     * (#982). */
    find?: FindOptions
    ribbon?: boolean
    /** Draws formatting-only blocks as unchanged. */
    hideFormatting?: boolean
    stops?: Stop[]
    /** The stop being read, or -1. */
    current?: number
    /** Counts the requests to scroll to the current stop. */
    jump?: number
    labels: { before: string; after: string }
    onStep: (by: 1 | -1) => void
    /** Opens a move that changed against where it came from, by its index. */
    onCompare?: (move: number) => void
    /** Makes a stop the one being read, as a click on a move's note does. */
    onPick?: (stop: number) => void
  } = $props()

  /** The rename whose places are lit up on both sides, from 1, or 0. */
  let hot = 0
  function toggleHot(r: number) {
    hot = hot === r ? 0 : r
    for (const v of [left, right, merge?.a, merge?.b]) v?.dispatch({ effects: setHot.of(hot) })
  }

  /** Goes to a move: its stop becomes the one being read, which brings both ends into view. */
  function goToMove(k: number) {
    const s = stops.findIndex((st) => st.kind === 'move' && st.move === k)
    if (s >= 0) onPick?.(s)
  }

  /** The ribbon's width; the canvas draws the bands across 74 of its 76 pixels. */
  const W = 74

  let host = $state<HTMLDivElement>()
  let leftHost = $state<HTMLDivElement>()
  let rightHost = $state<HTMLDivElement>()
  let svg = $state<SVGSVGElement>()
  let left: EditorView | undefined
  let right: EditorView | undefined
  let merge: MergeView | undefined

  interface Band {
    key: string
    kind: Stop['kind']
    /** A move's colour, cycling. */
    tone: string
    d: string
    top: string
    bottom: string
    current: boolean
  }
  let bands = $state<Band[]>([])
  let height = $state(0)
  const gradient = `ribbon-change-${Math.random().toString(36).slice(2, 8)}`

  /** Where each line starts in a text, as CodeMirror counts: line i at starts[i], the end at starts[n]. */
  function starts(text: string): number[] {
    const out = [0]
    for (const l of splitLines(text)) out.push(out[out.length - 1] + l.length)
    return out
  }

  function base(lang: Extension | null, wrapped: boolean, search: FindOptions | undefined): Extension[] {
    return [
      EditorView.editable.of(false),
      EditorState.readOnly.of(true),
      lineNumbers(),
      syntaxHighlighting(classHighlighter),
      EditorView.theme({}, { dark: true }),
      ...(wrapped ? [EditorView.lineWrapping] : []),
      ...(lang ? [lang] : []),
      ...(search ? [foundMarks(search)] : []),
    ]
  }

  /** Marks what a search finds wherever it's drawn, as Find marks it in a file. */
  function foundMarks(search: FindOptions): Extension {
    const re = findPattern(search)
    if (!re || !search.text) return []
    const decorator = new MatchDecorator({ regexp: re, decoration: Decoration.mark({ class: 'found' }), maxLength: 2000 })
    return ViewPlugin.fromClass(
      class {
        marks: DecorationSet
        constructor(view: EditorView) {
          this.marks = decorator.createDeco(view)
        }
        update(u: ViewUpdate) {
          this.marks = decorator.updateDeco(u, this.marks)
        }
      },
      { decorations: (v) => v.marks },
    )
  }

  // Build the views whenever the inputs, the layout or word wrap change.
  $effect(() => {
    const [p, a, b, d, wrapped, withRibbon, hide, search] = [path, oldText, newText, diff, wrap.on, ribbon, hideFormatting, find]
    let cancelled = false
    const cleanups: (() => void)[] = []
    hot = 0
    const act: SideActions = { rename: toggleHot, pick: goToMove, compare: onCompare }
    void languageFor(p).then((lang) => {
      if (cancelled) return
      const ext = base(lang, wrapped, search)
      if (withRibbon && leftHost && rightHost) {
        const redraw = EditorView.updateListener.of((u) => {
          if (u.geometryChanged || u.viewportChanged || u.heightChanged) schedule()
        })
        left = new EditorView({ doc: a, extensions: [...ext, sideMarks(sideModel(d, 'old', hide), act), redraw], parent: leftHost })
        right = new EditorView({ doc: b, extensions: [...ext, sideMarks(sideModel(d, 'new', hide), act), redraw], parent: rightHost })
        cleanups.push(linkScrolling(left, right))
        const resize = new ResizeObserver(() => schedule())
        if (svg) resize.observe(svg)
        cleanups.push(() => resize.disconnect())
        showCurrent(false)
        schedule()
      } else if (!withRibbon && host) {
        const sa = starts(a)
        const sb = starts(b)
        // Hidden formatting still lines the sides up; it just isn't marked.
        const changes = d.blocks.map((blk) => new Change(sa[blk.oldStart], sa[blk.oldEnd], sb[blk.newStart], sb[blk.newEnd]))
        merge = new MergeView({
          a: { doc: a, extensions: [...ext, sideMarks(sideModel(d, 'old', hide), act)] },
          b: { doc: b, extensions: [...ext, sideMarks(sideModel(d, 'new', hide), act)] },
          parent: host,
          collapseUnchanged: { margin: 3, minSize: 6 },
          gutter: true,
          highlightChanges: false,
          diffConfig: { override: () => changes },
        })
        showCurrent(false)
      }
    })
    return () => {
      cancelled = true
      for (const c of cleanups) c()
      left?.destroy()
      right?.destroy()
      merge?.destroy()
      left = right = merge = undefined
      bands = []
    }
  })

  // Outline the change being read, and scroll to it when asked.
  let lastJump = 0
  $effect(() => {
    void current
    void stops
    const scroll = jump !== lastJump
    lastJump = jump
    showCurrent(scroll)
  })

  function showCurrent(scroll: boolean) {
    const stop = stops[current]
    const l = left ?? merge?.a
    const r = right ?? merge?.b
    if (!l || !r) return
    const moved = stop?.kind === 'move'
    l.dispatch({ effects: setCurrent.of(stop ? { from: stop.oldStart, to: stop.oldEnd, moved } : null) })
    r.dispatch({ effects: setCurrent.of(stop ? { from: stop.newStart, to: stop.newEnd, moved } : null) })
    if (stop && scroll) {
      if (left && right) {
        // The new side leads; the old one follows the line map.
        driver = right
        right.scrollDOM.scrollTop = docY(right, stop.newStart) + pad(right) - right.scrollDOM.clientHeight / 3
        follow(right, left)
      } else {
        const line = r.state.doc.line(Math.min(stop.newStart + 1, r.state.doc.lines))
        r.dispatch({ effects: EditorView.scrollIntoView(line.from, { y: 'start', yMargin: 80 }) })
      }
    }
    schedule()
  }

  // --- Scrolling together ---------------------------------------------------------------

  /** The side the reader is moving. Scrolls of the other side are ours, following it. */
  let driver: EditorView | undefined

  function linkScrolling(a: EditorView, b: EditorView): () => void {
    const offs: (() => void)[] = []
    for (const [view, other] of [
      [a, b],
      [b, a],
    ] as const) {
      const take = () => (driver = view)
      for (const ev of ['wheel', 'pointerdown', 'keydown', 'touchstart'] as const) {
        view.dom.addEventListener(ev, take, { passive: true })
        offs.push(() => view.dom.removeEventListener(ev, take))
      }
      const onScroll = () => {
        if (driver === view) follow(view, other)
        schedule()
      }
      view.scrollDOM.addEventListener('scroll', onScroll, { passive: true })
      offs.push(() => view.scrollDOM.removeEventListener('scroll', onScroll))
    }
    return () => offs.forEach((f) => f())
  }

  /** The gap between the scroller's top and the document's, at scrollTop 0. */
  function pad(view: EditorView): number {
    return view.documentTop - view.scrollDOM.getBoundingClientRect().top + view.scrollDOM.scrollTop
  }

  /** The document height of the top of a line (counting from 0, fractions allowed). */
  function docY(view: EditorView, line: number): number {
    const doc = view.state.doc
    const i = Math.floor(line)
    if (i + 1 > doc.lines) return view.lineBlockAt(doc.line(doc.lines).from).bottom
    const blk = view.lineBlockAt(doc.line(i + 1).from)
    return blk.top + (line - i) * blk.height
  }

  /** The line (with a fraction) at a document height. */
  function lineAtY(view: EditorView, y: number): number {
    const blk = view.lineBlockAtHeight(Math.max(0, y))
    const n = view.state.doc.lineAt(blk.from).number - 1
    return n + (blk.height > 0 ? Math.min(1, Math.max(0, (y - blk.top) / blk.height)) : 0)
  }

  /** Scrolls `to` so the line across from `from`'s middle sits at its middle. */
  function follow(from: EditorView, to: EditorView) {
    const anchor = from.scrollDOM.clientHeight / 2
    const line = lineAtY(from, from.scrollDOM.scrollTop + anchor - pad(from))
    const there = mapLine(diff.blocks, line, from === left ? 'old' : 'new')
    to.scrollDOM.scrollTop = docY(to, there) + pad(to) - anchor
  }

  // --- The ribbon -----------------------------------------------------------------------

  let frame = 0
  function schedule() {
    if (frame) return
    frame = requestAnimationFrame(() => {
      frame = 0
      draw()
    })
  }

  function draw() {
    if (!left || !right || !svg) return
    const box = svg.getBoundingClientRect()
    height = box.height
    const lt = left.documentTop - box.top
    const rt = right.documentTop - box.top
    const stop = stops[current]
    const out: Band[] = []
    const add = (key: string, kind: Stop['kind'], o0: number, o1: number, n0: number, n1: number, isCurrent: boolean, tone = '') => {
      const ot = lt + docY(left!, o0)
      const ob = o1 > o0 ? lt + docY(left!, o1) : ot
      const nt = rt + docY(right!, n0)
      const nb = n1 > n0 ? rt + docY(right!, n1) : nt
      if (Math.max(ob, nb) < 0 || Math.min(ot, nt) > box.height) return
      out.push({ key, kind, tone, d: bandPath(ot, ob, nt, nb, W), top: edgePath(ot, nt, W), bottom: edgePath(ob, nb, W), current: isCurrent })
    }
    for (const s of stops) {
      add(`${s.kind}${s.block ?? s.move}`, s.kind, s.oldStart, s.oldEnd, s.newStart, s.newEnd, s === stop, s.kind === 'move' ? `t${moveTone(s.move ?? 0)}` : '')
    }
    // Moves cross the others, so they're drawn last; the band being read goes on top.
    out.sort((x, y) => Number(x.kind === 'move') - Number(y.kind === 'move') || Number(x.current) - Number(y.current))
    bands = out
  }
</script>

<div class="split" class:ribbon>
  {#if ribbon}
    <div class="heads">
      <span class="head" title={labels.before}>{labels.before}</span>
      <span class="rnav">
        <button class="nav-btn" aria-label="Previous change" title="Previous change (Shift+F7)" onclick={() => onStep(-1)} disabled={!stops.length}
          ><span class="up"><Icon name="chevron" size={12} /></span></button
        >
        <button class="nav-btn" aria-label="Next change" title="Next change (F7)" onclick={() => onStep(1)} disabled={!stops.length}
          ><span class="down"><Icon name="chevron" size={12} /></span></button
        >
      </span>
      <span class="head" title={labels.after}>{labels.after}</span>
    </div>
    <div class="panes">
      <div class="pane" bind:this={leftHost} data-side="old"></div>
      <svg class="gap" bind:this={svg} aria-hidden="true" width={W} {height} viewBox="0 0 {W} {height}">
        <defs>
          <linearGradient id={gradient} x1="0" y1="0" x2={W} y2="0" gradientUnits="userSpaceOnUse">
            <stop class="from" offset="0"></stop>
            <stop class="to" offset="1"></stop>
          </linearGradient>
        </defs>
        {#each bands as b (b.key)}
          <g class="band {b.kind} {b.tone}" class:current={b.current} data-band={b.key}>
            <path class="fill" d={b.d} style:fill={b.kind === 'change' ? `url(#${gradient})` : undefined}></path>
            <path class="edge" d={b.top}></path>
            <path class="edge" d={b.bottom}></path>
          </g>
        {/each}
      </svg>
      <div class="pane" bind:this={rightHost} data-side="new"></div>
    </div>
  {:else}
    <div class="merge" bind:this={host}></div>
  {/if}
</div>

<style>
  .split {
    font-size: 13px;
    tab-size: 4;
    height: 100%;
  }

  .split.ribbon {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .heads {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 76px minmax(0, 1fr);
    background: var(--paper-warm);
    border-bottom: 1px solid var(--paper-edge);
    color: var(--ink-mute);
    font-family: var(--font-serif);
    font-size: 13px;
  }

  .head {
    padding: 6px 14px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .rnav {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
  }

  .nav-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 22px;
    border: 1px solid var(--paper-edge);
    border-radius: 6px;
    background: var(--paper);
    color: var(--ink-soft);
    cursor: pointer;
  }

  .nav-btn:hover:not(:disabled) {
    color: var(--ink);
    border-color: var(--ink-faint);
  }

  .nav-btn:disabled {
    opacity: 0.45;
    cursor: default;
  }

  .up,
  .down {
    display: inline-flex;
  }

  /* The chevron points down. */
  .up {
    transform: rotate(180deg);
  }

  .panes {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) 76px minmax(0, 1fr);
  }

  .pane {
    min-width: 0;
    min-height: 0;
  }

  .pane :global(.cm-editor) {
    height: 100%;
  }

  .gap {
    width: 76px;
    height: 100%;
    background: color-mix(in srgb, var(--paper) 86%, black);
    border-left: 1px solid var(--paper-edge);
    border-right: 1px solid var(--paper-edge);
    box-sizing: border-box;
    overflow: hidden;
  }

  .gap .from {
    stop-color: color-mix(in srgb, var(--danger) 34%, transparent);
  }

  .gap .to {
    stop-color: color-mix(in srgb, var(--success) 34%, transparent);
  }

  .band .edge {
    fill: none;
    stroke-width: 1;
  }

  .band.change .edge {
    stroke: color-mix(in srgb, var(--code-punctuation) 55%, transparent);
  }

  .band.add .fill {
    fill: color-mix(in srgb, var(--success) 30%, transparent);
  }

  .band.add .edge {
    stroke: color-mix(in srgb, var(--success) 85%, transparent);
  }

  .band.del .fill {
    fill: color-mix(in srgb, var(--danger) 30%, transparent);
  }

  .band.del .edge {
    stroke: color-mix(in srgb, var(--danger) 85%, transparent);
  }

  /* A rename is ochre; formatting is barely there, outlined in dashes. */
  .band.rename .fill {
    fill: color-mix(in srgb, var(--syn-number) 28%, transparent);
  }

  .band.rename .edge {
    stroke: color-mix(in srgb, var(--syn-number) 85%, transparent);
  }

  .band.format .fill {
    fill: color-mix(in srgb, var(--code-punctuation) 8%, transparent);
  }

  .band.format .edge {
    stroke: color-mix(in srgb, var(--code-punctuation) 65%, transparent);
    stroke-dasharray: 3 3;
  }

  /* Moves cycle teal, plum, sepia and ochre, never the add or delete colours; so do renames,
     starting from ochre. */
  .split :global(.t0),
  .split :global(.r0) {
    --tone: var(--syn-string);
  }

  .split :global(.t1),
  .split :global(.r1) {
    --tone: var(--lane-2);
  }

  .split :global(.t2),
  .split :global(.r2) {
    --tone: var(--lane-3);
  }

  .split :global(.t3),
  .split :global(.r3) {
    --tone: var(--syn-number);
  }

  .band.move .fill {
    fill: color-mix(in srgb, var(--tone, var(--syn-string)) 26%, transparent);
  }

  .band.move .edge {
    stroke: var(--tone, var(--syn-string));
  }

  .band.current .edge {
    stroke: var(--ink);
    stroke-width: 1.8;
  }

  .merge :global(.cm-mergeView) {
    height: 100%;
  }

  /* A history search's text, marked as Find marks it in a file: over whatever else it is. */
  .split :global(.found) {
    border-radius: 2px;
    box-shadow: 0 0 0 1px var(--highlight);
    background: var(--highlight);
  }

  .split :global(.cm-editor) {
    background: var(--paper);
    color: var(--ink);
  }

  .split :global(.cm-gutters) {
    background: var(--paper-warm);
    color: var(--ink-faint);
    border-right: 1px solid var(--paper-edge);
  }

  .split :global(.cm-content) {
    font-family: var(--font-mono);
  }

  /* The lines and words come from the Go diff (sideMarks), not MergeView's own. */
  .split :global(.cm-changedLine),
  .split :global(.cm-changedText) {
    background: none;
  }

  .split :global(.cm-del) {
    background: var(--danger-wash);
  }

  .split :global(.cm-add) {
    background: var(--success-wash);
  }

  .split :global(.cm-moved) {
    background: color-mix(in srgb, var(--tone, var(--syn-string)) 12%, transparent);
  }

  /* A token that moved within its change, such as a reordered argument. */
  .split :global(.cm-word-move) {
    background: color-mix(in srgb, var(--syn-string) 30%, transparent);
    border-radius: 2px;
  }

  .split :global(.cm-format) {
    box-shadow: inset 3px 0 0 color-mix(in srgb, var(--code-punctuation) 50%, transparent);
  }

  .split :global(.cm-rename) {
    background: color-mix(in srgb, var(--syn-number) 9%, transparent);
  }

  .split :global(.cm-word-format) {
    text-decoration: underline dotted var(--code-punctuation);
    text-underline-offset: 4px;
  }

  .split :global(.cm-word-rename) {
    background: color-mix(in srgb, var(--tone, var(--syn-number)) 34%, transparent);
    border-radius: 2px;
    cursor: pointer;
  }

  .split :global(.cm-word-rename.cm-hot) {
    background: color-mix(in srgb, var(--tone, var(--syn-number)) 62%, transparent);
    outline: 1px solid var(--tone, var(--syn-number));
  }

  .split :global(.cm-block-note) {
    padding-left: 12px;
    color: var(--ink-mute);
    font-family: var(--font-serif);
    font-style: italic;
    font-size: 12px;
    user-select: none;
  }

  .split :global(.cm-block-note-rename) {
    color: var(--syn-number);
  }

  .split :global(.cm-word-del) {
    background: color-mix(in srgb, var(--danger) 35%, transparent);
    border-radius: 2px;
  }

  .split :global(.cm-word-add) {
    background: color-mix(in srgb, var(--success) 35%, transparent);
    border-radius: 2px;
  }

  .split :global(.cm-current) {
    box-shadow: inset 3px 0 0 var(--syn-keyword);
  }

  .split :global(.cm-note-link) {
    padding: 0;
    border: 0;
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }

  .split :global(.cm-note-link:hover) {
    text-decoration: underline;
  }

  .split :global(.cm-compare) {
    margin-left: 10px;
    font-style: normal;
    text-decoration: underline;
    text-underline-offset: 3px;
  }

  .split :global(.cm-move-note) {
    padding-left: 12px;
    color: var(--tone, var(--syn-string));
    font-family: var(--font-serif);
    font-style: italic;
    font-size: 12px;
    user-select: none;
  }

  .split :global(.cm-collapsedLines) {
    background: var(--paper-warm);
    color: var(--ink-mute);
    font-family: var(--font-serif);
    font-style: italic;
  }

  .split :global(.cm-selectionBackground) {
    background: var(--selection) !important;
  }
</style>
