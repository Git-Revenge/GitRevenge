<script lang="ts">
  import { onMount } from 'svelte'
  import { GroupBatch, GroupDashboard } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import { formatTime } from './graph/draw'
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'
  import Loading from './ui/Loading.svelte'

  let { name, onClose }: { name: string; onClose: () => void } = $props()

  let rows = $state<app.GroupRepo[]>([])
  let loading = $state(true)
  let running = $state<'' | 'fetch' | 'pull'>('')
  /** Result of the last batch, by repo path. */
  let results = $state<Record<string, app.BatchResult>>({})

  async function refresh() {
    loading = true
    rows = (await session.run(() => GroupDashboard(name))) ?? []
    loading = false
  }

  async function batch(op: 'fetch' | 'pull') {
    running = op
    results = {}
    const res = await session.run(() => GroupBatch(name, op))
    for (const r of res ?? []) results[r.path] = r
    running = ''
    await refresh()
  }

  async function openAll() {
    for (const r of rows) if (r.exists) await session.open(r.path)
  }

  onMount(() => {
    void refresh()
    // Show each repository's result as soon as it finishes.
    return EventsOn('group:progress', (p: app.BatchResult & { group: string }) => {
      if (p.group === name) results[p.path] = p
    })
  })
</script>

<section class="dashboard">
  <header>
    <button class="btn quiet back" onclick={onClose} title="Back to the start page"><Icon name="back" />Back</button>
    <div class="title">
      <span class="hand">dashboard</span>
      <h2>{name}</h2>
    </div>
    <span class="spacer"></span>
    <button class="btn quiet" disabled={!!running} onclick={refresh}><Icon name="refresh" />Refresh</button>
    <button class="btn quiet" disabled={!!running || rows.length === 0} onclick={() => batch('fetch')}
      ><Icon name="fetch" />{running === 'fetch' ? 'Fetching…' : 'Fetch all'}</button
    >
    <button class="btn quiet" disabled={!!running || rows.length === 0} onclick={() => batch('pull')} title="Fast-forward only: diverged repositories are reported, never merged"
      ><Icon name="pull" />{running === 'pull' ? 'Pulling…' : 'Pull all'}</button
    >
    <button class="btn quiet" disabled={rows.length === 0} onclick={openAll}><Icon name="folder" />Open all as tabs</button>
  </header>

  {#if loading && rows.length === 0}
    <Loading />
  {:else if rows.length === 0}
    <p class="muted">This group has no repositories yet. Add some from the start page.</p>
  {:else}
    <table>
      <thead>
        <tr><th class="meta">Repository</th><th class="meta">Branch</th><th class="meta">Sync</th><th class="meta">Changes</th><th class="meta">Last fetch</th><th></th></tr>
      </thead>
      <tbody>
        {#each rows as r (r.path)}
          {@const res = results[r.path]}
          <tr class:missing={!r.exists}>
            <td class="repo" title={r.path}>{r.name}</td>
            <td class="mono">{r.branch || '—'}</td>
            <td>
              {#if !r.upstream && r.exists}<span class="muted">no upstream</span>
              {:else if r.ahead || r.behind}{#if r.ahead}↑{r.ahead} {/if}{#if r.behind}↓{r.behind}{/if}
              {:else if r.exists}<span class="ok">up to date</span>{/if}
            </td>
            <td>
              {#if r.conflicts}<span class="bad">{r.conflicts} conflicted</span>
              {:else if r.changes}{r.changes}
              {:else if r.exists}<span class="muted">clean</span>{/if}
            </td>
            <td class="muted mono">{r.lastFetch ? formatTime(r.lastFetch) : r.exists ? 'never' : ''}</td>
            <td class="action">
              {#if res?.skipped}
                <span class="muted" title={res.message}>– {res.message}</span>
              {:else if res}
                <span class={res.ok ? 'ok' : 'bad'} title={res.message}>{res.ok ? '✓' : '✕'} {res.ok ? '' : res.message}</span>
              {:else if r.error}
                <span class="bad">{r.error}</span>
              {:else}
                <button class="btn sm text" onclick={() => session.open(r.path)}>Open</button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>

<style>
  .dashboard {
    max-width: 1080px;
    margin: 0 auto;
    padding: 28px 40px 40px;
  }

  header {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 4px;
  }

  .back {
    align-self: center;
    margin: 0 12px 0 -10px;
  }

  .title {
    display: flex;
    flex-direction: column;
  }

  h2 {
    margin: 0;
    font-size: 34px;
    font-weight: 400;
    letter-spacing: -0.02em;
    line-height: 1.1;
    font-variation-settings:
      'SOFT' 50,
      'WONK' 1;
  }

  .spacer {
    flex: 1;
  }

  table {
    width: 100%;
    margin-top: 24px;
    border-collapse: collapse;
  }

  th {
    text-align: left;
    font-weight: normal;
    padding: 6px 10px;
    border-bottom: 1px solid var(--paper-edge);
  }

  td {
    padding: 12px 10px;
    border-bottom: 1px dotted var(--paper-edge);
    white-space: nowrap;
  }

  tbody tr {
    transition: background-color var(--dur-base);
  }

  tbody tr:hover {
    background: var(--wash-hover);
  }

  .repo {
    font-size: 17px;
    font-weight: 500;
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 12.5px;
  }

  td.action {
    white-space: normal;
  }

  tr.missing {
    opacity: 0.6;
  }

  .ok {
    color: var(--success);
  }

  .bad {
    color: var(--danger);
  }

  .muted {
    color: var(--ink-mute);
  }
</style>
