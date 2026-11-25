<script lang="ts">
  // The strip above the graph while a smart filter is on: how many commits it keeps, a chip
  // for the history search and for each value picked, the way to pick another, the two
  // modes, and Clear. A history search (#982) says under the chips how it's getting on.
  //
  // Search moves between matches; a filter changes what the graph shows. Only these takes
  // the other commits out. Highlight leaves them and lets the matches stand out.
  import type { app } from '../../../wailsjs/go/models'
  import Icon from '../ui/Icon.svelte'
  import { chipText, keptText } from './filters'
  import HistoryChip from './HistoryChip.svelte'
  import { historyKey, historyStatus, lastYear, narrowText, offersNarrowing, type HistoryOptions } from './history'

  let {
    filter,
    only,
    onlyOffered = true,
    onRemove,
    onAdd,
    onMode,
    onClear,
    onStep,
    onHistorySearch,
    onHistoryStop,
    onHistoryCarryOn,
    onHistoryRemove,
  }: {
    filter: app.TrailerFilterState
    /** Whether only the kept commits show: the mode in force, which may not be the one asked
     * for (see onlyOffered). */
    only: boolean
    /** False where taking commits out isn't possible: the Horizontal view, whose lanes it
     * would break. */
    onlyOffered?: boolean
    onRemove: (pick: app.FilterPick) => void
    onAdd: () => void
    onMode: (only: boolean) => void
    onClear: () => void
    /** Under Highlight, goes to the next commit the filter keeps, or with -1 the one before.
     * Enter on the graph stays the author's card, so stepping has its own buttons (#866). */
    onStep?: (dir: 1 | -1) => void
    /** The history search with other options, from its chip or the offer to narrow it. */
    onHistorySearch?: (o: HistoryOptions) => void
    onHistoryStop?: () => void
    onHistoryCarryOn?: () => void
    onHistoryRemove?: () => void
  } = $props()

  let history = $derived(filter.history ?? null)
  let status = $derived(history ? historyStatus(history) : null)
  let chip = $state<HistoryChip>()

  // How many commits each way of matching found, once a search that way finished, so the
  // options can say so beside each. A search that changes the text or where starts afresh.
  let found = $state<Record<string, number>>({})
  $effect(() => {
    const h = history
    if (h && !h.running && !h.stopped && !h.problem) found[historyKey(h.options)] = h.found
  })
  let counts = $derived(history ? { S: found[historyKey({ ...history.options, lines: false })], G: found[historyKey({ ...history.options, lines: true })] } : {})
</script>

