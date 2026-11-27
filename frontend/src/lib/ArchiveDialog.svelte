<script lang="ts">
  import { ArchivePlan, CancelDownload, ChooseFolder, DownloadFiles, SaveArchive, ShowFolder, StopArchive, StopArchivePlan } from '../../wailsjs/go/app/App.js'
  import type { git } from '../../wailsjs/go/models'
  import { archiveBase, archiveCommand, archiveFile, archiveFolder, archiveFormat, folderOf, FORMATS, planSaid, rememberArchive, type ArchiveFormat } from './archive'
  import { errorText } from './errors'
  import { versions } from './patches'
  import { toasts } from './toasts.svelte'
  import Busy from './ui/Busy.svelte'
  import Icon from './ui/Icon.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  /** Save as an archive (#936): a commit, tag or branch as a zip or tar file, as git archive
   * writes it, with what goes in said before anything is written. */
  let {
    path,
    rev,
    label,
    onClose,
  }: {
    path: string
    /** What to archive, as git takes it: a tag's name, a branch's or a commit's id. */
    rev: string
    /** How it's named to the user, and in the archive's name: "v1.1.0", "feature/x", "a1b2c3d". */
    label: string
    onClose: () => void
  } = $props()
  const uid = $props.id()
  const store = (() => {
    try {
      return localStorage
    } catch {
      return null
    }
  })()

  // svelte-ignore state_referenced_locally
  const base = archiveBase(path, label)
  // svelte-ignore state_referenced_locally
  let format = $state<ArchiveFormat>(archiveFormat(path, store))
  let prefix = $state(`${base}/`)
  let sub = $state('')
  // svelte-ignore state_referenced_locally
  let folder = $state(archiveFolder(path, store))
  // The name follows the format until it's typed in.
  let typedName = $state<string | null>(null)
  let name = $derived(typedName ?? archiveFile(base, format))

  let plan = $state<git.ArchivePlan | null>(null)
  let planError = $state('')
  let readTimer: ReturnType<typeof setTimeout> | undefined
  // What goes in follows the folder chosen, a moment behind the typing. On the kernel it reads
  // for about 3 s; asking again stops the read before.
  $effect(() => {
    const s = sub
    clearTimeout(readTimer)
    plan = null
    planError = ''
    readTimer = setTimeout(() => {
      void ArchivePlan(path, rev, s)
        .then((p) => {
          if (s === sub && p.commit) plan = p
        })
        .catch((e) => {
          if (s === sub) planError = errorText(e)
        })
    }, 250)
    return () => clearTimeout(readTimer)
  })
  $effect(() => () => void StopArchivePlan(path).catch(() => {}))

  let busy = $state(false)
  let downloading = $state(false)
  let downloaded = $state(false)
  let error = $state('')
  /** The file's already in the folder: asked about before it's replaced. */
  let exists = $state(false)
  let token = ''

  let blocked = $derived(!plan || !plan.files || !folder.trim() || !name.trim())
  let command = $derived(archiveCommand({ format, prefix, sub, file: name, rev: label }))

  async function browse() {
    const chosen = await ChooseFolder('Save the archive into').catch(() => '')
    if (chosen) folder = chosen
  }

  async function save(overwrite = false) {
    if (!plan || busy) return
    busy = true
    error = ''
    exists = false
    try {
      // In a partial clone its files come first, in one download: archiving doesn't fetch
      // them one by one.
      if (plan.missing.length && !downloaded) {
        downloading = true
        token = `archive-${Date.now()}`
        await DownloadFiles(path, token, plan.missing)
        downloaded = true
        downloading = false
      }
      const res = await SaveArchive(path, rev, { format, prefix: folderOf(prefix), sub } as git.ArchiveOptions, folder.trim(), name.trim(), overwrite)
      if (res.exists) {
        exists = true
        return
      }
      if (res.stopped) return
      rememberArchive(path, folder.trim(), format, store)
      toasts.show(`Saved ${res.file} to ${res.folder}`, { label: 'Show in folder', run: () => ShowFolder(res.folder) })
      onClose()
    } catch (e) {
      const text = errorText(e)
      error = downloading && /cancel|stopp?ed|killed|interrupt/i.test(text) ? 'The download stopped, so nothing was saved.' : text
    } finally {
      busy = false
      downloading = false
    }
  }

  function stop() {
    if (downloading) void CancelDownload(token)
    else void StopArchive(path)
  }
</script>

