<script lang="ts" module>
  /** Unsent commit messages per repository, kept while switching tabs. A plain cache that is
   * never rendered, so it needn't be reactive. */
  // eslint-disable-next-line svelte/prefer-svelte-reactivity
  const drafts = new Map<string, { summary: string; body: string }>()
</script>

<script lang="ts">
  import {
    CommitChanges,
    DiscardFiles,
    LastCommitMessage,
    StashSave,
    StageFiles,
    UnstageFiles,
    TakeConflictSide,
    WorkingTree,
  } from '../../wailsjs/go/app/App.js'
  import type { app, git } from '../../wailsjs/go/models'
  import type { DiffTarget } from './diff/DiffView.svelte'
  import { gitConfig } from './gitconfig/config.svelte'
  import { session } from './session.svelte'
  import Icon, { type IconName } from './ui/Icon.svelte'
  import InkButton from './ui/InkButton.svelte'

  let {
    path,
    openFile,
    current,
    onCommitted,
  }: {
    path: string
    openFile: (t: DiffTarget | null) => void
    /** The diff currently open, to highlight its file. */
    current?: DiffTarget | null
    onCommitted?: (id: string) => void
  } = $props()

  let wt = $state<app.WorkingTree | null>(null)
  let confirmDiscard = $state<string[] | null>(null)
  let busy = $state(false)
  let stashing = $state(false)
  let stashMessage = $state('')
  let stashUntracked = $state(true)

  async function saveStash() {
    busy = true
    const ok = (await session.run(async () => {
      await StashSave(path, stashMessage, stashUntracked)
      return true
    })) ?? false
    busy = false
    if (ok) {
      stashing = false
      stashMessage = ''
      if (current?.mode !== 'commit') openFile(null)
    }
    await refresh()
  }

  // Read once: the repo view is keyed by path, so this panel never switches repositories.
  // svelte-ignore state_referenced_locally
  const draft = drafts.get(path) ?? { summary: '', body: '' }
  let summary = $state(draft.summary)
  let body = $state(draft.body)
  let amend = $state(false)
  /** The draft set aside while amending, restored if amend is unticked. */
  let stash: { summary: string; body: string } | null = null

  $effect(() => {
    if (!amend) drafts.set(path, { summary, body })
  })

  let files = $derived(wt?.status.files ?? [])
  let conflicted = $derived(files.filter((f) => f.conflicted))
  let unstaged = $derived(files.filter((f) => !f.conflicted && (f.untracked || f.workTree !== '.')))
  let staged = $derived(files.filter((f) => !f.untracked && !f.conflicted && f.index !== '.'))
  /** Commit was asked for before status had loaded: it runs once status says what's staged. */
  let commitWaiting = $state(false)
  let canCommit = $derived(summary.trim() !== '' && (staged.length > 0 || amend) && !busy)
  let commitLabel = $derived(amend ? 'Amend commit' : commitWaiting ? 'Reading status…' : wt ? `Commit ${staged.length} ${staged.length === 1 ? 'file' : 'files'}` : 'Commit')

  async function refresh() {
    const res = await session.run(() => WorkingTree(path))
    const waiting = commitWaiting
    commitWaiting = false
    if (!res) return
    wt = res
    if (waiting) await commit()
  }

  // Refresh on open and whenever the backend reports a change on disk.
  $effect(() => {
    void session.revision[path]
    void refresh()
  })

  function show(f: git.FileStatus, inStaged: boolean) {
    openFile({ mode: inStaged ? 'staged' : 'unstaged', file: f.path, oldPath: f.origPath })
  }

  const isOpen = (f: git.FileStatus, inStaged: boolean) =>
    current?.file === f.path && current.mode === (inStaged ? 'staged' : 'unstaged')

  async function act(fn: () => Promise<void>) {
    busy = true
    await session.run(fn)
    busy = false
    await refresh()
  }

  // Rows that just moved between the lists land with a rose wash that fades, so the eye
  // follows them (the row-land keyframes in style.css).
  let landed = $state<{ staged: boolean; paths: Set<string> } | null>(null)
  let landedTimer: ReturnType<typeof setTimeout> | undefined

  function land(staged: boolean, paths: string[]) {
    clearTimeout(landedTimer)
    landed = { staged, paths: new Set(paths) }
    landedTimer = setTimeout(() => (landed = null), 1000)
  }

  async function stage(paths: string[]) {
    const moving = paths.length ? paths : unstaged.filter((f) => !f.conflicted).map((f) => f.path)
    await act(() => StageFiles(path, paths))
    land(true, moving)
  }

  async function unstage(paths: string[]) {
    const moving = paths.length ? paths : staged.map((f) => f.path)
    await act(() => UnstageFiles(path, paths))
    land(false, moving)
  }

  async function discard(paths: string[]) {
    confirmDiscard = null
    if (current && paths.includes(current.file)) openFile(null)
    await act(() => DiscardFiles(path, paths))
  }

  async function toggleAmend() {
    if (amend) {
      stash = { summary, body }
      const msg = (await session.run(() => LastCommitMessage(path))) ?? ''
      const nl = msg.indexOf('\n')
      summary = nl < 0 ? msg : msg.slice(0, nl)
      body = nl < 0 ? '' : msg.slice(nl + 1).trim()
    } else if (stash) {
      ;({ summary, body } = stash)
      stash = null
    }
  }

  async function commit() {
    if (!wt && !amend && summary.trim()) {
      commitWaiting = true
      return
    }
    if (!canCommit) return
    busy = true
    const message = body.trim() ? `${summary.trim()}\n\n${body.trim()}` : summary.trim()
    const id = await session.run(() => CommitChanges(path, message, amend))
    busy = false
    if (id) {
      summary = ''
      body = ''
      amend = false
      stash = null
      drafts.delete(path)
      if (current?.mode !== 'commit') openFile(null)
      onCommitted?.(id)
    }
    await refresh()
  }

  function onKey(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault()
      void commit()
    }
  }

  interface StatusGlyph {
    icon: IconName
    label: string
    /** Picks the colour; the icon's shape carries the meaning on its own. */
    kind: 'added' | 'modified' | 'deleted' | 'renamed' | 'untracked' | 'conflict'
  }

  const STATUS: Record<string, StatusGlyph> = {
    A: { icon: 'st-added', label: 'Added', kind: 'added' },
    M: { icon: 'st-modified', label: 'Modified', kind: 'modified' },
    T: { icon: 'st-modified', label: 'Type changed', kind: 'modified' },
    D: { icon: 'st-deleted', label: 'Deleted', kind: 'deleted' },
    R: { icon: 'st-renamed', label: 'Renamed', kind: 'renamed' },
    C: { icon: 'st-renamed', label: 'Copied', kind: 'renamed' },
    '?': { icon: 'st-untracked', label: 'Untracked', kind: 'untracked' },
    '!': { icon: 'st-conflict', label: 'Conflicted', kind: 'conflict' },
  }

  function status(f: git.FileStatus, inStaged: boolean): StatusGlyph {
    const code = f.untracked ? '?' : f.conflicted ? '!' : inStaged ? f.index : f.workTree
    return STATUS[code] ?? STATUS.M
  }
