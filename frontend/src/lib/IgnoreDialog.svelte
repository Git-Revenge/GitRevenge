<script lang="ts" module>
  /** What the dialog was opened on: a changed file, or a folder heading. */
  export interface IgnoreSubject {
    kind: 'file' | 'folder'
    path: string
  }
</script>

<script lang="ts">
  import { AddIgnore, IgnoreFileNames, PreviewIgnore } from '../../wailsjs/go/app/App.js'
  import type { app, git } from '../../wailsjs/go/models'
  import { fileFolder, fileName } from './fileList'
  import { fileChoices, folderChoices, patternIn, plural, targetName, type IgnoreScope, type IgnoreTarget } from './ignore'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  let {
    path,
    subject,
    initial,
    onClose,
    onDone,
  }: {
    path: string
    subject: IgnoreSubject
    /** A rule to start from, such as a quick pick that needs a decision about tracked files. */
    initial?: { pattern: string; scope: IgnoreScope }
    onClose: () => void
    /** The rule was written. */
    onDone: (added: app.IgnoreAdded, target: IgnoreTarget) => void
  } = $props()
  const id = $props.id()

  // The dialog is opened afresh for each subject, so these are read once.
  // svelte-ignore state_referenced_locally
  const choices = subject.kind === 'file' ? fileChoices(subject.path) : folderChoices(subject.path)
  /** A folder's own .gitignore: the file's folder, or the folder itself. */
  // svelte-ignore state_referenced_locally
  const home = subject.kind === 'file' ? fileFolder(subject.path) : subject.path

  // svelte-ignore state_referenced_locally
  const start = initial ? choices.findIndex((c) => c.pattern === initial.pattern) : 0
  let picked = $state<number | 'custom'>(start < 0 ? 'custom' : start)
  // svelte-ignore state_referenced_locally
  let scope = $state<IgnoreScope>(initial?.scope ?? 'root')
  let target = $derived<IgnoreTarget>(scope === 'dir' ? { scope, dir: home } : { scope })
  // svelte-ignore state_referenced_locally
  let custom = $state(start < 0 ? (initial?.pattern ?? '') : '')
  let pattern = $derived(picked === 'custom' ? custom.trim() : (patternIn(choices[picked].pattern, target) ?? ''))
  let untrack = $state(true)
  let globalFile = $state('')
  // svelte-ignore state_referenced_locally
  void IgnoreFileNames(path)
    .then((n) => (globalFile = n.global))
    .catch(() => {})

  /** A folder's .gitignore can't hold a rule for anything above it. */
  const fits = (i: number) => patternIn(choices[i].pattern, target) !== null

  $effect(() => {
    if (picked !== 'custom' && !fits(picked)) picked = choices.findIndex((_, i) => fits(i))
  })

  function pickCustom() {
    if (picked !== 'custom' && !custom) custom = pattern
    picked = 'custom'
  }

  let preview = $state<git.IgnorePreview | null>(null)
  let previewing = $state(false)
  let asked = 0
  $effect(() => {
    const p = pattern
    const t = target
    const n = ++asked
    previewing = true
    const timer = setTimeout(async () => {
      const res = p ? await PreviewIgnore(path, t, p).catch(() => null) : null
      if (n !== asked) return
      preview = res
      previewing = false
    }, 150)
    return () => clearTimeout(timer)
  })

  let hidden = $derived(preview?.hidden ?? [])
  let tracked = $derived(preview?.tracked ?? [])
  let error = $state('')
  let busy = $state(false)
  const SHOWN = 5

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    if (!pattern) return
    busy = true
    error = ''
    try {
      const added = await AddIgnore(path, target, pattern, untrack ? tracked : [])
      onDone(added, target)
      onClose()
    } catch (err) {
      error = String(err)
    }
    busy = false
  }

  let submitLabel = $derived(
    tracked.length && untrack ? `Ignore and stop tracking ${plural(tracked.length, 'file')}` : hidden.length ? `Ignore ${plural(hidden.length, 'file')}` : 'Add the rule',
  )
</script>

