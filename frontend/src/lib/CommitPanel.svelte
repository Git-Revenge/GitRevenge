<script lang="ts">
  import { errorText } from './errors'
  import { CommitDetail, CommitNotes, CommitSignatures, IssueURL, SetCommitNote } from '../../wailsjs/go/app/App.js'
  import type { app, git } from '../../wailsjs/go/models'
  import { BrowserOpenURL, ClipboardSetText } from '../../wailsjs/runtime/runtime.js'
  import { FILTER_FROM, fileName, fileRows, readTreeChoice } from './fileList'
  import FileListTools from './FileListTools.svelte'
  import { TWO_COLUMNS } from './rightPanel'
  import { formatTime, type RefLabel } from './graph/draw'
  import RefChip from './graph/RefChip.svelte'
  import type { DiffTarget } from './diff/DiffView.svelte'
  import { session } from './session.svelte'
  import { reflow } from './text'
  import { asPerson, groupTrailers, valueParts } from './trailers'
  import Markdown from './Markdown.svelte'
  import Icon, { type IconName } from './ui/Icon.svelte'
  import Loading from './ui/Loading.svelte'

  let {
    path,
    commitId,
    refs = [],
    openFile,
    onActions,
    onReveal,
    current,
    onSearch,
  }: {
    path: string
    commitId: string
    /** Labels for the branches and tags on the commit. */
    refs?: RefLabel[]
    openFile: (t: DiffTarget) => void
    /** The diff currently open, to highlight its file. */
    current?: DiffTarget | null
    /** Searches the graph, e.g. for other commits with the same trailer. */
    onSearch?: (query: string) => void
    /** Opens the commit's menu at this position. */
    onActions?: (x: number, y: number) => void
    /** Shows another commit in the graph, such as a parent. */
    onReveal?: (id: string) => void
  } = $props()

  /** The graph search for commits sharing this trailer: by name for a person. */
  function trailerQuery(key: string, value: string): string {
    return `${key}: ${asPerson(value)?.name ?? value}`
  }

  let info = $state<app.CommitInfo | null>(null)

  // Which tab shows. It stays put as you move between commits, and is remembered across
  // sessions; a commit it can't apply to shows the message instead, without forgetting it.
  type Tab = 'message' | 'files' | 'trailers' | 'details'
  const TAB_LABELS: Record<Tab, string> = { message: 'Message', files: 'Files', trailers: 'Trailers', details: 'Details' }
  const TAB_KEY = 'gitrevenge.commitTab'
  let chosenTab = $state<Tab>(readTab())
  function readTab(): Tab {
    try {
      const saved = localStorage.getItem(TAB_KEY)
      return saved && saved in TAB_LABELS ? (saved as Tab) : 'message'
    } catch {
      return 'message'
    }
  }
  function chooseTab(t: Tab) {
    chosenTab = t
    try {
      localStorage.setItem(TAB_KEY, t)
    } catch {
      // Storage unavailable; the choice lasts for this session only.
    }
  }

  let panelWidth = $state(0)
  /** With room for it, the files stay beside the other tabs instead of being one of them. */
  let wide = $derived(panelWidth >= TWO_COLUMNS)
  let tabs = $derived.by(() => {
    const out: Tab[] = wide ? ['message'] : ['message', 'files']
    if (info?.commit.trailers.length) out.push('trailers')
    out.push('details')
    return out
  })
  let tab = $derived(tabs.includes(chosenTab) ? chosenTab : 'message')

  /** Arrow keys move along the tabs, as a tab list should. */
  function tabKey(e: KeyboardEvent, t: Tab) {
    const i = tabs.indexOf(t)
    const to = e.key === 'ArrowRight' ? (i + 1) % tabs.length : e.key === 'ArrowLeft' ? (i - 1 + tabs.length) % tabs.length : e.key === 'Home' ? 0 : e.key === 'End' ? tabs.length - 1 : -1
    if (to < 0) return
    e.preventDefault()
    chooseTab(tabs[to])
    const list = (e.currentTarget as HTMLElement).parentElement
    list?.querySelectorAll<HTMLElement>('[role=tab]')[to]?.focus()
  }

  let fileQuery = $state('')
  let fileTree = $state(readTreeChoice())
  let closedFolders = $state(new Set<string>())
  let rows = $derived(fileRows(info?.files ?? [], { query: fileQuery, tree: fileTree, closed: closedFolders }))
  function toggleFolder(path: string) {
    closedFolders = new Set(closedFolders.has(path) ? [...closedFolders].filter((p) => p !== path) : [...closedFolders, path])
  }
  // A filter is for the commit it was typed on.
  $effect(() => {
    void commitId
    fileQuery = ''
  })

  /** How many of a long Cc list show until asked for the rest. */
  const CC_SHOWN = 2
  let ccOpen = $state(false)
  $effect(() => {
    void commitId
    ccOpen = false
  })

  let copied = $state(false)
  let copiedTimer: ReturnType<typeof setTimeout> | undefined
  async function copySha(id: string) {
    try {
      copied = (await ClipboardSetText(id)) !== false
    } catch {
      copied = false
    }
    clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => (copied = false), 1500)
  }

  const STATUS_ICONS: Record<string, IconName> = { A: 'st-added', M: 'st-modified', T: 'st-modified', D: 'st-deleted', R: 'st-renamed', C: 'st-renamed' }
  const STATUS_KINDS: Record<string, string> = { A: 'added', M: 'modified', T: 'modified', D: 'deleted', R: 'renamed', C: 'renamed' }

  // git notes on the commit, from every notes ref; the default ref's note can be edited.
  let notes = $state<git.Note[]>([])
  let notesError = $state('')
  let noteDraft = $state<string | null>(null)
  let noteBusy = $state(false)
  const DEFAULT_NOTES = 'refs/notes/commits'
  let ownNote = $derived(notes.find((n) => n.ref === DEFAULT_NOTES))

  async function loadNotes(id: string) {
    try {
      const res = await CommitNotes(path, id)
      if (id === commitId) {
        notes = res ?? []
        notesError = ''
      }
    } catch (e) {
      // Not the same as having no notes, so it's worth saying.
      if (id === commitId) notesError = errorText(e)
    }
  }

  $effect(() => {
    const id = commitId
    notes = []
    noteDraft = null
    void loadNotes(id)
  })

  async function saveNote(text: string) {
    const id = commitId
    noteBusy = true
    try {
      await session.run(() => SetCommitNote(path, id, text))
      noteDraft = null
      await loadNotes(id)
    } finally {
      noteBusy = false
    }
  }

  /** git's verdict on the selected commit's signature, for signed commits. */
  let sig = $state<git.SignatureCheck | null>(null)

  $effect(() => {
    const id = commitId
    const signed = info?.commit.id === id && info.commit.signed
    sig = null
    if (!signed) return
    void CommitSignatures(path, [id])
      .then(([c]) => {
        if (id === commitId) sig = c ?? null
      })
      // The signature is decoration: a failure just means no mark.
      .catch(() => {})
  })

  const SIG_TEXT: Record<string, string> = {
    good: 'Verified',
    bad: 'Bad signature',
    unknown: "Can't verify: unknown or untrusted key",
    expired: 'Expired signature or key',
    revoked: 'Revoked key',
  }
  /** Where "#123" in a trailer links to, from the repository's remote; "" for nowhere. */
  let issueURL = $state('')

  $effect(() => {
    const p = path
    issueURL = ''
    void IssueURL(p)
      .then((u) => {
        if (p === path) issueURL = u
      })
      // Without it, issue references just aren't links.
      .catch(() => {})
  })

  // Messages render as markdown; the raw text is a toggle away, remembered across sessions.
  const RAW_KEY = 'gitrevenge.rawMessages'
  let raw = $state(readRaw())
  function readRaw(): boolean {
    try {
      return localStorage.getItem(RAW_KEY) === 'on'
    } catch {
      return false
    }
  }
  function toggleRaw() {
    raw = !raw
    try {
      localStorage.setItem(RAW_KEY, raw ? 'on' : 'off')
    } catch {
      // Storage unavailable; the choice lasts for this session only.
    }
  }

  function openLink(e: MouseEvent, href: string) {
    e.preventDefault()
    BrowserOpenURL(href)
  }

  const statusNames: Record<string, string> = {
    A: 'Added',
    M: 'Modified',
    D: 'Deleted',
    R: 'Renamed',
    C: 'Copied',
    T: 'Type changed',
  }

  // Load whenever the selected commit changes; drop answers for a commit no longer selected.
  $effect(() => {
    const id = commitId
    info = null
    void session.run(() => CommitDetail(path, id)).then((res) => {
      if (res && id === commitId) info = res
    })
  })

  function showFile(f: git.FileChange) {
    openFile({ mode: 'commit', commit: commitId, file: f.path, oldPath: f.oldPath })
  }
