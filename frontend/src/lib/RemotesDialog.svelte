<script lang="ts">
  import { onMount } from 'svelte'
  import { RemoteAction, RemoteFetchRules, RemotePlaces, Remotes, SetRemoteFetchRules, SetRemotePushURLs } from '../../wailsjs/go/app/App.js'
  import PushURLsEditor from './PushURLsEditor.svelte'
  import { git } from '../../wailsjs/go/models'
  import FetchRulesEditor from './FetchRulesEditor.svelte'
  import PlaceGlyph from './PlaceGlyph.svelte'
  import { placeLook, type URLPlace } from './remoteHeader'
  import { prefs } from './prefs.svelte'
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
  // The remote's fetch rules while it's edited, and what they were, so Save writes only a change.
  let rules = $state<git.FetchRules | null>(null)
  let rulesWere = ''
  // Where its pushes go while it's edited, its own URL among them, and what that was.
  let pushUrls = $state<string[]>([])
  let pushWere = ''
  // On removing a remote others push to: whether they stop pushing there too.
  let stopFollowers = $state(true)
  let places = $state<Record<string, URLPlace>>({})

  async function load() {
    try {
      remotes = (await Remotes(path)) ?? []
      places = ((await RemotePlaces(path).catch(() => ({}))) ?? {}) as Record<string, URLPlace>
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
    rules = null
    pushUrls = r ? [...(r.pushUrls?.length ? r.pushUrls : [r.fetchUrl])] : []
    pushWere = JSON.stringify(pushUrls)
    if (r) {
      RemoteFetchRules(path, r.name)
        .then((fr) => {
          rules = fr
          rulesWere = JSON.stringify(fr)
        })
        .catch((e) => (error = errorText(e)))
    }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    let ok = true
    if (editing === '') ok = await act('add', [name, url])
    else if (editing !== null) {
      const old = remotes.find((r) => r.name === editing)
      if (old && name !== old.name) ok = await act('rename', [old.name, name])
      if (ok && old && url !== old.fetchUrl) ok = await act('seturl', [name, url])
      // Written last, under the remote's new name, and only when something changed. A new URL
      // has already replaced its own push URL; the list here still has the old one.
      if (ok && old && JSON.stringify(pushUrls) !== pushWere) {
        const urls = pushUrls.map((u) => (u === old.fetchUrl ? url : u))
        try {
          await SetRemotePushURLs(path, name, urls)
        } catch (err) {
          error = errorText(err)
          ok = false
        }
      }
      if (ok && rules && JSON.stringify(rules) !== rulesWere) {
        try {
          await SetRemoteFetchRules(path, name, git.FetchRules.createFrom($state.snapshot(rules)))
        } catch (err) {
          error = errorText(err)
          ok = false
        }
      }
    }
    if (ok) {
      editing = null
      await load()
    }
  }

  /** Other remotes that push to this one's URL, and their push URLs without it. */
  function followersOf(n: string) {
    const gone = remotes.find((r) => r.name === n)
    if (!gone) return []
    return remotes
      .filter((r) => r.name !== n && (r.pushes ?? []).some((p) => p.remote === n))
      .map((r) => ({ name: r.name, keep: (r.pushUrls ?? []).filter((u) => !(r.pushes ?? []).some((p) => p.url === u && p.remote === n)) }))
  }

  async function remove(n: string) {
    confirmRemove = null
    const followers = followersOf(n)
    if (!(await act('remove', [n]))) return
    if (stopFollowers) {
      for (const f of followers) {
        try {
          await SetRemotePushURLs(path, f.name, f.keep)
        } catch (e) {
          error = errorText(e)
        }
      }
    }
    await load()
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
            {@const followers = followersOf(r.name)}
            {#if followers.length}
              <label class="stop"><input type="checkbox" bind:checked={stopFollowers} /> Stop {followers.map((f) => f.name).join(' and ')} pushing there</label>
            {/if}
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
      {#if editing}
        {@const r = remotes.find((x) => x.name === editing)}
        {#if r}<p class="where"><PlaceGlyph look={placeLook(places[r.fetchUrl])} brand={prefs.forgeColors} /><span class="url">{r.fetchUrl}</span></p>{/if}
      {/if}
      <label class="form-field"><span>Name</span><input bind:value={name} placeholder="origin" /></label>
      <label class="form-field"><span>URL</span><input bind:value={url} placeholder="git@github.com:you/repo.git" /></label>
      {#if rules}<FetchRulesEditor remote={editing || name} bind:rules />{/if}
      {#if editing}
        {@const r = remotes.find((x) => x.name === editing)}
        {#if r}
          <PushURLsEditor remote={name || editing} fetchUrl={r.fetchUrl} bind:urls={pushUrls} others={remotes.filter((x) => x.name !== editing).map((x) => ({ name: x.name, url: x.fetchUrl }))} {places} brand={prefs.forgeColors} />
        {/if}
      {/if}
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
  .stop {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
  }

  .stop input {
    accent-color: var(--accent);
  }

  .where {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0 0 8px;
    color: var(--ink-soft);
  }

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
