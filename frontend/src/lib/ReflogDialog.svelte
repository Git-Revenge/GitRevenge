<script lang="ts">
  // Browses the reflog of HEAD or a branch: every place it pointed, newest first, with a way
  // back to each (show it in the graph, or start a branch there). The undo in the toolbar is
  // the action; this is the record.
  import { Reflog, Refs } from '../../wailsjs/go/app/App.js'
  import type { git } from '../../wailsjs/go/models'
  import { errorText } from './errors'
  import { relativeTime } from './graph/draw'
  import type { Ref } from './graph/rowCache'
  import Modal from './ui/Modal.svelte'
  import Icon from './ui/Icon.svelte'
  import Loading from './ui/Loading.svelte'

  let {
    path,
    initialRef = 'HEAD',
    onReveal,
    onCreateBranch,
    onClose,
  }: {
    path: string
    initialRef?: string
    onReveal: (commit: string) => void
    onCreateBranch: (commit: string) => void
    onClose: () => void
  } = $props()

  // svelte-ignore state_referenced_locally
  let ref = $state(initialRef)
  let branches = $state<Ref[]>([])
  let entries = $state<git.ReflogEntry[] | null>(null)
  let error = $state('')
  const now = Date.now() / 1000

  $effect(() => {
    void Refs(path)
      .then((all) => (branches = ((all ?? []) as Ref[]).filter((r) => r.kind === 'local')))
      .catch(() => {})
  })

  $effect(() => {
    const r = ref
    entries = null
    error = ''
    void Reflog(path, r)
      .then((res) => {
        if (r === ref) entries = res ?? []
      })
      .catch((e) => (error = errorText(e)))
  })
</script>

<Modal title="Reflog" eyebrow="where it's been" width={820} {onClose}>
  <div class="bar">
    <label class="form-field">
      <span>Ref</span>
      <select bind:value={ref}>
        <option value="HEAD">HEAD</option>
        {#each branches as b (b.name)}<option value={'refs/heads/' + b.short}>{b.short}</option>{/each}
      </select>
    </label>
    {#if entries}<span class="meta">{entries.length.toLocaleString()} entries</span>{/if}
  </div>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {:else if !entries}
    <Loading />
  {:else if entries.length === 0}
    <p class="muted">No reflog for this ref. Git keeps reflogs for local refs it has moved, for 90 days by default.</p>
  {:else}
    <div class="list">
      <table>
        <thead>
          <tr><th scope="col">Entry</th><th scope="col">What happened</th><th scope="col">When</th><th scope="col">Commit</th><th><span class="visually-hidden">Actions</span></th></tr>
        </thead>
        <tbody>
          {#each entries as e (e.selector)}
            <tr>
              <td class="mono">{e.selector}</td>
              <td class="action" title={e.action}>{e.action}</td>
              <td class="mono when" title={new Date(e.time * 1000).toLocaleString()}>{e.time ? relativeTime(e.time, now) : ''}</td>
              <td class="mono">{e.commit.slice(0, 7)}</td>
              <td class="row-actions">
                <button class="btn sm quiet" onclick={() => onReveal(e.commit)} title="Show this commit in the graph"><Icon name="search" size={14} />Show</button>
                <button class="btn sm quiet" onclick={() => onCreateBranch(e.commit)} title="Start a branch at this commit"><Icon name="branch" size={14} />New branch…</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</Modal>

<style>
  .bar {
    display: flex;
    align-items: end;
    gap: 16px;
    margin-bottom: 12px;
  }

  .list {
    max-height: 60vh;
    overflow: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13.5px;
  }

  th {
    position: sticky;
    top: 0;
    padding: 6px 8px;
    background: var(--paper);
    text-align: left;
    font-family: var(--font-mono);
    font-size: 11px;
    font-weight: 400;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  td {
    padding: 4px 8px;
    border-top: 1px dashed var(--hairline);
    vertical-align: middle;
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 12.5px;
    white-space: nowrap;
  }

  .when {
    color: var(--ink-mute);
  }

  .action {
    max-width: 320px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .row-actions {
    white-space: nowrap;
    text-align: right;
  }

  .muted {
    color: var(--ink-mute);
    font-style: italic;
  }

  .error {
    color: var(--danger);
  }
</style>
