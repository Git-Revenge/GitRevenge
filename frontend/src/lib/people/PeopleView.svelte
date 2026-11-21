<script lang="ts">
  import type { app } from '../../../wailsjs/go/models'
  import { provideCommands } from '../commands.svelte'
  import { errorText } from '../errors'
  import { initials } from '../graph/draw'
  import { appendLine, deleteLine, setLine, splitLines } from '../ignorerules/rules'
  import { session } from '../session.svelte'
  import Empty from '../ui/Empty.svelte'
  import Icon from '../ui/Icon.svelte'
  import InkButton from '../ui/InkButton.svelte'
  import Loading from '../ui/Loading.svelte'
  import Modal from '../ui/Modal.svelte'
  import { WORK_TREE, canTakeApart, changedLines, describeNote, joined, linesText, sourceLabel, sourceNote, summary, wrote } from './mailmap'
  import { people as t } from './people.svelte'
  import SamePersonDialog from './SamePersonDialog.svelte'
  import { asSuggested, suggestionLabel } from './suggest'
  import { peopleTab } from './tab.svelte'

  // Re-read when the repository changes on disk, such as a .mailmap edited elsewhere.
  $effect(() => {
    void session.revision[t.repo]
    void t.load()
  })

  // The tab bar marks an unsaved edit, and closing asks about it, without loading this view.
  $effect(() => {
    peopleTab.dirty = t.dirty
  })

  let editable = $derived(t.source?.source === WORK_TREE)
  let text = $derived(editable ? (t.doc?.text ?? '') : (t.others[t.source?.source ?? ''] ?? ''))
  let lines = $derived(splitLines(text))
  let changed = $derived(t.doc ? changedLines(t.doc.base, t.doc.text) : 0)
  /** The first lines of what would be written, for the question before a .mailmap is created. */
  let preview = $derived(splitLines(t.doc?.text ?? '').filter((l) => l.trim() !== ''))

  // Editing one line in Text.
  let editing = $state(0)
  let draft = $state('')
  let adding = $state('')
  /** Text as one box, for a lot of changes at once. */
  let plain = $state(false)

  function startEdit(n: number, value: string) {
    editing = n
    draft = value
  }

  function finishEdit(keep: boolean) {
    if (keep && editing) t.edit(setLine(t.doc?.text ?? '', editing, draft))
    editing = 0
  }

  function editKey(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      finishEdit(true)
    } else if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      finishEdit(false)
    }
  }

  function add(e: SubmitEvent) {
    e.preventDefault()
    const v = adding.trim()
    if (!v) return
    t.edit(appendLine(t.doc?.text ?? '', v))
    adding = ''
  }

  // Who is this? Asked of the text as it stands, saved or not.
  let probe = $state('')
  let answers = $state<app.WhoAnswer[] | null>(null)
  let probeError = $state('')
  let asked = 0
  $effect(() => {
    const q = probe.trim()
    void t.doc?.text
    void session.revision[t.repo]
    const n = ++asked
    if (!q) {
      answers = null
      return
    }
    const timer = setTimeout(async () => {
      try {
        const res = await t.who(q)
        if (n === asked) [answers, probeError] = [res, '']
      } catch (e) {
        if (n === asked) [answers, probeError] = [null, errorText(e)]
      }
    }, 200)
    return () => clearTimeout(timer)
  })

  /** Reads the next page once the end of the list comes into view. */
  function nearEnd(el: HTMLElement) {
    const io = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting)) void t.more()
    })
    io.observe(el)
    return { destroy: () => io.disconnect() }
  }

  /** A colour for someone's initials, the same each time for the same name. */
  function colorOf(name: string): string {
    let h = 0
    for (const c of name) h = (h * 31 + c.codePointAt(0)!) >>> 0
    return `var(--lane-${h % 6})`
  }

  const others = $derived(t.sources.filter((s) => s.source !== WORK_TREE))

  // Saying that two or more people are one. The dialog shows the line before anything changes.
  let joining = $state<app.SuggestedPerson[] | null>(null)
  /** The person picked first, when joining two from the list by hand. */
  let picking = $state<app.Person | null>(null)
  let openShared = $state(false)
  let openApart = $state(false)
  const same = (a: app.Person, b: app.Person) => a.name === b.name && a.email === b.email

  function joinWith(other: app.Person) {
    if (!picking) return
    joining = [asSuggested(picking), asSuggested(other)]
    picking = null
  }
  let asking = $derived(!!t.suggestions && !t.query.trim() && t.suggestions.offered + t.suggestions.sharedCount + t.suggestions.apartCount > 0)

  provideCommands(() => [
    { id: 'people.save', group: 'People', title: t.exists ? 'Save the mailmap' : 'Create the mailmap', shortcut: 'Mod+S', whenTyping: true, run: () => void (t.dirty && t.save()) },
    { id: 'people.view', group: 'People', title: t.view === 'people' ? 'Show the mailmap as text' : 'Show people', run: () => void (t.view = t.view === 'people' ? 'text' : 'people') },
  ])
