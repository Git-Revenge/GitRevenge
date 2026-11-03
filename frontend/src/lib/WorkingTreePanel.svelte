<script lang="ts" module>
  /** Unsent commit messages per repository, kept while switching tabs. A plain cache that is
   * never rendered, so it needn't be reactive. */
  // eslint-disable-next-line svelte/prefer-svelte-reactivity
  const drafts = new Map<string, { summary: string; body: string }>()
  /** Repositories where the person set the case-clash notice aside, for this session. */
  // eslint-disable-next-line svelte/prefer-svelte-reactivity
  const caseNoticeSeen = new Set<string>()
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
  import { fileGroups, fileName } from './fileList'
  import { gitConfig } from './gitconfig/config.svelte'
  import { session } from './session.svelte'
  import Icon, { type IconName } from './ui/Icon.svelte'
  import InkButton from './ui/InkButton.svelte'

  let {
    path,
    openFile,
    current,
    onCommitted,
    onNewBranch,
    head,
  }: {
    path: string
    openFile: (t: DiffTarget | null) => void
    /** The diff currently open, to highlight its file. */
    current?: DiffTarget | null
    onCommitted?: (id: string) => void
    /** Offers to start a branch at HEAD. */
    onNewBranch?: () => void
    /** What's checked out, as the tab shows it: a branch, or "detached at …". Defaults to the
     * active tab's. */
    head?: string
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
  /** The description opens on request; a draft already has it open. */
  let showBody = $state(draft.body !== '')
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
  let canCommit = $derived(summary.trim() !== '' && (staged.length > 0 || amend) && conflicted.length === 0 && !busy)
  /** The checked-out branch, for saying where a commit will go. '' when there isn't one. */
  let headText = $derived(head ?? session.activeTab?.head ?? '')
  let detached = $derived(headText.startsWith('detached'))
  let branch = $derived(detached ? '' : headText)
  const plural = (n: number, what: string) => (n === 1 ? `1 ${what}` : `${n} ${what}s`)
  /** What to do next, in a word: each step of committing leads to the next. */
  let nextStep = $derived.by(() => {
    if (detached) return 'Make a branch first, so this work has somewhere to live.'
    if (conflicted.length) return `Fix ${conflicted.length === 1 ? 'this conflict' : `these ${conflicted.length} conflicts`}, and the commit can go ahead.`
    if (amend) return summary.trim() ? 'Ready to amend the last commit.' : 'Say what the amended commit does.'
    if (!staged.length) return 'Tick what belongs in this commit, then say what it does.'
    if (!summary.trim()) return `${plural(staged.length, 'file')} going in. Now say what the commit does.`
    return `${plural(staged.length, 'file')} going in, and ready to go.`
  })
  let commitLabel = $derived.by(() => {
    if (amend) return 'Amend commit'
    if (commitWaiting) return 'Reading status…'
    if (!wt) return 'Commit'
    if (conflicted.length) return `Resolve ${plural(conflicted.length, 'conflict')} to commit`
    if (!staged.length) return 'Tick a file to commit it'
    if (!summary.trim()) return `Describe it to commit ${plural(staged.length, 'file')}`
    if (detached) return 'Commit anyway, without a branch'
    return `Commit ${plural(staged.length, 'file')}${branch ? ` to ${branch}` : ''}`
  })

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

  /** How much of a file is going in the commit. */
  type Tick = 'none' | 'part' | 'all'
  const isStaged = (f: git.FileStatus) => !f.untracked && !f.conflicted && f.index !== '.'
  const isUnstaged = (f: git.FileStatus) => !f.conflicted && (f.untracked || f.workTree !== '.')
  const tickOf = (f: git.FileStatus): Tick => (!isStaged(f) ? 'none' : isUnstaged(f) ? 'part' : 'all')

  let listed = $derived(files.filter((f) => !f.conflicted))
  // svelte-ignore state_referenced_locally
  let caseNoticeHidden = $state(caseNoticeSeen.has(path))
  let clashes = $derived(listed.filter((f) => f.caseClash))
  function hideCaseNotice() {
    caseNoticeSeen.add(path)
    caseNoticeHidden = true
  }
  /** What the tick at the top covers. Case clashes aren't real edits, so they stay out unless
   * they're all there is. */
  let tickable = $derived(clashes.length < listed.length ? listed.filter((f) => !f.caseClash) : listed)
  let ticked = $derived(tickable.filter((f) => tickOf(f) !== 'none').length)
  let allTicked = $derived(tickable.length > 0 && tickable.every((f) => tickOf(f) === 'all'))

  /** Opens a file's changes: what's still unstaged, if any is. */
  function show(f: git.FileStatus) {
    openFile({ mode: isUnstaged(f) ? 'unstaged' : 'staged', file: f.path, oldPath: f.origPath })
  }

  const isOpen = (f: git.FileStatus) => current?.file === f.path && (current.mode === 'staged' || current.mode === 'unstaged')

  /** A tick shows what's staged, not what was clicked, so a failed stage doesn't leave it wrong. */
  function tickChanged(e: Event, f: git.FileStatus) {
    const box = e.currentTarget as HTMLInputElement
    const tick = tickOf(f)
    box.checked = tick === 'all'
    box.indeterminate = tick === 'part'
    if (tick === 'all') void unstage([f.path])
    else void stage([f.path])
  }

  function tickAllChanged(e: Event) {
    const box = e.currentTarget as HTMLInputElement
    box.checked = allTicked
    box.indeterminate = ticked > 0 && !allTicked
    const paths = tickable.map((f) => f.path)
    if (allTicked) void unstage(tickable.length === listed.length ? [] : paths)
    else void stage(tickable.length === listed.length ? [] : paths)
  }

  async function act(fn: () => Promise<void>) {
    busy = true
    await session.run(fn)
    busy = false
    await refresh()
  }

  // Rows that just moved between the lists land with a rose wash that fades, so the eye
  // follows them (the row-land keyframes in style.css).
  let landed = $state<Set<string> | null>(null)
  let landedTimer: ReturnType<typeof setTimeout> | undefined

  function land(paths: string[]) {
    clearTimeout(landedTimer)
    landed = new Set(paths)
    landedTimer = setTimeout(() => (landed = null), 1000)
  }

  async function stage(paths: string[]) {
    const moving = paths.length ? paths : unstaged.filter((f) => !f.conflicted).map((f) => f.path)
    await act(() => StageFiles(path, paths))
    land(moving)
  }

  async function unstage(paths: string[]) {
    const moving = paths.length ? paths : staged.map((f) => f.path)
    await act(() => UnstageFiles(path, paths))
    land(moving)
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

{#snippet fileRow(f: git.FileStatus)}
  {@const st = status(f, !isUnstaged(f))}
  {@const tick = tickOf(f)}
  <li class:active={isOpen(f)} class:going={tick !== 'none'} class:landed={landed?.has(f.path)}>
    <input type="checkbox" class="tick" aria-label="Include {f.path} in the commit" checked={tick === 'all'} indeterminate={tick === 'part'} disabled={busy} onchange={(e) => tickChanged(e, f)} />
    <button class="name" onclick={() => show(f)} title={f.origPath ? `${f.origPath} → ${f.path}` : f.path}>
      <span class="code s-{st.kind}" title={st.label}><Icon name={st.icon} size={14} /><span class="visually-hidden">{st.label}</span></span>
      <span class="path">{fileName(f.path)}</span>
      {#if f.origPath}<span class="from">from {f.origPath}</span>{/if}
      {#if tick === 'part'}<span class="partly">partly staged</span>{/if}
      {#if f.caseClash}<span class="partly" title="Another file has the same name in different letter case">case clash</span>{/if}
      {#if f.binary}
        <span class="binary">binary</span>
      {:else if f.added || f.deleted}
        <span class="added">+{f.added}</span>
        <span class="deleted">−{f.deleted}</span>
      {/if}
    </button>
    {#if confirmDiscard?.length === 1 && confirmDiscard[0] === f.path}
      <span class="confirm">
        Discard?
        <button class="btn sm danger" onclick={() => discard([f.path])}>Yes</button>
        <button class="btn sm secondary" onclick={() => (confirmDiscard = null)}>No</button>
      </span>
    {:else if isUnstaged(f)}
      <button class="btn sm quiet icon row-act" disabled={busy} onclick={() => (confirmDiscard = [f.path])} title="Discard changes" aria-label="Discard changes to {f.path}"><Icon name="undo" size={14} /></button>
    {/if}
  </li>
{/snippet}

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<aside class="panel" onkeydown={onKey} role="region" aria-label="Uncommitted changes">
  <div class="top">
    <div class="title">
      <span class="hand">{detached ? headText : branch ? `on ${branch}` : 'still on your desk'}</span>
      <h2>{wt && files.length ? `${plural(files.length, 'file')} changed` : 'Uncommitted changes'}</h2>
      {#if wt && files.length}<p class="next">{nextStep}</p>{/if}
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
  {#if detached && files.length}
    <div class="notice detached" role="note">
      <strong>You're not on a branch.</strong>
      You're {headText}. A commit made here is easy to lose once you switch away.
      {#if onNewBranch}<button class="btn sm" onclick={onNewBranch}>Create a branch here</button>{/if}
    </div>
  {/if}
  {#if clashes.length && !caseNoticeHidden}
    <div class="notice" role="note">
      <strong>{clashes.length === 1 ? 'This may not be your edit.' : `These ${clashes.length} may not be your edits.`}</strong>
      {clashes.length === 1 ? 'Its name differs' : 'Each name differs'} from another file's only by letter case, which this computer can't keep apart, so git sees a change you never made.
      <button class="btn sm secondary" onclick={hideCaseNotice}>{clashes.length === 1 ? 'Leave it out' : `Leave these ${clashes.length} out`}</button>
    </div>
  {/if}
  <div class="lists">
    {#if conflicted.length}
      <section class="conflicts">
        <header><span class="meta">Fix these first · {conflicted.length}</span></header>
        <ul>
          {#each conflicted as f (f.path)}
            <li class:active={current?.mode === 'conflict' && current.file === f.path}>
              <button class="name" onclick={() => openFile({ mode: 'conflict', file: f.path })} title="Open the conflict tool">
                <span class="code c-!">!</span>
                <span class="path">{f.path}</span>
              </button>
              <button class="btn sm quiet" disabled={busy} title="Resolve with ours" onclick={() => act(() => TakeConflictSide(path, f.path, 'ours'))}>Ours</button>
              <button class="btn sm quiet" disabled={busy} title="Resolve with theirs" onclick={() => act(() => TakeConflictSide(path, f.path, 'theirs'))}>Theirs</button>
            </li>
          {/each}
        </ul>
      </section>
    {/if}
    {#if wt && files.length === 0}
      <p class="clean">Nothing to commit: your working tree matches the last commit.</p>
    {:else}
      <section>
        <header>
          {#if wt}
            <label class="tick-all">
              <input type="checkbox" class="tick" checked={allTicked} indeterminate={ticked > 0 && !allTicked} disabled={busy || !listed.length} onchange={tickAllChanged} />
              <span class="meta">{allTicked ? `All ${tickable.length} ticked` : ticked ? `${ticked} of ${tickable.length} ticked` : `Tick all ${tickable.length}`}</span>
            </label>
          {:else}
            <span class="meta">Files</span>
          {/if}
          {#if unstaged.length}
            {#if confirmDiscard && confirmDiscard.length > 1}
              <span class="confirm">
                Discard all unstaged changes?
                <button class="btn sm danger" onclick={() => discard(confirmDiscard ?? [])}>Yes</button>
                <button class="btn sm secondary" onclick={() => (confirmDiscard = null)}>No</button>
              </span>
            {:else}
              <button class="btn sm text" disabled={busy} onclick={() => (confirmDiscard = unstaged.filter((f) => !f.conflicted).map((f) => f.path))} title="Discard all unstaged changes">Discard all</button>
            {/if}
          {/if}
        </header>
        {#each fileGroups(listed) as g (g.folder)}
          {#if g.folder}<div class="folder" title={g.folder}>{g.folder}/</div>{/if}
          <ul>
            {#each g.files as f (f.path)}{@render fileRow(f)}{/each}
          </ul>
        {/each}
      </section>
    {/if}
  </div>

  <form
    class="composer"
    onsubmit={(e) => {
      e.preventDefault()
      void commit()
    }}
  >
    <label class="what" for="commit-summary">What does this commit do?</label>
    <div class="summary">
      <input id="commit-summary" bind:value={summary} aria-label="Commit summary" />
      <span class="count" class:long={summary.length > 72} title="Keep summaries under about 72 characters"
        >{summary.length}</span
      >
    </div>
    <textarea class:folded={!showBody && !body} bind:value={body} placeholder="Why, and anything a reviewer should know" rows="4" aria-label="Commit description"></textarea>
    <div class="row">
      {#if !showBody && !body}
        <button type="button" class="btn sm quiet add-body" onclick={() => (showBody = true)}><Icon name="plus" size={13} />Description</button>
      {/if}
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

    .composer textarea.folded {
      display: block;
    }

    .add-body {
      display: none;
    }

    .lists {
      grid-column: 1;
    }
  }

  .notice {
    margin: 2px 16px 8px 20px;
    padding: 10px 12px;
    border-radius: 6px;
    background: color-mix(in srgb, var(--warning) 14%, transparent);
    font-size: 14px;
    line-height: 1.4;
  }

  .notice strong {
    font-weight: 600;
  }

  .notice.detached {
    background: color-mix(in srgb, var(--accent) 12%, transparent);
  }

  .notice :global(.btn) {
    display: block;
    margin-top: 8px;
  }

  .next {
    margin: 2px 0 0;
    font-size: 14px;
    line-height: 1.35;
    color: var(--ink-soft);
  }

  .clean {
    margin: 12px 16px 12px 20px;
    color: var(--ink-soft);
  }

  .tick-all {
    display: flex;
    align-items: center;
    gap: 10px;
    flex: 1;
    min-width: 0;
  }

  .tick {
    flex: none;
    width: 16px;
    height: 16px;
    margin: 0;
    accent-color: var(--accent);
  }

  li .tick {
    margin-left: 12px;
  }

  li.going {
    background: color-mix(in srgb, var(--accent) 7%, transparent);
  }

  .folder {
    margin: 8px 16px 0 46px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .from,
  .partly,
  .binary {
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .from {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .added,
  .deleted {
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .added,
  .binary {
    margin-left: auto;
  }

  .added {
    color: var(--success);
  }

  .deleted {
    color: var(--danger);
  }

  .what {
    font-size: 14px;
    color: var(--ink-soft);
  }

  .folded {
    display: none;
  }

  .add-body {
    margin-right: auto;
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
    gap: 8px;
    min-width: 0;
    padding: 6px 12px 6px 10px;
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
