<script lang="ts">
  // A JSON, YAML or TOML file as a collapsible tree with search. A file that doesn't parse
  // shows the parse error; Raw shows its text.
  import { SvelteSet } from 'svelte/reactivity'
  import Icon from '../../ui/Icon.svelte'
  import type { PreviewProps } from './registry'
  import { flatten, parseStructured, visibleNodes } from './structured'

  const MAX_LINES = 5000

  let { path, text }: PreviewProps = $props()

  let parsed = $derived(parseStructured(path, text))
  let nodes = $derived(parsed.ok ? flatten(parsed.value) : [])
  let query = $state('')
  const collapsed = new SvelteSet<string>()

  let shown = $derived(visibleNodes(nodes, collapsed, query))

  function toggle(id: string) {
    if (collapsed.has(id)) collapsed.delete(id)
    else collapsed.add(id)
  }

  function collapseAll() {
    for (const n of nodes) if (n.container && n.depth > 0) collapsed.add(n.id)
  }
</script>

<div class="tree">
  {#if !parsed.ok}
    <p class="error" role="alert"><Icon name="warning" /> Couldn't parse this file: {parsed.error}. Raw shows its text.</p>
  {:else}
    <div class="bar">
      <label class="search">
        <Icon name="search" size={14} />
        <input type="search" placeholder="Search keys and values" aria-label="Search keys and values" bind:value={query} />
      </label>
      <button class="btn quiet" onclick={() => collapsed.clear()}>Expand all</button>
      <button class="btn quiet" onclick={collapseAll}>Collapse all</button>
      <span class="meta">{shown.length.toLocaleString()} of {nodes.length.toLocaleString()} lines</span>
    </div>
    <ul role="tree" aria-label={path}>
      {#each shown.slice(0, MAX_LINES) as n (n.id)}
        <li role="treeitem" aria-selected="false" aria-level={n.depth + 1} aria-expanded={n.container ? !collapsed.has(n.id) || !!query : undefined} style:padding-left="{n.depth * 16}px">
          {#if n.container}
            <button class="twisty" onclick={() => toggle(n.id)} aria-label="{collapsed.has(n.id) ? 'Expand' : 'Collapse'} {n.key || 'root'}"
              ><Icon name={collapsed.has(n.id) && !query ? 'right' : 'chevron'} size={12} /></button
            >
          {:else}<span class="leaf" aria-hidden="true"></span>{/if}
          {#if n.key}<span class="key">{n.key}</span><span class="colon">:</span>{/if}
          <span class="value {n.kind}">{n.summary}</span>
        </li>
      {/each}
    </ul>
    {#if shown.length > MAX_LINES}<p class="meta">First {MAX_LINES.toLocaleString()} lines shown; search to narrow it down.</p>{/if}
  {/if}
</div>

<style>
  .tree {
    padding: 12px 16px 24px;
    font-size: 13px;
  }

  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 10px;
    margin-bottom: 10px;
  }

  .search {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 0 8px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-sm);
    background: var(--paper);
    color: var(--ink-mute);
  }

  .search input {
    width: 220px;
    border: none;
    background: none;
    padding: 5px 0;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.7;
  }

  li {
    display: flex;
    align-items: baseline;
    gap: 4px;
    white-space: nowrap;
  }

  .twisty {
    display: inline-flex;
    align-self: center;
    padding: 1px;
    background: none;
    border: none;
    color: var(--ink-mute);
  }

  .leaf {
    display: inline-block;
    width: 14px;
  }

  .key {
    color: var(--accent-a);
  }

  .colon {
    color: var(--ink-faint);
  }

  .value {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .string {
    color: var(--accent-c);
  }

  .number,
  .boolean,
  .null {
    color: var(--accent-b);
  }

  .object,
  .array {
    color: var(--ink-mute);
  }

  .error {
    display: flex;
    gap: 8px;
    align-items: center;
    color: var(--danger);
  }
</style>
