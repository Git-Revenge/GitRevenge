<script lang="ts">
  import { BranchPatchBase, CancelDownload, ChooseFolder, DownloadFiles, PatchSeries, SavePatches, ShowFolder } from '../../wailsjs/go/app/App.js'
  import type { app, git } from '../../wailsjs/go/models'
  import { errorText } from './errors'
  import { patchFolder, patchSignoff, rememberPatchChoices, versions, type PatchStart } from './patches'
  import { toasts } from './toasts.svelte'
  import Busy from './ui/Busy.svelte'
  import Icon from './ui/Icon.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  /** Save as patches (#1021): format-patch's choices, with the commits it would save listed
   * before anything is written. Every default is what format-patch does with no options. */
  let { path, start, onClose }: { path: string; start: PatchStart; onClose: () => void } = $props()
  const uid = $props.id()
  const store = (() => {
    try {
      return localStorage
    } catch {
      return null
    }
  })()

  // svelte-ignore state_referenced_locally
  const first = start
  /** Where the commits start: the branch's base, the last N up to the tip, or another branch,
   * tag or commit. Commits picked in the graph have none. */
  let since = $state<'base' | 'last' | 'ref'>(first.kind === 'branch' ? 'base' : 'last')
  let base = $state('')
  let last = $state(first.kind === 'branch' ? 3 : 1)
  let ref = $state('')
  const tip = first.kind === 'branch' ? first.branch : first.kind === 'commit' || first.kind === 'range' ? first.tip : ''
  const name = first.kind === 'branch' ? first.branch : first.kind === 'commit' ? first.subject : ''

  // A branch's series starts where it left its base, once that's known.
  if (first.kind === 'branch') {
    // svelte-ignore state_referenced_locally
    void BranchPatchBase(path, first.branch, first.stackParent ?? '')
      .then((b) => {
        base = b
        if (!b) since = 'last'
      })
      .catch(() => (since = 'last'))
  }

  let revs = $derived<git.PatchRevs | null>(
    first.kind === 'ids'
      ? { since: '', tip: '', last: 0, ids: first.ids }
      : first.kind === 'range'
        ? { since: first.since, tip: first.tip, last: 0, ids: [] }
        : since === 'base'
        ? base
          ? { since: base, tip, last: 0, ids: [] }
          : null
        : since === 'ref'
          ? ref.trim()
            ? { since: ref.trim(), tip, last: 0, ids: [] }
            : null
          : { since: '', tip, last: Math.max(1, Math.floor(Number(last)) || 1), ids: [] },
  )

  let series = $state<app.PatchSeries | null>(null)
  let seriesError = $state('')
  let readTimer: ReturnType<typeof setTimeout> | undefined
  // The list follows the choice of commits, a moment behind the typing.
  $effect(() => {
    const r = revs
    clearTimeout(readTimer)
    series = null
    seriesError = ''
    if (!r) return
    readTimer = setTimeout(() => {
      void PatchSeries(path, r)
        .then((s) => {
          if (r === revs) series = s
        })
        .catch((e) => {
          if (r === revs) seriesError = errorText(e)
        })
    }, 250)
    return () => clearTimeout(readTimer)
  })

  let asMbox = $state(false)
  let mboxName = $state(`${(name && first.kind === 'branch' ? name : 'patches').replace(/[\\/:*?"<>|]+/g, '-')}.mbox`)
  // svelte-ignore state_referenced_locally
  let folder = $state(patchFolder(path, store))
  let prefix = $state('PATCH')
  let version = $state(1)
  let numbered = $state(true)
  let cover = $state(false)
  let coverSubject = $state('')
  let coverText = $state('')
  // svelte-ignore state_referenced_locally
  let signoff = $state(patchSignoff(path, store))
  let withBase = $state(false)

  let count = $derived(series?.count ?? 0)
  let v = $derived(Math.max(1, Math.floor(Number(version)) || 1))
  let example = $derived(
    series?.commits.length ? `[${prefix.trim() || 'PATCH'}${v > 1 ? ` v${v}` : ''}${numbered && count > 1 ? ` 1/${count}` : ''}] ${series.commits[0].subject}` : '',
  )
  let files = $derived(count + (cover ? 1 : 0))
  let blocked = $derived(!series || !count || series.edge || !folder.trim() || (asMbox && !mboxName.trim()))

  let busy = $state(false)
  let downloading = $state(false)
  let error = $state('')
  /** Files already in the folder that saving would replace: asked about before it does. */
  let exists = $state<string[] | null>(null)
  let token = ''
  let downloaded = $state(false)

  async function browse() {
    const chosen = await ChooseFolder('Save the patches into').catch(() => '')
    if (chosen) folder = chosen
  }

  async function save(overwrite = false) {
    if (!series || !revs || busy) return
    busy = true
    error = ''
    exists = null
    try {
      // In a partial clone the patches' files come first, in one download: by themselves for
      // one commit, and for more once Save, which says so, is pressed (#1003).
      if (series.missing?.length && !downloaded) {
        downloading = true
        token = `patches-${Date.now()}`
        await DownloadFiles(path, token, series.missing)
        downloaded = true
        downloading = false
      }
      const opts = { prefix, version: v, numbered, cover, coverSubject, coverText, signoff, base: withBase ? series.base : '' } as git.PatchOptions
      const res = await SavePatches(path, revs, opts, folder.trim(), asMbox ? mboxName.trim() : '', overwrite)
      if (res.exists?.length) {
        exists = res.exists
        return
      }
      rememberPatchChoices(path, folder.trim(), signoff, store)
      const what = res.files.length === 1 ? res.files[0] : `${res.files.length} files`
      toasts.show(`Saved ${what} to ${res.folder}`, { label: 'Show in folder', run: () => ShowFolder(res.folder) })
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
    void CancelDownload(token)
  }

  const short = (id: string) => id.slice(0, 7)
  const title = first.kind === 'branch' ? `Save ${first.branch} as patches` : 'Save as patches'
</script>

<Modal {title} eyebrow="send it along" width={620} onClose={() => !busy && onClose()} scrimCloses={!busy}>
  <form
    onsubmit={(e) => {
      e.preventDefault()
      void save()
    }}
  >
    <fieldset disabled={busy}>
      <legend>Commits</legend>
      <div class="choices">
        {#if first.kind === 'branch' || first.kind === 'commit'}
          <span class="line">
            <label for="{uid}-since">Since</label>
            <select id="{uid}-since" bind:value={since}>
              {#if first.kind === 'branch' && base}<option value="base">{base}, where {first.branch} left it</option>{/if}
              <option value="last">the last few commits</option>
              <option value="ref">another branch, tag or commit</option>
            </select>
            {#if since === 'last'}
              <input class="num" aria-label="How many commits" inputmode="numeric" bind:value={last} />
            {:else if since === 'ref'}
              <input class="ref" aria-label="Since which branch, tag or commit" placeholder="main, v1.2 or a commit" bind:value={ref} />
            {/if}
            <span class="up">up to {first.kind === 'branch' ? first.branch : short(tip)}</span>
          </span>
        {/if}
        {#if seriesError}
          <p class="problem" role="alert">{seriesError}</p>
        {:else if !series}
          <p class="hint"><Busy /> Listing them…</p>
        {:else if !series.count}
          <p class="problem">There are no commits to save{series.merges ? ': merges are left out, as format-patch leaves them out' : ''}.</p>
        {:else}
          <ol class="commits" aria-label="The commits, oldest first">
            {#each series.commits as c, i (c.id)}
              <li><span class="n">{i + 1}/{series.count}</span><span class="id">{short(c.id)}</span><span class="subject">{c.subject}</span></li>
            {/each}
          </ol>
          {#if series.count > series.commits.length}<p class="hint">and {(series.count - series.commits.length).toLocaleString()} more</p>{/if}
          <p class="hint">
            Oldest first.{#if series.merges}
              {series.merges === 1 ? 'A merge is' : `${series.merges} merges are`} left out, as format-patch leaves merges out.{/if}
          </p>
          {#if series.edge}
            <p class="problem" role="alert">The oldest is where this shallow clone’s history stops: its parent isn’t here, so its patch would add every file. Choose fewer commits, or fetch more history first.</p>
          {/if}
        {/if}
      </div>
    </fieldset>

    <fieldset disabled={busy}>
      <legend>Save as</legend>
      <div class="choices">
        <label><input type="radio" name="{uid}-as" value={false} bind:group={asMbox} /> One file per commit</label>
        <span class="line">
          <label><input type="radio" name="{uid}-as" value={true} bind:group={asMbox} /> One mailbox file</label>
          {#if asMbox}<input class="ref" aria-label="The mailbox file's name" bind:value={mboxName} />{/if}
        </span>
      </div>
    </fieldset>

    <fieldset disabled={busy}>
      <legend>Into folder</legend>
      <span class="row">
        <input aria-label="Into folder" bind:value={folder} />
        <button type="button" class="btn quiet" onclick={browse}><Icon name="folder" />Browse…</button>
      </span>
    </fieldset>

    <fieldset disabled={busy}>
      <legend>Subject</legend>
      <div class="choices">
        <span class="line">
          <label for="{uid}-prefix">Prefix</label>
          <input id="{uid}-prefix" class="prefix" bind:value={prefix} />
          <label for="{uid}-version">Version</label>
          <input id="{uid}-version" class="num" type="number" min="1" bind:value={version} />
          <label><input type="checkbox" bind:checked={numbered} /> Number them</label>
        </span>
        {#if example}<code class="example">{example}</code>{/if}
      </div>
    </fieldset>

    <fieldset disabled={busy}>
      <legend>Also</legend>
      <div class="choices">
        <label><input type="checkbox" bind:checked={cover} /> A cover letter first</label>
        {#if cover}
          <div class="cover">
            <input aria-label="The cover letter's subject" placeholder="What the series does" bind:value={coverSubject} />
            <textarea aria-label="The cover letter's text" rows="3" placeholder="Why, and what to look at first" bind:value={coverText}></textarea>
          </div>
        {/if}
        <label><input type="checkbox" bind:checked={signoff} /> Add Signed-off-by</label>
        {#if series?.base}
          <label><input type="checkbox" bind:checked={withBase} /> Say which commit they apply to</label>
        {/if}
      </div>
    </fieldset>

    {#if series?.missing?.length}
      <p class="hint" role="status">
        {#if downloading}
          <Busy /> Downloading their files… <button type="button" class="btn text" onclick={stop}>Stop</button>
        {:else if series.count > 1}
          They need {versions(series.missing.length)} this partial clone hasn’t got yet. Save downloads them first, in one download.
        {:else}
          Its files download first, by themselves.
        {/if}
      </p>
    {/if}
    {#if exists}
      <div class="exists" role="alert">
        <p>{exists.length === 1 ? `${exists[0]} is` : `${exists.length} of them are`} already in {folder.trim()}.</p>
        <span class="row">
          <button type="button" class="btn secondary" onclick={() => void save(true)}>Replace {exists.length === 1 ? 'it' : 'them'}</button>
          <button type="button" class="btn quiet" onclick={() => (exists = null)}>Keep {exists.length === 1 ? 'it' : 'them'}</button>
        </span>
      </div>
    {/if}
    {#if error}<p class="form-error" role="alert">{error}</p>{/if}
    <div class="form-actions">
      <InkButton variant="secondary" onclick={onClose} disabled={busy}>Cancel</InkButton>
      <InkButton type="submit" disabled={busy || blocked || !!exists}>
        {series?.missing?.length && series.count > 1 && !downloaded ? 'Download and save' : asMbox ? `Save ${mboxName.trim() || 'the mailbox'}` : `Save ${files.toLocaleString()} ${files === 1 ? 'file' : 'files'}`}
      </InkButton>
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

  .choices label,
  .line {
    display: flex;
    align-items: center;
    gap: 7px;
  }

  .line {
    flex-wrap: wrap;
  }

  input[type='radio'],
  input[type='checkbox'] {
    accent-color: var(--accent);
    margin: 0;
  }

  .line input:not([type='radio'], [type='checkbox']),
  .line select,
  .cover input,
  .cover textarea,
  .row input {
    font-size: 13px;
    padding: 4px 7px;
    border: 1px solid var(--paper-edge);
    border-radius: 4px;
    background: var(--paper);
    color: var(--ink);
  }

  .line input:not([type='radio'], [type='checkbox']),
  .row input {
    font-family: var(--font-mono);
  }

  .num {
    width: 56px;
  }

  .prefix {
    width: 110px;
  }

  .ref {
    flex: 1;
    min-width: 140px;
  }

  .up {
    font-size: 13px;
    color: var(--ink-mute);
  }

  .row {
    display: flex;
    gap: 6px;
  }

  .row input {
    flex: 1;
    min-width: 0;
  }

  .commits {
    margin: 0;
    padding: 8px 10px;
    list-style: none;
    max-height: 168px;
    overflow-y: auto;
    border-radius: var(--r-md);
    background: var(--paper-warm);
    font-size: 13px;
  }

  .commits li {
    display: flex;
    gap: 10px;
    align-items: baseline;
    min-height: 22px;
  }

  .n,
  .id {
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .n {
    color: var(--ink-mute);
  }

  .id {
    color: var(--accent-b);
  }

  .subject {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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

  .cover {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding-left: 22px;
  }

  .cover textarea {
    font-family: inherit;
    resize: vertical;
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
