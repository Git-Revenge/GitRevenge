<script lang="ts">
  import { tick } from 'svelte'
  import type { git } from '../../../wailsjs/go/models'
  import { provideCommands } from '../commands.svelte'
  import { errorText } from '../errors'
  import { session } from '../session.svelte'
  import Icon from '../ui/Icon.svelte'
  import InkButton from '../ui/InkButton.svelte'
  import { ignoreRules as t } from './ignoreRules.svelte'
  import { appendLine, deleteLine, describe, moveLine, parseLines, setLine, statsFor } from './rules'

  // Re-read when the repository changes on disk, such as a .gitignore edited elsewhere.
  $effect(() => {
    void session.revision[t.repo]
    void t.load()
  })

  let lines = $derived(parseLines(t.doc?.text ?? ''))
  let stats = $derived(statsFor(t.stats, t.source?.source ?? ''))
  let isDirty = $derived(!!t.doc && t.doc.text !== t.doc.base)
  let changed = $derived.by(() => {
    if (!t.doc || !isDirty) return 0
    const a = t.doc.base.split('\n')
    const b = t.doc.text.split('\n')
    let n = Math.abs(a.length - b.length)
    for (let i = 0; i < Math.min(a.length, b.length); i++) if (a[i] !== b[i]) n++
    return n
  })

  /** The line picked in the list, whose matches show on the right. */
  let picked = $state(0)
  let editing = $state(0)
  let draft = $state('')
  let adding = $state('')
  let list = $state<HTMLElement>()

  $effect(() => {
    const n = t.focusLine
    if (!n) return
    picked = n
    t.focusLine = 0
    void tick().then(() => list?.querySelector(`[data-line="${n}"]`)?.scrollIntoView({ block: 'center' }))
  })

  function startEdit(n: number, text: string) {
    editing = n
    draft = text
    picked = n
  }

  function finishEdit(keep: boolean) {
    if (keep && editing && draft.trim() !== '') t.edit(setLine(t.doc?.text ?? '', editing, draft))
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
    picked = lines.length + 1
    adding = ''
  }

  // A folder's own .gitignore, created on first save.
  let newFolder = $state<string | null>(null)
  function createIn(e: SubmitEvent) {
    e.preventDefault()
    const dir = (newFolder ?? '').trim().replace(/\\/g, '/').replace(/^\/+|\/+$/g, '')
    newFolder = null
    if (!dir) return
    const source = `${dir}/.gitignore`
    if (!t.sources.some((s) => s.source === source)) {
      const at = t.sources.findIndex((s) => s.scope === 'exclude')
      const entry = { source, path: '', scope: 'dir', dir, exists: false, rules: 0 } as git.IgnoreSource
      t.sources = [...t.sources.slice(0, at), entry, ...t.sources.slice(at)]
      t.docs[source] = { base: '', text: '' }
    }
    void t.select(source)
  }

  // Checking a path against the saved rules.
  let probe = $state('')
  let answer = $state<git.IgnoreCheck | null>(null)
  let probeError = $state('')
  let asked = 0
  $effect(() => {
    const p = probe.trim()
    void t.stats
    const n = ++asked
    if (!p) {
      answer = null
      return
    }
    const timer = setTimeout(async () => {
      try {
        const res = await t.check(p)
        if (n === asked) [answer, probeError] = [res, '']
      } catch (e) {
        if (n === asked) [answer, probeError] = [null, errorText(e)]
      }
    }, 200)
    return () => clearTimeout(timer)
  })

  const sourceLabel = (s: git.IgnoreSource) => (s.scope === 'global' ? 'Global excludes' : s.scope === 'exclude' ? '.git/info/exclude' : s.source)
  const scopeNote: Record<string, string> = {
    root: 'shared · committed with the repository',
    dir: 'shared · applies inside its folder',
    exclude: 'this clone only · never committed',
    global: 'every repository on this computer',
  }
  let inRepo = $derived(t.sources.filter((s) => s.scope === 'root' || s.scope === 'dir'))
  let exclude = $derived(t.sources.find((s) => s.scope === 'exclude'))
  let global = $derived(t.sources.find((s) => s.scope === 'global'))
  let pickedStats = $derived(picked ? stats.get(picked) : undefined)

  const ruleName = (r: git.IgnoreLine) => {
    const s = t.sources.find((x) => x.source === r.source)
    return s ? sourceLabel(s) : r.source
  }

  provideCommands(() => [
    { id: 'ignore.save', group: 'Ignore rules', title: 'Save this ignore file', shortcut: 'Mod+S', whenTyping: true, run: () => void (isDirty && t.save()) },
    { id: 'ignore.view', group: 'Ignore rules', title: t.view === 'rules' ? 'Edit as text' : 'Edit as rules', run: () => void (t.view = t.view === 'rules' ? 'text' : 'rules') },
  ])
</script>

