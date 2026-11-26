<script lang="ts">
  // Where a shallow clone's history stops (#1006): after the graph's last commit, and a file's
  // history's. Its oldest commits aren't where the project began: their lines run on into this
  // band, which offers the rest as This clone does (#1005), each run as a fetch.
  import { FetchMoreHistory } from '../../wailsjs/go/app/App.js'
  import type { git } from '../../wailsjs/go/models'
  import { network } from './network.svelte'
  import { day, moreHistory } from './thisClone'

  let {
    path,
    since,
    file = '',
    why = '',
    onFetched,
  }: {
    path: string
    /** The oldest commit date at the clone's edge, Unix seconds. */
    since: number
    /** For a file's history: the file. */
    file?: string
    /** What the edge stops, in words, where the band isn't at the end of a history. */
    why?: string
    /** Called once more history has come, for a view that asks again. */
    onFetched?: () => void
  } = $props()

  let busy = $derived(!!network.running[path])
  const LABELS = ['Fetch a year more', 'Fetch 1,000 commits more', 'Fetch the whole history']
  let choices = $derived(moreHistory(since).map((c, i) => ({ ...c, label: LABELS[i], title: c.label })))

  async function fetchMore(more: { commits: number; since: number; all: boolean }) {
    await network.run(path, 'history', () => FetchMoreHistory(path, more as git.MoreHistory))
    onFetched?.()
  }
</script>

<div class="band" class:inline={!!file || !!why} role="note" aria-label="Where this history stops">
  <p class="head">
    <span class="hand stops">history stops here</span>
    <span class="kind">a shallow clone · since {day(since)}</span>
  </p>
  <p class="why">
    {#if why}
      {why}
    {:else if file}
      This file’s history goes on, on the server: its older commits weren’t fetched.
    {:else}
      The lines above go on, on the server. Their older commits weren’t fetched, so this isn’t where the project began.
    {/if}
  </p>
  <div class="acts">
    {#each choices as c, i (c.label)}
      <button type="button" class="btn sm" class:secondary={i < 2} class:quiet={i === 2} disabled={busy} title={c.title} onclick={() => void fetchMore(c.more)}>{c.label}</button>
    {/each}
  </div>
</div>

<style>
  .band {
    box-sizing: border-box;
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 8px;
    /* The foot stays clear for the graph's floating button back to the branch. */
    padding: 14px 16px 56px;
    border-top: 2px dashed var(--paper-edge);
    /* Faint hatching: the ground the history isn't on. */
    background: repeating-linear-gradient(-45deg, var(--wash-hover) 0 8px, transparent 8px 16px);
  }

  /* In a file's history: no floating button to keep clear of. */
  .band.inline {
    height: auto;
    padding-bottom: 14px;
  }

  p {
    margin: 0;
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 4px 10px;
  }

  .stops {
    font-size: 24px;
    line-height: 1;
    color: var(--link);
  }

  .kind {
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .why {
    font-size: 14px;
    line-height: 1.5;
    color: var(--ink-soft);
    max-width: 60ch;
  }

  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
</style>
