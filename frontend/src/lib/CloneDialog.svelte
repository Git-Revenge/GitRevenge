<script lang="ts">
  import { onMount } from 'svelte'
  import { CancelClone, ChooseFolder, Clone, CloneName, HomeDir } from '../../wailsjs/go/app/App.js'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import { errorText } from './errors'
  import { prefs } from './prefs.svelte'
  import { session } from './session.svelte'
  import Busy from './ui/Busy.svelte'
  import Icon from './ui/Icon.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  let { onClose }: { onClose: () => void } = $props()

  const dirOf = (p: string) => p.replace(/[\\/][^\\/]*$/, '')

  let url = $state('')
  // Start in the clone folder from Settings, else the folder holding the most recently opened
  // repository, else the home folder (filled in on mount, below).
  let parent = $state(prefs.cloneDir || (session.recent[0] ? dirOf(session.recent[0].path) : ''))
  let name = $state('')
  /** True once the user types a name, so it stops following the URL. */
  let nameEdited = $state(false)
  let running = $state(false)
  let progress = $state('')
  let error = $state('')

  async function onUrl() {
    if (nameEdited) return
    const u = url
    const n = await CloneName(u)
    if (u === url && !nameEdited) name = n
  }

  async function browse() {
    const dir = await session.run(() => ChooseFolder('Clone into folder'))
    if (dir) parent = dir
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    running = true
    progress = ''
    error = ''
    try {
      const dir = await Clone(url, parent, name)
      await session.open(dir)
      onClose()
    } catch (err) {
      error = errorText(err)
    } finally {
      running = false
    }
  }

  /** Cancel stops a running clone (its error then shows here); otherwise it closes. */
  function cancel() {
    if (running) CancelClone()
    else onClose()
  }

  onMount(() => {
    if (!parent)
      HomeDir().then((home) => {
        if (!parent && home) parent = home
      })
  })

  onMount(() =>
    EventsOn('net:progress', (p: { op: string; phase: string; percent: number; line: string }) => {
      if (p.op !== 'clone') return
      progress = p.phase ? `${p.phase} ${p.percent}%` : p.line
    }),
  )
</script>

<Modal title="Clone a repository" eyebrow="bring it aboard" width={540} onClose={cancel} scrimCloses={!running}>
  <form onsubmit={submit}>
    <label class="form-field">
      <span>Repository URL</span>
      <input bind:value={url} oninput={onUrl} disabled={running} placeholder="https://… or git@host:owner/repo.git" />
    </label>
    <label class="form-field">
      <span>Into folder</span>
      <span class="row">
        <input bind:value={parent} disabled={running} placeholder="Choose a folder" />
        <button type="button" class="btn quiet" disabled={running} onclick={browse}><Icon name="folder" />Browse…</button>
      </span>
    </label>
    <label class="form-field">
      <span>Folder name</span>
      <input bind:value={name} oninput={() => (nameEdited = true)} disabled={running} />
    </label>
    {#if running}
      <p class="status" role="status"><Busy /> {progress || 'Connecting…'}</p>
    {/if}
    {#if error}<p class="form-error" role="alert">{error}</p>{/if}
    <div class="form-actions">
      <InkButton variant="secondary" onclick={cancel}>{running ? 'Stop' : 'Cancel'}</InkButton>
      <InkButton type="submit" disabled={running || !url.trim() || !parent.trim() || !name.trim()}>Clone</InkButton>
    </div>
  </form>
</Modal>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .row {
    display: flex;
    gap: 6px;
  }

  .row input {
    flex: 1;
    min-width: 0;
  }

  .status {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    overflow-wrap: anywhere;
  }

</style>
