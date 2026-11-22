<script lang="ts">
  // The strip above the graph while a smart filter is on: how many commits it keeps, a chip
  // for each value picked, the way to pick another, the two modes, and Clear.
  //
  // Search moves between matches; a filter changes what the graph shows. Only these takes
  // the other commits out. Highlight leaves them and lets the matches stand out.
  import type { app } from '../../../wailsjs/go/models'
  import Icon from '../ui/Icon.svelte'
  import { chipText, keptText } from './filters'

  let {
    filter,
    only,
    onlyOffered = true,
    onRemove,
    onAdd,
    onMode,
    onClear,
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
  } = $props()
</script>

<div class="strip" role="group" aria-label="Smart filter">
  <span class="meta count" role="status">{keptText(filter.kept, filter.total)}</span>
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
  <button class="btn sm text" onclick={onClear}>Clear</button>
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
</style>