</script>

{#snippet trailerValue(key: string, value: string)}
  {@const who = asPerson(value)}
  {#if who}
    {who.name}<span class="email" title={who.email}>{who.email}</span>
  {:else}
    {#each valueParts(value, issueURL) as part, k (k)}{#if part.href}<a href={part.href} title={part.href} onclick={(e) => openLink(e, part.href!)}>{part.text}</a>{:else}{part.text}{/if}{/each}
  {/if}
  {#if onSearch && key !== 'stable'}
    <button class="find" title="Find commits with this trailer" aria-label="Find commits with {trailerQuery(key, value)}" onclick={() => onSearch(trailerQuery(key, value))}><Icon name="search" size={13} /></button>
  {/if}
{/snippet}

{#snippet trailerSection(title: string, rows: { key: string; label: string; values: string[] }[])}
  <section class="trailer-section">
    <h3>{title}</h3>
    <dl class="trailers">
      {#each rows as row (row.key)}
        <dt class="meta">{row.label}</dt>
        <dd class="trailer-values">
          {#each row.values as v, i (i)}<span class="trailer">{@render trailerValue(row.key, v)}</span>{/each}
        </dd>
      {/each}
    </dl>
  </section>
{/snippet}

{#snippet fileList()}
  <div class="files-head">
    <span class="meta label">Files changed</span>
    {#if info && info.commit.parents.length > 1}<span class="merge-note" title="Changes are shown against the first parent">First parent</span>{/if}
  </div>
  {#if (info?.files.length ?? 0) >= FILTER_FROM}
    <div class="file-tools"><FileListTools bind:query={fileQuery} bind:tree={fileTree} /></div>
  {/if}
  <div class="files">
    <ul>
      {#each rows as r (r.key)}
        {#if r.type === 'folder'}
          <li class="folder-row" style:--depth={r.depth}>
            {#if r.collapsible}
              <button class="folder" aria-expanded={r.open} title={r.path} onclick={() => toggleFolder(r.path)}><Icon name="chevron" size={12} />{r.name}/</button>
            {:else}
              <div class="folder" title={r.path}>{r.name}</div>
            {/if}
          </li>
        {:else}
          {@const f = r.file}
          <li style:--depth={r.depth}>
            <button
              class="file-row"
              class:active={current?.mode === 'commit' && current.commit === commitId && current.file === f.path}
              onclick={() => showFile(f)}
              title={f.oldPath ? `${f.oldPath} → ${f.path}` : f.path}
            >
              <span class="code s-{STATUS_KINDS[f.status] ?? 'modified'}" title={statusNames[f.status] ?? f.status}><Icon name={STATUS_ICONS[f.status] ?? 'st-modified'} size={14} /><span class="visually-hidden">{statusNames[f.status] ?? f.status}</span></span>
              <span class="file">{fileName(f.path)}</span>
              {#if f.oldPath}<span class="from">from {f.oldPath}</span>{/if}
              {#if f.binary}
                <span class="binary">binary</span>
              {:else if f.added || f.deleted}
                <span class="added">+{f.added}</span>
                <span class="deleted">−{f.deleted}</span>
              {/if}
            </button>
          </li>
        {/if}
      {/each}
    </ul>
    {#if fileQuery.trim() && !rows.length}<p class="muted">No files match that.</p>{/if}
  </div>
{/snippet}

<aside class="panel" aria-label="Commit details" bind:clientWidth={panelWidth}>
  <span class="blob" aria-hidden="true"></span>
  {#if !info}
    <Loading />
  {:else}
    {@const c = info.commit}
    <header>
      <h2>{c.subject}</h2>
      <div class="who">
        <button class="author" onclick={() => onSearch?.(c.author.name)} title="Find commits by {c.author.name}">{c.author.name}</button>
        <span class="when">{formatTime(c.author.time)}</span>
      </div>
      <div class="ids">
        <button class="btn sm secondary sha" onclick={() => copySha(c.id)} title="Copy the full SHA" aria-label="Copy SHA {c.id.slice(0, 7)}"
          >{#if copied}Copied{:else}{c.id.slice(0, 7)}{/if}<Icon name={copied ? 'check' : 'copy'} size={13} /></button
        >
        {#each c.parents as p (p)}
          <button class="btn sm text" onclick={() => onReveal?.(p)} title="Show this parent in the graph">parent {p.slice(0, 7)}</button>
        {/each}
        {#if info.commit.signed && sig}
          <span class="sig-mark sig-{sig.status}" title={SIG_TEXT[sig.status] ?? 'Unsigned'}><Icon name={sig.status === 'good' ? 'check' : 'warning'} size={13} />{sig.status === 'good' ? 'Verified' : 'Signature'}</span>
        {/if}
        <span class="spacer"></span>
        {#if onActions}
          <button class="btn sm quiet" onclick={(e) => onActions?.(e.clientX, e.clientY)}>Actions<Icon name="chevron" size={13} /></button>
        {/if}
      </div>
      {#if refs.length}
        <div class="refs" aria-label="Branches and tags on this commit">
          {#each refs as l (l.ref.name)}<RefChip label={l} />{/each}
        </div>
      {/if}
    </header>

    <div class="tabs" role="tablist" aria-label="Commit">
      {#each tabs as t (t)}
        <button role="tab" id="commit-tab-{t}" class="tab" class:active={tab === t} aria-selected={tab === t} aria-controls="commit-panel" tabindex={tab === t ? 0 : -1} onclick={() => chooseTab(t)} onkeydown={(e) => tabKey(e, t)}>{TAB_LABELS[t]}</button>
      {/each}
    </div>

    <div class="content" class:wide>
      <div class="tabpanel" id="commit-panel" role="tabpanel" aria-labelledby="commit-tab-{tab}" tabindex="-1">
        {#if tab === 'message'}
          {#if c.body}
            <div class="message-tools">
              <button class="btn sm quiet raw-toggle" aria-pressed={raw} onclick={toggleRaw} title={raw ? 'Show the message rendered as markdown' : 'Show the message as plain text'}>{raw ? 'Raw' : 'Markdown'}</button>
            </div>
            <div class="message">
              {#if raw}<p class="body">{reflow(c.body)}</p>{:else}<Markdown text={c.body} {issueURL} />{/if}
            </div>
          {:else}
            <p class="muted">No description.</p>
          {/if}
        {:else if tab === 'files'}
          {@render fileList()}
        {:else if tab === 'trailers'}
          <div class="message-tools">
            <button class="btn sm quiet raw-toggle" aria-pressed={raw} onclick={toggleRaw} title={raw ? 'Show the trailers grouped by purpose' : 'Show the trailers as written in the message'}>{raw ? 'Raw' : 'Grouped'}</button>
          </div>
          {#if raw}
            <dl class="trailers as-written" aria-label="Trailers">
              {#each c.trailers as t, i (i)}
                <dt class="meta">{t.key}</dt>
                <dd class="trailer">{@render trailerValue(t.key, t.value)}</dd>
              {/each}
            </dl>
          {:else}
            {@const g = groupTrailers(c.trailers)}
            {#if g.related.length}{@render trailerSection('Related', g.related)}{/if}
            {#if g.people.length || g.cc.length}
              <section class="trailer-section">
                <h3>People</h3>
                <dl class="trailers">
                  {#each g.people as row (row.key)}
                    <dt class="meta">{row.label}</dt>
                    <dd class="trailer-values">
                      {#each row.values as v, i (i)}<span class="trailer">{@render trailerValue(row.key, v)}</span>{/each}
                    </dd>
                  {/each}
                  {#if g.cc.length}
                    <dt class="meta">Cc</dt>
                    <dd class="trailer-values">
                      {#each ccOpen ? g.cc : g.cc.slice(0, CC_SHOWN) as v, i (i)}<span class="trailer">{@render trailerValue('Cc', v)}</span>{/each}
                      {#if g.cc.length > CC_SHOWN}
                        <button class="btn sm text" aria-expanded={ccOpen} onclick={() => (ccOpen = !ccOpen)}>{ccOpen ? 'Show fewer' : `+ ${g.cc.length - CC_SHOWN} more`}</button>
                      {/if}
                    </dd>
                  {/if}
                </dl>
              </section>
            {/if}
            {#if g.other.length}{@render trailerSection('Other', g.other)}{/if}
          {/if}
        {:else}
          <dl class="details">
            <dt class="meta">Author</dt>
            <dd>
              {c.author.name}
              <span class="email" title={c.author.email}>{c.author.email}</span>
              <span class="when">{formatTime(c.author.time)}</span>
            </dd>
            {#if c.committer.name !== c.author.name || c.committer.time !== c.author.time}
              <dt class="meta">Committer</dt>
              <dd>
                {c.committer.name}
                <span class="when">{formatTime(c.committer.time)}</span>
              </dd>
            {/if}
            <dt class="meta">Notes</dt>
            <dd class="notes">
              {#if notesError}<p class="muted" role="alert">Couldn't read notes: {notesError}</p>{/if}
              {#each notes as n (n.ref)}
                {#if !(n.ref === DEFAULT_NOTES && noteDraft !== null)}
                  <div class="note">
                    {#if n.ref !== DEFAULT_NOTES}<span class="meta note-ref">{n.ref.replace(/^refs\/notes\//, '')}</span>{/if}
                    <p>{n.text}</p>
                  </div>
                {/if}
              {/each}
              {#if noteDraft !== null}
                <textarea bind:value={noteDraft} rows="3" aria-label="Note" disabled={noteBusy}></textarea>
                <span class="note-actions">
                  <button class="btn sm secondary" disabled={noteBusy} onclick={() => saveNote(noteDraft ?? '')}>Save note</button>
                  <button class="btn sm quiet" disabled={noteBusy} onclick={() => (noteDraft = null)}>Cancel</button>
                </span>
              {:else}
                <span class="note-actions">
                  <button class="btn sm quiet" onclick={() => (noteDraft = ownNote?.text ?? '')}><Icon name={ownNote ? 'edit' : 'plus'} size={13} />{ownNote ? 'Edit note' : 'Add note'}</button>
                  {#if ownNote}<button class="btn sm quiet" disabled={noteBusy} onclick={() => saveNote('')}><Icon name="trash" size={13} />Remove</button>{/if}
                </span>
              {/if}
            </dd>
            <dt class="meta">Signature</dt>
            {#if info.commit.signed}
              <dd class="sig sig-{sig?.status ?? 'pending'}">
                {#if !sig}
                  Checking…
                {:else}
                  <Icon name={sig.status === 'good' ? 'check' : 'warning'} size={14} />
                  {SIG_TEXT[sig.status] ?? 'Unsigned'}{sig.format ? ` · ${sig.format.toUpperCase()}` : ''}
                  {#if sig.signer}<span class="email" title={sig.signer}>{sig.signer}</span>{/if}
                  {#if sig.key}<span class="key-id" title="Key">{sig.key}</span>{/if}
                {/if}
              </dd>
            {:else}
              <dd class="muted">Not signed</dd>
            {/if}
            <dt class="meta">Commit</dt>
            <dd><code>{c.id}</code></dd>
            {#if c.parents.length}
              <dt class="meta">{c.parents.length > 1 ? 'Parents' : 'Parent'}</dt>
              <dd class="parents">{c.parents.map((p) => p.slice(0, 10)).join(' ')}</dd>
            {/if}
          </dl>
        {/if}
      </div>
      {#if wide}
        <div class="side-files">{@render fileList()}</div>
      {/if}
    </div>
  {/if}
</aside>

<style>
  .panel {
    position: relative;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    border-left: 1px solid var(--paper-edge);
    background: var(--paper);
    overflow: hidden;
  }

  .who {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 2px 8px;
    font-size: 14px;
  }

  .author {
    padding: 0;
    border: 0;
    background: none;
    font: inherit;
    color: var(--ink);
    text-align: left;
  }

  .author:hover:not(:disabled) {
    background: none;
    color: var(--link);
  }

  .ids {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 8px;
    margin-top: 8px;
  }

  .spacer {
    flex: 1;
  }

  .sha {
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .sig-mark {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .sig-mark.sig-good {
    color: var(--success);
  }

  .sig-mark:not(.sig-good) {
    color: var(--warning);
  }

  /* Plain headings, under one torn rose bar, like the repository tabs above. */
  .tabs {
    position: relative;
    flex: none;
    display: flex;
    gap: 2px;
    padding: 0 12px;
    border-bottom: 1px solid var(--paper-edge);
  }

  .tab {
    position: relative;
    padding: 8px 10px 9px;
    border: 0;
    border-radius: 0;
    background: none;
    font-size: 14px;
    color: var(--ink-soft);
    white-space: nowrap;
  }

  .tab:hover:not(:disabled) {
    background: none;
    color: var(--ink);
  }

  .tab.active {
    color: var(--ink);
    font-weight: 500;
  }

  .tab.active::after {
    content: '';
    position: absolute;
    left: 6px;
    right: 6px;
    bottom: 1px;
    height: 3px;
    border-radius: 45% 55% 50% 50% / 55% 45%;
    background: var(--link);
  }

  .content {
    flex: 1;
    min-height: 0;
    display: flex;
  }

  .tabpanel {
    flex: 1;
    min-width: 0;
    box-sizing: border-box;
    padding: 12px 20px 16px;
    overflow-y: auto;
    outline: none;
  }

  .content.wide .tabpanel {
    flex: 3;
  }

  .side-files {
    flex: 2;
    min-width: 0;
    box-sizing: border-box;
    padding-top: 12px;
    border-left: 1px solid var(--paper-edge);
    overflow-y: auto;
  }

  .trailer-section {
    margin-bottom: 12px;
    padding-bottom: 12px;
    border-bottom: 1px dashed var(--hairline);
  }

  .trailer-section:last-child {
    margin-bottom: 0;
    border-bottom: 0;
  }

  h3 {
    margin: 0 0 6px;
    font-size: 14px;
    font-weight: 500;
    color: var(--ink-soft);
  }

  .trailer-values {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
  }

  .message-tools {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 4px;
  }

  .files-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
    margin: 0 0 6px;
  }

  .side-files .files-head,
  .side-files .files {
    margin-left: 20px;
    margin-right: 12px;
  }

  .merge-note {
    font-size: 13px;
    color: var(--ink-mute);
  }

  .file-tools {
    margin-bottom: 6px;
  }

  .side-files .file-tools {
    margin-left: 20px;
    margin-right: 12px;
  }

  .files li {
    padding-left: calc(var(--depth, 0) * 14px);
  }

  button.folder {
    display: flex;
    align-items: center;
    gap: 3px;
    width: 100%;
    margin-top: 2px;
    padding: 2px 0;
    border: 0;
    background: none;
    text-align: left;
  }

  button.folder :global(svg) {
    flex: none;
    transition: transform var(--dur-quick);
  }

  button.folder[aria-expanded='true'] :global(svg) {
    transform: rotate(90deg);
  }

  button.folder:hover:not(:disabled) {
    background: none;
    color: var(--ink);
  }

  .folder {
    margin-top: 8px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .files ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .files li button {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    min-height: 28px;
    padding: 2px 4px;
    border: 0;
    border-radius: 4px;
    background: none;
    text-align: left;
  }

  .files li button:hover:not(:disabled) {
    background: var(--wash-hover);
  }

  .files li button.active {
    background: var(--selected);
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

  .s-deleted {
    color: var(--danger);
  }

  .s-renamed {
    color: var(--accent-a);
  }

  .file {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--ink);
  }

  .from {
    flex: 1 1 0;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .added,
  .deleted,
  .binary {
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .added {
    margin-left: auto;
    color: var(--success);
  }

  .from + .added {
    margin-left: 0;
  }

  .deleted {
    color: var(--danger);
  }

  .binary {
    margin-left: auto;
    color: var(--ink-mute);
  }

  /* One watercolour blob behind the title: decoration only. */
  .blob {
    position: absolute;
    top: -50px;
    right: -40px;
    width: 220px;
    height: 170px;
    border-radius: 45% 55% 50% 50% / 55% 45%;
    background: var(--ink-rose);
    filter: blur(60px);
    opacity: 0.16;
    pointer-events: none;
  }

  :global([data-theme='dark']) .blob {
    opacity: 0.12;
  }

  header {
    position: relative;
    flex: none;
    padding: 16px 16px 10px 20px;
  }

  h2 {
    margin: 0 0 6px;
    font-size: 19px;
    font-weight: 500;
    line-height: 1.25;
    letter-spacing: -0.015em;
    overflow-wrap: anywhere;
    font-variation-settings:
      'SOFT' 50,
      'WONK' 1;
  }

  .body {
    margin: 0 0 12px;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    font-size: 14.5px;
    line-height: 1.6;
    color: var(--ink-soft);
  }

  dl {
    display: grid;
    grid-template-columns: 70px minmax(0, 1fr);
    gap: 8px 12px;
    align-items: baseline;
    margin: 12px 0 8px;
  }

  dd {
    margin: 0;
    overflow-wrap: anywhere;
    font-size: 14px;
  }

  .refs {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    margin-top: 8px;
  }

  .message {
    margin: 0 0 12px;
  }

  .message .body {
    margin: 0;
  }

  /* A quiet switch above the message's top right corner. */
  /* The kicker and the message toggle share a row above the subject, so a long subject
     can't cover the toggle. */
  .head-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .raw-toggle {
    min-width: 0;
    flex-shrink: 0;
  }

  .notes .note p {
    margin: 0 0 6px;
    white-space: pre-wrap;
  }

  .note-ref {
    display: block;
  }

  .notes textarea {
    width: 100%;
    font: inherit;
    font-size: 13.5px;
  }

  .note-actions {
    display: flex;
    gap: 4px;
    margin-left: -12px;
  }

  .sig :global(svg) {
    vertical-align: -2px;
  }

  .sig-good {
    color: var(--success);
  }

  .sig-bad,
  .sig-revoked {
    color: var(--danger);
  }

  .sig-unknown,
  .sig-expired {
    color: var(--warning);
  }

  .sig-pending {
    color: var(--ink-mute);
  }

  .key-id {
    display: block;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    overflow-wrap: anywhere;
  }

  /* Trailers read as a short record under the message: key, then person or value. */
  dl.trailers {
    grid-template-columns: max-content minmax(0, 1fr);
    margin: 0 0 12px;
    padding-top: 10px;
    border-top: 1px dotted var(--paper-edge);
  }

  .trailers dt {
    max-width: 150px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* The find button shows on hover or focus, so the record stays quiet. */
  .trailer {
    position: relative;
    padding-right: 24px;
  }

  .find {
    position: absolute;
    top: 0;
    right: 0;
    display: inline-flex;
    padding: 3px;
    background: none;
    border: none;
    border-radius: 4px;
    color: var(--ink-mute);
    opacity: 0;
  }

  .trailer:hover .find,
  .find:focus-visible {
    opacity: 1;
  }

  .find:hover:not(:disabled) {
    background: var(--wash-hover);
    color: var(--link);
  }

  .trailers a {
    color: var(--link);
  }

  .trailers a:hover {
    color: var(--link-hover);
  }

  .email {
    display: block;
    font-size: 13px;
    color: var(--ink-mute);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .when {
    display: block;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  code {
    padding: 1px 4px;
    border-radius: 3px;
    background: var(--paper-warm);
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-b);
    overflow-wrap: anywhere;
  }

  .parents {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-b);
  }

  .note {
    margin: 0 0 8px;
    font-size: 13px;
    font-style: italic;
    color: var(--ink-mute);
  }

  .files-head {
    position: relative;
    display: flex;
    gap: 10px;
    align-items: baseline;
    margin: 12px 20px 0;
    padding-bottom: 6px;
    border-bottom: 1px solid var(--paper-edge);
  }

  .label {
    color: var(--ink-soft);
  }

  .files {
    list-style: none;
    margin: 0 8px;
    padding: 0;
  }

  .files li {
    border-bottom: 1px dotted var(--paper-edge);
  }

  .files button {
    display: grid;
    grid-template-columns: 78px minmax(0, 1fr);
    align-items: baseline;
    gap: 10px;
    width: 100%;
    padding: 7px 12px;
    background: none;
    border: none;
    border-radius: 4px;
    text-align: left;
    transition:
      background-color var(--dur-base),
      transform var(--dur-base) var(--ease-ink);
  }

  .files button:hover:not(:disabled) {
    background: var(--wash-hover);
    transform: translateX(3px);
  }

  .files button.active {
    background: var(--selected);
  }

  .status {
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .s-A {
    color: var(--success);
  }

  .s-D {
    color: var(--danger);
  }

  .s-M,
  .s-T {
    color: var(--accent-c);
  }

  .s-R,
  .s-C {
    color: var(--accent-a);
  }

  .file {
    font-family: var(--font-mono);
    font-size: 12.5px;
    overflow-wrap: anywhere;
  }

  .muted {
    color: var(--ink-mute);
  }

  .pad {
    padding: 16px 20px;
    font-style: italic;
  }

  @media (prefers-reduced-motion: reduce) {
    .files button {
      transition: none;
    }

    .files button:hover:not(:disabled) {
      transform: none;
    }
  }
</style>
