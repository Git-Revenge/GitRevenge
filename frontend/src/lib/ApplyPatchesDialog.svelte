<script lang="ts">
  import { onDestroy, untrack } from 'svelte'
  import { ApplyPatches, CancelDownload, ClosePatches, DownloadFiles, RepoState, TryPatches } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import { branchNameOf, fileSummary, seriesOf, titleOf } from './amPlan'
  import { errorText } from './errors'
  import { onBackend } from './events'
  import { versions } from './patches'
  import { toasts } from './toasts.svelte'
  import Busy from './ui/Busy.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  /** Apply patches (#1022): a file's series, each patch tried in order on a throwaway copy of
   * the index before anything changes, then git am --3way with the ones ticked. */
  let { path, file, onClose }: { path: string; file: app.PatchFile; onClose: () => void } = $props()
  const uid = $props.id()
  const store = (() => {
    try {
      return localStorage
    } catch {
      return null
    }
  })()
  // svelte-ignore state_referenced_locally
  const f = file
  const signoffKey = `gitrevenge.am.signoff:${untrack(() => path)}`

  const msgs = f.messages
  const series = seriesOf(msgs)
  /** The version shown: the newest, when a file holds more than one. */
  let version = $state(series.versions[series.versions.length - 1]?.version ?? 1)
  let v = $derived(series.versions.find((x) => x.version === version))
  let title = $derived(v ? titleOf(msgs, v) : f.name)
  /** The patches left out by unticking them, by their index in the file. */
  let off = $state<Set<number>>(new Set())
  let picked = $derived(v ? v.patches.filter((i) => !off.has(i)) : [])

  let onto = $state<'here' | 'new'>(f.branch ? 'here' : 'new')
  let branch = $state('')
  let branchEdited = false
  $effect(() => {
    const name = branchNameOf(title)
    if (!branchEdited) branch = name
  })
  let signoff = $state(store?.getItem(signoffKey) === '1')

  // The try: rows fill in as each patch's finishes. A new one starts when the ticks or the
  // version change; its run number tells its rows from an older one's.
  let results = $state<Record<number, app.PatchTry>>({})
  let run = 0
  let trying = $state(false)
  let needs = $state(f.missing?.length ?? 0)
  let downloading = $state(false)
  let token = ''
  const off$ = onBackend('patches:tried', (row) => {
    if (row.token !== f.token || row.run !== run) return
    results = { ...results, [row.index]: row }
  })
  onDestroy(() => {
    off$()
    void ClosePatches(f.token)
  })

  let tryTimer: ReturnType<typeof setTimeout> | undefined
  /** Bumped when the person ticks or unticks a patch: a try follows. An automatic untick
   * doesn't bump it, as it changes nothing tried after it. */
  let ticks = $state(0)
  $effect(() => {
    void ticks
    void version
    const p = untrack(() => picked)
    if (needs) return
    clearTimeout(tryTimer)
    tryTimer = setTimeout(() => void tryThem(p), 150)
    return () => clearTimeout(tryTimer)
  })

  async function tryThem(p: number[]) {
    const mine = ++run
    results = {}
    trying = true
    try {
      const rows = await TryPatches(path, f.token, p, mine)
      if (mine !== run) return
      const next: Record<number, app.PatchTry> = {}
      for (const r of rows ?? []) next[r.index] = r
      results = next
      // One that's in already starts unticked; leaving it out changes nothing tried after it.
      const already = (rows ?? []).filter((r) => r.status === 'applied' && !touched.has(r.index)).map((r) => r.index)
      if (already.length) off = new Set([...off, ...already])
    } catch (e) {
      if (mine === run) error = errorText(e)
    } finally {
      if (mine === run) trying = false
    }
  }

  /** The patches the person ticked or unticked themselves: an automatic untick leaves them be. */
  // Nothing shown reads it, so it needn't be reactive.
  // eslint-disable-next-line svelte/prefer-svelte-reactivity
  const touched = new Set<number>()
  function toggle(i: number) {
    touched.add(i)
    // A copy, assigned whole: off is state.
    // eslint-disable-next-line svelte/prefer-svelte-reactivity
    const next = new Set(off)
    if (next.has(i)) next.delete(i)
    else next.add(i)
    off = next
    ticks++
  }

  async function download() {
    downloading = true
    token = `am-${Date.now()}`
    error = ''
    try {
      await DownloadFiles(path, token, f.missing ?? [])
      needs = 0
    } catch (e) {
      error = /cancel|stopp?ed|killed|interrupt/i.test(errorText(e)) ? '' : errorText(e)
    } finally {
      downloading = false
    }
  }

  let error = $state('')
  let busy = $state(false)
  let conflictRow = $derived(picked.find((i) => results[i]?.status === 'conflicts'))

  async function apply() {
    if (busy || !picked.length) return
    busy = true
    error = ''
    try {
      store?.setItem(signoffKey, signoff ? '1' : '0')
      await ApplyPatches(path, f.token, picked, onto === 'new' ? branch.trim() : '', signoff)
      toasts.show(`Applied ${picked.length === 1 ? 'a patch' : `${picked.length} patches`} onto ${onto === 'new' ? branch.trim() : f.branch || 'HEAD'}.`)
      onClose()
    } catch (e) {
      // Stopped part way, git am is in progress: the strip over the graph carries on from here.
      const state = await RepoState(path).catch(() => null)
      if (state?.operation === 'am') return onClose()
      error = errorText(e)
    } finally {
      busy = false
    }
  }

  const short = (id: string) => id.slice(0, 7)
  const date = (s: string) => {
    const d = new Date(s)
    return isNaN(d.getTime()) ? s : d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
  }
  function status(i: number): { text: string; tone: 'ok' | 'warn' | 'mute' } {
    const r = results[i]
    // One that's in already says so, ticked or not; any other unticked one is left out.
    if (off.has(i) && r?.status !== 'applied') return { text: 'left out', tone: 'mute' }
    if (!r) return { text: needs ? 'not tried' : 'trying…', tone: 'mute' }
    if (r.waits) return { text: conflictRow !== undefined ? `after you resolve ${msgs[conflictRow].number || picked.indexOf(conflictRow) + 1}` : 'waits', tone: 'mute' }
    switch (r.status) {
      case 'applies':
        return { text: 'applies', tone: 'ok' }
      case 'merges':
        return { text: 'applies by merging', tone: 'ok' }
      case 'conflicts':
        return { text: `conflicts in ${(r.files ?? []).join(', ')}`, tone: 'warn' }
      case 'applied':
        return { text: r.already ? `already here as ${short(r.already)}` : 'already here', tone: 'mute' }
    }
    return { text: 'can’t apply', tone: 'warn' }
  }
  const n = (i: number) => (msgs[i].number && msgs[i].total ? `${msgs[i].number}/${msgs[i].total}` : `${(v?.patches.indexOf(i) ?? 0) + 1}`)
  const plus = (i: number) => msgs[i].files.reduce((a, x) => a + x.added, 0)
  const minus = (i: number) => msgs[i].files.reduce((a, x) => a + x.deleted, 0)
