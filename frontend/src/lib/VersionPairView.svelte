<script lang="ts">
  import type { app } from '../../wailsjs/go/models'
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime.js'
  import Icon from './ui/Icon.svelte'
  import { aroundBy, differsSaid, markedRows, opens, parsePair, sideRows, type MarkedRow } from './versionPair'

  /** A changed pair of two versions of a branch, read (#1039), in the diff area: the new
   * version of the commit's change with what differs from the old marked, or the two side by
   * side. */
  let {
    compared,
    index,
    onClose,
    onStep,
  }: {
    compared: app.VersionsCompared
    /** The pair's place in compared.pairs. */
    index: number
    onClose: () => void
    /** Opens another pair, by its place. */
    onStep: (index: number) => void
  } = $props()

  const KEY = 'gitrevenge.pairView'
  const stored = (() => {
    try {
      return localStorage.getItem(KEY)
    } catch {
      return null
    }
  })()
  let view = $state<'marked' | 'side'>(stored === 'side' ? 'side' : 'marked')
  function choose(v: 'marked' | 'side') {
    view = v
    try {
      localStorage.setItem(KEY, v)
    } catch {
      // A per-viewer convenience: without storage it's only this time.
    }
  }

  let pair = $derived(compared.pairs[index])
  let sections = $derived(pair ? parsePair(pair.text) : [])
  let by = $derived(aroundBy(compared.pairs, index))
  let steps = $derived(compared.pairs.map((p, i) => (opens(p) ? i : -1)).filter((i) => i >= 0))
  let prev = $derived(steps.filter((i) => i < index).at(-1) ?? -1)
  let next = $derived(steps.find((i) => i > index) ?? -1)
  let copied = $state(false)
  const short = (id: string) => id.slice(0, 7)

  async function copy() {
    if (!pair || (await ClipboardSetText(pair.text).catch(() => false)) === false) return
    copied = true
    setTimeout(() => (copied = false), 1500)
  }
  const sign = (r: MarkedRow) => (r.inner === '+' || r.inner === '-' ? r.inner : r.inner === ' ' ? ' ' : '')
</script>