<div class="strip" role="group" aria-label="Smart filter">
  <span class="meta count" role="status">{keptText(filter.kept, filter.total)}</span>
  {#if history && onHistorySearch && onHistoryRemove}
    <HistoryChip bind:this={chip} options={history.options} {counts} onSearch={onHistorySearch} onRemove={onHistoryRemove} />
  {/if}
  {#each filter.picks as p (p.key + '\n' + p.value)}
    <span class="chip">
      <span class="key">{p.keyLabel}</span>
      <span class="value">{chipText(p)}</span>
      <button class="x" aria-label="Remove the {p.keyLabel}: {chipText(p)} filter" title="Remove this filter" onclick={() => onRemove(p)}><Icon name="close" size={11} /></button>
    </span>
  {/each}
  <button class="btn sm quiet" title="Pick another value in the Smart filters panel" onclick={onAdd}><Icon name="plus" size={12} />Filter</button>
  <div class="seg" role="group" aria-label="What a filter does to other commits">
    <button aria-pressed={only} disabled={!onlyOffered} title={onlyOffered ? 'Commits that don’t match leave the graph' : 'Not in the Horizontal view: taking commits out would break its lanes'} onclick={() => onMode(true)}>Only these</button>
    <button aria-pressed={!only} title="Nothing leaves: the matches keep their ink and the rest fade" onclick={() => onMode(false)}>Highlight</button>
  </div>
  {#if !only && onStep}
    <div class="steps">
      <button class="btn sm quiet icon step up" disabled={!filter.kept} aria-label="Previous match" title="The commit before this one that the filter keeps" onclick={() => onStep(-1)}
        ><Icon name="chevron" size={13} /></button
      >
      <button class="btn sm quiet icon step" disabled={!filter.kept} aria-label="Next match" title="The next commit the filter keeps" onclick={() => onStep(1)}><Icon name="chevron" size={13} /></button>
    </div>
  {/if}
  <button class="btn sm text" onclick={onClear}>Clear</button>
  {#if history && status}
    <div class="history-state {status.tone}" aria-live="polite">
      {#if status.tone === 'done'}<Icon name="check" size={12} />{/if}
      <span class="says">{status.text}</span>
      {#if history.running && onHistoryStop}
        <button class="btn sm" onclick={onHistoryStop}>Stop</button>
      {:else if history.stopped && onHistoryCarryOn}
        <button class="btn sm" onclick={onHistoryCarryOn}>Carry on</button>
      {/if}
    </div>
    {#if offersNarrowing(history) && onHistorySearch}
      <div class="narrow" role="note">
        <span class="says">{narrowText(history)}</span>
        <button class="btn sm" onclick={() => onHistorySearch({ ...history.options, since: lastYear() })}>The last year</button>
        <button class="btn sm" onclick={() => chip?.show('paths')}>Only in a folder…</button>
      </div>
    {/if}
  {/if}
</div>

<style>
  .strip {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px 8px;
    min-height: 34px;
    padding: 4px 12px;
    box-sizing: border-box;
    border-bottom: 1px dashed var(--paper-edge);
    background: var(--paper);
  }

  .count {
    flex: none;
    font-variant-numeric: tabular-nums;
  }

  .steps {
    flex: none;
    display: inline-flex;
    gap: 2px;
  }

  .step.up {
    transform: rotate(180deg);
  }

  .step.up:hover:not(:disabled) {
    transform: rotate(180deg) translateY(1px);
  }

  /* A picked value: the key in small mono, the value in ink, washed in rose as it is in the
     panel. */
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    max-width: 100%;
    padding: 2px 4px 2px 9px;
    border-radius: 999px;
    background: var(--selected);
    color: var(--ink);
    font-size: 13px;
  }

  .key {
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .value {
    min-width: 0;
    overflow: hidden;
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .x {
    flex: none;
    display: grid;
    place-items: center;
    width: 18px;
    height: 18px;
    padding: 0;
    border: 0;
    border-radius: 50%;
    background: none;
    color: var(--ink-mute);
    cursor: pointer;
  }

  .x:hover,
  .x:focus-visible {
    background: var(--paper);
    color: var(--ink);
  }

  /* The modes sit at the far end, and stay there when the chips wrap the strip to a second line. */
  .seg {
    display: inline-flex;
    flex: none;
    margin-left: auto;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    overflow: hidden;
  }

  .seg button {
    padding: 3px 10px;
    border: 0;
    background: none;
    color: var(--ink-mute);
    font-size: 13px;
    cursor: pointer;
  }

  .seg button + button {
    border-left: 1px solid var(--paper-edge);
  }

  .seg button[aria-pressed='true'] {
    background: var(--selected);
    color: var(--ink);
  }

  .seg button:disabled {
    cursor: default;
    opacity: 0.5;
  }

  /* The history search's own line, under the chips: the whole width, in mono like the count. */
  .history-state,
  .narrow {
    flex: 1 0 100%;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 10px;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .says {
    flex: 0 1 auto;
    min-width: 0;
  }

  .history-state.done {
    color: var(--success);
  }

  .history-state.problem {
    color: var(--danger);
  }

  .history-state.partial {
    color: var(--ink-teal);
  }

  .narrow {
    padding: 4px 8px;
    border-radius: var(--r-sm);
    background: color-mix(in srgb, var(--ink-ochre) 16%, transparent);
    color: var(--ink);
  }
</style>