</script>

<Modal title={title} eyebrow="bring it in" width={680} onClose={() => !busy && onClose()} scrimCloses={!busy}>
  <p class="lead"><span class="mono">{f.name}</span> · {fileSummary(msgs, series, v)}</p>

  {#if series.versions.length > 1}
    <fieldset class="versions" disabled={busy}>
      <legend>Version</legend>
      <div class="choices row">
        {#each series.versions as x (x.version)}
          <label><input type="radio" name="{uid}-version" value={x.version} bind:group={version} /> v{x.version} · {x.patches.length} {x.patches.length === 1 ? 'patch' : 'patches'}</label>
        {/each}
      </div>
    </fieldset>
  {/if}

  {#if v && v.cover >= 0}
    <p class="cover">{titleOf(msgs, v)} <span class="who">{msgs[v.cover].author} · the cover letter isn’t applied</span></p>
  {/if}

  {#if needs}
    <p class="note" role="status">
      {#if downloading}
        <Busy /> Downloading… <button type="button" class="btn text" onclick={() => void CancelDownload(token)}>Stop</button>
      {:else}
        The preview needs {versions(needs)} this partial clone hasn’t got yet.
        <button type="button" class="btn text" onclick={download}>Download them</button>
      {/if}
    </p>
  {/if}

  {#if v}
    <ol class="rows" aria-label="The patches, in the order they apply">
      {#each v.patches as i (i)}
        {@const s = status(i)}
        <li class:off={off.has(i)}>
          <input type="checkbox" checked={!off.has(i)} disabled={busy} aria-label="Apply {msgs[i].subject}" onchange={() => toggle(i)} />
          <span class="n">{n(i)}</span>
          <span class="what">
            <span class="subject">{msgs[i].subject}</span>
            <span class="meta">{msgs[i].author} · {date(msgs[i].date)} · +{plus(i)} −{minus(i)} · {msgs[i].files.length} {msgs[i].files.length === 1 ? 'file' : 'files'}</span>
          </span>
          <span class="status {s.tone}" title={results[i]?.error || undefined}>{s.text}</span>
        </li>
      {/each}
    </ol>
  {:else}
    <p class="note">There are no patches in it to apply.</p>
  {/if}

  <fieldset disabled={busy}>
    <legend>Onto</legend>
    <div class="choices">
      {#if f.branch}<label><input type="radio" name="{uid}-onto" value="here" bind:group={onto} /> {f.branch}, where you are</label>{/if}
      <span class="line">
        <label><input type="radio" name="{uid}-onto" value="new" bind:group={onto} /> A new branch from here</label>
        <input
          class="branch"
          aria-label="The new branch's name"
          bind:value={branch}
          oninput={() => (branchEdited = true)}
          onfocus={() => (onto = 'new')}
        />
      </span>
    </div>
  </fieldset>
  <fieldset disabled={busy}>
    <legend>Also</legend>
    <div class="choices"><label><input type="checkbox" bind:checked={signoff} /> Add Signed-off-by</label></div>
  </fieldset>

  {#if f.staged}
    <p class="form-error" role="alert">git am needs nothing staged. Commit or unstage your staged changes first.</p>
  {/if}
  {#if error}<p class="form-error" role="alert">{error}</p>{/if}
  <div class="form-actions">
    <span class="foot">
      {#if trying}<Busy /> Trying them…{:else if conflictRow !== undefined}Patch {n(conflictRow)} stops for {(results[conflictRow]?.files ?? []).join(', ')}; you resolve it there.{/if}
    </span>
    <InkButton variant="secondary" onclick={onClose} disabled={busy}>Cancel</InkButton>
    <InkButton disabled={busy || f.staged || !picked.length || (onto === 'new' && !branch.trim())} onclick={apply}>
      {picked.length === 1 ? 'Apply 1 patch' : `Apply ${picked.length} patches`}
    </InkButton>
  </div>
</Modal>

<style>
  .lead,
  .cover,
  .note {
    margin: 0 0 12px;
    font-size: 14px;
    line-height: 1.5;
    color: var(--ink-soft);
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 12.5px;
  }

  .cover {
    padding: 8px 12px;
    border-radius: var(--r-md);
    background: var(--paper-warm);
    color: var(--ink);
  }

  .who,
  .meta,
  .foot {
    font-size: 12.5px;
    color: var(--ink-mute);
  }

  .note {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }

  .rows {
    margin: 0 0 14px;
    padding: 0;
    list-style: none;
    max-height: 300px;
    overflow-y: auto;
    border-top: 1px dashed var(--hairline);
  }

  .rows li {
    display: grid;
    grid-template-columns: 20px 38px minmax(0, 1fr) auto;
    gap: 4px 8px;
    align-items: baseline;
    padding: 8px 2px;
    border-bottom: 1px dashed var(--hairline);
  }

  .rows li.off .subject {
    color: var(--ink-mute);
  }

  .rows input {
    accent-color: var(--accent);
    margin: 0;
  }

  .n {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .what {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .subject {
    font-size: 14px;
    overflow-wrap: anywhere;
  }

  .meta {
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.04em;
  }

  .status {
    padding: 2px 8px;
    border-radius: 10px;
    font-size: 12.5px;
    white-space: nowrap;
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .status.ok {
    background: var(--success-wash);
    color: var(--success);
  }

  .status.warn {
    background: color-mix(in srgb, var(--warning) 18%, transparent);
    color: var(--warning);
  }

  .status.mute {
    background: var(--wash-hover);
    color: var(--ink-soft);
  }

  fieldset {
    display: grid;
    grid-template-columns: 70px minmax(0, 1fr);
    gap: 12px;
    align-items: baseline;
    margin: 0 0 10px;
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
    gap: 5px;
    font-size: 14px;
  }

  .choices.row {
    flex-direction: row;
    flex-wrap: wrap;
    gap: 14px;
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

  .choices input[type='radio'],
  .choices input[type='checkbox'] {
    accent-color: var(--accent);
    margin: 0;
  }

  .branch {
    flex: 1;
    min-width: 180px;
    font-family: var(--font-mono);
    font-size: 12.5px;
    padding: 3px 7px;
    border: 1px solid var(--paper-edge);
    border-radius: 4px;
    background: var(--paper);
    color: var(--ink);
  }

  .form-actions {
    align-items: center;
  }

  .foot {
    display: flex;
    align-items: center;
    gap: 6px;
    flex: 1;
    min-width: 0;
  }
</style>
