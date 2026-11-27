<script lang="ts">
  import { onDestroy, untrack } from 'svelte'
  import { ApplyDiff, CancelDownload, ClosePatches, DownloadFiles, TryDiff, UndoDiff } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import { appliedMessage, applyLabel, blockedBy, diffSummary, MODES, rowStatus, undoLabel, type DiffMode } from './diffPlan'
  import { errorText } from './errors'
  import { versions } from './patches'
  import { session } from './session.svelte'
  import { toasts } from './toasts.svelte'
  import Busy from './ui/Busy.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  /** Apply a plain diff (#1024): each file tried on a throwaway copy of the index first, then
   * git apply with the ones ticked, into the files, the index or both. It has no author or
   * message, so it lands in Changes, to commit as your own. */
  let { path, file, onClose }: { path: string; file: app.PatchFile; onClose: () => void } = $props()
  const uid = $props.id()
  // svelte-ignore state_referenced_locally
  const f = file
  const info = f.diff as app.DiffInfo
  const files = info.files

  let mode = $state<DiffMode>('files')
  let reverse = $state(false)
  /** The files left out by unticking them. */
  let off = $state<Set<string>>(new Set())
  let tries = $state<Record<string, app.DiffTry>>({})
  let trying = $state(false)
  let needs = $state(f.missing?.length ?? 0)
  let downloading = $state(false)
  let token = ''
  let error = $state('')
  let busy = $state(false)

  onDestroy(() => void ClosePatches(f.token))

  // A try when the direction changes, or once what's missing is downloaded.
  let run = 0
  $effect(() => {
    const back = reverse
    if (needs) return
    void untrack(() => tryThem(back))
  })

  async function tryThem(back: boolean) {
    const mine = ++run
    tries = {}
    trying = true
    try {
      const rows = await TryDiff(path, f.token, back)
      if (mine !== run) return
      const next: Record<string, app.DiffTry> = {}
      for (const r of rows ?? []) next[r.path] = r
      tries = next
    } catch (e) {
      if (mine === run) error = errorText(e)
    } finally {
      if (mine === run) trying = false
    }
  }

  const status = (x: (typeof files)[number]) => rowStatus(x, tries[x.path], reverse, info.unclean.includes(x.path))
  /** Ticked, and able to go: a file that doesn't fit or is in already never is. */
  let picked = $derived(files.filter((x) => !off.has(x.path) && status(x).can).map((x) => x.path))
  let blocked = $derived(blockedBy(picked, info.unclean, tries, mode))

  function toggle(p: string) {
    // A copy, assigned whole: off is state.
    // eslint-disable-next-line svelte/prefer-svelte-reactivity
    const next = new Set(off)
    if (next.has(p)) next.delete(p)
    else next.add(p)
    off = next
  }

  async function download() {
    downloading = true
    token = `apply-${Date.now()}`
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

  async function apply() {
    if (busy || !picked.length || blocked || trying) return
    busy = true
    error = ''
    const back = reverse
    try {
      const done = await ApplyDiff(path, f.token, picked, mode, back)
      const repo = path
      toasts.show(appliedMessage(done, back), done.id ? { label: undoLabel(done), run: () => void session.run(() => UndoDiff(repo, done.id)).then((r) => r !== undefined && toasts.show('Its files are back as they were.')) } : undefined)
      onClose()
    } catch (e) {
      error = errorText(e)
    } finally {
      busy = false
    }
  }

  const name = (x: (typeof files)[number]) => x.path
</script>

<Modal title="Apply a diff" eyebrow="bring it in" width={640} onClose={() => !busy && onClose()} scrimCloses={!busy}>
  <p class="lead"><span class="mono">{f.name}</span> · {diffSummary(info)}</p>
  <p class="note">It has no author or message, so its changes land in Changes, to look at and commit as your own.</p>
  {#if !info.named}
    <p class="note">It doesn’t name the versions it was made from, as <span class="mono">diff -u</span> doesn’t, so it applies only where it fits exactly.</p>
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

  <ul class="rows" aria-label="The files it changes">
    {#each files as x (x.path)}
      {@const s = status(x)}
      <li class:off={off.has(x.path) || !s.can}>
        <input type="checkbox" checked={!off.has(x.path) && s.can} disabled={busy || !s.can} aria-label="Apply to {name(x)}" onchange={() => toggle(x.path)} />
        <span class="path mono" title={x.path}>{name(x)}</span>
        <span class="counts">{#if x.binary}binary{:else}+{x.added} −{x.deleted}{/if}</span>
        <span class="status {s.tone}" title={tries[x.path]?.error || undefined}>{s.text}</span>
      </li>
    {/each}
  </ul>

  <fieldset disabled={busy}>
    <legend>Apply to</legend>
    <div class="choices">
      {#each MODES as m (m.mode)}
        <label><input type="radio" name="{uid}-mode" value={m.mode} aria-label={m.label} bind:group={mode} /> {m.label} <span class="hint">{m.note}</span></label>
      {/each}
    </div>
  </fieldset>
  <fieldset disabled={busy}>
    <legend>Also</legend>
    <div class="choices"><label><input type="checkbox" bind:checked={reverse} /> In reverse: take it back out <span class="hint">-R</span></label></div>
  </fieldset>

  {#if blocked}<p class="form-error" role="alert">{blocked}</p>{/if}
  {#if error}<p class="form-error" role="alert">{error}</p>{/if}
  <div class="form-actions">
    <span class="foot">{#if trying}<Busy /> Trying each file…{/if}</span>
    <InkButton variant="secondary" onclick={onClose} disabled={busy}>Cancel</InkButton>
    <InkButton disabled={busy || trying || !!needs || !picked.length || !!blocked} onclick={apply}>{applyLabel(picked.length, reverse)}</InkButton>
  </div>
</Modal>

<!-- The same look as the series preview (ApplyPatchesDialog). -->
<style>
  .lead,
  .note {
    margin: 0 0 10px;
    font-size: 14px;
    line-height: 1.5;
    color: var(--ink-soft);
  }

  .note {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    font-size: 13px;
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 12.5px;
  }

  .rows {
    margin: 4px 0 14px;
    padding: 0;
    list-style: none;
    max-height: 300px;
    overflow-y: auto;
    border-top: 1px dashed var(--hairline);
  }

  .rows li {
    display: grid;
    grid-template-columns: 20px minmax(0, 1fr) auto auto;
    gap: 4px 10px;
    align-items: baseline;
    padding: 8px 2px;
    border-bottom: 1px dashed var(--hairline);
  }

  .rows li.off .path,
  .rows li.off .counts {
    color: var(--ink-mute);
  }

  .rows input {
    accent-color: var(--accent);
    margin: 0;
  }

  .path {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--ink);
  }

  .counts {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .hint,
  .foot {
    font-size: 12.5px;
    color: var(--ink-mute);
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

  .choices label {
    display: flex;
    align-items: center;
    gap: 7px;
  }

  .choices input[type='radio'],
  .choices input[type='checkbox'] {
    accent-color: var(--accent);
    margin: 0;
  }

  .foot {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    margin-right: auto;
  }
</style>

