<script lang="ts">
  // The sidebar's Smart filters panel: what the trailers on a repository's commits offer to
  // filter by. Each key is a folder, with the kind of thing its values are beside it, and its
  // values under it with how many commits have each. Picking a value filters the graph to
  // it; picking it again takes it out of the filter.
  //
  // Nothing here is written to a commit. It's a reading of what's there already, and the
  // name is "Smart filters", never "tags", which means git tags.
  import { TrailerMore, TrailerPanel } from '../../../wailsjs/go/app/App.js'
  import type { app, trailerindex } from '../../../wailsjs/go/models'
  import { errorText } from '../errors'
  import { onBackend } from '../events'
  import { typeahead } from '../menuKeys'
  import { session } from '../session.svelte'
  import { matchParts } from '../sidebarTree'
  import Empty from '../ui/Empty.svelte'
  import Icon from '../ui/Icon.svelte'
  import { commitsText, folderKey, hasPick, heldText, heldWhy, isOpen, kindWord, moreText, queryFor, soFar, valueName } from './filters'

  let {
    path,
    filter,
    picks,
    open,
    onPick,
    onSearch,
    onStatus,
  }: {
    path: string
    /** The sidebar's filter: narrows the keys by name, and the rest to their values. */
    filter: string
    /** The values the graph is filtered by, which show as picked. */
    picks: { key: string; value: string }[]
    /** The sidebar's open folders, by key: this panel's are kept with the branch folders'. */
    open: Record<string, boolean>
    /** Adds a value to the graph's filter, or takes it out if it's there. */
    onPick: (key: string, value: string) => void
    /** Searches the graph for text: a key held back from the list can still be searched for. */
    onSearch: (search: string) => void
    /** Reports how many keys and values match the filter, for the rail's count. */
    onStatus?: (status: { matches: number }) => void
  } = $props()

  const uid = $props.id()

  let panel = $state<app.TrailerPanel | null>(null)
  let error = $state('')
  let showHeld = $state(false)
  /** Numbers each ask, so a slow answer can't replace a newer one. */
  let asks = 0

  async function load() {
    const n = ++asks
    try {
      const p = await TrailerPanel(path, filter, 0)
      if (n !== asks) return
      panel = p
      error = ''
      onStatus?.({ matches: p.matches })
    } catch (e) {
      if (n === asks) error = errorText(e)
    }
  }

  // The filter is typed a letter at a time, and each ask reads through every value.
  let waiting: ReturnType<typeof setTimeout> | undefined
  let first = true
  $effect(() => {
    void path
    void filter
    // A changed mailmap, smart link or bot pattern changes what's shown, with no new commit.
    void session.revision[path]
    clearTimeout(waiting)
    if (first) {
      first = false
      void load()
    } else waiting = setTimeout(() => void load(), 180)
    return () => clearTimeout(waiting)
  })

  // While the history is being read the counts climb: look again now and then, and once
  // more when it's done.
  let lastLook = 0
  $effect(() =>
    onBackend('trailers:progress', (p) => {
      if (p.path !== path) return
      const now = Date.now()
      if (p.reading && now - lastLook < 3000) return
      lastLook = now
      void load()
    }),
  )

  /** More of one key's values, asked for with "24 more…". */
  async function more(k: app.TrailerPanelKey) {
    try {
      const longer = await TrailerMore(path, k.key.id, filter, k.top.length + 50)
      if (!panel) return
      panel.keys = panel.keys.map((have) => (have.key.id === longer.key.id ? longer : have))
    } catch (e) {
      error = errorText(e)
    }
  }

  // The rows, as the keyboard walks them: a key, its values while it's open, and "more…".
  type Row =
    | { type: 'key'; id: string; label: string; k: app.TrailerPanelKey; open: boolean }
    | { type: 'value'; id: string; label: string; k: app.TrailerPanelKey; v: trailerindex.Value }
    | { type: 'more'; id: string; label: string; k: app.TrailerPanelKey }
  let filtering = $derived(filter.trim().length > 0)
  let rows = $derived.by(() => {
    const out: Row[] = []
    for (const [i, k] of (panel?.keys ?? []).entries()) {
      // While filtering, folders are held open to show what matched.
      const isOpenNow = filtering || isOpen(open, k.key.id, i)
      out.push({ type: 'key', id: `k:${k.key.id}`, label: k.key.label, k, open: isOpenNow })
      if (!isOpenNow) continue
      for (const v of k.top) out.push({ type: 'value', id: `v:${k.key.id}\n${v.id}`, label: valueName(v), k, v })
      if (k.more > 0) out.push({ type: 'more', id: `m:${k.key.id}`, label: moreText(k.more), k })
    }
    return out
  })

  // The list is one Tab stop. Focus stays on it, and aria-activedescendant names the row.
  let list = $state<HTMLElement>()
  let activeId = $state('')
  let active = $derived(Math.max(0, rows.findIndex((r) => r.id === activeId)))
  const rowId = (i: number) => `${uid}-row-${i}`

  function setActive(i: number) {
    const row = rows[Math.min(Math.max(i, 0), rows.length - 1)]
    if (!row) return
    activeId = row.id
    document.getElementById(rowId(rows.indexOf(row)))?.scrollIntoView({ block: 'nearest' })
  }

  function toggle(k: app.TrailerPanelKey, isOpenNow: boolean) {
    if (!filtering) open[folderKey(k.key.id)] = !isOpenNow
  }

  function act(row: Row) {
    activeId = row.id
    if (row.type === 'key') toggle(row.k, row.open)
    else if (row.type === 'more') void more(row.k)
    // Picking a picked value takes it out again.
    else onPick(row.k.key.id, row.v.id)
  }

  let typed = ''
  let typedTimer: ReturnType<typeof setTimeout> | undefined
  function onKey(e: KeyboardEvent) {
    const row = rows[active]
    if (!row) return
    const typing = e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey && (e.key !== ' ' || typed !== '')
    if (typing) {
      e.preventDefault()
      clearTimeout(typedTimer)
      typed += e.key
      typedTimer = setTimeout(() => (typed = ''), 600)
      const found = typeahead(rows, active, typed)
      if (found >= 0) setActive(found)
      return
    }
    const parent = () => rows.findIndex((r) => r.type === 'key' && r.k === row.k)
    let handled = true
    if (e.key === 'ArrowDown') setActive(active + 1)
    else if (e.key === 'ArrowUp') setActive(active - 1)
    else if (e.key === 'Home') setActive(0)
    else if (e.key === 'End') setActive(rows.length - 1)
    else if (e.key === 'ArrowRight') {
      if (row.type === 'key' && !row.open) toggle(row.k, false)
      else if (row.type === 'key') setActive(active + 1)
    } else if (e.key === 'ArrowLeft') {
      if (row.type === 'key' && row.open) toggle(row.k, true)
      else if (row.type !== 'key') setActive(parent())
    } else if (e.key === 'Enter' || e.key === ' ') act(row)
    else handled = false
    if (handled) e.preventDefault()
  }

  let note = $derived(panel ? soFar(panel.status) : '')
  let nothing = $derived(!!panel && panel.keys.length === 0 && panel.held.length === 0 && !filtering)
