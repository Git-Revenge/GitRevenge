<script lang="ts">
  // Lists the repository's worktrees, and adds, opens and removes linked ones.
  import { AddWorktree, ChooseFolder, Refs, RemoveWorktree, Worktrees } from '../../wailsjs/go/app/App.js'
  import type { git } from '../../wailsjs/go/models'
  import { errorText } from './errors'
  import type { Ref } from './graph/rowCache'
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'
  import Loading from './ui/Loading.svelte'

  let { path, onClose }: { path: string; onClose: () => void } = $props()

  let list = $state<git.Worktree[] | null>(null)
  let branches = $state<string[]>([])
  let error = $state('')
  let busy = $state(false)
  let adding = $state(false)
  let branch = $state('')
  let dir = $state('')
  let dirTouched = $state(false)
  let confirmRemove = $state('')
  let force = $state(false)

  const sep = $derived(path.includes('\\') ? '\\' : '/')
  const parent = $derived(path.slice(0, path.lastIndexOf(sep)))
  const repoName = $derived(path.slice(path.lastIndexOf(sep) + 1))
  const slug = (b: string) => b.trim().replace(/[\\/:*?"<>|\s]+/g, '-') || 'worktree'

  /** A sibling folder named after the repo and branch, until the user picks their own. */
  $effect(() => {
    if (!dirTouched) dir = `${parent}${sep}${repoName}-${slug(branch)}`
  })

  async function load() {
    try {
      list = await Worktrees(path)
    } catch (e) {
      error = errorText(e)
    }
  }

  $effect(() => {
    void load()
    void Refs(path)
      .then((all) => (branches = ((all ?? []) as Ref[]).filter((r) => r.kind === 'local').map((r) => r.short)))
      // Only used for suggestions.
      .catch(() => {})
  })

  async function run(fn: () => Promise<void>) {
    error = ''
    busy = true
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

  async function add() {
    if (await run(() => AddWorktree(path, dir, branch))) {
      adding = false
      branch = ''
      dirTouched = false
    }
  }

  async function remove(w: git.Worktree) {
    if (await run(() => RemoveWorktree(path, w.path, force))) {
      confirmRemove = ''
      force = false
    }
  }

  async function browse() {
    const chosen = await ChooseFolder('Folder to create the worktree in')
    if (chosen) {
      dir = `${chosen}${sep}${repoName}-${slug(branch)}`
      dirTouched = true
    }
  }

  function describe(w: git.Worktree): string {
    const parts = [w.branch || `detached at ${w.head.slice(0, 7)}`]
    if (w.main) parts.push('main worktree')
    if (w.current) parts.push('this window')
    if (w.locked) parts.push('locked')
    if (w.prunable) parts.push('missing on disk')
    return parts.join(' · ')
  }
</script>

<Modal title="Worktrees" eyebrow="many checkouts" width={720} {onClose}>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if !list}
    <Loading />
  {:else}
    <ul class="list">
      {#each list as w (w.path)}
        <li>
          <span class="info">
            <span class="where" title={w.path}>{w.path}</span>
            <span class="meta">{describe(w)}</span>
          </span>
          <span class="actions">
            {#if !w.current && !w.prunable}
              <button class="btn quiet" disabled={busy} onclick={() => session.open(w.path)}><Icon name="folder-open" size={14} />Open</button>
            {/if}
            {#if !w.main}
              {#if confirmRemove === w.path}
                <label class="force"><input type="checkbox" bind:checked={force} /> even with changes</label>
                <button class="btn sm danger" disabled={busy} onclick={() => remove(w)}>Remove</button>
                <button class="btn quiet" disabled={busy} onclick={() => (confirmRemove = '')}>Cancel</button>
              {:else}
                <button
                  class="btn quiet"
                  disabled={busy || w.current}
                  onclick={() => (confirmRemove = w.path)}
                  title={w.current ? 'This worktree is open here' : 'Remove this worktree'}><Icon name="trash" size={14} />Remove…</button
                >
              {/if}
            {/if}
          </span>
        </li>
      {/each}
    </ul>

    {#if adding}
      <form
        class="add"
        onsubmit={(e) => {
          e.preventDefault()
          void add()
        }}
      >
        <label class="form-field">
          <span>Branch: an existing one is checked out, a new name is created at HEAD</span>
          <input bind:value={branch} list="worktree-branches" placeholder="feature/my-change" />
          <datalist id="worktree-branches">
            {#each branches as b (b)}<option value={b}></option>{/each}
          </datalist>
        </label>
        <label class="form-field">
          <span>Folder</span>
          <span class="row">
            <input bind:value={dir} oninput={() => (dirTouched = true)} />
            <button type="button" class="btn quiet" onclick={browse}><Icon name="folder" size={14} />Choose…</button>
          </span>
        </label>
        <span class="buttons">
          <InkButton type="submit" disabled={busy || !dir.trim()}>Add worktree</InkButton>
          <InkButton variant="secondary" onclick={() => (adding = false)}>Cancel</InkButton>
        </span>
      </form>
    {:else}
      <button class="btn quiet" onclick={() => (adding = true)}><Icon name="plus" size={14} />Add worktree…</button>
    {/if}
  {/if}
</Modal>

<style>
  .list {
    list-style: none;
    margin: 0 0 12px;
    padding: 0;
  }

  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 12px;
    padding: 8px 0;
    border-top: 1px dotted var(--paper-edge);
  }

  .info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .where {
    font-family: var(--font-mono);
    font-size: 13px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
  }

  .force {
    font-size: 13px;
  }

  .add {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .row {
    display: flex;
    gap: 6px;
  }

  .row input {
    flex: 1;
  }

  .buttons {
    display: flex;
    gap: 8px;
  }

  .error {
    color: var(--danger);
    white-space: pre-wrap;
  }
</style>
