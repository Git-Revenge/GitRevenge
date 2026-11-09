<script lang="ts">
  import { onMount } from 'svelte'
  import { RemoteAction, Remotes } from '../../wailsjs/go/app/App.js'
  import type { git } from '../../wailsjs/go/models'
  import { errorText } from './errors'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  let { path, onClose }: { path: string; onClose: () => void } = $props()

  let remotes = $state<git.Remote[]>([])
  let error = $state('')
  /** Row being edited ("" for a new remote), or null. */
  let editing = $state<string | null>(null)
  let name = $state('')
  let url = $state('')
  let confirmRemove = $state<string | null>(null)

  async function load() {
    try {
      remotes = (await Remotes(path)) ?? []
    } catch (e) {
      error = errorText(e)
    }
  }

  async function act(action: string, args: string[]) {
    error = ''
    try {
      await RemoteAction(path, action, args)
      return true
    } catch (e) {
      error = errorText(e)
      return false
    }
  }

  function startEdit(r?: git.Remote) {
    editing = r?.name ?? ''
    name = r?.name ?? (remotes.length ? '' : 'origin')
    url = r?.fetchUrl ?? ''
    error = ''
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    let ok = true
    if (editing === '') ok = await act('add', [name, url])
    else if (editing !== null) {
      const old = remotes.find((r) => r.name === editing)
      if (old && name !== old.name) ok = await act('rename', [old.name, name])
      if (ok && old && url !== old.fetchUrl) ok = await act('seturl', [name, url])
    }
    if (ok) {
      editing = null
      await load()
    }
  }

  async function remove(n: string) {
    confirmRemove = null
    if (await act('remove', [n])) await load()
  }

  onMount(load)
  import Empty from './ui/Empty.svelte'
</script>

<Modal title="Remotes" eyebrow="ports of call" width={580} {onClose}>
  {#if remotes.length === 0 && editing === null}<Empty note="none yet">No remotes yet. Add one to fetch from and push to.</Empty>{/if}
  {#if remotes.length}
    <ul>
      {#each remotes as r (r.name)}
        <li>
          <div class="info">
            <span class="name">{r.name}</span>
            <span class="url" title={r.fetchUrl}>{r.fetchUrl}</span>
            {#if r.pushUrl && r.pushUrl !== r.fetchUrl}<span class="url" title={r.pushUrl}>push: {r.pushUrl}</span>{/if}
          </div>
          {#if confirmRemove === r.name}
            <button class="btn sm danger" onclick={() => remove(r.name)}>Remove {r.name}</button>
            <button class="btn quiet" onclick={() => (confirmRemove = null)}>Keep</button>
          {:else}
            <button class="btn quiet" onclick={() => startEdit(r)}>Edit</button>
            <button class="btn quiet" onclick={() => (confirmRemove = r.name)}>Remove</button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}

  {#if editing !== null}
    <form onsubmit={save}>
      <label class="form-field"><span>Name</span><input bind:value={name} placeholder="origin" /></label>
      <label class="form-field"><span>URL</span><input bind:value={url} placeholder="git@github.com:you/repo.git" /></label>
      {#if error}<p class="form-error" role="alert">{error}</p>{/if}
      <div class="form-actions">
        <InkButton variant="secondary" onclick={() => (editing = null)}>Cancel</InkButton>
        <InkButton type="submit">{editing === '' ? 'Add remote' : 'Save'}</InkButton>
      </div>
    </form>
  {:else}
    {#if error}<p class="form-error" role="alert">{error}</p>{/if}
    <div class="form-actions">
      <InkButton variant="secondary" onclick={() => startEdit()}>Add remote…</InkButton>
      <InkButton onclick={onClose}>Done</InkButton>
    </div>
  {/if}
</Modal>

<style>
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  li {
    display: flex;
    gap: 4px;
    align-items: center;
    padding: 10px 0;
    border-bottom: 1px dotted var(--paper-edge);
  }

  .info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .name {
    font-size: 16px;
    font-weight: 500;
  }

  .url {
    color: var(--ink-mute);
    font-family: var(--font-mono);
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
</style>