<Modal title="Save {label} as an archive" eyebrow="git archive" width={620} onClose={() => !busy && onClose()} scrimCloses={!busy}>
  <form
    onsubmit={(e) => {
      e.preventDefault()
      void save()
    }}
  >
    <fieldset disabled={busy}>
      <legend>Format</legend>
      <div class="choices formats">
        {#each FORMATS as f (f)}
          <label><input type="radio" name="{uid}-format" value={f} bind:group={format} /> {f}</label>
        {/each}
      </div>
    </fieldset>

    <fieldset disabled={busy}>
      <legend>Inside it</legend>
      <div class="choices">
        <span class="line">
          <label for="{uid}-prefix">Everything in</label>
          <input id="{uid}-prefix" class="ref" bind:value={prefix} placeholder="no folder" />
        </span>
        <span class="line">
          <label for="{uid}-sub">Only</label>
          <input id="{uid}-sub" class="ref" bind:value={sub} placeholder="the whole tree, or one folder of it" />
        </span>
        {#if planError}
          <p class="problem" role="alert">{planError}</p>
        {:else if !plan}
          <p class="hint"><Busy /> Counting what goes in…</p>
        {:else if !plan.files}
          <p class="problem">Nothing goes in: {plan.ignored ? 'export-ignore in .gitattributes leaves every file out.' : 'there are no files here.'}</p>
        {:else}
          <p class="hint">{planSaid(plan)}</p>
        {/if}
      </div>
    </fieldset>

    <fieldset disabled={busy}>
      <legend>Save to</legend>
      <div class="choices">
        <span class="line">
          <label for="{uid}-folder">Folder</label>
          <input id="{uid}-folder" class="ref" aria-label="Into folder" bind:value={folder} />
          <button type="button" class="btn quiet" onclick={browse}><Icon name="folder" />Browse…</button>
        </span>
        <span class="line">
          <label for="{uid}-name">Named</label>
          <input id="{uid}-name" class="ref" aria-label="The archive's name" value={name} oninput={(e) => (typedName = e.currentTarget.value)} />
        </span>
      </div>
    </fieldset>

    <code class="example">{command}</code>

    {#if plan?.missing.length}
      <p class="hint" role="status">
        {#if downloading}
          <Busy /> Downloading its files… <button type="button" class="btn text" onclick={stop}>Stop</button>
        {:else if !downloaded}
          It needs {versions(plan.missing.length)} this partial clone hasn’t got yet. Save downloads them first, in one download.
        {/if}
      </p>
    {/if}
    {#if busy && !downloading}
      <p class="hint" role="status"><Busy /> Writing {name.trim()}… <button type="button" class="btn text" onclick={stop}>Stop</button></p>
    {/if}
    {#if exists}
      <div class="exists" role="alert">
        <p>{name.trim()} is already in {folder.trim()}.</p>
        <span class="row">
          <button type="button" class="btn secondary" onclick={() => void save(true)}>Replace it</button>
          <button type="button" class="btn quiet" onclick={() => (exists = false)}>Keep it</button>
        </span>
      </div>
    {/if}
    {#if error}<p class="form-error" role="alert">{error}</p>{/if}
    <div class="form-actions">
      <InkButton variant="secondary" onclick={onClose} disabled={busy}>Cancel</InkButton>
      <InkButton type="submit" disabled={busy || blocked || exists}>{plan?.missing.length && !downloaded ? 'Download and save' : `Save ${name.trim() || 'it'}`}</InkButton>
    </div>
  </form>
</Modal>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  fieldset {
    display: grid;
    grid-template-columns: 96px minmax(0, 1fr);
    gap: 12px;
    align-items: baseline;
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }

  legend {
    float: left;
    padding: 0;
    font-size: 13px;
    color: var(--ink-soft);
  }

  .choices {
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 14px;
    min-width: 0;
  }

  .formats {
    flex-direction: row;
    gap: 18px;
  }

  .choices label,
  .line {
    display: flex;
    align-items: center;
    gap: 7px;
  }

  .line label {
    flex: none;
    width: 92px;
    font-size: 13px;
    color: var(--ink-soft);
  }

  input[type='radio'] {
    accent-color: var(--accent);
    margin: 0;
  }

  .line input,
  .row input {
    font-size: 13px;
    font-family: var(--font-mono);
    padding: 4px 7px;
    border: 1px solid var(--paper-edge);
    border-radius: 4px;
    background: var(--paper);
    color: var(--ink);
  }

  .ref {
    flex: 1;
    min-width: 140px;
  }

  .row {
    display: flex;
    gap: 6px;
  }

  .row input {
    flex: 1;
    min-width: 0;
  }

  .example {
    align-self: flex-start;
    padding: 3px 7px;
    border-radius: 3px;
    background: var(--paper-warm);
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-b);
    overflow-wrap: anywhere;
  }

  .hint {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
    margin: 0;
    font-size: 12.5px;
    line-height: 1.45;
    color: var(--ink-mute);
  }

  .problem {
    margin: 0;
    font-size: 13px;
    color: var(--warning);
  }

  .exists {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px 12px;
    border-radius: var(--r-md);
    background: var(--paper-warm);
    font-size: 14px;
  }

  .exists p {
    margin: 0;
  }
</style>
