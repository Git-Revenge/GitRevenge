<script lang="ts" module>
  import { SvelteMap, SvelteSet } from 'svelte/reactivity'

  /** The files seen in conflict during the operation in progress, by repository, so the list
   * can keep showing the ones already settled. Forgotten when the operation ends. */
  const seenIn = new SvelteMap<string, SvelteSet<string>>()
</script>

<script lang="ts">
  import { untrack } from 'svelte'
  import { syntaxHighlighting, type LanguageSupport } from '@codemirror/language'
  import { EditorState } from '@codemirror/state'
  import { EditorView, lineNumbers } from '@codemirror/view'
  import { classHighlighter } from '@lezer/highlight'
  import { ConflictContext, ConflictFile, ConflictOrigin, OpenMergeTool, RepoState, ResolveConflict, TakeConflictSide } from '../../../wailsjs/go/app/App.js'
  import type { app, git } from '../../../wailsjs/go/models'
  import { session } from '../session.svelte'
  import Icon from '../ui/Icon.svelte'
  import Loading from '../ui/Loading.svelte'
  import { highlightLines, languageFor, type Token } from './highlight'
  import { nextPlace, parsePlaces, placeLines, sideView, sourceOf, startingResult, textFor, undecided, withChoice, type Parsed, type PlaceState, type Side, type Status } from './places'
  import { placeDecorations } from './placesDeco'
  import { currentField, placesField, setCurrent, setPlaces } from './placesField'

  /** The conflict tool (issue #597): the result is the main view, each conflicted place a block
   * in it with its own choices. Conflict markers are never shown and never saved. */
  let { path, file, onClose, onOpen }: { path: string; file: string; onClose: () => void; onOpen?: (file: string) => void } = $props()

  type Show = 'result' | 'two' | 'three'
  const SHOWS: { show: Show; label: string }[] = [
    { show: 'result', label: 'Result only' },
    { show: 'two', label: 'Ours and theirs' },
    { show: 'three', label: 'Three-way' },
  ]

  let cf = $state<git.ConflictFile | null>(null)
  let op = $state<app.RepoState | null>(null)
  let around = $state<app.ConflictContext | null>(null)
  let parsed = $state.raw<Parsed>({ text: [''], places: [] })
  let places = $state.raw<PlaceState[]>([])
  let current = $state(-1)
  let show = $state<Show>('two')
  let busy = $state(false)
  let inTool = $state(false)
  let lang = $state.raw<LanguageSupport | null>(null)

  let host = $state<HTMLDivElement>()
  let view: EditorView | undefined

  let left = $derived(undecided(places))
  let total = $derived(places.length)
  /** A side deleted the file, or it's binary: there are no lines to choose between. */
  let wholeOnly = $derived(!!cf && (cf.binary || !cf.ours.exists || !cf.theirs.exists))
  let hasBase = $derived(parsed.places.some((p) => p.base !== null))

  // In a rebase git swaps the meaning: "ours" is what's being rebased onto.
  let rebasing = $derived(op?.operation === 'rebase')
  let oursName = $derived((rebasing ? op?.theirs : op?.ours) || 'HEAD')
  let theirsName = $derived((rebasing ? op?.ours : op?.theirs) || (op?.commit ? op.commit.slice(0, 7) : 'incoming'))
  let oursIs = $derived(rebasing ? "what you're rebasing onto" : op?.operation === 'merge' ? "what you're merging into" : 'where you are')
  let theirsIs = $derived(rebasing ? 'your commit being replayed' : "what's coming in")

  async function refreshFiles() {
    const [st, ctx] = await Promise.all([RepoState(path).catch(() => null), ConflictContext(path).catch(() => null)])
    op = st
    around = ctx
    if (!st?.operation) seenIn.delete(path)
    else {
      const seen = seenIn.get(path) ?? new SvelteSet<string>()
      for (const f of st.conflicts) seen.add(f)
      seenIn.set(path, seen)
    }
  }

  let conflicted = $derived(op?.conflicts ?? [])
  let settled = $derived([...(seenIn.get(path) ?? [])].filter((f) => !conflicted.includes(f)).sort())

  async function load() {
    const c = await session.run(() => ConflictFile(path, file))
    if (!c) return
    lang = await languageFor(file)
    parsed = parsePlaces(sourceOf(c))
    cf = c
    current = parsed.places.length ? 0 : -1
  }

  $effect(() => {
    void file
    void load()
  })

  $effect(() => {
    void session.revision[path]
    void refreshFiles()
  })

  // The result editor, built once the file is loaded. Starting the file again rebuilds it.
  let round = $state(0)
  $effect(() => {
    void round
    if (!cf || wholeOnly || !host) return
    const parent = host
    // The editor reports its places as it's built, which mustn't count as this effect reading
    // them: it would run again for every change it caused.
    return untrack(() => build(parent))
  })

  function build(parent: HTMLDivElement) {
    const start = startingResult(parsed)
    const v = new EditorView({
      parent,
      state: EditorState.create({
        doc: start.doc,
        extensions: [
          lineNumbers(),
          syntaxHighlighting(classHighlighter),
          ...(lang ? [lang] : []),
          placesField,
          currentField,
          // Clicking a place puts the cursor there, ready to type.
          placeDecorations(parsed, { choose, go: (i) => go(i, true, true) }),
          EditorView.updateListener.of((u) => {
            const next = u.state.field(placesField)
            if (next !== places) places = next
            if (u.selectionSet && !u.transactions.some((t) => t.effects.some((e) => e.is(setCurrent)))) {
              // The place the cursor is in becomes the current one.
              const at = u.state.selection.main.head
              const i = next.findIndex((p) => at >= p.from && at <= p.to && p.from !== p.to)
              if (i >= 0 && i !== current) queueMicrotask(() => go(i, false))
            }
          }),
        ],
      }),
    })
    v.dispatch({ effects: [setPlaces.of(start.places), setCurrent.of(start.places.length ? 0 : -1)] })
    view = v
    return () => {
      v.destroy()
      view = undefined
    }
  }

  function choose(index: number, status: Status) {
    if (!view) return
    const now = view.state.field(placesField)
    const p = now[index]
    if (!p) return
    const text = textFor(parsed.places[index], status)
    view.dispatch({ changes: { from: p.from, to: p.to, insert: text }, effects: [setPlaces.of(withChoice(now, index, status, text)), setCurrent.of(index)] })
    current = index
  }

  function go(index: number, scroll = true, focus = false) {
    if (!view) return
    const p = view.state.field(placesField)[index]
    if (!p) return
    current = index
    view.dispatch({ effects: [setCurrent.of(index), ...(scroll ? [EditorView.scrollIntoView(p.from, { y: 'center' })] : [])], ...(scroll ? { selection: { anchor: p.from } } : {}) })
    if (focus) view.focus()
  }

  const step = (by: 1 | -1) => {
    const i = nextPlace(places, current, by)
    if (i >= 0) go(i)
  }

  // The sides of the current place, above the result.
  const SIDE_NAME: Record<Side, string> = { ours: 'Ours', base: 'Original', theirs: 'Theirs' }
  let panes = $derived.by(() => {
    if (show === 'result' || current < 0 || !parsed.places[current]) return []
    const sides: Side[] = show === 'three' && hasBase ? ['ours', 'base', 'theirs'] : ['ours', 'theirs']
    return sides.map((side) => {
      const v = sideView(parsed, current, side)
      const tokens: Token[][] = highlightLines(v.lines.join('\n'), lang)
      return { side, ...v, tokens }
    })
  })

  // Who last changed each side of the current place. Asked once per place and side.
  let origins = $state<Record<string, git.PlaceOrigin | null>>({})
  $effect(() => {
    const c = cf
    const i = current
    if (!c || i < 0 || show === 'result') return
    untrack(() => ask(c, i))
  })

  function ask(c: git.ConflictFile, i: number) {
    for (const side of ['ours', 'base', 'theirs'] as Side[]) {
      const key = `${i}:${side}`
      if (key in origins) continue
      const lines = placeLines(side === 'ours' ? c.ours.text : side === 'theirs' ? c.theirs.text : c.base.text, parsed, i, side)
      origins[key] = null
      if (!lines) continue
      void ConflictOrigin(path, file, side, lines.start, lines.end)
        .then((o) => {
          if (o?.commit) origins[key] = o
        })
        .catch(() => {})
    }
  }

  /** After a file is settled: on to the next conflicted one, or back if that was the last. */
  async function afterwards() {
    await refreshFiles()
    const next = conflicted.find((f) => f !== file)
    if (next && onOpen) onOpen(next)
    else onClose()
  }

  async function settle(run: () => Promise<void>) {
    busy = true
    const ok = await session.run(async () => {
      await run()
      return true
    })
    busy = false
    if (ok) await afterwards()
  }

  const markResolved = () => {
    if (left || busy || !view) return
    const result = view.state.doc.toString()
    void settle(() => ResolveConflict(path, file, result))
  }
  const take = (side: 'ours' | 'theirs') => void settle(() => TakeConflictSide(path, file, side))

  function startAgain() {
    origins = {}
    round++
    current = parsed.places.length ? 0 : -1
  }

  async function mergeTool() {
    inTool = true
    await session.run(() => OpenMergeTool(path, file))
    inTool = false
    await refreshFiles()
    // The tool resolved it: there's nothing left to do here.
    if (!conflicted.includes(file)) {
      const next = conflicted[0]
      if (next && onOpen) onOpen(next)
      else onClose()
    }
  }

  function keys(e: KeyboardEvent) {
    // A dialog or menu over the tool has the keys.
    if (e.defaultPrevented || document.querySelector('[aria-modal="true"], [role="menu"]')) return
    if (e.key === 'F7') {
      e.preventDefault()
      step(e.shiftKey ? -1 : 1)
      return
    }
    // 1 and 2 take a side for the current place: bare when not typing, with Alt anywhere.
    const typing = (e.target as HTMLElement).closest?.('.cm-content, input, textarea')
    if ((e.key === '1' || e.key === '2') && current >= 0 && !wholeOnly && (e.altKey || !typing) && !e.ctrlKey && !e.metaKey) {
      e.preventDefault()
      choose(current, e.key === '1' ? 'ours' : 'theirs')
    }
  }

  const count = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`
  let blocked = $derived(left ? `${count(left, 'place is', 'places are')} still undecided` : '')
</script>

<!-- The keys are heard at the window: on WebKit a clicked button doesn't take focus, so a
     listener on the tool itself would miss a key pressed after a click. -->
<svelte:window onkeydown={keys} />

<div class="conflict">
  <header class="view-head">
    <button class="btn quiet" onclick={onClose} title="Back"><Icon name="back" />Back</button>
    <span class="view-badge danger">Conflict</span>
    <span class="view-file" title={file}>{file}</span>
    {#if cf && !wholeOnly && total}
      <span class={['tag', left ? 'left' : 'green']} role="status">{left ? `${left} of ${count(total, 'place', 'places')} left` : `${total === 1 ? 'The place is' : `All ${total} places are`} decided`}</span>
    {/if}
    <span class="spacer"></span>
    {#if cf && !wholeOnly}
      {#if total > 1}
        <button class="btn sm quiet" onclick={() => step(-1)} title="The previous place (Shift+F7)"><span class="flip"><Icon name="chevron" size={14} /></span>Previous</button>
        <button class="btn sm quiet" onclick={() => step(1)} title="The next place (F7)"><Icon name="chevron" size={14} />Next <kbd>F7</kbd></button>
      {/if}
      {#if total}
        <span class="segmented" role="group" aria-label="Which sides to show above the result">
          {#each SHOWS as s (s.show)}
            <button aria-pressed={show === s.show} disabled={s.show === 'three' && !hasBase} title={s.show === 'three' && !hasBase ? "The original isn't known for this file" : undefined} onclick={() => (show = s.show)}>{s.label}</button>
          {/each}
        </span>
      {/if}
      <button class={['btn primary', left && 'off']} aria-disabled={left > 0 || undefined} disabled={busy} title={blocked || 'Save the result and mark the file resolved'} onclick={markResolved}>Mark resolved</button>
    {/if}
  </header>

  <div class="body">
    <div class="work">
      {#if !cf}
        <Loading />
      {:else if wholeOnly}
        <div class="whole">
          <p>
            {#if cf.binary}This file is binary, so it can't be merged line by line.{:else if !cf.ours.exists}Ours ({oursName}) deleted this file and theirs ({theirsName}) changed it.{:else}Theirs ({theirsName}) deleted this file and ours ({oursName})
              changed it.{/if}
            Choose a whole version.
          </p>
          <div class="pair">
            <button class="btn secondary" disabled={busy} onclick={() => take('ours')}>{cf.ours.exists ? 'Take ours' : 'Delete it, as ours did'}</button>
            <button class="btn secondary" disabled={busy} onclick={() => take('theirs')}>{cf.theirs.exists ? 'Take theirs' : 'Delete it, as theirs did'}</button>
          </div>
        </div>
      {:else}
        {#if panes.length}
          <div class="panes code-ground" style:--cols={panes.length} aria-label="The versions of this place, read-only">
            {#each panes as pane (pane.side)}
              {@const origin = origins[`${current}:${pane.side}`]}
              <section class="pane {pane.side}">
                <h4>
                  <span class="name">{SIDE_NAME[pane.side]}</span>
                  <span class="whose" title={pane.side === 'ours' ? oursIs : pane.side === 'theirs' ? theirsIs : ''}>{pane.side === 'ours' ? oursName : pane.side === 'theirs' ? theirsName : 'where both started'}</span>
                  <span class="spacer"></span>
                  {#if pane.side !== 'base'}
                    <button class="chip" onclick={() => choose(current, pane.side as Status)} title="Take {pane.side} for this place">Take <kbd>{pane.side === 'ours' ? '1' : '2'}</kbd></button>
                  {/if}
                </h4>
                <pre>{#each pane.tokens as line, n (n)}<span class={['ln', n >= pane.from && n < pane.to && 'own']}><span class="n">{pane.first + n}</span><span class="t">{#each line as t, k (k)}<span class={t.cls}>{t.text}</span>{/each}</span>
</span>{/each}{#if pane.from === pane.to}<span class="ln none"><span class="n"></span><span class="t">nothing here on this side</span>
</span>{/if}</pre>
                <p class="origin">{#if origin}{origin.author} · {origin.subject}{:else}&nbsp;{/if}</p>
              </section>
            {/each}
          </div>
        {/if}
        <section class="result">
          <h4 class="meta">
            <span>Result</span>
            <span class="plain">editable: type anywhere, as in any editor</span>
            <span class="spacer"></span>
            <span class="plain">saved without conflict markers</span>
          </h4>
          {#key round}
            <div class="editor code-ground" bind:this={host}></div>
          {/key}
        </section>
      {/if}
    </div>

    <aside class="files" aria-label="Files in this {op?.operation || 'operation'}">
      <div>
        <span class="hand">both sides changed these</span>
        <h3>{conflicted.length ? `${count(conflicted.length, 'file', 'files')} to settle` : 'Nothing left to settle'}</h3>
      </div>
      <ul class="list">
        {#each conflicted as f (f)}
          <li class:on={f === file}>
            <span class="ic warn"><Icon name="warning" size={16} /></span>
            {#if f === file || !onOpen}<span class="mono name" title={f}>{f}</span>{:else}<button class="btn sm text name" title="Open {f}" onclick={() => onOpen(f)}>{f}</button>{/if}
            {#if f === file && !wholeOnly && total}<span class="note">{total - left} of {count(total, 'place', 'places')}</span>{/if}
          </li>
        {/each}
        {#each settled as f (f)}
          <li><span class="ic ok"><Icon name="check" size={16} /></span><span class="mono name" title={f}>{f}</span><span class="note">resolved</span></li>
        {/each}
      </ul>

      {#if cf}
        <div class="card">
          <span class="meta">For the whole file</span>
          {#if !wholeOnly}
            <div class="pair">
              <button class="btn sm secondary" disabled={busy} onclick={() => take('ours')} title="Resolve with ours throughout ({oursName})">Take ours</button>
              <button class="btn sm secondary" disabled={busy} onclick={() => take('theirs')} title="Resolve with theirs throughout ({theirsName})">Take theirs</button>
            </div>
            <button class="btn sm text" disabled={busy || (total > 0 && left === total)} onclick={startAgain}>Start this file again</button>
          {/if}
          {#if around?.mergeTool}
            <button class="btn sm text" disabled={busy || inTool} onclick={mergeTool} title="Opens {file} in {around.mergeTool}, from your git config">{inTool ? `Waiting for ${around.mergeTool}…` : `Open in ${around.mergeTool}`}</button>
          {/if}
        </div>
      {/if}

      {#if around?.clean.length}
        <div class="clean">
          <span class="meta">Merged cleanly · {around.clean.length}</span>
          <ul>
            {#each around.clean.slice(0, 8) as f (f)}<li class="mono" title={f}>{f}</li>{/each}
            {#if around.clean.length > 8}<li class="note">and {around.clean.length - 8} more</li>{/if}
          </ul>
        </div>
      {/if}
    </aside>
  </div>
</div>

<style>
  .conflict {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    container: conflict / inline-size;
  }

  .spacer {
    flex: 1;
  }

  .view-head {
    flex-wrap: wrap;
  }

  kbd {
    padding: 0 5px;
    border-radius: var(--r-sm);
    background: var(--paper-deep);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 18px;
    color: var(--ink-soft);
  }

  .flip {
    display: inline-flex;
    transform: rotate(180deg);
  }

  .tag.left {
    background: var(--warning-wash);
    color: var(--warning);
  }

  /* Off until every place is decided, but still there to say why. */
  .btn.off {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .body {
    flex: 1;
    min-height: 0;
    display: flex;
  }

  .work {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .whole {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
    padding: 24px;
    color: var(--ink-soft);
  }

  .whole p {
    margin: 0;
    max-width: 60ch;
  }

  .pair {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  /* The sides of the current place: read-only, a few lines each. */
  .panes {
    flex: none;
    display: grid;
    grid-template-columns: repeat(var(--cols), minmax(0, 1fr));
    max-height: 40%;
    border-bottom: 2px solid var(--code-edge, var(--paper-edge));
    overflow: auto;
    font-size: 13px;
  }

  .pane {
    min-width: 0;
    display: flex;
    flex-direction: column;
    border-right: 1px solid var(--code-edge, rgba(255, 255, 255, 0.12));
  }

  .pane:last-child {
    border-right: none;
  }

  .pane h4 {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 32px;
    margin: 0;
    padding: 2px 8px 2px 12px;
    font-family: var(--font-serif);
    font-size: 13px;
    font-weight: 600;
    color: var(--code-text);
    background: rgba(255, 255, 255, 0.06);
  }

  .whose {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-weight: 400;
    color: var(--code-comment);
  }

  .pane pre {
    flex: 1;
    margin: 0;
    padding: 4px 0;
    overflow-x: auto;
    font-family: var(--font-mono);
    color: var(--code-text);
    tab-size: 4;
  }

  .ln {
    display: block;
    white-space: pre;
  }

  .ln .n {
    display: inline-block;
    width: 36px;
    padding-right: 10px;
    text-align: right;
    color: var(--code-comment);
    user-select: none;
  }

  .pane.ours .ln.own {
    background: color-mix(in srgb, var(--code-string) 22%, transparent);
  }

  .pane.theirs .ln.own {
    background: color-mix(in srgb, var(--code-function) 26%, transparent);
  }

  .pane.base .ln.own {
    background: rgba(255, 255, 255, 0.08);
  }

  .ln.none .t {
    font-family: var(--font-serif);
    font-style: italic;
    color: var(--code-comment);
  }

  .origin {
    margin: 0;
    padding: 2px 12px 6px;
    font-family: var(--font-serif);
    font-size: 12px;
    color: var(--code-comment);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .chip,
  .editor :global(.chip) {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 24px;
    padding: 0 10px;
    border: 1px solid rgba(255, 255, 255, 0.22);
    border-radius: 999px;
    background: none;
    font-family: var(--font-serif);
    font-size: 13px;
    color: var(--code-text);
    cursor: pointer;
  }

  .chip:hover,
  .editor :global(.chip:hover:not(:disabled)) {
    background: rgba(255, 255, 255, 0.1);
  }

  .editor :global(.chip.on) {
    border-color: transparent;
    background: var(--code-text);
    color: var(--code-bg);
  }

  .editor :global(.chip:disabled) {
    cursor: default;
  }

  .chip kbd {
    background: rgba(255, 255, 255, 0.16);
    color: var(--code-text);
  }

  .result {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }

  .result h4 {
    display: flex;
    gap: 12px;
    margin: 0;
    padding: 8px 14px 6px;
    font-weight: normal;
  }

  .plain {
    text-transform: none;
    letter-spacing: 0;
    font-family: var(--font-serif);
    color: var(--ink-mute);
  }

  .editor {
    flex: 1;
    min-height: 0;
    overflow: auto;
    font-size: 13px;
    tab-size: 4;
  }

  .editor :global(.cm-editor) {
    height: 100%;
    background: var(--code-bg);
    color: var(--code-text);
  }

  .editor :global(.cm-gutters) {
    background: var(--code-bg);
    color: var(--code-comment);
    border-right: 1px solid rgba(255, 255, 255, 0.08);
  }

  .editor :global(.cm-content) {
    font-family: var(--font-mono);
    caret-color: var(--code-text);
  }

  .editor :global(.cm-cursor) {
    border-left-color: var(--code-text);
  }

  .editor :global(.cm-focused) {
    outline: none;
  }

  /* A place in the result: its heading with the choices, then its own lines. */
  .editor :global(.place) {
    margin-top: 2px;
    border-top: 1px solid rgba(255, 255, 255, 0.16);
    font-family: var(--font-serif);
    cursor: default;
  }

  .editor :global(.place-head) {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    min-height: 34px;
    padding: 4px 10px 4px 14px;
    background: rgba(255, 255, 255, 0.07);
    color: var(--code-text);
  }

  .editor :global(.place.current .place-head) {
    background: rgba(255, 255, 255, 0.13);
    box-shadow: inset 3px 0 0 var(--code-keyword);
  }

  .editor :global(.place-label) {
    font-size: 13px;
    font-weight: 600;
  }

  .editor :global(.place.undecided .place-label) {
    color: var(--code-number);
  }

  .editor :global(.place-gap) {
    flex: 1;
  }

  .editor :global(.place-sides) {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    border-bottom: 1px solid rgba(255, 255, 255, 0.16);
  }

  .editor :global(.place-side) {
    min-width: 0;
    padding: 6px 14px 8px;
    opacity: 0.85;
  }

  .editor :global(.place-side.theirs) {
    border-left: 1px solid rgba(255, 255, 255, 0.12);
  }

  .editor :global(.place-side-name) {
    display: block;
    font-size: 12px;
    color: var(--code-comment);
  }

  .editor :global(.place-side-text) {
    margin: 2px 0 0;
    overflow-x: auto;
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--code-text);
  }

  .editor :global(.place-side-none) {
    font-size: 13px;
    font-style: italic;
    color: var(--code-comment);
  }

  .editor :global(.place-line.ours) {
    background: color-mix(in srgb, var(--code-string) 18%, transparent);
  }

  .editor :global(.place-line.theirs) {
    background: color-mix(in srgb, var(--code-function) 22%, transparent);
  }

  .editor :global(.place-line.both) {
    background: rgba(255, 255, 255, 0.07);
  }

  .editor :global(.place-line.edited) {
    background: color-mix(in srgb, var(--code-number) 16%, transparent);
  }

  /* The files of the operation, beside the tool. */
  .files {
    flex: none;
    width: 300px;
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 18px 18px;
    border-left: 1px solid var(--paper-edge);
    overflow-y: auto;
  }

  h3 {
    margin: 0;
    font-size: 18px;
    font-weight: 500;
    letter-spacing: -0.01em;
  }

  .list,
  .clean ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .list li {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 36px;
    padding: 0 8px;
    border-radius: var(--r-sm);
  }

  .list li.on {
    background: var(--selected);
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
    justify-content: flex-start;
  }

  .note {
    flex: none;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .ic {
    display: inline-flex;
    flex: none;
  }

  .ic.warn {
    color: var(--warning);
  }

  .ic.ok {
    color: var(--success);
  }

  .card {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
    padding: 12px 14px;
    border-radius: var(--r-md);
    background: var(--paper-warm);
  }

  .clean li {
    min-height: 26px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--ink-soft);
  }

  /* In a narrow column the files go under the tool. */
  @container conflict (max-width: 760px) {
    .body {
      flex-direction: column;
      overflow-y: auto;
    }

    .work {
      flex: none;
      min-height: 420px;
    }

    .files {
      width: auto;
      border-left: none;
      border-top: 1px solid var(--paper-edge);
      overflow: visible;
    }
  }
</style>
