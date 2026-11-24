<script lang="ts">
  import { SvelteSet } from 'svelte/reactivity'
  import { tick } from 'svelte'
  import { kindOf, marksByLine, moveNote, movesByLine, moveTone, notesByLine, renameTone, viewOps, type Kind, type Mark, type Result, type Stop } from './blocks'
  import { highlightLines, languageFor, withMarks, type Token } from './highlight'
  import { applyChanges, splitLines, toHunks, type Hunk } from './lineDiff'
  import { wrap } from './wrap.svelte'

  /** What the hunk and line buttons do, and how the result is written. */
  export type Actions = {
    /** Label for the apply action, e.g. "Stage". */
    apply?: { label: string; run: (content: string) => Promise<void> }
    /** Label for the revert action, e.g. "Unstage" or "Discard". */
    revert?: { label: string; danger?: boolean; run: (content: string) => Promise<void> }
  }

  let {
    path,
    oldText,
    newText,
    diff,
    actions = {},
    hideFormatting = false,
    stops = [],
    current = -1,
    jump = 0,
    onCompare,
    onPick,
  }: {
    path: string
    oldText: string
    newText: string
    /** The Go diff of the two texts (internal/semdiff). */
    diff: Result
    actions?: Actions
    /** Shows formatting-only blocks as unchanged lines. Views that stage never do. */
    hideFormatting?: boolean
    stops?: Stop[]
    /** The stop being read, or -1. */
    current?: number
    /** Counts the requests to scroll to the current stop. */
    jump?: number
    /** Opens a move that changed against where it came from, by its index. */
    onCompare?: (move: number) => void
    /** Makes a stop the one being read, as a click on a move's note does. */
    onPick?: (stop: number) => void
  } = $props()

  let oldLines = $derived(splitLines(oldText))
  let newLines = $derived(splitLines(newText))
  let ops = $derived(viewOps(diff.blocks, oldLines.length, newLines.length, hideFormatting))
  /** A change's block kind; formatting never shares a hunk with a change, so it stages alone. */
  const kindAt = (i: number): Kind | null => {
    const b = ops[i]?.block
    return b === undefined ? null : kindOf(diff.blocks[b])
  }
  let hunks = $derived(toHunks(ops, 3, (i) => (kindAt(i) === 'format' ? 'format' : 'change')))
  let marks = $derived(marksByLine(diff.blocks, hideFormatting))
  let notes = $derived(notesByLine(diff.blocks, hideFormatting))
  /** Whether a hunk changes formatting alone. */
  const formattingOnly = (h: Hunk) => {
    for (let i = h.start; i < h.end; i++) if (ops[i].kind !== 'eq' && kindAt(i) !== 'format') return false
    return true
  }
  /** The note a row carries: a move's, or its formatting or rename block's. */
  function noteOf(i: number, moved: { note?: string; k: number } | null): { text: string; kind: string; move?: number } | null {
    if (moved?.note) return { text: moved.note, kind: 'move', move: moved.k }
    const op = ops[i]
    const text = op.kind === 'add' ? notes.new.get(op.newIndex) : op.kind === 'del' ? notes.old.get(op.oldIndex) : undefined
    return text ? { text, kind: kindAt(i) ?? 'change' } : null
  }
  let moves = $derived(movesByLine(diff.moves ?? []))

  /** The move a changed row belongs to, if any, by index, and the note on its first line. */
  function moveOf(i: number): { note?: string; k: number } | null {
    const op = ops[i]
    const side = op.kind === 'del' ? 'old' : 'new'
    const line = side === 'old' ? op.oldIndex : op.newIndex
    const k = moves[side].get(line)
    if (k === undefined) return null
    const m = diff.moves[k]
    return { k, note: line === (side === 'old' ? m.oldStart : m.newStart) ? moveNote(m, side) : undefined }
  }

  /** Goes to a move: its stop becomes the one being read, which scrolls to it. */
  function goToMove(k: number) {
    const s = stops.findIndex((st) => st.kind === 'move' && st.move === k)
    if (s >= 0) onPick?.(s)
  }

  /** The rename whose places are lit up, from 1, or 0. A click on a rename picks it. */
  let hot = $state(0)
  function pickRename(e: MouseEvent) {
    const r = Number((e.target as HTMLElement).closest<HTMLElement>('[data-rename]')?.dataset.rename ?? 0)
    if (!r) return
    e.stopPropagation()
    hot = hot === r ? 0 : r
  }

  /** Whether a row is part of the change being read. */
  function isCurrent(i: number): boolean {
    const s = stops[current]
    const op = ops[i]
    if (!s || op.kind === 'eq') return false
    const [side, line, from, to] = op.kind === 'del' ? (['old', op.oldIndex, s.oldStart, s.oldEnd] as const) : (['new', op.newIndex, s.newStart, s.newEnd] as const)
    // A change's moved lines belong to their move, and a move's to it alone.
    return line >= from && line < to && moves[side].has(line) === (s.kind === 'move')
  }

  // Scroll to the change being read when asked.
  let root = $state<HTMLDivElement>()
  let lastJump = 0
  $effect(() => {
    if (jump === lastJump) return
    lastJump = jump
    const s = stops[current]
    if (!s) return
    void tick().then(() => {
      const row = root?.querySelector('tr.current')
      row?.scrollIntoView({ block: 'center' })
    })
  })

  let oldTokens = $state<Token[][]>([])
  let newTokens = $state<Token[][]>([])
  $effect(() => {
    const [o, n] = [oldText, newText]
    oldTokens = highlightLines(o, null)
    newTokens = highlightLines(n, null)
    void languageFor(path).then((lang) => {
      if (!lang || o !== oldText || n !== newText) return
      oldTokens = highlightLines(o, lang)
      newTokens = highlightLines(n, lang)
    })
  })

  /** Selected change ops (by op index), for line-level actions. */
  const selected = new SvelteSet<number>()
  let anchor = -1
  let busy = $state(false)
  let confirming = $state<string | null>(null)

  // A new diff invalidates any selection.
  $effect(() => {
    void ops
    selected.clear()
    anchor = -1
    confirming = null
  })

  function toggle(i: number, e: MouseEvent) {
    if (!actions.apply && !actions.revert) return
    if (e.shiftKey && anchor >= 0) {
      const [a, b] = anchor < i ? [anchor, i] : [i, anchor]
      for (let j = a; j <= b; j++) if (ops[j].kind !== 'eq') selected.add(j)
    } else if (selected.has(i)) selected.delete(i)
    else selected.add(i)
    anchor = i
  }

  function inHunk(h: Hunk) {
    return (i: number) => i >= h.start && i < h.end
  }

  /** "Apply" applies the chosen changes; "revert" removes them and keeps the rest. */
  async function run(which: 'apply' | 'revert', chosen: (i: number) => boolean, key: string) {
    const action = actions[which]
    if (!action) return
    if (which === 'revert' && actions.revert?.danger && confirming !== key) {
      confirming = key
      return
    }
    confirming = null
    const content =
      which === 'apply'
        ? applyChanges(ops, oldLines, newLines, chosen)
        : applyChanges(ops, oldLines, newLines, (i) => !chosen(i))
    busy = true
    try {
      await action.run(content)
    } finally {
      busy = false
    }
  }

  const opIndexes = (h: Hunk) => Array.from({ length: h.end - h.start }, (_, k) => h.start + k)
  const header = (h: Hunk) => `@@ -${h.oldStart},${h.oldCount} +${h.newStart},${h.newCount} @@`
  import Empty from '../ui/Empty.svelte'
