<script lang="ts">
  // The list under the search box: the text search first, always, then the trailer keys and
  // values that what's typed could mean. Picking a value filters the graph by it; picking a
  // key writes it in the box, to choose among its values.
  //
  // The box keeps the focus and the keys: this only shows the rows and takes clicks.
  import { isKey, markParts, personParts, suggestionLabel, type Suggestion } from './suggest'

  let {
    id,
    query,
    items,
    match,
    active,
    soFar = false,
    find,
    onPick,
    onSearch,
    onFind,
    history = false,
    onHistory,
  }: {
    /** The list's id, which the box names as what it controls; rows are `${id}-${n}`. */
    id: string
    /** What's typed, for the first row. */
    query: string
    items: Suggestion[]
    /** The part of what's typed that the rows were found by, to mark in them. */
    match: string
    /** The row the keys are on: 0 is the text search, 1 the first suggestion. */
    active: number
    /** Trailers are still being read: the rows are of what's read so far. */
    soFar?: boolean
    onPick: (item: Suggestion) => void
    /** The first row was clicked: search for the text as it is. */
    onSearch: () => void
    /** Find in files' row, second (#980), with what it counted, or undefined while counting.
     * Without it the trailer suggestions follow the first row. */
    find?: { count?: string }
    onFind?: () => void
    /** The history search's row, after Find's (#982): the commits that add or remove the
     * text, as a filter on the graph. */
    history?: boolean
    onHistory?: () => void
  } = $props()

  // The trailer suggestions' rows come after the text search's, Find's and the history search's.
  let historyAt = $derived(find ? 2 : 1)
  let first = $derived(historyAt + (history ? 1 : 0))
</script>

<!-- A click must not take the focus from the box: the list closes when the box loses it. -->
<div class="suggest" {id} role="listbox" aria-label="Search suggestions" tabindex="-1" onmousedown={(e) => e.preventDefault()}>
  <div class="row text" id="{id}-0" role="option" tabindex="-1" aria-selected={active === 0} class:active={active === 0} onclick={onSearch} onkeydown={() => {}}>
    <span class="what">Search for</span>
    <span class="value">“{query.trim()}”</span>
    <span class="count">Enter: next match</span>
  </div>
  {#if find}
    <div class="row text find" id="{id}-1" role="option" tabindex="-1" aria-selected={active === 1} class:active={active === 1} onclick={() => onFind?.()} onkeydown={() => {}}>
      <span class="what">Find</span>
      <span class="value">“{query.trim()}” in the files</span>
      <span class="count">{find.count ?? '…'}</span>
    </div>
  {/if}
  {#if history}
    <div class="row text history" id="{id}-{historyAt}" role="option" tabindex="-1" aria-selected={active === historyAt} class:active={active === historyAt} onclick={() => onHistory?.()} onkeydown={() => {}}>
      <span class="what">Commits</span>
      <span class="value">that add or remove “{query.trim()}”</span>
      <span class="count">filter</span>
    </div>
  {/if}
  {#each items as item, n (item.key + '\n' + item.value)}
    <div class="row" id="{id}-{n + first}" role="option" tabindex="-1" aria-selected={active === n + first} aria-label={suggestionLabel(item)} class:active={active === n + first} onclick={() => onPick(item)} onkeydown={() => {}}>
      {#if isKey(item)}
        <span class="key own"
          >{#each markParts(item.keyLabel, match) as p, i (i)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}:</span
        >
        <span class="value hint">choose a value</span>
      {:else}
        <span class="key">{item.keyLabel}</span>
        {#if item.kind === 'person'}
          {@const who = personParts(item.label)}
          <span class="value"
            >{#each markParts(who.name, match) as p, i (i)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}{#if who.email}<span class="email"
                >{#each markParts(who.email, match) as p, i (i)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span
              >{/if}</span
          >
        {:else}
          <span class="value"
            >{#each markParts(item.label, match) as p, i (i)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span
          >
        {/if}
      {/if}
      <span class="count">{item.commits.toLocaleString()}</span>
    </div>
  {/each}
  {#if soFar}
    <p class="so-far">Trailers are still being read: these are the ones found so far.</p>
  {/if}
</div>

<style>
  .suggest {
    position: absolute;
    top: calc(100% + 4px);
    right: 0;
    z-index: 30;
    width: min(420px, 90vw);
    padding: 4px;
    box-sizing: border-box;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    background: var(--paper);
    box-shadow: var(--shadow-pop, 0 6px 24px rgb(0 0 0 / 0.16));
    color: var(--ink);
    font-size: 13px;
    text-align: left;
  }

  .row {
    display: flex;
    align-items: baseline;
    gap: 8px;
    min-height: 28px;
    padding: 4px 8px;
    box-sizing: border-box;
    border-radius: var(--r-sm);
    cursor: pointer;
  }

  .row.active {
    background: var(--selected);
  }

  .row:hover:not(.active) {
    background: color-mix(in srgb, var(--selected) 45%, transparent);
  }

  .row.text {
    margin-bottom: 2px;
  }

  .what {
    flex: none;
    color: var(--ink-mute);
  }

  .key {
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .key.own {
    color: var(--ink);
  }

  .value {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 600;
  }

  .value.hint {
    font-weight: 400;
    font-style: italic;
    color: var(--ink-mute);
  }

  .email {
    margin-left: 6px;
    font-weight: 400;
    color: var(--ink-mute);
  }

  .count {
    flex: none;
    margin-left: auto;
    color: var(--ink-mute);
    font-size: 12px;
    font-variant-numeric: tabular-nums;
  }

  mark {
    background: none;
    color: inherit;
    text-decoration: underline;
    text-decoration-color: var(--accent-a);
    text-decoration-thickness: 2px;
    text-underline-offset: 2px;
  }

  .so-far {
    margin: 4px 0 0;
    padding: 6px 8px 4px;
    border-top: 1px dashed var(--paper-edge);
    color: var(--ink-mute);
    font-size: 12px;
  }
</style>