</script>

{#if error}
  <p class="note failed" role="alert">{error}</p>
{/if}
{#if note}
  <p class="note meta" role="status">{note}</p>
{/if}

{#if panel && rows.length}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <ul
    class="keys"
    role="tree"
    aria-label="Smart filters"
    tabindex="0"
    bind:this={list}
    aria-activedescendant={rowId(active)}
    onkeydown={onKey}
    onfocus={() => setActive(active)}
  >
    {#each rows as row, i (row.id)}
      {#if row.type === 'key'}
        {@const key = row.k.key}
        {@const parts = matchParts(key.label, filter)}
        <li
          id={rowId(i)}
          role="treeitem"
          aria-level="1"
          aria-expanded={row.open}
          aria-selected="false"
          class="row key"
          class:active={i === active}
          class:group={i > 0}
          title={key.spellings.length > 1 ? `Written as ${key.spellings.map((s) => s.text).join(', ')}` : `${key.label} on ${commitsText(key.commits)}`}
          onclick={() => act(row)}
        >
          <span class="caret" class:open={row.open}><Icon name="right" size={10} /></span>
          <span class="name">{parts.pre}{#if parts.hit}<mark>{parts.hit}</mark>{/if}{parts.post}</span>
          <span class="meta kind">{kindWord(key.kind)}</span>
          <span class="meta count" aria-label={commitsText(key.commits)}>{key.commits.toLocaleString()}</span>
        </li>
      {:else if row.type === 'value'}
        {@const picked = hasPick(picks, row.k.key.id, row.v.id)}
        {@const parts = matchParts(row.label, filter)}
        <li
          id={rowId(i)}
          role="treeitem"
          aria-level="2"
          aria-selected={picked}
          class="row value"
          class:active={i === active}
          class:picked
          title={picked ? `Filtering by ${queryFor(row.k.key, row.v)}. Click to take it out.` : `${row.v.label}: ${commitsText(row.v.commits)}. Click to filter the graph to them.`}
          onclick={() => act(row)}
        >
          <span class="name">{parts.pre}{#if parts.hit}<mark>{parts.hit}</mark>{/if}{parts.post}</span>
          {#if row.v.bot}<span class="meta bot"><Icon name="bot" size={12} />bot</span>{/if}
          <span class="meta count" aria-label={commitsText(row.v.commits)}>{row.v.commits.toLocaleString()}</span>
        </li>
      {:else}
        <li id={rowId(i)} role="treeitem" aria-level="2" aria-selected="false" class="row value more" class:active={i === active} onclick={() => act(row)}>
          <span class="name">{row.label}</span>
        </li>
      {/if}
    {/each}
  </ul>
  {#if panel.moreKeys > 0}
    <p class="note meta">and {panel.moreKeys.toLocaleString()} more keys. The filter above finds any of them.</p>
  {/if}
{:else if panel && filtering}
  <Empty note="no match" compact>Nothing matches that.</Empty>
{:else if nothing && panel?.status.done}
  <Empty note="none yet" compact>No trailers in this history yet. A trailer is a line such as Reviewed-by or Closes at the end of a commit message. Trailer, in the commit form, adds one.</Empty>
{:else if nothing}
  <Empty note="reading" compact>Reading the history for its trailers.</Empty>
{/if}

{#if panel && panel.held.length > 0 && !filtering}
  <div class="held">
    <p class="held-line">
      <span class="meta">{heldText(panel.held.length)}</span>
      <button class="btn sm text" aria-expanded={showHeld} onclick={() => (showHeld = !showHeld)}>{showHeld ? 'Hide them' : 'Show them'}</button>
    </p>
    {#if showHeld}
      <ul class="held-keys" aria-label="Keys held back">
        {#each panel.held as k (k.id)}
          <li>
            <!-- Held back from the list, not from search. -->
            <button class="row value" title="Not offered as a filter: {heldWhy(k.held)}. Click to search for it all the same." onclick={() => onSearch(queryFor(k))}>
              <span class="name">{k.label}</span>
              <span class="meta why">{heldWhy(k.held)}</span>
              <span class="meta count" aria-label={commitsText(k.commits)}>{k.commits.toLocaleString()}</span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}

<style>
  ul {
    box-sizing: border-box;
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .keys {
    outline: none;
  }

  /* The same rows as the other panels: 28px, no boxes, a hairline between groups. */
  .row {
    position: relative;
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    height: 28px;
    box-sizing: border-box;
    background: none;
    border: none;
    border-radius: 0;
    color: var(--ink-soft);
    font-size: 14px;
    text-align: left;
    white-space: nowrap;
    cursor: pointer;
    user-select: none;
  }

  .row:hover {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .key {
    padding: 2px 10px;
  }

  .key.group {
    border-top: 1px dashed var(--paper-edge);
  }

  .value {
    padding: 2px 10px 2px 26px;
    color: var(--ink-mute);
  }

  .keys:focus-visible .row.active {
    box-shadow: inset 2px 0 0 var(--accent);
    color: var(--ink);
  }

  /* The picked value is washed in rose, as the checked-out branch is. */
  .picked,
  .picked:hover {
    background: var(--selected);
    color: var(--ink);
    font-weight: 600;
  }

  .more .name {
    font-style: italic;
  }

  mark {
    background: var(--highlight);
    color: inherit;
  }

  .caret {
    flex: none;
    display: grid;
    place-items: center;
    width: 10px;
    color: var(--ink-mute);
    transition: transform var(--dur-base) var(--ease-ink);
  }

  .caret.open {
    transform: rotate(90deg);
  }

  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .kind,
  .bot,
  .why {
    flex: none;
    font-family: var(--font-mono);
    letter-spacing: 0;
    text-transform: none;
    color: var(--ink-mute);
  }

  .bot {
    display: inline-flex;
    align-items: center;
    gap: 3px;
  }

  .why {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .count {
    margin-left: auto;
    flex: none;
  }

  .note {
    margin: 6px 12px;
    color: var(--ink-mute);
    white-space: normal;
  }

  .failed {
    color: var(--danger);
  }

  .held {
    margin-top: 8px;
    border-top: 1px dashed var(--paper-edge);
  }

  .held-line {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 8px 12px;
  }

  .held-keys .value {
    padding-left: 12px;
  }

  @media (prefers-reduced-motion: reduce) {
    .caret {
      transition: none;
    }
  }
</style>
