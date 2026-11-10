<script lang="ts">
  import { Integrate, PreviewMerge, PreviewRebase, RepoState } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import { errorText } from './errors'
  import { mergeChoices, mergeSummary, mergeVerb, plural, rebaseVerb, type MergeHow } from './preview'
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'
  import Modal from './ui/Modal.svelte'

  /** What a merge or a rebase would do, shown before it runs (issue #595). */
  let {
    path,
    kind,
    target,
    label,
    current,
    onClose,
    onEdit,
  }: {
    path: string
    kind: 'merge' | 'rebase'
    /** The ref or commit to merge in, or to rebase onto. */
    target: string
    /** How the target is named to the user. */
    label: string
    /** The checked-out branch. */
    current: string
    onClose: () => void
    /** Opens the rebase editor for the commits after base. */
    onEdit: (base: string) => void
  } = $props()

  let merge = $state<app.MergePreview | null>(null)
  let rebase = $state<app.RebasePreview | null>(null)
  let failed = $state('')
  let how = $state<MergeHow | null>(null)
  let fold = $state(true)
  let edit = $state(false)

  // The dialog is useful without a forecast: it says so and still offers the operation.
  $effect(() => {
    if (kind === 'merge') {
      void PreviewMerge(path, target)
        .then((p) => (merge = p))
        .catch((e) => (failed = errorText(e)))
    } else {
      void PreviewRebase(path, target)
        .then((p) => (rebase = p))
        .catch((e) => (failed = errorText(e)))
    }
  })

  let choices = $derived(merge ? mergeChoices(merge, current, label) : [])
  let chosen = $derived(how ?? choices.find((c) => !c.off)?.how ?? 'no-ff')
  let conflicts = $derived((kind === 'merge' ? merge?.conflicts : rebase?.conflicts) ?? [])
  let unknown = $derived(!!failed || (kind === 'merge' ? !!merge?.unknown : !!rebase?.incomplete))
  let loaded = $derived(!!failed || (kind === 'merge' ? !!merge : !!rebase))
  let nothing = $derived(kind === 'merge' ? !!merge?.alreadyMerged : !!rebase && rebase.commitCount === 0)
  let fixups = $derived(rebase?.fixups ?? 0)
  // A fast-forward or a squash can't stop on these conflicts the way a merge commit does.
  let showConflicts = $derived(conflicts.length > 0 && !(kind === 'merge' && chosen === 'ff-only'))

  const short = (id: string) => id.slice(0, 7)

  function run() {
    // Closing takes the props away, so everything the operation needs is read first.
    const [repo, action, onto] = [path, kind, target]
    if (action === 'rebase' && edit) {
      onClose()
      onEdit(onto)
      return
    }
    const args = action === 'merge' ? [onto, chosen] : fold && fixups > 0 ? [onto, 'autosquash'] : [onto]
    const expected = showConflicts
    onClose()
    void Integrate(repo, action, args).catch(async (e: unknown) => {
      // Stopping on the conflicts the preview named is what was asked for, not an error: the
      // operation strip takes over from here.
      const state = expected ? await RepoState(repo).catch(() => null) : null
      if (!state?.conflicts?.length) session.error = errorText(e)
    })
  }
</script>

