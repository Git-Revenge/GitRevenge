<script lang="ts">
  // The repository's LFS patterns, files and locks. Tracking edits .gitattributes, which you
  // then commit; locks need the server, so they're only asked for on request.
  import { LFS, LFSAction, LFSLocks } from '../../wailsjs/go/app/App.js'
  import type { app, git } from '../../wailsjs/go/models'
  import { errorText } from './errors'
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'
  import Modal from './ui/Modal.svelte'
  import Loading from './ui/Loading.svelte'

  let { path, onClose }: { path: string; onClose: () => void } = $props()

  let lfs = $state<app.LFSInfo | null>(null)
  /** Locks from the server, by file path; null until checked. */
  let locks = $state<Record<string, git.LFSLock> | null>(null)
  let busy = $state(false)
  let error = $state('')
  let pattern = $state('')
  let lockable = $state(false)

  async function load() {
    lfs = (await session.run(() => LFS(path))) ?? null
  }

  $effect(() => {
    void session.revision[path]
    void load()
  })

  /** Runs an action, keeping any error in the dialog; false if it failed. */
  async function run(fn: () => Promise<unknown>): Promise<boolean> {
    busy = true
    error = ''
    try {
      await fn()
      await load()
      return true
    } catch (e) {
      error = errorText(e)
      return false
    } finally {
      busy = false
    }
  }

  async function track() {
    if (await run(() => LFSAction(path, 'track', [pattern.trim(), lockable ? 'true' : '']))) {
      pattern = ''
      lockable = false
    }
  }

  async function checkLocks() {
    await run(async () => {
      const list = await LFSLocks(path)
      locks = Object.fromEntries((list ?? []).map((l) => [l.path, l]))
    })
  }

  async function lockAction(file: string, action: 'lock' | 'unlock', force = false) {
    if (await run(() => LFSAction(path, action, [file, force ? 'true' : '']))) await checkLocks()
  }
  import Empty from './ui/Empty.svelte'
</script>

<Modal title="LFS" eyebrow="large files" width={640} {onClose}>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if !lfs}
    <Loading />
  {:else}
    <h3>Tracked patterns</h3>
    <ul>
      {#each lfs.status.patterns as p (p.pattern)}
        <li>
          <span class="name mono" title="From {p.source}">{p.pattern}</span>
          {#if p.lockable}<span class="badge">lockable</span>{/if}
          <button class="btn sm quiet icon" disabled={busy} onclick={() => run(() => LFSAction(path, 'untrack', [p.pattern]))} aria-label="Stop tracking {p.pattern}" title="Stop tracking"><Icon name="close" /></button>
        </li>
      {:else}
        <li><Empty note="none yet">No patterns tracked.</Empty></li>
      {/each}
    </ul>
    <form
      class="track"
      onsubmit={(e) => {
        e.preventDefault()
        void track()
      }}
    >
      <label class="pattern">
        <span class="visually-hidden">Pattern to track</span>
        <input bind:value={pattern} placeholder="*.psd" />
      </label>
      <label class="form-check"><input type="checkbox" bind:checked={lockable} /> Lockable</label>
      <button class="btn sm secondary" type="submit" disabled={busy || !pattern.trim()}>Track</button>
    </form>
    <p class="muted hint">Matching files go to LFS from now on. This edits .gitattributes, which you then commit. Lockable files are read-only until locked.</p>

    <div class="files-head">
      <h3>Files <span class="meta">{lfs.status.files.length.toLocaleString()}</span></h3>
      {#if lfs.status.files.length}
        <button class="btn sm quiet" disabled={busy} onclick={checkLocks} title="Ask the LFS server which files are locked">Check locks</button>
      {/if}
    </div>
    <ul class="files">
      {#each lfs.status.files as f (f.name)}
        {@const lock = locks?.[f.name]}
        <li>
          <span class="name" title={f.name}>{f.name}</span>
          {#if !f.downloaded}<span class="badge" title="Only the pointer is here; the content isn't downloaded">pointer</span>{/if}
          {#if lock}
            <span class="badge" class:mine={lock.ours} title="Locked {lock.lockedAt}">locked · {lock.ours ? 'you' : lock.owner}</span>
            <button class="btn sm quiet" disabled={busy} onclick={() => lockAction(f.name, 'unlock', !lock.ours)}>{lock.ours ? 'Unlock' : 'Force unlock'}</button>
          {:else if locks}
            <button class="btn sm quiet" disabled={busy} onclick={() => lockAction(f.name, 'lock')}>Lock</button>
          {/if}
        </li>
      {:else}
        <li><Empty note="nothing large">No files in LFS.</Empty></li>
      {/each}
    </ul>
  {/if}
</Modal>

<style>
  h3 {
    margin: 12px 0 4px;
    font-size: 15px;
    font-weight: 500;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  li {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 32px;
    border-bottom: 1px dotted var(--paper-edge);
  }

  .files {
    max-height: 280px;
    overflow-y: auto;
  }

  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .track {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-top: 10px;
  }

  .pattern {
    flex: 1;
    min-width: 0;
  }

  .pattern input {
    width: 100%;
    box-sizing: border-box;
  }

  .hint {
    margin: 6px 0 0;
    font-size: 13px;
  }

  .files-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 8px;
  }

  .badge {
    flex: none;
    padding: 0 5px;
    border-radius: var(--r-sm);
    background: var(--wash-hover);
    color: var(--ink-mute);
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    white-space: nowrap;
  }

  .badge.mine {
    color: var(--success);
  }

  .muted {
    color: var(--ink-mute);
  }

  .error {
    margin: 0 0 8px;
    color: var(--danger);
  }
</style>
