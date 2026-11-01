<script lang="ts">
  // The Hotspots view of the Insights tab (#251): the files the window's ships changed most,
  // flagged where open branches change them too, and the window's rework.
  import { session } from '../session.svelte'
  import { insights } from './insights.svelte'
  import { latest, utcDay } from './model'

  let multi = $derived(insights.repos.length > 1)
  let loaded = $derived(insights.repos.every((r) => insights.data[r]?.hotspots || insights.data[r]?.error))
  let files = $derived(insights.hot)
  let peak = $derived(Math.max(1, ...files.map((f) => f.value.lines)))
  let rework = $derived(latest(insights.items.filter((i) => i.value.revert || i.value.fixup)))

  const repoName = (path: string) => session.tabs.find((t) => t.path === path)?.name ?? path.split(/[\\/]/).pop() ?? path
  const kind = (title: string) => (title.startsWith('Revert "') ? 'revert' : title.split('!')[0])
</script>

<div class="hotspots">
  <section aria-labelledby="hot">
    <h2 id="hot">Most changed files <span class="meta"><span class="why" title="Lines added plus removed by the shipped items in the window">lines changed</span></span></h2>
    {#if !loaded}
      <p class="sub quiet">Counting…</p>
    {:else}
      {#each files as f (f.repo + f.value.path)}
        <div class="hot">
          <div class="what">
            <code>{f.value.path}</code>
            <span class="sub">
              {f.value.items}
              {f.value.items === 1 ? 'item' : 'items'}{multi ? ` · ${repoName(f.repo)}` : ''}{#if f.value.openBranches}<span> · </span><span class="danger">{f.value.openBranches} open {f.value.openBranches === 1 ? 'branch touches' : 'branches touch'} it</span>{/if}
            </span>
          </div>
          <span class="bar-track" aria-hidden="true"><span class="bar" style:width="{(f.value.lines / peak) * 100}%"></span></span>
          <span class="lines">{f.value.lines}</span>
        </div>
      {:else}
        <p class="sub quiet">No files changed in this window.</p>
      {/each}
      {#if files.some((f) => f.value.openBranches)}
        <p class="note">
          <span class="hand">next conflict?</span>Files that keep changing on the target and are also changed on open branches are where merges get hard.
        </p>
      {/if}
    {/if}
  </section>

  <section aria-labelledby="rework">
    <h2 id="rework">Rework <span class="meta"><span class="why" title="Reverts, and fixup!, squash! or amend! commits, among the shipped items">{rework.length} {rework.length === 1 ? 'item' : 'items'}</span></span></h2>
    {#each rework as r (r.repo + r.value.sha)}
      <div class="rw">
        <span class="tag {r.value.revert ? 'sepia' : ''}">{kind(r.value.title)}</span>
        <div class="what">
          <span>{r.value.title}</span>
          <span class="sub">{r.value.author}{multi ? ` · ${repoName(r.repo)}` : ''}</span>
        </div>
        <span class="when">{utcDay(r.value.committed)}</span>
      </div>
    {:else}
      <p class="sub quiet">No reverts or fixups landed in this window.</p>
    {/each}
  </section>
</div>

<style>
  .hotspots {
    display: flex;
    flex-direction: column;
    gap: 30px;
  }

  section {
    display: flex;
    flex-direction: column;
  }

  h2 {
    display: flex;
    align-items: baseline;
    gap: 12px;
    margin: 0 0 6px;
    font-size: 20px;
    font-weight: 500;
    letter-spacing: -0.01em;
  }

  .why {
    border-bottom: 1px dotted var(--ink-faint);
    cursor: help;
  }

  .hot,
  .rw {
    display: grid;
    gap: 18px;
    align-items: center;
    padding: 11px 4px;
    border-top: 1px dotted var(--paper-edge);
  }

  .hot {
    grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr) 80px;
  }

  .rw {
    grid-template-columns: 90px minmax(0, 1fr) 80px;
  }

  .rw .tag {
    justify-self: start;
  }

  .what {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  code {
    font-family: var(--font-mono);
    font-size: 14px;
    overflow-wrap: anywhere;
  }

  .sub {
    font-size: 13px;
    color: var(--ink-mute);
  }

  .quiet {
    margin: 0;
    padding: 12px 4px;
    border-top: 1px dotted var(--paper-edge);
  }

  .danger {
    color: var(--danger);
  }

  .bar-track {
    display: block;
    height: 10px;
  }

  .bar {
    display: block;
    height: 100%;
    min-width: 4px;
    border-radius: 999px;
    background: var(--accent-a);
    opacity: 0.8;
  }

  .lines,
  .when {
    font-family: var(--font-mono);
    font-size: 13px;
    text-align: right;
  }

  .when {
    font-size: 12px;
    color: var(--ink-faint);
  }

  .note {
    display: flex;
    gap: 12px;
    align-items: baseline;
    margin: 10px 0 0;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .hand {
    font-size: 20px;
    color: var(--danger);
    white-space: nowrap;
  }
</style>