<Modal title={kind === 'merge' ? `Merge ${label} into ${current}` : `Rebase ${current} onto ${label}`} eyebrow="before it happens" width={620} {onClose}>
  <form
    class="preview"
    onsubmit={(e) => {
      e.preventDefault()
      if (loaded && !nothing) run()
    }}
  >
    {#if !loaded}
      <p class="form-note" role="status">Working out what this would do…</p>
    {:else if nothing}
      <p class="form-note" role="status">
        {#if kind === 'merge'}<span class="mono">{current}</span> already has everything in <span class="mono">{label}</span>. There's nothing to merge.{:else}<span class="mono">{current}</span>
          has no commits that <span class="mono">{label}</span> doesn't. There's nothing to rebase.{/if}
      </p>
    {:else}
      {#if merge}
        <div class="how">
          <span class="meta">How</span>
          <span class="segmented" role="group" aria-label="How to merge">
            {#each choices as c (c.how)}
              <button type="button" aria-pressed={chosen === c.how} disabled={c.off} title={c.why || undefined} onclick={() => (how = c.how)}>{c.label}</button>
            {/each}
          </span>
        </div>
        {#each choices.filter((c) => c.off) as c (c.how)}
          <p class="form-note off">{c.why}</p>
        {/each}
      {/if}

      {#if unknown}
        <div class="card">
          <div class="head">
            <Icon name="info" size={20} />
            <strong>{kind === 'rebase' && rebase?.incomplete && !failed ? 'Not every commit could be checked' : "Conflicts couldn't be checked ahead of time"}</strong>
          </div>
          <p class="form-note">
            {kind === 'rebase' && rebase?.incomplete && !failed ? 'None of the commits that were tried conflict.' : ''} If there are any, the {kind} stops and shows them, and you can resolve them or abort.
          </p>
        </div>
      {:else if showConflicts}
        <div class="card warn" role="status">
          <div class="head">
            <span class="ic warn"><Icon name="warning" size={20} /></span>
            <strong>
              {#if rebase}It will stop at commit {rebase.stopsAtNumber || ''}: {plural(conflicts.length, 'file conflicts', 'files conflict')}{:else}{plural(conflicts.length, 'file', 'files')} will conflict{/if}
            </strong>
            <span class="form-note">checked without touching your files</span>
          </div>
          {#if rebase?.stopsAt}
            <p class="at"><span class="mono sha">{short(rebase.stopsAt)}</span> {rebase.stopsAtSubject}</p>
          {/if}
          <ul class="rows">
            {#each conflicts as c (c.path)}
              <li>
                <span class="mono path" title={c.path}>{c.path}</span>
                {#if c.places}<span class="form-note">{plural(c.places, 'place', 'places')}</span>{/if}
              </li>
            {/each}
          </ul>
          <p class="form-note">
            {#if merge && chosen === 'squash'}You'll resolve them in your files before you commit.{:else}You'll resolve them in the conflict tool, then continue. Nothing is committed until you do.{/if}
            {#if rebase}Later commits may conflict too: they can't be tried until this one is settled.{/if}
          </p>
        </div>
      {:else}
        <div class="card ok" role="status">
          <div class="head">
            <span class="ic ok"><Icon name="check" size={20} /></span>
            <strong>No conflicts expected</strong>
            <span class="form-note">{rebase ? 'each commit was tried on top of the one before it' : 'checked without touching your files'}</span>
          </div>
        </div>
      {/if}

      {#if merge}
        <section>
          <h3 class="meta">Coming in · {plural(merge.commitCount, 'commit', 'commits')} · {plural(merge.files, 'file', 'files')}</h3>
          <ul class="rows scroll">
            {#each merge.commits as c (c.id)}
              <li><span class="mono sha">{short(c.id)}</span><span class="subject">{c.subject}</span><span class="form-note who">{c.author}</span></li>
            {/each}
            {#if merge.commitCount > merge.commits.length}<li class="form-note">and {(merge.commitCount - merge.commits.length).toLocaleString()} more</li>{/if}
          </ul>
        </section>
        <p class="form-note">{mergeSummary(merge, chosen, current, label)}</p>
      {:else if rebase}
        <section>
          <h3 class="meta">{rebase.upToDate ? `${plural(rebase.commitCount, 'commit', 'commits')} already on top of ${label}` : `${plural(rebase.commitCount, 'commit', 'commits')} will be written again, oldest first`}</h3>
          <ol class="rows scroll">
            {#each rebase.commits as c, i (c.id)}
              <li class:stops={c.id === rebase.stopsAt}>
                <span class="meta n">{i + 1}</span><span class="mono sha">{short(c.id)}</span><span class="subject">{c.subject}</span>
                {#if c.fixupOf}<span class="tag plum">fixup</span>{/if}
                {#if c.id === rebase.stopsAt}<span class="tag stop">stops here</span>{/if}
              </li>
            {/each}
            {#if rebase.commitCount > rebase.commits.length}<li class="form-note">and {(rebase.commitCount - rebase.commits.length).toLocaleString()} more</li>{/if}
          </ol>
        </section>

        <div class="card warm">
          {#if fixups > 0}
            <label class="form-check"><input type="checkbox" bind:checked={fold} disabled={edit} /> Fold the {plural(fixups, 'fixup', 'fixups')} into the {fixups === 1 ? 'commit it names' : 'commits they name'}</label>
          {/if}
          <label class="form-check"><input type="checkbox" bind:checked={edit} /> Let me reorder, reword or drop commits first</label>
        </div>

        {#if rebase.pushed}
          <div class="card danger">
            <div class="head">
              <span class="ic danger"><Icon name="warning" size={20} /></span>
              <strong>This branch is already pushed</strong>
            </div>
            <p>
              Its commits get new ids, so <span class="mono">{rebase.upstream}</span> will need a force push afterwards. Anyone who has it will need to reset to yours.
            </p>
          </div>
        {/if}
      {/if}
    {/if}

    <div class="form-actions">
      <button type="button" class="btn quiet" onclick={onClose}>{nothing ? 'Close' : 'Cancel'}</button>
      {#if !nothing}
        <button type="submit" class="btn lg primary" disabled={!loaded}>
          {#if !loaded || failed}{kind === 'merge' ? 'Merge' : 'Rebase'}{:else if merge}{mergeVerb(merge, chosen, current)}{:else if rebase}{rebaseVerb(rebase, edit)}{/if}
          <span class="arrow" aria-hidden="true">→</span>
        </button>
      {/if}
    </div>
  </form>
</Modal>

<style>
  .preview {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .how {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .off {
    margin-top: -8px;
  }

  h3 {
    margin: 0 0 6px;
    font-weight: 400;
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .card {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px 14px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
  }

  .card p {
    margin: 0;
  }

  .card.warn {
    border-color: var(--warning);
    background: var(--warning-wash);
  }

  .card.ok {
    border-color: var(--success);
    background: var(--success-wash);
  }

  .card.danger {
    border-color: var(--danger);
    background: var(--danger-wash);
    color: var(--ink-soft);
  }

  .card.warm {
    border: none;
    background: var(--paper-warm);
  }

  .head {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px 10px;
  }

  .head strong {
    font-weight: 600;
    color: var(--ink);
  }

  .ic {
    display: inline-flex;
  }

  .ic.warn {
    color: var(--warning);
  }

  .ic.ok {
    color: var(--success);
  }

  .ic.danger {
    color: var(--danger);
  }

  .at {
    display: flex;
    gap: 10px;
    align-items: baseline;
  }

  .rows {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .rows.scroll {
    max-height: 200px;
    overflow-y: auto;
  }

  .rows li {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 32px;
    border-top: 1px dashed var(--hairline);
  }

  .rows li:first-child {
    border-top: none;
  }

  .sha {
    flex: none;
    color: var(--ink-mute);
  }

  .n {
    flex: none;
    width: 22px;
  }

  .subject,
  .path {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .who {
    flex: none;
    max-width: 30%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .stops .subject {
    color: var(--ink);
    font-weight: 500;
  }

  .tag.stop {
    background: var(--warning-wash);
    color: var(--warning);
  }

  .arrow {
    font-family: var(--font-hand);
    font-size: 22px;
    line-height: 1;
  }
</style>
