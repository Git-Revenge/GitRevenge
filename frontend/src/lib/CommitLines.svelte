<script lang="ts">
  // A short list of commits in a warning, naming what's at stake and who wrote it.
  import type { git } from '../../wailsjs/go/models'

  let { commits, total }: { commits: git.CommitLine[]; total: number } = $props()
  let more = $derived(total - commits.length)
</script>

<ul class="lines">
  {#each commits as c (c.id)}
    <li>
      <code>{c.short}</code>
      <span class="subject" title={c.subject}>{c.subject}</span>
      <span class="author">{c.author}</span>
    </li>
  {/each}
</ul>
{#if more > 0}<p class="more">and {more.toLocaleString()} more</p>{/if}

<style>
  .lines {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-height: 180px;
    margin: 0;
    padding: 0;
    overflow-y: auto;
    list-style: none;
    font-size: 14px;
  }

  li {
    display: flex;
    align-items: baseline;
    gap: 8px;
    min-width: 0;
  }

  code {
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .subject {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .author {
    flex: none;
    max-width: 30%;
    overflow: hidden;
    color: var(--ink-mute);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .more {
    margin: 4px 0 0;
    color: var(--ink-mute);
    font-size: 13px;
  }
</style>
