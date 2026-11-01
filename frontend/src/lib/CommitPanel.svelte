<script lang="ts">
  import { errorText } from './errors'
  import { CommitDetail, CommitNotes, CommitSignatures, IssueURL, SetCommitNote } from '../../wailsjs/go/app/App.js'
  import type { app, git } from '../../wailsjs/go/models'
  import { BrowserOpenURL } from '../../wailsjs/runtime/runtime.js'
  import { formatTime } from './graph/draw'
  import type { DiffTarget } from './diff/DiffView.svelte'
  import { session } from './session.svelte'
  import { reflow } from './text'
  import { asPerson, valueParts } from './trailers'
  import Markdown from './Markdown.svelte'
  import Icon from './ui/Icon.svelte'

  let {
    path,
    commitId,
    openFile,
    current,
    onSearch,
  }: {
    path: string
    commitId: string
    openFile: (t: DiffTarget) => void
    /** The diff currently open, to highlight its file. */
    current?: DiffTarget | null
    /** Searches the graph, e.g. for other commits with the same trailer. */
    onSearch?: (query: string) => void
  } = $props()

  /** The graph search for commits sharing this trailer: by name for a person. */
  function trailerQuery(key: string, value: string): string {
    return `${key}: ${asPerson(value)?.name ?? value}`
  }

  let info = $state<app.CommitInfo | null>(null)

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

<aside class="panel" aria-label="Commit details">
  <span class="blob" aria-hidden="true"></span>
  {#if !info}
    <p class="muted pad">Loading…</p>
  {:else}
    {@const c = info.commit}
    <header>
      <div class="head-row">
        <span class="hand">this commit</span>
        {#if c.body}
          <button
            class="btn sm quiet raw-toggle"
            aria-pressed={raw}
            onclick={toggleRaw}
            title={raw ? 'Show the message rendered as markdown' : 'Show the message as plain text'}>{raw ? 'Raw' : 'Markdown'}</button
          >
        {/if}
      </div>
      <h2>{c.subject}</h2>
      {#if c.body}
        <div class="message">
          {#if raw}<p class="body">{reflow(c.body)}</p>{:else}<Markdown text={c.body} {issueURL} />{/if}
        </div>
      {/if}
      {#if c.trailers.length}
        <dl class="trailers" aria-label="Trailers">
          {#each c.trailers as t, i (i)}
            {@const who = asPerson(t.value)}
            <dt class="meta">{t.key}</dt>
            <dd class="trailer">
              {#if who}
                {who.name}<span class="email" title={who.email}>{who.email}</span>
              {:else}
                {#each valueParts(t.value, issueURL) as part, k (k)}{#if part.href}<a
                      href={part.href}
                      title={part.href}
                      onclick={(e) => openLink(e, part.href!)}>{part.text}</a
                    >{:else}{part.text}{/if}{/each}
              {/if}
              {#if onSearch}
                <button
                  class="find"
                  title="Find commits with this trailer"
                  aria-label="Find commits with {trailerQuery(t.key, t.value)}"
                  onclick={() => onSearch(trailerQuery(t.key, t.value))}><Icon name="search" size={13} /></button
                >
              {/if}
            </dd>
          {/each}
        </dl>
      {/if}
      <dl>
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
              <button class="btn sm quiet" onclick={() => (noteDraft = ownNote?.text ?? '')}><Icon name={ownNote ? 'edit' : 'plus'} size={14} />{ownNote ? 'Edit note' : 'Add note'}</button>
              {#if ownNote}<button class="btn sm quiet" disabled={noteBusy} onclick={() => saveNote('')}><Icon name="trash" size={14} />Remove</button>{/if}
            </span>
          {/if}
        </dd>
        {#if info.commit.signed}
          <dt class="meta">Signature</dt>
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
        {/if}
        <dt class="meta">Commit</dt>
        <dd><code>{c.id}</code></dd>
        {#if c.parents.length}
          <dt class="meta">{c.parents.length > 1 ? 'Parents' : 'Parent'}</dt>
          <dd class="parents">{c.parents.map((p) => p.slice(0, 10)).join(' ')}</dd>
        {/if}
      </dl>
      {#if c.parents.length > 1}
        <p class="note">Changes are shown against the first parent.</p>
      {/if}
    </header>

    <div class="files-head">
      <span class="meta label">Files changed</span>
      <span class="meta">{info.files.length}</span>
    </div>
    <ul class="files">
      {#each info.files as f (f.path)}
        <li>
          <button
            class:active={current?.mode === 'commit' && current.commit === commitId && current.file === f.path}
            onclick={() => showFile(f)}
            title={f.oldPath ? `${f.oldPath} → ${f.path}` : f.path}
          >
            <span class="status s-{f.status}">{statusNames[f.status] ?? f.status}</span>
            <span class="file">{f.oldPath ? `${f.oldPath} → ${f.path}` : f.path}</span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</aside>

<style>
  .panel {
    position: relative;
    display: flex;
    flex-direction: column;
    width: 420px;
    min-width: 300px;
    padding-bottom: 16px;
    border-left: 1px solid var(--paper-edge);
    background: var(--paper);
    overflow-x: hidden;
    overflow-y: auto;
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
    padding: 18px 20px 4px;
  }

  h2 {
    margin: 4px 0 8px;
    font-size: 21px;
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
    font-size: 11.5px;
    color: var(--ink-mute);
    overflow-wrap: anywhere;
  }

  /* Trailers read as a short record under the message: key, then person or value. */
  dl.trailers {
    grid-template-columns: max-content minmax(0, 1fr);
    margin: 0 0 12px;
    padding-top: 10px;
    border-top: 1px dashed var(--hairline);
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
    font-size: 11.5px;
    color: var(--ink-mute);
  }

  code {
    padding: 1px 4px;
    border-radius: 3px;
    background: var(--paper-warm);
    font-family: var(--font-mono);
    font-size: 11.5px;
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
    border-bottom: 1px dashed var(--hairline);
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
    font-size: 11px;
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