{#snippet sourceButton(s: git.IgnoreSource, label: string)}
  <button class="src" class:on={t.source?.source === s.source} aria-current={t.source?.source === s.source ? 'true' : undefined} onclick={() => t.select(s.source)} title={s.path}>
    <span class="name">{label}</span>
    {#if t.dirty.includes(s.source)}<span class="dot" title="Unsaved edits">●</span>{/if}
    <span class="n">{s.exists ? s.rules : 'new'}</span>
  </button>
{/snippet}

<div class="view">
  <nav class="side" aria-label="Ignore files">
    {#if session.tabs.length > 1}
      <label class="repo">
        <span class="visually-hidden">Repository</span>
        <select value={t.repo} disabled={t.dirty.length > 0} title={t.dirty.length ? 'Save or revert your edits to switch repository' : 'Repository'} onchange={(e) => void (t.setRepo(e.currentTarget.value), t.load())}>
          {#each session.tabs as tab (tab.path)}<option value={tab.path}>{tab.name}</option>{/each}
        </select>
      </label>
    {/if}
    <span class="eb hand">this repository</span>
    {#each inRepo as s (s.source)}{@render sourceButton(s, s.source)}{/each}
    {#if newFolder === null}
      <button class="src add" onclick={() => (newFolder = '')}><Icon name="plus" size={14} />New .gitignore in a folder…</button>
    {:else}
      <form class="newdir" onsubmit={createIn}>
        <!-- svelte-ignore a11y_autofocus -->
        <input bind:value={newFolder} placeholder="folder/path" aria-label="Folder for the new .gitignore" autofocus onkeydown={(e) => e.key === 'Escape' && (newFolder = null)} />
        <button class="btn sm" type="submit">Add</button>
      </form>
    {/if}
    {#if exclude}
      <span class="eb hand">only this clone</span>
      {@render sourceButton(exclude, '.git/info/exclude')}
    {/if}
    {#if global}
      <span class="eb hand">every repository</span>
      {@render sourceButton(global, 'Global excludes')}
      <span class="path" title={global.path}>{global.path}</span>
    {/if}
    <span class="spacer"></span>
    <p class="aside">Later lines win, and a folder's own .gitignore wins over the one above it for files inside.</p>
  </nav>

  <main class="main">
    {#if !t.repo}
      <p class="empty">Open a repository to see its ignore rules.</p>
    {:else if t.source}
      <header class="head">
        <h1>{sourceLabel(t.source)}</h1>
        <span class="note">{scopeNote[t.source.scope] ?? ''}</span>
        <span class="spacer"></span>
        <div class="seg" role="group" aria-label="Edit as">
          <button aria-pressed={t.view === 'rules'} onclick={() => (t.view = 'rules')}>Rules</button>
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
        <p class="empty">Reading…</p>
      {:else if t.view === 'text'}
        <textarea class="text" spellcheck="false" aria-label="{sourceLabel(t.source)} as text" value={t.doc.text} oninput={(e) => t.edit(e.currentTarget.value)}></textarea>
      {:else}
        <div class="rules" bind:this={list}>
          {#if !lines.length}
            <p class="empty">{t.source.exists ? 'This file is empty.' : "This file doesn't exist yet. Add a rule and save to create it."}</p>
          {/if}
          {#each lines as l (l.n)}
            {@const s = stats.get(l.n)}
            {@const what = isDirty ? '' : describe(l, s, t.stats?.truncated ?? false)}
            <div class="rule k-{l.kind}" class:picked={picked === l.n} class:warn={!!s?.tracked && l.kind === 'rule'} data-line={l.n}>
              <span class="ln">{l.n}</span>
              {#if editing === l.n}
                <!-- svelte-ignore a11y_autofocus -->
                <input class="pat" bind:value={draft} aria-label="Line {l.n}" spellcheck="false" autofocus onkeydown={editKey} onblur={() => finishEdit(true)} />
              {:else}
                <button class="pat" onclick={() => (picked = l.n)} ondblclick={() => startEdit(l.n, l.text)} title="Double-click to edit">{l.text || ' '}</button>
              {/if}
              <span class="what" class:none={what.includes('nothing')}>{what}</span>
              <span class="acts">
                <button class="btn sm quiet icon" aria-label="Edit line {l.n}" title="Edit" onclick={() => startEdit(l.n, l.text)}><Icon name="edit" size={14} /></button>
                <button class="btn sm quiet icon" aria-label="Move line {l.n} up" title="Move up" disabled={l.n === 1} onclick={() => t.edit(moveLine(t.doc?.text ?? '', l.n, -1))}><span class="turn up"><Icon name="chevron" size={14} /></span></button>
                <button class="btn sm quiet icon" aria-label="Move line {l.n} down" title="Move down" disabled={l.n === lines.length} onclick={() => t.edit(moveLine(t.doc?.text ?? '', l.n, 1))}><span class="turn down"><Icon name="chevron" size={14} /></span></button>
                <button class="btn sm quiet icon del" aria-label="Delete line {l.n}" title="Delete" onclick={() => t.edit(deleteLine(t.doc?.text ?? '', l.n))}><Icon name="trash" size={14} /></button>
              </span>
            </div>
          {/each}
          <form class="addrow" onsubmit={add}>
            <span class="ln"><Icon name="plus" size={14} /></span>
            <input bind:value={adding} placeholder="Add a rule, such as *.log or /build/" aria-label="New rule" spellcheck="false" />
            <button class="btn sm" type="submit" disabled={!adding.trim()}>Add</button>
          </form>
        </div>
      {/if}
      <footer class="foot">
        {#if isDirty}
          <span><strong>{changed === 1 ? '1 line changed' : `${changed} lines changed`}</strong> <span class="mute">· not saved</span></span>
        {:else if t.source.scope === 'root' || t.source.scope === 'dir'}
          <span class="mute">Saved changes show up as a change to {t.source.source}, ready to commit.</span>
        {/if}
        <span class="spacer"></span>
        <InkButton variant="secondary" disabled={!isDirty || t.busy} onclick={() => t.revert()}>Revert</InkButton>
        <InkButton variant="primary" disabled={!isDirty || t.busy} onclick={() => t.save()}>Save {t.source.scope === 'global' ? 'global excludes' : sourceLabel(t.source)}</InkButton>
      </footer>
    {/if}
  </main>

  <aside class="check" aria-label="Check a path">
    <label class="form-field">
      <span>Is this path ignored?</span>
      <input class="mono" bind:value={probe} placeholder="build/cache/index.bin" spellcheck="false" />
    </label>
    {#if probe.trim()}
      <div class="answer" aria-live="polite">
        {#if probeError}
          <span>{probeError}</span>
        {:else if !answer}
          <span class="mute">Checking…</span>
        {:else if answer.matched && answer.ignored}
          <strong>Yes, ignored</strong>
          <span>by <code>{answer.rule.pattern}</code>, line {answer.rule.line} of {ruleName(answer.rule)}</span>
          {#if answer.tracked}<span class="mute">But git tracks it, so it still shows changes.</span>{/if}
          {#if t.sources.some((s) => s.source === answer?.rule.source)}
            <button class="btn sm text" onclick={() => answer && t.show(t.repo, answer.rule.source, answer.rule.line)}>Show that line</button>
          {/if}
        {:else if answer.matched}
          <strong>No, brought back</strong>
          <span>by <code>{answer.rule.pattern}</code>, line {answer.rule.line} of {ruleName(answer.rule)}</span>
        {:else}
          <strong>No</strong>
          <span>No rule matches it.</span>
        {/if}
        {#if isDirty}<span class="mute">This checks the saved rules. Save to include your edits.</span>{/if}
      </div>
    {/if}
    {#if pickedStats && !isDirty}
      <div class="hits">
        <span class="label">{pickedStats.restores ? 'Brought back' : 'Ignored'} by line {picked}</span>
        {#each pickedStats.paths as p (p)}<code>{p}</code>{/each}
        {#if pickedStats.paths.length < pickedStats.ignores + pickedStats.restores + pickedStats.tracked}<span class="mute">and more</span>{/if}
      </div>
    {/if}
  </aside>
</div>

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

  .src.add {
    color: var(--link);
    font-family: inherit;
  }

  .dot {
    color: var(--link);
    font-size: 12px;
  }

  .newdir {
    display: flex;
    gap: 6px;
    padding: 4px 14px;
  }

  .newdir input {
    flex: 1;
    min-width: 0;
    font-family: var(--font-mono);
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

  .empty {
    margin: 20px;
    color: var(--ink-mute);
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

  .rule.picked {
    background: var(--selected);
  }

  .rule.warn {
    background: var(--warning-wash);
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

  .k-negation .pat {
    color: var(--success);
  }

  .what {
    font-size: 12px;
    color: var(--ink-mute);
    white-space: nowrap;
  }

  .what.none {
    color: var(--ink-faint);
    font-style: italic;
  }

  .warn .what {
    color: var(--warning);
  }

  .acts {
    display: flex;
    gap: 2px;
    opacity: 0;
  }

  .rule:hover .acts,
  .rule:focus-within .acts,
  .rule.picked .acts {
    opacity: 1;
  }

  .turn {
    display: inline-flex;
  }

  /* The chevron icon points down. */
  .turn.up {
    transform: rotate(180deg);
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

  .text {
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

  .answer,
  .hits {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px;
    border-radius: var(--r-md);
    background: var(--paper-warm);
    font-size: 13px;
    line-height: 1.45;
  }

  .hits .label {
    font-weight: 600;
    color: var(--ink-soft);
  }

  code {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-b);
    overflow-wrap: anywhere;
  }

  .hits code {
    color: var(--ink-soft);
  }

  @media (max-width: 1000px) {
    .check {
      display: none;
    }
  }
</style>
