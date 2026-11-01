<script lang="ts">
  // The Shipped view of the Insights tab (#250): what landed on the target branch in the
  // window, grouped by ticket reference or by day.
  import type { git } from '../../../wailsjs/go/models'
  import { session } from '../session.svelte'
  import { insights } from './insights.svelte'
  import { byDay, byReference, duration, utcDay, type Tagged } from './model'

  let grouping = $state<'reference' | 'date'>('reference')
  let w = $derived(insights.resolved)
  let multi = $derived(insights.repos.length > 1)
  // Dependency updates a bot opened (ADR 0009) are listed apart, after the work people did.
  let work = $derived(insights.items.filter((i) => !i.value.bot))
  let deps = $derived(insights.items.filter((i) => i.value.bot))
  let refGroups = $derived(byReference(work))
  let dayGroups = $derived(byDay(work))
  let refCount = $derived(refGroups.filter((g) => g.ref).length)

  const repoName = (path: string) => session.tabs.find((t) => t.path === path)?.name ?? path.split(/[\\/]/).pop() ?? path
  const target = $derived(w?.target.replace(/^origin\//, '') ?? 'the target')
</script>

{#snippet item(i: Tagged<git.ShippedItem>)}
  <div class="item">
    <div class="what">
      <span class="title">{i.value.title}</span>
      <span class="sub">
        {i.value.author}
        {#if multi}· <span class="tag">{repoName(i.repo)}</span>{/if}
        {#if grouping === 'date' && i.value.refs.length}· <code>{i.value.refs.join(' ')}</code>{/if}
        {#if i.value.revert}· <span class="rework">revert</span>{:else if i.value.fixup}· <span class="rework">fixup</span>{/if}
      </span>
    </div>
    <span class="size" title="{i.value.files} {i.value.files === 1 ? 'file' : 'files'} changed against the first parent">
      {i.value.files}
      {i.value.files === 1 ? 'file' : 'files'}
      {#if i.value.added}<span class="added">+{i.value.added}</span>{/if}
      {#if i.value.removed}<span class="removed">−{i.value.removed}</span>{/if}
    </span>
    {#if i.value.merge && i.value.cycle}
      <span class="cycle" title="From the merged branch's first commit to the merge">{duration(i.value.cycle)}</span>
    {:else}
      <span class="cycle none" title="Squash merges and direct commits don't keep the branch's history, so there's no cycle time to measure"
        >{i.value.merge ? 'merge' : 'single commit'}</span
      >
    {/if}
  </div>
{/snippet}

<div class="shipped">
  <div class="bar">
    <p class="summary">
      <span title="git log --first-parent {w?.range || target}, committer dates in the window">{insights.items.length} {insights.items.length === 1 ? 'item' : 'items'}</span>
      on {target}{#if multi}
        across {insights.repos.length} repositories{/if} · {refCount}
      {refCount === 1 ? 'reference' : 'references'}
    </p>
    <span class="segmented" role="group" aria-label="Group items">
      <button aria-pressed={grouping === 'reference'} onclick={() => (grouping = 'reference')}>by reference</button>
      <button aria-pressed={grouping === 'date'} onclick={() => (grouping = 'date')}>by date</button>
    </span>
  </div>

  {#if insights.items.length === 0}
    <p class="empty"><span class="hand">quiet stretch</span> Nothing landed on {target} in this window.</p>
  {:else if grouping === 'reference'}
    {#each refGroups as g (g.ref)}
      <section class="group" aria-label={g.ref || 'No reference'}>
        <h2>
          {#if g.ref}<code class="ref">{g.ref}</code>{:else}<span class="noref">No reference</span>{/if}
          <span class="meta">{g.items.length} {g.items.length === 1 ? 'item' : 'items'}</span>
          <span class="spacer"></span>
          {#if g.cycle}<span class="meta" title="Median cycle time of the merges in this group">cycle {duration(g.cycle)}</span>{/if}
          {#if !g.ref}<span class="hand nudge">worth a ticket?</span>{/if}
        </h2>
        {#each g.items as i (i.repo + i.value.sha)}{@render item(i)}{/each}
      </section>
    {/each}
  {:else}
    {#each dayGroups as d (d.day)}
      <section class="group" aria-label={utcDay(d.day)}>
        <h2><span class="day">{utcDay(d.day)}</span> <span class="meta">{d.items.length} {d.items.length === 1 ? 'item' : 'items'}</span></h2>
        {#each d.items as i (i.repo + i.value.sha)}{@render item(i)}{/each}
      </section>
    {/each}
  {/if}
  {#if deps.length}
    <details class="group deps">
      <summary>
        <h2><span>Dependency updates</span> <span class="meta">{deps.length} {deps.length === 1 ? 'item' : 'items'}</span></h2>
      </summary>
      {#each deps as i (i.repo + i.value.sha)}{@render item(i)}{/each}
    </details>
  {/if}

  <p class="foot">References come from subjects and trailers (<code>TIDE-88</code>, <code>#41</code>, <code>Refs:</code>, <code>Closes:</code>) and are shown as text, never links. An item with several goes under its first.</p>
</div>

<style>
  .deps summary {
    cursor: pointer;
    list-style: none;
  }

  .deps summary h2 {
    display: inline-flex;
    gap: 8px;
  }

  .shipped {
    display: flex;
    flex-direction: column;
    gap: 22px;
  }

  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px 20px;
  }

  .summary {
    flex: 1;
    margin: 0;
    color: var(--ink-mute);
  }

  .summary span {
    border-bottom: 1px dotted var(--ink-faint);
    cursor: help;
  }

  .segmented button {
    white-space: nowrap;
  }

  .group {
    display: flex;
    flex-direction: column;
  }

  h2 {
    display: flex;
    align-items: baseline;
    gap: 12px;
    margin: 0 0 4px;
    font-size: 15px;
    font-weight: 400;
  }

  .ref {
    font-family: var(--font-mono);
    font-size: 15px;
    color: var(--accent-a);
  }

  .noref,
  .day {
    font-style: italic;
    color: var(--ink-mute);
  }

  .day {
    font-style: normal;
    font-weight: 500;
    color: var(--ink);
  }

  .spacer {
    flex: 1;
  }

  .nudge {
    font-size: 18px;
    color: var(--accent-c);
  }

  .item {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 160px 120px;
    gap: 18px;
    align-items: baseline;
    padding: 12px 4px 12px 22px;
    border-top: 1px dotted var(--paper-edge);
  }

  .what {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .sub {
    font-size: 13px;
    color: var(--ink-mute);
  }

  .sub code,
  .foot code {
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .rework {
    color: var(--accent-c);
  }

  .size {
    font-family: var(--font-mono);
    font-size: 12.5px;
    color: var(--ink-soft);
  }

  .added {
    color: var(--success);
  }

  .removed {
    color: var(--danger);
  }

  .cycle {
    font-size: 13px;
    color: var(--ink-mute);
    text-align: right;
  }

  .cycle.none {
    color: var(--ink-faint);
  }

  .empty,
  .foot {
    margin: 0;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .empty {
    display: flex;
    gap: 12px;
    align-items: baseline;
  }

  .empty .hand {
    font-size: 20px;
    color: var(--accent-c);
  }
</style>
