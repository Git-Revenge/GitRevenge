<script lang="ts">
  import Icon from '../ui/Icon.svelte'
  import { CloneShape, FileHistory, IsShallowEdge } from '../../../wailsjs/go/app/App.js'
  import ShallowBand from '../ShallowBand.svelte'
  import type { git } from '../../../wailsjs/go/models'
  import { formatTime } from '../graph/draw'
  import { session } from '../session.svelte'
  import DiffView, { type DiffTarget } from './DiffView.svelte'
  import Loading from '../ui/Loading.svelte'

  let {
    path,
    file,
    onClose,
    onOpen,
  }: { path: string; file: string; onClose: () => void; onOpen: (t: DiffTarget) => void } = $props()

  let commits = $state<git.FileCommit[] | null>(null)
  let selected = $state<git.FileCommit | null>(null)
  /** Where a shallow clone's history stops, when this file's reaches its edge (#1006): its
   * oldest commit here isn't where the file began. 0 otherwise. */
  let edgeSince = $state(0)

  $effect(() => {
    const f = file
    void session.revision[path]
    commits = null
    selected = null
    edgeSince = 0
    void session.run(() => FileHistory(path, f)).then(async (c) => {
      if (f !== file) return
      commits = c ?? []
      selected = commits[0] ?? null
      const oldest = commits.at(-1)
      if (oldest && (await IsShallowEdge(path, oldest.id).catch(() => false)) && f === file) {
        const shape = await CloneShape(path).catch(() => null)
        if (f === file) edgeSince = shape?.shallow?.since ?? oldest.time
      }
    })
  })
  import Empty from '../ui/Empty.svelte'
</script>

<div class="history">
  <header class="view-head">
    <button class="btn quiet" onclick={onClose} title="Back"><Icon name="back" />Back</button>
    <span class="view-badge">History</span>
    <span class="view-file" title={file}>{file}</span>
    {#if commits}<span class="meta">{commits.length} {commits.length === 1 ? 'commit' : 'commits'}</span>{/if}
    <span class="spacer"></span>
    <button class="btn quiet" onclick={() => onOpen({ mode: 'blame', file })}>Blame</button>
  </header>
  <ul class="list">
    {#if !commits}
      <li><Loading /></li>
    {:else}
      {#each commits as c (c.id)}
        <li>
          <button class:active={selected?.id === c.id} onclick={() => (selected = c)}>
            <span class="sha">{c.id.slice(0, 7)}</span>
            <span class="subject">{c.subject}</span>
            {#if c.oldPath}<span class="badge" title="Renamed from {c.oldPath}">renamed</span>{/if}
            <span class="muted">{c.author}</span>
            <span class="muted when">{formatTime(c.time)}</span>
          </button>
        </li>
      {:else}
        <li><Empty note="untouched">No commits touch this file.</Empty></li>
      {/each}
      {#if edgeSince}
        <li class="edge"><ShallowBand {path} since={edgeSince} {file} /></li>
      {/if}
    {/if}
  </ul>
  {#if selected}
    {#key selected.id}
      <div class="diff">
        <DiffView
          {path}
          target={{ mode: 'commit', commit: selected.id, file: selected.path, oldPath: selected.oldPath }}
          onClose={() => (selected = null)}
        />
      </div>
    {/key}
  {/if}
</div>

<style>
  .history {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }

  .spacer {
    flex: 1;
  }

  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 35%;
    overflow-y: auto;
    border-bottom: 1px solid var(--paper-edge);
  }

  .list li {
    border-bottom: 1px dotted var(--paper-edge);
  }

  /* A commit's row: not the buttons of the band at the end. */
  .list li > button {
    display: flex;
    gap: 12px;
    align-items: baseline;
    width: 100%;
    padding: 8px 14px;
    background: none;
    border: none;
    border-radius: 0;
    text-align: left;
  }

  .list li > button:hover:not(:disabled) {
    background: var(--wash-hover);
  }

  .list li > button.active {
    background: var(--selected);
  }

  .list .edge {
    border-bottom: none;
  }

  .sha {
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

  .when {
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .badge {
    padding: 0 6px;
    border-radius: var(--r-sm);
    background: var(--selected);
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-a);
  }

  .diff {
    flex: 1;
    min-height: 0;
  }

  .muted {
    color: var(--ink-mute);
  }

</style>