</script>

{#snippet suggestion(s: app.Suggestion, mode: 'offer' | 'shared' | 'apart')}
  <div class="offer">
    <div class="about">
      <span class="label">{suggestionLabel(s)}{s.why ? ` · ${s.why}` : ''}</span>
      {#each s.people as p (p.name + '\n' + p.email)}
        <span class="line"><span class="pname">{p.name || '(no name)'}</span> <span class="mail">{p.email}</span> <span class="why">· {wrote(p.commits, p.coAuthored)}</span></span>
      {/each}
    </div>
    <div class="answer-to">
      {#if mode === 'apart'}
        <button class="btn sm secondary" onclick={() => t.keepApart(s.key, false)}>Offer again</button>
      {:else}
        <button class="btn sm secondary" onclick={() => (joining = s.people)}>Same person…</button>
        {#if mode === 'offer'}<button class="btn sm quiet" title="Remembered for this repository, so it isn't offered again" onclick={() => t.keepApart(s.key, true)}>Different people</button>{/if}
      {/if}
    </div>
  </div>
{/snippet}

<div class="view">
  <nav class="side" aria-label="Mailmap files">
    {#if session.tabs.length > 1}
      <label class="repo">
        <span class="visually-hidden">Repository</span>
        <select value={t.repo} disabled={t.dirty} title={t.dirty ? 'Save or revert your edits to switch repository' : 'Repository'} onchange={(e) => peopleTab.setRepo(e.currentTarget.value)}>
          {#each session.tabs as tab (tab.path)}<option value={tab.path}>{tab.name}</option>{/each}
        </select>
      </label>
    {/if}
    <span class="eb hand">this repository</span>
    {#if t.shared}
      <button class="src" class:on={editable} aria-current={editable ? 'true' : undefined} onclick={() => t.select(WORK_TREE)} title={t.shared.path}>
        <span class="name">.mailmap</span>
        {#if t.dirty}<span class="dot" title="Unsaved edits">●</span>{/if}
        <span class="n">{t.exists ? t.shared.lines : 'none'}</span>
      </button>
    {/if}
    {#if others.length}
      <span class="eb hand">from your git config</span>
      {#each others as s (s.source)}
        <button class="src" class:on={t.source?.source === s.source} aria-current={t.source?.source === s.source ? 'true' : undefined} onclick={() => (t.select(s.source), (t.view = 'text'))} title={s.path}>
          <span class="name">{s.source}</span>
          <span class="n">{s.exists ? s.lines : 'missing'}</span>
        </button>
        <span class="path" title={s.path}>{s.path}</span>
      {/each}
    {/if}
    <span class="spacer"></span>
    <p class="aside">The graph follows these files, as git log does in a terminal. Insights reads only what's committed.</p>
  </nav>

  <main class="main">
    {#if !t.repo}
      <Empty note="nothing open">Open a repository to see who its history names.</Empty>
    {:else if t.source}
      <header class="head">
        <h1>{sourceLabel(t.source.source)}</h1>
        <span class="note">{sourceNote(t.source)}</span>
        <span class="spacer"></span>
        <div class="seg" role="group" aria-label="Show">
          <button aria-pressed={t.view === 'people'} onclick={() => (t.view = 'people')}>People</button>
          <button aria-pressed={t.view === 'text'} onclick={() => (t.view = 'text')}>Text</button>
        </div>
      </header>
      {#if t.saveError || t.error}
        <div class="notice" role="alert">
          {t.saveError || t.error}
          <button class="btn sm" onclick={() => t.reload()}>Reload from disk</button>
        </div>
      {/if}

      {#if !t.doc}
        <Loading label="Reading…" />
      {:else if t.view === 'people'}
        <div class="people">
          {#if !t.exists && !t.dirty}
            <!-- A repository works without a mailmap. Say so, and don't make it sound like a fault. -->
            <div class="none">
              <span class="hand">nothing to set up</span>
              <p>This repository has no mailmap and doesn't need one. Names show as the commits have them, and everything here still works. A mailmap only matters once two names are one person.</p>
            </div>
          {/if}
          {#if asking && t.suggestions}
            {@const sg = t.suggestions}
            <section class="suggest" aria-label="Likely the same person">
              {#if sg.offered}
                <div class="shead">
                  <h2>Looks like the same person</h2>
                  <span class="count">{sg.offered.toLocaleString()}</span>
                  <span class="mute">Offers only. Nothing is joined unless you say so.</span>
                </div>
                {#each sg.offers as s (s.key)}{@render suggestion(s, 'offer')}{/each}
                {#if sg.offered > sg.offers.length}<p class="rest">and {(sg.offered - sg.offers.length).toLocaleString()} more, which show as these are answered</p>{/if}
              {/if}
              {#if sg.sharedCount}
                <button class="fold" aria-expanded={openShared} onclick={() => (openShared = !openShared)}>
                  <span class="turn" class:open={openShared}><Icon name="chevron" size={12} /></span>
                  <span>Bots and shared addresses</span>
                  <span class="count">{sg.sharedCount.toLocaleString()}</span>
                  <span class="mute">not suggested; open to join them yourself</span>
                </button>
                {#if openShared}{#each sg.shared as s (s.key)}{@render suggestion(s, 'shared')}{/each}{/if}
              {/if}
              {#if sg.apartCount}
                <button class="fold" aria-expanded={openApart} onclick={() => (openApart = !openApart)}>
                  <span class="turn" class:open={openApart}><Icon name="chevron" size={12} /></span>
                  <span>Kept apart</span>
                  <span class="count">{sg.apartCount.toLocaleString()}</span>
                  <span class="mute">you said these are different people</span>
                </button>
                {#if openApart}{#each sg.apart as s (s.key)}{@render suggestion(s, 'apart')}{/each}{/if}
              {/if}
            </section>
          {/if}
          <div class="bar">
            <label class="find">
              <Icon name="search" size={14} />
              <span class="visually-hidden">Find a person</span>
              <input value={t.query} placeholder="Find a person by any name or address" spellcheck="false" oninput={(e) => t.search(e.currentTarget.value)} />
            </label>
            {#if t.list}
              <span class="sum">{summary(t.list)}{t.query.trim() ? ` · ${t.list.matching.toLocaleString()} found` : ''}{t.dirty ? ' · as your edits read them' : ''}</span>
            {/if}
          </div>
          {#if t.reading && !t.list}
            <Loading label="Reading the history for everyone it names…" />
          {:else if t.list && !t.people.length}
            <Empty note="no match" compact>{t.query.trim() ? 'Nobody by that name or address.' : 'This history has no commits yet.'}</Empty>
          {:else}
            {#if picking}
              <div class="picking" role="status">
                <span>Who is <strong>{picking.name || picking.email}</strong> the same person as? Choose them below.</span>
                <button class="btn sm" onclick={() => (picking = null)}>Cancel</button>
              </div>
            {/if}
            <ul class="list" aria-label="People">
              {#each t.people as p (p.name + '\n' + p.email)}
                <li class="person" class:first={!!picking && same(picking, p)}>
                  <span class="face" style:--c={colorOf(p.name)} aria-hidden="true">{initials(p.name)}</span>
                  <div class="who">
                    <div class="id">
                      <span class="pname">{p.name || '(no name)'}</span>
                      <span class="mail">{p.email}</span>
                      {#if p.bot}<span class="tag"><Icon name="bot" size={12} />bot</span>{/if}
                    </div>
                    {#each joined(p) as id (id.name + '\n' + id.email)}
                      <div class="also">
                        <span class="lead">also</span>
                        <span>{id.name || '(no name)'}</span>
                        <span class="mail">{id.email}</span>
                        <span class="why">· {wrote(id.commits, id.coAuthored)}{id.lines.length ? `, ${linesText(id.lines)}` : ''}</span>
                        {#if canTakeApart(id)}
                          <button class="btn sm text" title="Removes {linesText(id.lines)} of .mailmap. Nothing is saved until you save." onclick={() => t.takeApart(id)}>Take apart</button>
                        {/if}
                      </div>
                    {/each}
                  </div>
                  <span class="n">{wrote(p.commits, p.coAuthored)}</span>
                  <span class="join">
                    {#if !picking}
                      <button class="btn sm quiet" title="Say that this is the same person as someone else in the list" onclick={() => (picking = p)}>Same person as…</button>
                    {:else if !same(picking, p)}
                      <button class="btn sm secondary" onclick={() => joinWith(p)}>This one</button>
                    {/if}
                  </span>
                </li>
              {/each}
            </ul>
            {#if t.hasMore}
              <div class="more" use:nearEnd>
                <button class="btn sm" disabled={t.loadingMore} onclick={() => t.more()}>{t.loadingMore ? 'Reading…' : `Show more of ${(t.list?.matching ?? 0).toLocaleString()}`}</button>
              </div>
            {/if}
          {/if}
        </div>
      {:else if plain && editable}
        <textarea class="whole" spellcheck="false" aria-label=".mailmap as text" value={t.doc.text} oninput={(e) => t.edit(e.currentTarget.value)}></textarea>
      {:else}
        <div class="rules">
          {#if !lines.length}
            {#if !editable}
              <Empty note="nothing here">{t.source.exists ? 'This file is empty.' : "The setting points at something that isn't there."}</Empty>
            {:else if t.exists}
              <Empty note="a blank page">This file is empty.</Empty>
            {:else}
              <Empty note="not yet">There's no .mailmap, and there doesn't have to be. Add a line and you'll be asked before the file is created.</Empty>
            {/if}
          {/if}
          {#each lines as line, i (i)}
            {@const n = i + 1}
            {@const note = t.notes[i] && t.notes[i].line === n ? t.notes[i] : undefined}
            {@const said = note ? describeNote(note) : { text: '', tone: '' }}
            <div class="rule k-{note?.kind ?? 'map'} t-{said.tone}" data-line={n}>
              <span class="ln">{n}</span>
              {#if editing === n}
                <!-- svelte-ignore a11y_autofocus -->
                <input class="pat" bind:value={draft} aria-label="Line {n}" spellcheck="false" autofocus onkeydown={editKey} onblur={() => finishEdit(true)} />
              {:else if editable}
                <button class="pat" ondblclick={() => startEdit(n, line)} title="Double-click to edit">{line || ' '}</button>
              {:else}
                <span class="pat">{line || ' '}</span>
              {/if}
              <span class="what">{said.text}</span>
              {#if editable}
                <span class="acts">
                  <button class="btn sm quiet icon" aria-label="Edit line {n}" title="Edit" onclick={() => startEdit(n, line)}><Icon name="edit" size={14} /></button>
                  <button class="btn sm quiet icon del" aria-label="Delete line {n}" title="Delete" onclick={() => t.edit(deleteLine(t.doc?.text ?? '', n))}><Icon name="trash" size={14} /></button>
                </span>
              {/if}
            </div>
          {/each}
          {#if editable}
            <form class="addrow" onsubmit={add}>
              <span class="ln"><Icon name="plus" size={14} /></span>
              <input bind:value={adding} placeholder="Add a line, such as Proper Name <proper@email> <commit@email>" aria-label="New line" spellcheck="false" />
              <button class="btn sm" type="submit" disabled={!adding.trim()}>Add</button>
            </form>
          {:else}
            <p class="readonly">This file is read as it is and isn't edited here. To change it, edit it where your git config points.</p>
          {/if}
        </div>
      {/if}

      <footer class="foot">
        {#if t.dirty}
          <span><strong>{changed === 1 ? '1 line not saved' : `${changed} lines not saved`}</strong>{#if !t.exists}<span class="mute"> · nothing is written until you say so</span>{/if}</span>
        {:else if t.exists}
          <span class="mute">Saved changes show up as a change to .mailmap, ready to commit.</span>
        {:else}
          <span class="mute">Nothing is written here unless you save, and a .mailmap is only created when you say so.</span>
        {/if}
        <span class="spacer"></span>
        {#if t.view === 'text' && editable}
          <button class="btn sm quiet" aria-pressed={plain} onclick={() => (plain = !plain)} title={plain ? 'Show each line with what it does' : 'Edit the whole file as one box of text'}>{plain ? 'Show line notes' : 'Edit as plain text'}</button>
        {/if}
        <InkButton variant="secondary" disabled={!t.dirty || t.busy} onclick={() => t.revert()}>Revert</InkButton>
        <InkButton variant="primary" disabled={!t.dirty || t.busy} onclick={() => t.save()}>{t.exists ? 'Save .mailmap' : 'Create .mailmap…'}</InkButton>
      </footer>
    {/if}
  </main>

  <aside class="check" aria-label="Who is this?">
    <label class="form-field">
      <span>Who is this?</span>
      <input class="mono" bind:value={probe} placeholder="a name or an address, as a commit has it" spellcheck="false" />
    </label>
    {#if probe.trim()}
      <div class="answers" aria-live="polite">
        {#if probeError}
          <div class="answer"><span>{probeError}</span></div>
        {:else if !answers}
          <div class="answer"><span class="mute">Looking…</span></div>
        {:else if !answers.length}
          <div class="answer"><strong>Nobody here</strong><span>No commit has that name. Type an address to see how a mailmap would read it.</span></div>
        {:else}
          {#each answers as a (a.written.name + '\n' + a.written.email)}
            <div class="answer">
              <strong>{a.shown.name || '(no name)'}</strong>
              <span class="mail">{a.shown.email}</span>
              {#if a.lines.length}
                <span>by {linesText(a.lines)}{a.lines.every((l) => l.source === WORK_TREE) ? ' of ' : ''}{#if a.lines.every((l) => l.source === WORK_TREE)}<code>.mailmap</code>{/if}{a.known ? `, for ${wrote(a.commits, a.coAuthored)}` : ''}</span>
                <span class="mute">A commit says {a.written.name} &lt;{a.written.email}&gt;</span>
              {:else if a.known}
                <span>as the commits have it: no mailmap line applies. {wrote(a.commits, a.coAuthored)}.</span>
              {:else}
                <span>No mailmap line applies.</span>
              {/if}
              {#if !a.known}<span class="mute">No commit here uses this.</span>{/if}
            </div>
          {/each}
        {/if}
        {#if t.dirty}<span class="mute">This reads your edits, saved or not.</span>{/if}
      </div>
    {/if}
    <p class="aside">A commit always keeps what was written. A mailmap only changes what's shown.</p>
  </aside>
</div>

{#if joining}
  <SamePersonDialog who={joining} onClose={() => (joining = null)} />
{/if}

{#if t.confirmCreate}
  <Modal title="Create .mailmap?" eyebrow="a new file" width={520} onClose={() => (t.confirmCreate = false)}>
    <p>This repository has no mailmap yet. Saving creates one at the top of the repository:</p>
    <p><code class="where">{t.shared?.path ?? '.mailmap'}</code></p>
    <pre class="lines">{preview.slice(0, 8).join('\n')}{preview.length > 8 ? `\n… and ${preview.length - 8} more` : ''}</pre>
    <p>It shows up as a new file, ready for you to commit or throw away. No commit changes.</p>
    <div class="form-actions">
      <InkButton variant="secondary" onclick={() => (t.confirmCreate = false)}>Not now</InkButton>
      <InkButton variant="primary" disabled={t.busy} onclick={() => t.save(true)}>Create .mailmap</InkButton>
    </div>
  </Modal>
{/if}

{#if peopleTab.confirmClose}
  <Modal title="Discard unsaved changes to the mailmap?" onClose={() => (peopleTab.confirmClose = false)}>
    <p>Closing People discards your unsaved changes to <code>.mailmap</code>. Nothing has been written.</p>
    <div class="form-actions">
      <InkButton variant="secondary" onclick={() => (peopleTab.confirmClose = false)}>Keep editing</InkButton>
      <InkButton variant="danger" onclick={() => peopleTab.close()}>Discard and close</InkButton>
    </div>
  </Modal>
{/if}

<style>
  .view {
    display: flex;
    height: 100%;
    min-height: 0;
  }

  .side {
    width: 260px;
    flex: none;
    display: flex;
    flex-direction: column;
    padding: 8px 0;
    background: var(--paper-warm);
    border-right: 1px solid var(--paper-edge);
    overflow-y: auto;
  }

  .repo {
    padding: 4px 12px 8px;
  }

  .repo select {
    width: 100%;
  }

  .eb {
    font-size: 20px;
    color: var(--accent-a);
    padding: 10px 14px 2px;
    transform: rotate(-1.5deg);
    transform-origin: left;
  }

  .src {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 32px;
    padding: 4px 14px;
    border: 0;
    border-radius: 0;
    background: transparent;
    font-size: 13px;
    text-align: left;
    width: 100%;
  }

  .src:hover:not(:disabled) {
    background: var(--wash-hover);
  }

  .src.on {
    background: var(--selected);
  }

  .src .name {
    font-family: var(--font-mono);
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .src .n {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .dot {
    color: var(--link);
    font-size: 12px;
  }

  .path {
    padding: 0 14px;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    overflow-wrap: anywhere;
  }

  .spacer {
    flex: 1;
  }

  .aside {
    margin: 12px 14px;
    font-size: 12px;
    line-height: 1.45;
    color: var(--ink-mute);
  }

  .check .aside {
    margin: 0;
  }

  .main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .head {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 20px;
    border-bottom: 1px solid var(--paper-edge);
  }

  .head h1 {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 16px;
    font-weight: 500;
  }

  .note,
  .mute {
    font-size: 12px;
    color: var(--ink-mute);
  }

  .seg {
    display: inline-flex;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    overflow: hidden;
  }

  .seg button {
    height: 26px;
    padding: 0 12px;
    border: 0;
    border-radius: 0;
    background: transparent;
    font-size: 13px;
    color: var(--ink-soft);
  }

  .seg button[aria-pressed='true'] {
    background: var(--selected);
    color: var(--ink);
  }

  .notice {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 12px 20px 0;
    padding: 10px 12px;
    border-radius: var(--r-md);
    background: var(--danger-wash);
    font-size: 13px;
  }

  .people {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0 20px 16px;
  }

  /* No mailmap is the usual case, so this reads as a note, not a warning. */
  .none {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 14px 0 4px;
  }

  .none .hand {
    align-self: flex-start;
    font-size: 20px;
    color: var(--accent-a);
    transform: rotate(-1.5deg);
  }

  .none p {
    margin: 0;
    max-width: 68ch;
    font-size: 14px;
    line-height: 1.5;
    color: var(--ink-soft);
  }

  .bar {
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 14px;
    padding: 12px 0 10px;
    background: var(--paper);
    border-bottom: 1px solid var(--paper-edge);
  }

  .find {
    flex: 1 1 260px;
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    padding: 0 10px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    color: var(--ink-mute);
  }

  .find:focus-within {
    outline: 2px solid var(--link);
    outline-offset: 2px;
  }

  .find input {
    flex: 1;
    min-width: 0;
    height: 30px;
    border: 0;
    outline: 0;
    background: transparent;
    font-size: 14px;
    color: var(--ink);
  }

  .sum {
    font-size: 13px;
    color: var(--ink-mute);
  }

  .list {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .person {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 9px 0;
    border-bottom: 1px dashed var(--hairline);
  }

  .face {
    flex: none;
    width: 30px;
    height: 30px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 2px solid var(--c);
    border-radius: 50%;
    background: var(--paper-warm);
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .who {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .id,
  .also {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 2px 8px;
  }

  .pname {
    font-weight: 500;
  }

  .mail {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    overflow-wrap: anywhere;
  }

  .tag {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .also {
    font-size: 14px;
    color: var(--ink-soft);
  }

  .also .lead {
    font-style: italic;
    color: var(--ink-mute);
  }

  .also .why {
    font-size: 13px;
    color: var(--ink-mute);
  }

  .person .n {
    flex: none;
    padding-top: 3px;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-soft);
    white-space: nowrap;
  }

  /* Who looks like the same person. Offers sit above the list; what isn't offered folds away. */
  .suggest {
    display: flex;
    flex-direction: column;
    padding: 12px 0 4px;
  }

  .shead {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 4px 10px;
    padding-bottom: 6px;
  }

  .shead h2 {
    margin: 0;
    font-size: 17px;
    font-weight: 500;
  }

  .count {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--link);
  }

  .fold .count {
    color: var(--ink-mute);
  }

  .offer {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 16px;
    padding: 10px 0;
    border-top: 1px dashed var(--hairline);
  }

  .about {
    flex: 1 1 320px;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .label {
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .line .why {
    font-size: 13px;
    color: var(--ink-mute);
  }

  .answer-to {
    display: flex;
    gap: 8px;
  }

  .rest {
    margin: 0;
    padding: 8px 0;
    border-top: 1px dashed var(--hairline);
    font-size: 13px;
    font-style: italic;
    color: var(--ink-mute);
  }

  .fold {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 4px 8px;
    width: 100%;
    padding: 9px 0;
    border: 0;
    border-top: 1px dashed var(--hairline);
    border-radius: 0;
    background: transparent;
    font-size: 14px;
    color: var(--ink-soft);
    text-align: left;
  }

  /* The chevron icon points down: turned to the side while its section is shut. */
  .turn {
    display: inline-flex;
    transform: rotate(-90deg);
  }

  .turn.open {
    transform: none;
  }

  .picking {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 12px;
    margin-top: 10px;
    padding: 9px 12px;
    border-radius: var(--r-md);
    background: var(--selected);
    font-size: 14px;
  }

  .picking span {
    flex: 1 1 260px;
  }

  .person.first {
    background: var(--selected);
  }

  .join {
    flex: none;
    display: flex;
    min-width: 132px;
    justify-content: flex-end;
  }

  /* Quiet until the row is pointed at or tabbed into; the "This one" buttons always show. */
  .join .quiet {
    opacity: 0;
  }

  .person:hover .join .quiet,
  .person:focus-within .join .quiet {
    opacity: 1;
  }

  .more {
    display: flex;
    justify-content: center;
    padding: 12px 0 4px;
  }

  .rules {
    flex: 1;
    overflow-y: auto;
    padding: 4px 20px 16px;
  }

  .rule,
  .addrow {
    display: grid;
    grid-template-columns: 36px minmax(0, 1fr) auto auto;
    align-items: center;
    gap: 8px;
    min-height: 32px;
    border-bottom: 1px dashed var(--hairline);
  }

  .ln {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-faint);
    text-align: right;
    padding-right: 4px;
    display: flex;
    justify-content: flex-end;
  }

  .pat {
    font-family: var(--font-mono);
    font-size: 13px;
    text-align: left;
    border: 0;
    background: transparent;
    padding: 4px 6px;
    white-space: pre;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }

  input.pat {
    border: 1px solid var(--accent);
    border-radius: var(--r-sm);
    background: var(--paper);
  }

  .k-comment .pat {
    font-style: italic;
    color: var(--ink-mute);
  }

  .what {
    font-size: 12px;
    color: var(--ink-soft);
    white-space: nowrap;
  }

  .t-idle .what {
    color: var(--ink-faint);
    font-style: italic;
  }

  /* A line git reads past: said in words, and marked, never by colour alone. */
  .t-bad {
    background: var(--danger-wash);
  }

  .t-bad .what {
    color: var(--danger);
  }

  .acts {
    display: flex;
    gap: 2px;
    opacity: 0;
  }

  .rule:hover .acts,
  .rule:focus-within .acts {
    opacity: 1;
  }

  .acts .del {
    color: var(--danger);
  }

  .addrow {
    grid-template-columns: 36px minmax(0, 1fr) auto;
    color: var(--link);
    border-bottom: 0;
    margin-top: 6px;
  }

  .addrow input {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .readonly {
    margin: 14px 0 0;
    font-size: 13px;
    color: var(--ink-mute);
  }

  /* Not named "text": the app's text buttons carry that class, and would take this on. */
  .whole {
    flex: 1;
    margin: 12px 20px;
    resize: none;
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.6;
    padding: 10px 12px;
    border-radius: var(--r-md);
  }

  .foot {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 20px;
    border-top: 1px solid var(--paper-edge);
    background: var(--paper-warm);
    font-size: 13px;
  }

  .check {
    width: 320px;
    flex: none;
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 16px;
    border-left: 1px solid var(--paper-edge);
    overflow-y: auto;
  }

  .check .mono {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .answers {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .answer {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px;
    border-radius: var(--r-md);
    background: var(--paper-warm);
    font-size: 13px;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  code {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-b);
    overflow-wrap: anywhere;
  }

  .where {
    color: var(--ink-soft);
  }

  .lines {
    margin: 0;
    padding: 12px 14px;
    border-radius: var(--r-sm);
    background: var(--code-bg);
    color: var(--code-text);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    max-height: 200px;
    overflow-y: auto;
  }

  @media (max-width: 1000px) {
    .check {
      display: none;
    }
  }
</style>