</script>

{#snippet fileRow(f: git.FileStatus, inStaged: boolean)}
  {@const st = status(f, inStaged)}
  <li class:active={isOpen(f, inStaged)} class:landed={landed?.staged === inStaged && landed.paths.has(f.path)}>
    <button class="name" onclick={() => show(f, inStaged)} title={f.origPath ? `${f.origPath} → ${f.path}` : f.path}>
      <span class="code s-{st.kind}" title={st.label}><Icon name={st.icon} size={14} /><span class="visually-hidden">{st.label}</span></span>
      <span class="path">{f.origPath ? `${f.origPath} → ${f.path}` : f.path}</span>
    </button>
    {#if confirmDiscard?.length === 1 && confirmDiscard[0] === f.path}
      <span class="confirm">
        Discard?
        <button class="btn sm danger" onclick={() => discard([f.path])}>Yes</button>
        <button class="btn sm secondary" onclick={() => (confirmDiscard = null)}>No</button>
      </span>
    {:else if inStaged}
      <button class="btn sm quiet icon row-act" disabled={busy} onclick={() => unstage([f.path])} title="Unstage" aria-label="Unstage {f.path}"><Icon name="minus" size={14} /></button>
    {:else}
      {#if !f.conflicted}
        <button class="btn sm quiet icon row-act" disabled={busy} onclick={() => (confirmDiscard = [f.path])} title="Discard changes" aria-label="Discard changes to {f.path}"
          ><Icon name="undo" size={14} /></button
        >
      {/if}
      <button class="btn sm quiet icon row-act" disabled={busy} onclick={() => stage([f.path])} title="Stage" aria-label="Stage {f.path}"><Icon name="plus" size={14} /></button>
    {/if}
  </li>
{/snippet}

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<aside class="panel" onkeydown={onKey} role="region" aria-label="Uncommitted changes">
  <div class="top">
    <div class="title">
      <span class="hand">still on your desk</span>
      <h2>Uncommitted changes</h2>
    </div>
    {#if !stashing}
      <button class="btn quiet" disabled={busy || files.length === 0} onclick={() => (stashing = true)} title="Set these changes aside in a stash"
        ><Icon name="stash" />Stash…</button
      >
    {/if}
  </div>
  {#if stashing}
    <form
      class="stashbar"
      onsubmit={(e) => {
        e.preventDefault()
        void saveStash()
      }}
    >
      <label class="form-field"><span>Stash message</span><input bind:value={stashMessage} placeholder="Optional" /></label>
      <div class="stash-row">
        <label class="form-check"><input type="checkbox" bind:checked={stashUntracked} /> Include untracked</label>
        <span class="spacer"></span>
        <button type="button" class="btn quiet" onclick={() => (stashing = false)}>Cancel</button>
        <button type="submit" class="btn secondary" disabled={busy}>Stash</button>
      </div>
    </form>
  {/if}
  <div class="lists">
    {#if conflicted.length}
      <section class="conflicts">
        <header><span class="meta">Conflicts · {conflicted.length}</span></header>
        <ul>
          {#each conflicted as f (f.path)}
            <li class:active={current?.mode === 'conflict' && current.file === f.path}>
              <button class="name" onclick={() => openFile({ mode: 'conflict', file: f.path })} title="Open the conflict tool">
                <span class="code c-!">!</span>
                <span class="path">{f.path}</span>
              </button>
              <button class="btn sm quiet" disabled={busy} title="Resolve with ours" onclick={() => act(() => TakeConflictSide(path, f.path, 'ours'))}
                >Ours</button
              >
              <button class="btn sm quiet" disabled={busy} title="Resolve with theirs" onclick={() => act(() => TakeConflictSide(path, f.path, 'theirs'))}
                >Theirs</button
              >
            </li>
          {/each}
        </ul>
      </section>
    {/if}
    <section>
      <header>
        <span class="meta">Unstaged{wt ? ` · ${unstaged.length}` : ''}</span>
        {#if unstaged.length}
          {#if confirmDiscard && confirmDiscard.length > 1}
            <span class="confirm">
              Discard all unstaged changes?
              <button class="btn sm danger" onclick={() => discard(confirmDiscard ?? [])}>Yes</button>
              <button class="btn sm secondary" onclick={() => (confirmDiscard = null)}>No</button>
            </span>
          {:else}
            <button
              class="btn sm text"
              disabled={busy}
              onclick={() => (confirmDiscard = unstaged.filter((f) => !f.conflicted).map((f) => f.path))}
              title="Discard all unstaged changes">Discard all</button
            >
            <button class="btn sm text" disabled={busy} onclick={() => stage([])}>Stage all</button>
          {/if}
        {/if}
      </header>
      <ul>
        {#each unstaged as f (f.path)}{@render fileRow(f, false)}{/each}
      </ul>
    </section>
    <section>
      <header>
        <span class="meta">Staged{wt ? ` · ${staged.length}` : ''}</span>
        {#if staged.length}<button class="btn sm text" disabled={busy} onclick={() => unstage([])}>Unstage all</button>{/if}
      </header>
      <ul>
        {#each staged as f (f.path)}{@render fileRow(f, true)}{/each}
      </ul>
    </section>
  </div>

  <form
    class="composer"
    onsubmit={(e) => {
      e.preventDefault()
      void commit()
    }}
  >
    <span class="meta label">Commit message</span>
    <div class="summary">
      <input bind:value={summary} placeholder="Summary (required)" aria-label="Commit summary" />
      <span class="count" class:long={summary.length > 72} title="Keep summaries under about 72 characters"
        >{summary.length}</span
      >
    </div>
    <textarea bind:value={body} placeholder="Description" rows="4" aria-label="Commit description"></textarea>
    <div class="row">
      <label class="form-check"><input type="checkbox" bind:checked={amend} onchange={toggleAmend} /> Amend last commit</label>
      {#if wt?.signing.enabled}
        <button type="button" class="signed meta" title="commit.gpgsign is set; key: {wt.signing.key || 'default'}. Change in Git Config." onclick={() => gitConfig.showKey(path, 'commit.gpgSign')}
          >Signed · {wt.signing.format}</button
        >
      {/if}
    </div>
    {#if wt}
      <p class="identity">
        {#if wt.identity.name || wt.identity.email}
          as <span class="who">{wt.identity.name || 'no name'}{#if wt.identity.email}&nbsp;&lt;{wt.identity.email}&gt;{/if}</span>
        {:else}
          <span class="missing">Git doesn't know who you are yet.</span>
        {/if}
        <button type="button" class="btn sm text" onclick={() => gitConfig.showKey(path, wt?.identity.email || !wt?.identity.name ? 'user.email' : 'user.name')}
          >{wt.identity.name || wt.identity.email ? 'change' : 'set it in Git Config'}</button
        >
      </p>
    {/if}
    <div class="commit-row">
      <span class="hint meta">Ctrl+Enter</span>
      <InkButton type="submit" size="lg" arrow disabled={!canCommit}>
        {commitLabel}
      </InkButton>
    </div>
  </form>
</aside>

<style>
  .panel {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    border-left: 1px solid var(--paper-edge);
    background: var(--paper);
  }

  /* With room for it, the commit form sits beside the files. */
  @container right-panel (min-width: 620px) {
    .panel {
      display: grid;
      grid-template-columns: minmax(0, 1fr) 300px;
      grid-template-rows: auto auto minmax(0, 1fr);
    }

    .composer {
      grid-column: 2;
      grid-row: 1 / -1;
      border-left: 1px solid var(--paper-edge);
    }

    .composer textarea {
      flex: 1;
    }

    .lists {
      grid-column: 1;
    }
  }

  .top {
    display: flex;
    align-items: flex-end;
    gap: 8px;
    padding: 16px 16px 6px 20px;
  }

  .title {
    flex: 1;
    min-width: 0;
  }

  h2 {
    margin: 2px 0 0;
    font-size: 21px;
    font-weight: 500;
    letter-spacing: -0.015em;
    font-variation-settings:
      'SOFT' 50,
      'WONK' 1;
  }

  .stashbar {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin: 4px 16px 4px 20px;
    padding: 12px;
    border-radius: 6px;
    background: var(--paper-warm);
  }

  .stash-row {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .spacer {
    flex: 1;
  }

  .lists {
    flex: 1;
    overflow-y: auto;
  }

  header {
    display: flex;
    align-items: center;
    gap: 4px;
    margin: 12px 16px 2px 20px;
    padding-bottom: 6px;
    border-bottom: 1px solid var(--paper-edge);
  }

  header > span:first-child {
    flex: 1;
    color: var(--ink-soft);
  }

  .conflicts header > span:first-child {
    color: var(--danger);
  }

  ul {
    list-style: none;
    margin: 0 8px;
    padding: 0;
  }

  li {
    display: flex;
    align-items: center;
    padding-right: 6px;
    border-radius: 4px;
    transition: background-color var(--dur-quick);
  }

  li:hover {
    background: var(--wash-hover);
  }

  li.active {
    background: var(--selected);
  }

  button {
    background: none;
    border: none;
  }

  button:hover:not(:disabled) {
    background: none;
  }

  .name {
    flex: 1;
    display: flex;
    align-items: baseline;
    gap: 10px;
    min-width: 0;
    padding: 6px 12px;
    border-radius: 0;
    text-align: left;
  }

  .path {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: 12.5px;
  }

  /* Row actions appear on hover or focus, so the list reads calmly. */
  .landed {
    animation: row-land calc(var(--dur-enter) * 2) var(--ease-ink);
  }

  /* Row actions (stage, unstage, discard) show on hover, focus or selection. */
  .row-act {
    opacity: 0;
    transition:
      opacity var(--dur-quick),
      background-color var(--dur-quick),
      transform var(--dur-base) var(--ease-ink);
  }

  li:hover .row-act,
  li.active .row-act,
  .row-act:focus-visible {
    opacity: 1;
  }

  .code {
    display: inline-flex;
    flex: none;
  }

  .s-added {
    color: var(--success);
  }

  .s-modified {
    color: var(--warning);
  }

  .s-deleted,
  .s-conflict {
    color: var(--danger);
  }

  .s-renamed {
    color: var(--accent-a);
  }

  .s-untracked {
    color: var(--ink-mute);
  }

  .confirm {
    display: flex;
    gap: 2px;
    align-items: center;
    font-size: 12.5px;
    font-style: italic;
    color: var(--danger);
  }

  .composer {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 14px 16px 16px 20px;
    border-top: 1px solid var(--paper-edge);
    background: var(--paper-warm);
  }

  .label {
    color: var(--ink-soft);
  }

  .summary {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  input:not([type='checkbox']),
  textarea {
    flex: 1;
    padding: 7px 10px;
    border-radius: 5px;
    font-size: 14px;
    resize: vertical;
  }

  input:not([type='checkbox']):focus,
  textarea:focus {
    border-color: var(--link);
    outline: none;
    box-shadow: 0 0 0 3px var(--selection);
  }

  .count {
    min-width: 22px;
    text-align: right;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .count.long {
    color: var(--warning);
  }

  .row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .signed {
    padding: 0;
    background: none;
    border: none;
    color: var(--success);
  }

  .signed:hover:not(:disabled) {
    background: none;
    text-decoration: underline;
  }

  .identity {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 2px 6px;
    margin: 0;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .who {
    min-width: 0;
    overflow-wrap: anywhere;
    color: var(--ink-soft);
  }

  .missing {
    color: var(--warning);
  }

  .commit-row {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 12px;
    margin-top: 2px;
  }

  .hint {
    color: var(--ink-faint);
  }

  @media (prefers-reduced-motion: reduce) {
    li,
    .row-act {
      transition: none;
    }
  }
</style>
