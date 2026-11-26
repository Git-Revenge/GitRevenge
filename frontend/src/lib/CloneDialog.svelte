<script lang="ts">
  import { onMount } from 'svelte'
  import { CancelClone, ChooseFolder, Clone, CloneBranches, CloneName, HomeDir } from '../../wailsjs/go/app/App.js'
  import { cloneOptions, cloneSummary, defaultPicks, picksProblem, type ClonePicks } from './clone'
  import { onBackend } from './events'
  import { errorText } from './errors'
  import { dayOfSince, lastYear } from './filters/history'
  import { prefs } from './prefs.svelte'
  import { session } from './session.svelte'
  import Busy from './ui/Busy.svelte'
  import Icon from './ui/Icon.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  let { onClose }: { onClose: () => void } = $props()

  const uid = $props.id()
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

  // Options (#1002): every default is what plain `git clone URL` does. "Since" starts a year
  // back, for when it's chosen.
  let picks = $state<ClonePicks>(defaultPicks(dayOfSince(lastYear())))
  let summary = $derived(cloneSummary(picks))
  let problem = $derived(picksProblem(picks))

  // The server's branches, for "Only": asked once one branch is wanted, for the URL as it
  // is, as the clone would ask, so a password prompt then comes now rather than later.
  let heads = $state<{ url: string; branches: string[]; error: string; listing: boolean }>({ url: '', branches: [], error: '', listing: false })
  let headsTimer: ReturnType<typeof setTimeout> | undefined
  $effect(() => {
    const u = url.trim()
    if (picks.branches !== 'one' || !u || u === heads.url) return
    clearTimeout(headsTimer)
    headsTimer = setTimeout(() => void listBranches(u), 400)
  })

  async function listBranches(u: string) {
    heads = { url: u, branches: [], error: '', listing: true }
    try {
      const got = await CloneBranches(u, parent, name)
      if (heads.url !== u) return
      heads = { url: u, branches: got.branches ?? [], error: '', listing: false }
      if (!picks.branch.trim() && got.default) picks.branch = got.default
    } catch (err) {
      if (heads.url !== u) return
      heads = { url: u, branches: [], error: errorText(err), listing: false }
    }
  }

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
    if (problem) return
    running = true
    progress = ''
    error = ''
    try {
      const dir = await Clone(url, parent, name, cloneOptions(picks))
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
    if (running) void CancelClone()
    else onClose()
  }

  onMount(() => {
    if (!parent)
      HomeDir()
        .then((home) => {
          if (!parent && home) parent = home
        })
        // Without an answer, the folder just starts empty.
        .catch(() => {})
  })

  onMount(() =>
    onBackend('net:progress', (p) => {
      if (p.op !== 'clone') return
      progress = p.phase ? `${p.phase} ${p.percent}%` : p.line
    }),
  )
</script>