</script>

{#snippet line(tokens: Token[] | undefined, fallback: string, lineMarks: Mark[] | undefined)}
  {#if tokens}{#each withMarks(tokens, lineMarks) as t, k (k)}<span
        class={[t.cls, t.changed && 'chg', t.kind, t.rename && `r${renameTone(t.rename)}`, t.rename && t.rename === hot && 'hot']}
        data-rename={t.rename || undefined}
        title={t.rename ? 'Show every place of this rename' : undefined}>{t.text}</span
      >{/each}{:else}{fallback.replace(
      /\r?\n$/,
      '',
    )}{/if}
{/snippet}

<!-- A click on a rename lights up every place of it; the rows' own clicks select lines. -->
<div class="unified" bind:this={root} onclickcapture={pickRename}>
  {#if hunks.length === 0}
    <Empty note="nothing changed">No changes.</Empty>
  {/if}
  {#each hunks as h (h.start)}
    <section class="hunk">
      <header>
        <span class="mono">{header(h)}</span>
        {#if formattingOnly(h)}<span class="tag" title="Only how the code is written changed here">formatting only</span>{/if}
        {#if actions.apply}
          <button class="btn sm secondary" disabled={busy} onclick={() => run('apply', inHunk(h), 'h' + h.start)}>{actions.apply.label} hunk</button>
        {/if}
        {#if actions.revert}
          <button
            class={['btn sm', actions.revert.danger && confirming === 'h' + h.start ? 'danger' : 'quiet']}
            disabled={busy}
            onclick={() => run('revert', inHunk(h), 'h' + h.start)}
            >{confirming === 'h' + h.start ? `Really ${actions.revert.label.toLowerCase()}?` : `${actions.revert.label} hunk`}</button
          >
        {/if}
      </header>
      <table class:wrap={wrap.on}>
        <tbody>
          {#each opIndexes(h) as i (i)}
            {@const op = ops[i]}
            {@const moved = op.kind === 'eq' ? null : moveOf(i)}
            {@const note = op.kind === 'eq' ? null : noteOf(i, moved)}
            <tr
              class={[op.kind, op.kind !== 'eq' && !moved && kindAt(i) !== 'change' && kindAt(i), moved && `t${moveTone(moved.k)}`]}
              class:moved={!!moved}
              class:current={isCurrent(i)}
              class:sel={selected.has(i)}
              onclick={(e) => op.kind !== 'eq' && toggle(i, e)}
            >
              <td class="num">{op.oldIndex >= 0 ? op.oldIndex + 1 : ''}</td>
              <td class="num">{op.newIndex >= 0 ? op.newIndex + 1 : ''}</td>
              <td class="sign">{op.kind === 'add' ? '+' : op.kind === 'del' ? '-' : ' '}</td>
              <td class="code"
                >{#if op.kind === 'del'}{@render line(oldTokens[op.oldIndex], oldLines[op.oldIndex], marks.old.get(op.oldIndex))}{:else}{@render line(
                    newTokens[op.newIndex],
                    newLines[op.newIndex],
                    op.kind === 'add' ? marks.new.get(op.newIndex) : undefined,
                  )}{/if}{#if note?.move !== undefined}<span class="move-note"
                    >{#if onPick}<button class="note-link" title="Go to this move" onclick={(e) => (e.stopPropagation(), goToMove(note.move!))}>{note.text}</button
                      >{:else}{note.text}{/if}{#if diff.moves[note.move].changed && onCompare}<button
                        class="note-link compare"
                        onclick={(e) => (e.stopPropagation(), onCompare(note.move!))}>Compare with the original</button
                      >{/if}</span
                  >{:else if note}<span class="block-note {note.kind}">{note.text}</span>{/if}</td
              >
            </tr>
          {/each}
        </tbody>
      </table>
    </section>
  {/each}

  {#if selected.size > 0}
    <footer>
      <span>{selected.size} {selected.size === 1 ? 'line' : 'lines'} selected</span>
      {#if actions.apply}
        <button class="btn sm secondary" disabled={busy} onclick={() => run('apply', (i) => selected.has(i), 'sel')}>{actions.apply.label} lines</button>
      {/if}
      {#if actions.revert}
        <button
          class={['btn sm', actions.revert.danger && confirming === 'sel' ? 'danger' : 'quiet']}
          disabled={busy}
          onclick={() => run('revert', (i) => selected.has(i), 'sel')}
          >{confirming === 'sel' ? `Really ${actions.revert.label.toLowerCase()}?` : `${actions.revert.label} lines`}</button
        >
      {/if}
      <button class="btn sm text" onclick={() => selected.clear()}>Clear</button>
    </footer>
  {/if}
</div>

<style>
  .unified {
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.6;
    tab-size: 4;
  }

  .hunk {
    margin-bottom: 10px;
  }

  header {
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    gap: 6px;
    align-items: center;
    min-height: 32px;
    padding: 3px 8px 3px 14px;
    background: var(--paper-warm);
    border-bottom: 1px solid var(--paper-edge);
    color: var(--accent-b);
  }

  header .mono {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  table {
    border-collapse: collapse;
    width: 100%;
  }

  td {
    padding: 0 8px;
    white-space: pre;
    vertical-align: top;
  }

  .num {
    width: 1%;
    text-align: right;
    color: var(--ink-faint);
    user-select: none;
  }

  .sign {
    width: 1%;
    padding: 0 4px;
    user-select: none;
  }

  .code {
    width: 100%;
  }

  /* Wrapped lines keep their numbers and sign on the first visual line (cells align top). */
  .wrap .code {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  tr.add,
  tr.del {
    cursor: pointer;
  }

  tr.add {
    background: var(--success-wash);
  }

  tr.del {
    background: var(--danger-wash);
  }

  tr.add .sign {
    color: var(--success);
  }

  tr.del .sign {
    color: var(--danger);
  }

  /* A moved line reads in its move's colour, whichever side it's on; its sign still says
     which. Several moves cycle teal, plum, sepia and ochre, never the add or delete colours. */
  .t0,
  .r0 {
    --tone: var(--syn-string);
  }

  .t1,
  .r1 {
    --tone: var(--lane-2);
  }

  .t2,
  .r2 {
    --tone: var(--lane-3);
  }

  .t3,
  .r3 {
    --tone: var(--syn-number);
  }

  tr.moved {
    background: color-mix(in srgb, var(--tone, var(--syn-string)) 12%, transparent);
  }

  tr.moved .sign {
    color: var(--tone, var(--syn-string));
  }

  /* A token that moved within its change, such as a reordered argument. */
  tr .chg.move {
    background: color-mix(in srgb, var(--syn-string) 30%, transparent);
  }

  .chg {
    border-radius: 2px;
  }

  tr.del .chg {
    background: color-mix(in srgb, var(--danger) 35%, transparent);
  }

  tr.add .chg {
    background: color-mix(in srgb, var(--success) 35%, transparent);
  }

  tr.current .num:first-child {
    box-shadow: inset 3px 0 0 var(--syn-keyword);
  }

  /* Formatting is grey, with its text underlined in dots; a rename is ochre. */
  tr.format {
    background: color-mix(in srgb, var(--code-punctuation) 7%, transparent);
  }

  tr.format .sign {
    color: var(--code-punctuation);
  }

  tr.rename {
    background: color-mix(in srgb, var(--syn-number) 9%, transparent);
  }

  tr.rename .sign {
    color: var(--syn-number);
  }

  /* As specific as the added and removed rows' own marks, which they replace. */
  tr .chg.format {
    background: none;
    text-decoration: underline dotted var(--code-punctuation);
    text-underline-offset: 4px;
  }

  /* Each rename has its colour; a click lights up all its places. */
  tr .chg.rename {
    background: color-mix(in srgb, var(--tone, var(--syn-number)) 34%, transparent);
    cursor: pointer;
  }

  tr .chg.rename.hot {
    background: color-mix(in srgb, var(--tone, var(--syn-number)) 62%, transparent);
    outline: 1px solid var(--tone, var(--syn-number));
  }

  .tag {
    flex: none;
    padding: 0 7px;
    border: 1px dotted var(--ink-faint);
    border-radius: 4px;
    color: var(--ink-soft);
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .block-note {
    padding-left: 12px;
    color: var(--ink-mute);
    font-family: var(--font-serif);
    font-style: italic;
    font-size: 12px;
    user-select: none;
  }

  .block-note.rename {
    color: var(--syn-number);
  }

  .note-link {
    padding: 0;
    border: 0;
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }

  .note-link:hover {
    text-decoration: underline;
  }

  .note-link.compare {
    margin-left: 10px;
    font-style: normal;
    text-decoration: underline;
    text-underline-offset: 3px;
  }

  .move-note {
    padding-left: 12px;
    color: var(--tone, var(--syn-string));
    font-family: var(--font-serif);
    font-style: italic;
    font-size: 12px;
    user-select: none;
  }

  tr.sel {
    outline: 1px solid var(--link);
    outline-offset: -1px;
    background-image: linear-gradient(var(--selection), var(--selection));
  }

  footer {
    position: sticky;
    bottom: 0;
    display: flex;
    gap: 8px;
    align-items: center;
    padding: 8px 12px;
    background: var(--paper-warm);
    border-top: 1px solid var(--paper-edge);
    font-family: var(--font-serif);
    font-size: 14px;
  }

  footer span {
    flex: 1;
  }

  .muted {
    display: flex;
    align-items: center;
    gap: 12px;
    margin: 0;
    padding: 10px 14px;
    color: var(--ink-mute);
    font-family: var(--font-serif);
    font-style: italic;
  }
</style>