<Modal title="Ignore {fileName(subject.path)}{subject.kind === 'folder' ? '/' : ''}" eyebrow="leave it out" width={720} {onClose} dismissable={!busy}>
  <form onsubmit={submit}>
    <p class="lede">Git stops listing matching files as changes. Nothing is deleted.</p>
    <div class="cols">
      <fieldset>
        <legend>What to ignore</legend>
        {#each choices as c, i (c.pattern)}
          <label class="opt" class:on={picked === i} class:off={!fits(i)}>
            <input type="radio" name="{id}-what" checked={picked === i} disabled={!fits(i)} onchange={() => (picked = i)} />
            <span class="what">{c.label}</span>
            <code>{patternIn(c.pattern, target) ?? c.pattern}</code>
          </label>
        {/each}
        <label class="opt" class:on={picked === 'custom'}>
          <input type="radio" name="{id}-what" checked={picked === 'custom'} onchange={pickCustom} />
          <span class="what">A pattern of my own</span>
        </label>
        {#if picked === 'custom'}
          <!-- svelte-ignore a11y_autofocus -->
          <input class="custom" bind:value={custom} aria-label="Pattern" spellcheck="false" autofocus />
          <span class="hint">As written in {targetName(target)}. <code>*</code> matches within a folder, <code>**</code> across folders, and a trailing <code>/</code> means folders only.</span>
        {/if}
      </fieldset>
      <div class="side">
        <label class="form-field">
          <span>Write it to</span>
          <select bind:value={scope}>
            <option value="root">.gitignore at the top (shared with everyone)</option>
            {#if home}<option value="dir">{home}/.gitignore (shared, next to the files)</option>{/if}
            <option value="exclude">.git/info/exclude (this clone only)</option>
            <option value="global">Global excludes (every repository here)</option>
          </select>
        </label>
        <p class="hint">
          {#if scope === 'root' || scope === 'dir'}
            The rule shows up as a change to {targetName(target)}, ready to commit.
          {:else if scope === 'exclude'}
            Never committed, so nobody else sees it.
          {:else}
            Applies to every repository on this computer{globalFile ? `, from ${globalFile}` : ''}.
          {/if}
        </p>
        <div class="preview" aria-live="polite" class:stale={previewing}>
          {#if !pattern}
            <span class="meta">Type a pattern to see what it hides.</span>
          {:else if pattern.startsWith('!')}
            <span class="meta">A rule starting with ! brings files back rather than hiding them.</span>
          {:else if !preview}
            <span class="meta">Checking…</span>
          {:else}
            <strong>{hidden.length ? `This hides ${plural(hidden.length, 'file')}` : 'This hides nothing listed now'}</strong>
            {#each hidden.slice(0, SHOWN) as h (h)}<code class="file">{h}</code>{/each}
            {#if hidden.length > SHOWN}<span class="meta">and {plural(hidden.length - SHOWN, 'more file')}</span>{/if}
          {/if}
        </div>
        {#if tracked.length}
          <div class="tracked" role="note">
            <p>
              <strong>{plural(tracked.length, 'tracked file')} {tracked.length === 1 ? 'matches' : 'match'}.</strong>
              Git keeps tracking {tracked.length === 1 ? `${tracked[0]}` : 'them'} until you stop tracking {tracked.length === 1 ? 'it' : 'them'}.
            </p>
            <label class="form-check"><input type="checkbox" bind:checked={untrack} /> Stop tracking {tracked.length === 1 ? 'it' : 'them'} too (the {tracked.length === 1 ? 'file stays' : 'files stay'} on disk)</label>
          </div>
        {/if}
      </div>
    </div>
    {#if error}<p class="form-error" role="alert">{error}</p>{/if}
    <div class="foot">
      {#if pattern}<code class="adds">+ {pattern} → {targetName(target)}</code>{/if}
      <span class="spacer"></span>
      <InkButton variant="secondary" onclick={onClose} disabled={busy}>Cancel</InkButton>
      <InkButton type="submit" variant="primary" disabled={busy || !pattern}>{submitLabel}</InkButton>
    </div>
  </form>
</Modal>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .lede {
    margin: 0;
    font-size: 14px;
    color: var(--ink-mute);
  }

  .cols {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 24px;
  }

  fieldset {
    border: 0;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  legend,
  .form-field > span {
    font-size: 13px;
    font-weight: 600;
    color: var(--ink-soft);
    margin-bottom: 6px;
  }

  .opt {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 34px;
    padding: 0 10px;
    border-radius: 6px;
    font-size: 14px;
    cursor: pointer;
  }

  .opt.on {
    background: var(--selected);
  }

  .opt.off {
    opacity: 0.55;
    cursor: default;
  }

  .opt input {
    accent-color: var(--accent);
    margin: 0;
  }

  .what {
    flex: none;
  }

  code {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-b);
    overflow-wrap: anywhere;
  }

  .opt code {
    margin-left: auto;
    text-align: right;
    min-width: 0;
  }

  .custom {
    margin: 4px 0 0 34px;
    height: 32px;
    padding: 0 10px;
    font-family: var(--font-mono);
    font-size: 13px;
    border-radius: 6px;
  }

  .hint {
    margin: 4px 0 0;
    font-size: 12px;
    line-height: 1.45;
    color: var(--ink-mute);
  }

  fieldset .hint {
    margin-left: 34px;
  }

  .side {
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-width: 0;
  }

  .side .hint {
    margin-top: -6px;
  }

  .preview {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px;
    background: var(--paper-warm);
    border-radius: 8px;
    font-size: 13px;
    transition: opacity var(--dur-base) var(--ease-ink);
  }

  .preview.stale {
    opacity: 0.6;
  }

  .preview .file {
    color: var(--ink-soft);
  }

  .tracked {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px;
    background: var(--warning-wash);
    border-radius: 8px;
    font-size: 13px;
    line-height: 1.45;
  }

  .tracked p {
    margin: 0;
    overflow-wrap: anywhere;
  }

  .tracked strong {
    color: var(--warning);
  }

  .foot {
    display: flex;
    align-items: center;
    gap: 10px;
    padding-top: 14px;
    border-top: 1px solid var(--paper-edge);
  }

  .adds {
    color: var(--ink-mute);
    min-width: 0;
  }

  .spacer {
    flex: 1;
  }

  @media (max-width: 760px) {
    .cols {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