<Modal title="Clone a repository" eyebrow="bring it aboard" width={560} onClose={cancel} scrimCloses={!running}>
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

    <details class="options">
      <summary>Options <span class="summary">· {summary}</span></summary>
      <div class="opts">
        <fieldset disabled={running}>
          <legend>History</legend>
          <div class="choices">
            <label><input type="radio" name="{uid}-history" value="all" bind:group={picks.history} /> Every commit</label>
            <span class="line">
              <label><input type="radio" name="{uid}-history" value="depth" bind:group={picks.history} /> The last</label>
              <input class="num" aria-label="How many commits" inputmode="numeric" bind:value={picks.depth} onfocus={() => (picks.history = 'depth')} />
              <span>{picks.branches === 'all' ? 'commits of each branch' : 'commits'}</span>
            </span>
            <span class="line">
              <label><input type="radio" name="{uid}-history" value="since" bind:group={picks.history} /> Since</label>
              <input type="date" aria-label="Since which day" bind:value={picks.since} onfocus={() => (picks.history = 'since')} />
            </span>
            {#if picks.history !== 'all'}
              <p class="hint">A shallow clone: the history stops at the oldest commit fetched, and more can be fetched later.</p>
            {/if}
          </div>
        </fieldset>

        <fieldset disabled={running}>
          <legend>File contents</legend>
          <div class="choices">
            <label><input type="radio" name="{uid}-contents" value="all" bind:group={picks.contents} /> Download every version now</label>
            <label><input type="radio" name="{uid}-contents" value="partial" bind:group={picks.contents} /> Only when something needs them</label>
            {#if picks.contents === 'partial'}
              <p class="hint">A partial clone: every commit and folder now, and a file's contents the first time it's opened.</p>
            {/if}
          </div>
        </fieldset>

        <fieldset disabled={running}>
          <legend>Branches</legend>
          <div class="choices">
            <label><input type="radio" name="{uid}-branches" value="all" bind:group={picks.branches} /> Every branch and tag</label>
            <span class="line">
              <label><input type="radio" name="{uid}-branches" value="one" bind:group={picks.branches} /> Only</label>
              <input
                class="branch"
                list="{uid}-heads"
                aria-label="Which branch"
                placeholder={heads.listing ? 'Listing the branches…' : 'branch'}
                spellcheck="false"
                bind:value={picks.branch}
                onfocus={() => (picks.branches = 'one')}
              />
              <datalist id="{uid}-heads">
                {#each heads.branches as b (b)}<option value={b}></option>{/each}
              </datalist>
            </span>
            {#if picks.branches === 'one' && heads.error}
              <p class="hint">The branches couldn't be listed: {heads.error} Type the name instead.</p>
            {/if}
          </div>
        </fieldset>

        <fieldset disabled={running}>
          <legend>Submodules</legend>
          <div class="choices">
            <label><input type="checkbox" bind:checked={picks.submodules} /> Clone them too</label>
          </div>
        </fieldset>

        {#if problem}<p class="problem" role="status">{problem}</p>{/if}
      </div>
    </details>

    {#if running}
      <p class="status" role="status"><Busy /> {progress || 'Connecting…'}</p>
    {/if}
    {#if error}<p class="form-error" role="alert">{error}</p>{/if}
    <div class="form-actions">
      <InkButton variant="secondary" onclick={cancel}>{running ? 'Stop' : 'Cancel'}</InkButton>
      <InkButton type="submit" disabled={running || !!problem || !url.trim() || !parent.trim() || !name.trim()}>Clone</InkButton>
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

  .options {
    border-top: 1px dashed var(--hairline);
    padding-top: 10px;
  }

  .options summary {
    cursor: pointer;
    font-size: 14px;
    color: var(--ink-soft);
  }

  /* Not the uppercase meta style: a branch's name is said as it's written. */
  .summary {
    margin-left: 4px;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .opts {
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-top: 12px;
  }

  fieldset {
    display: grid;
    grid-template-columns: 110px minmax(0, 1fr);
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
    gap: 5px;
    font-size: 14px;
    min-width: 0;
  }

  .choices label,
  .line {
    display: flex;
    align-items: center;
    gap: 7px;
  }

  .choices input[type='radio'],
  .choices input[type='checkbox'] {
    accent-color: var(--accent);
    margin: 0;
  }

  .line input:not([type='radio']) {
    font-family: var(--font-mono);
    font-size: 12px;
    padding: 3px 6px;
    border: 1px solid var(--paper-edge);
    border-radius: 4px;
    background: var(--paper);
    color: var(--ink);
  }

  .num {
    width: 64px;
  }

  .branch {
    flex: 1;
    min-width: 0;
  }

  .hint {
    margin: 2px 0 0;
    font-size: 12.5px;
    line-height: 1.45;
    color: var(--ink-mute);
  }

  .problem {
    margin: 0;
    font-size: 13px;
    color: var(--warning);
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