{#if pair}
  <section class="pair" aria-label="Pair {index + 1}: {pair.subject}">
    <header>
      <div class="titles">
        <span class="hand">pair {index + 1} of {compared.pairs.length}</span>
        <h2>{pair.subject}</h2>
        <p class="said">
          <span class="mono id">{short(pair.oldId)}</span> before → <span class="mono id">{short(pair.newId)}</span> now · {differsSaid(sections)}
        </p>
        {#if pair.oldSubject}<p class="said">Was “{pair.oldSubject}”.</p>{/if}
      </div>
      <div class="tools">
        <div class="toggle" role="group" aria-label="How to read it">
          <button class:on={view === 'marked'} aria-pressed={view === 'marked'} onclick={() => choose('marked')}>Marked</button>
          <button class:on={view === 'side'} aria-pressed={view === 'side'} onclick={() => choose('side')}>Side by side</button>
        </div>
        <button class="btn sm quiet" disabled={prev < 0} onclick={() => onStep(prev)}>← Previous pair</button>
        <button class="btn sm quiet" disabled={next < 0} onclick={() => onStep(next)}>Next pair →</button>
        <button class="btn sm text" onclick={copy}>{copied ? 'Copied' : 'Copy as text'}</button>
        <button class="btn sm quiet icon" aria-label="Close the pair" title="Close the pair" onclick={onClose}><Icon name="close" size={14} /></button>
      </div>
    </header>

    {#if pair.kind === 'around'}
      <p class="around">
        Its own lines are the same in both. Only lines around it differ{#if by}, because pair {compared.pairs.indexOf(by) + 1} changed them: <button class="link" onclick={() => onStep(compared.pairs.indexOf(by!))}>{by.subject}</button>{:else}, because something before it changed{/if}.
      </p>
    {/if}

    <div class="body">
      {#each sections as s (s.name)}
        <div class="section">
          <h3>{s.kind === 'message' ? 'The message' : s.kind === 'metadata' ? 'Its author and date' : s.name}</h3>
          {#if view === 'marked'}
            <div class="marked" role="table" aria-label="{s.name}, marked">
              {#each markedRows(s) as r, i (i)}
                <div class={['row', r.outer === '+' && 'new', r.outer === '-' && 'gone', r.inner === '+' && 'add', r.inner === '-' && 'del', r.inner === 'hunk' && 'hunk']} role="row">
                  <span class="mark" role="cell">{r.outer === '+' ? 'new' : r.outer === '-' ? 'gone' : ''}</span>
                  <span class="sign" role="cell">{sign(r)}</span>
                  <span class="text" role="cell">{#each r.pieces as p, k (k)}{#if p.marked}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span>
                </div>
              {/each}
            </div>
          {:else}
            <div class="side" role="table" aria-label="{s.name}, side by side">
              <div class="row head" role="row"><span role="columnheader">Before · {short(pair.oldId)}</span><span role="columnheader">Now · {short(pair.newId)}</span></div>
              {#each sideRows(s) as r, i (i)}
                <div class="row" role="row">
                  {#each [r.left, r.right] as half, h (h)}
                    <span class={['half', half && half.outer !== ' ' && (h === 0 ? 'gone' : 'new'), half?.inner === '+' && 'add', half?.inner === '-' && 'del', half?.inner === 'hunk' && 'hunk', !half && 'empty']} role="cell">
                      {#if half}<span class="sign">{sign(half)}</span><span class="text">{#each half.pieces as p, k (k)}{#if p.marked}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span>{/if}
                    </span>
                  {/each}
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {:else}
        <p class="said">Nothing in this pair differs.</p>
      {/each}
    </div>
  </section>
{/if}

<style>
  .pair {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: var(--paper);
  }

  header {
    display: flex;
    flex-wrap: wrap;
    gap: 10px 16px;
    align-items: flex-end;
    padding: 14px 18px 10px;
    border-bottom: 1px solid var(--paper-edge);
  }

  .titles {
    flex: 1 1 320px;
    min-width: 0;
  }

  .hand {
    font-family: var(--font-hand);
    font-size: 20px;
    color: var(--accent-a);
  }

  h2 {
    margin: 0;
    font-size: 19px;
    font-weight: 500;
    overflow-wrap: anywhere;
  }

  .said {
    margin: 2px 0 0;
    font-size: 13px;
    color: var(--ink-soft);
  }

  .mono,
  .marked,
  .side {
    font-family: var(--font-mono);
  }

  .id {
    font-size: 12px;
    color: var(--accent-b);
  }

  .tools {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
  }

  .toggle {
    display: flex;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    overflow: hidden;
  }

  .toggle button {
    border: 0;
    padding: 5px 12px;
    background: none;
    color: var(--ink);
    font: inherit;
    font-size: 13.5px;
    cursor: pointer;
  }

  .toggle button.on {
    background: var(--accent);
    color: var(--on-accent);
  }

  .around {
    margin: 0;
    padding: 10px 18px;
    font-size: 14px;
    line-height: 1.5;
    color: var(--ink-soft);
    background: var(--paper-warm);
  }

  .link {
    border: 0;
    padding: 0;
    background: none;
    color: var(--link);
    font: inherit;
    cursor: pointer;
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 12px 0 24px;
  }

  .section + .section {
    margin-top: 14px;
  }

  h3 {
    margin: 0 0 6px;
    padding: 0 18px;
    font-size: 14px;
    font-weight: 500;
  }

  .marked .row {
    display: grid;
    grid-template-columns: 52px 18px minmax(0, 1fr);
    font-size: 12.5px;
    line-height: 1.65;
  }

  .mark {
    padding-right: 8px;
    font-family: var(--font-serif, inherit);
    font-size: 12px;
    text-align: right;
  }

  .row.new .mark {
    color: var(--accent-a);
    box-shadow: inset -3px 0 0 var(--accent-a);
  }

  .row.gone .mark {
    color: var(--accent-b);
    box-shadow: inset -3px 0 0 var(--accent-b);
  }

  .sign {
    padding-left: 6px;
    color: var(--ink-mute);
  }

  .text {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .add .text {
    background: var(--success-wash);
  }

  .del .text {
    background: var(--danger-wash);
  }

  .hunk .text {
    color: var(--syn-keyword);
  }

  .row.gone .text,
  .half.gone .text {
    color: var(--ink-mute);
  }

  .marked .row.gone .text {
    text-decoration: line-through;
  }

  .row.new mark,
  .half.new mark {
    background: color-mix(in srgb, var(--accent-a) 24%, transparent);
    color: inherit;
    border-radius: 3px;
  }

  .row.gone mark,
  .half.gone mark {
    background: color-mix(in srgb, var(--accent-b) 20%, transparent);
    color: inherit;
    border-radius: 3px;
  }

  .side .row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 10px;
    padding: 0 18px;
    font-size: 12.5px;
    line-height: 1.65;
  }

  .side .row.head {
    font-family: var(--font-serif, inherit);
    font-size: 12.5px;
    color: var(--ink-mute);
    padding-bottom: 4px;
  }

  .half {
    display: grid;
    grid-template-columns: 18px minmax(0, 1fr);
    min-width: 0;
  }

  .half.gone {
    background: color-mix(in srgb, var(--accent-b) 10%, transparent);
  }

  .half.new {
    background: color-mix(in srgb, var(--accent-a) 10%, transparent);
  }

  .half.empty {
    background: repeating-linear-gradient(135deg, transparent 0 6px, var(--hairline) 6px 7px);
  }
</style>
