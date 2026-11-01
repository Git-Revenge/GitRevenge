<script lang="ts">
  // The Settings view of the Git Config tab (#225): the selected file's keys by section,
  // edited in place, with what other config files set for the same keys.
  import { tick } from 'svelte'
  import { AddGitConfigValue, GitConfigKeys, ReplaceGitConfigValue, SetGitConfig, UnsetGitConfig } from '../../../wailsjs/go/app/App.js'
  import { commands } from '../commands.svelte'
  import { session } from '../session.svelte'
  import Icon from '../ui/Icon.svelte'
  import InkButton from '../ui/InkButton.svelte'
  import { CATALOG, canonicalKey, keyInfo, splitKey } from './catalog'
  import { gitConfig } from './config.svelte'
  import { buildRows, filterSections, hasSecret, maskUrls, maskValue, samePath, type Inherited, type Row } from './rows'
  import ValueEditor from './ValueEditor.svelte'

  let filter = $state('')
  let showInherited = $state(true)
  let collapsed = $state<Record<string, boolean>>({})
  /** Rows whose secrets are shown, by canonical key. Forgotten when the view unmounts. */
  let revealed = $state<Record<string, boolean>>({})
  /** The row taking a new value, by canonical key. */
  let adding = $state('')
  let newValue = $state('')
  let addOpen = $state(false)
  let newKey = $state('')
  let newKeyValue = $state('')
  let known = $state<string[]>([])
  let tableEl = $state<HTMLDivElement>()

  let scope = $derived(gitConfig.scope)
  let doc = $derived(gitConfig.doc)
  let sections = $derived(doc ? buildRows(doc.entries, gitConfig.effective, doc.path) : [])
  let shown = $derived(filterSections(sections, filter, showInherited))
  let suggestions = $derived([...new Set([...CATALOG.map((c) => c.key), ...known])])
  let newInfo = $derived(keyInfo(newKey))

  /** The file's values as saved on disk, by canonical key, to mark what the edits changed.
   * Empty when git doesn't read the file from this scope's repository (nothing to compare). */
  let saved = $derived.by(() => {
    const m: Record<string, string[]> = {}
    if (!doc) return m
    for (const e of gitConfig.effective) {
      if (!samePath(e.file, doc.path)) continue
      const k = canonicalKey(e.key)
      m[k] = [...(m[k] ?? []), e.implicit ? 'true' : e.value]
    }
    return m
  })
  let comparable = $derived(!!doc?.exists && (Object.keys(saved).length > 0 || doc.entries.length === 0))

  $effect(() => {
    GitConfigKeys()
      .then((k) => (known = k ?? []))
      // Only used for suggestions.
      .catch(() => {})
  })

  // Point at a key another part of the app sent us to: scroll to its row and flash it, or
  // start adding it when nothing sets it.
  $effect(() => {
    const key = gitConfig.focusKey
    if (!key || !doc || !tableEl) return
    filter = ''
    showInherited = true
    const id = canonicalKey(key)
    void tick().then(() => {
      gitConfig.focusKey = ''
      const row = tableEl?.querySelector<HTMLElement>(`[data-key="${CSS.escape(id)}"]`)
      if (!row) {
        addOpen = true
        newKey = key
        return
      }
      row.scrollIntoView({ block: 'center' })
      row.classList.add('flash')
      row.querySelector<HTMLElement>('input, select, .segmented button, button')?.focus({ preventScroll: true })
      setTimeout(() => row.classList.remove('flash'), 1600)
    })
  })

  const edit = (fn: (path: string) => Promise<unknown>) => gitConfig.apply(fn as never)

  function setValue(r: Row, old: string, value: string) {
    if (r.multi) return edit((p) => ReplaceGitConfigValue(p, r.key, old, value))
    return edit((p) => SetGitConfig(p, r.key, value))
  }

  function remove(r: Row, value = '') {
    return edit((p) => UnsetGitConfig(p, r.key, value))
  }

  /** Copies the inherited value into this file, where it can be changed. */
  function override(r: Row) {
    const last = r.inherited[r.inherited.length - 1]
    const value = last.implicit ? 'true' : last.value
    return edit((p) => (r.multi ? AddGitConfigValue(p, r.key, value) : SetGitConfig(p, r.key, value)))
  }

  async function addValue(r: Row) {
    if (!newValue) return
    if (await edit((p) => AddGitConfigValue(p, r.key, newValue))) {
      adding = ''
      newValue = ''
    }
  }

  async function addKey(e: SubmitEvent) {
    e.preventDefault()
    const key = newKey.trim()
    if (!key) return
    if (key.includes('<')) {
      gitConfig.error = `Replace the placeholder in ${key} with a name, such as remote.origin.url.`
      return
    }
    const exists = sections.some((s) => s.rows.some((r) => r.values.length > 0 && canonicalKey(r.key) === canonicalKey(key)))
    const multi = keyInfo(key)?.multi
    if (await edit((p) => (exists && multi ? AddGitConfigValue(p, key, newKeyValue) : SetGitConfig(p, key, newKeyValue)))) {
      newKey = ''
      newKeyValue = ''
      addOpen = false
    }
  }

  /** A file that inherited values come from and that isn't already in the rail. */
  function editableSource(i: Inherited): boolean {
    return i.scope !== 'system' && i.file !== '' && !gitConfig.scopes.some((s) => samePath(s.path, i.file))
  }

  /** The tag for where an inherited value comes from: its scope, or "included" for a file
   * another one reads in. */
  function source(i: Inherited): { text: string; tone: string } {
    if (i.scope === 'system') return { text: 'system', tone: 'sepia' }
    if (editableSource(i)) return { text: 'included', tone: 'plum' }
    return { text: i.scope, tone: i.scope === 'local' ? 'plum' : '' }
  }

  /** What the edits did to a row since the file was opened or saved: '' for nothing. */
  function change(r: Row): string {
    if (!comparable || r.values.length === 0) return ''
    const was = saved[canonicalKey(r.key)]
    if (!was) return 'new'
    const now = r.values.map((v) => (v.implicit ? 'true' : v.value))
    if (was.length === now.length && was.every((v, i) => v === now[i])) return ''
    return !r.multi && was.length === 1 ? `changed from ${maskValue(r.key, was[0]) || '(empty)'}` : 'changed'
  }

  async function manageRemotes(repo: string) {
    await session.select(repo)
    await tick()
    commands.all().find((c) => c.id === 'repo.remotes')?.run()
  }

  const shownValue = (r: Row, v: string) => (revealed[canonicalKey(r.key)] ? v : maskValue(r.key, v))
  const shownKey = (r: Row) => (revealed[canonicalKey(r.key)] ? r.key : maskUrls(r.key))
  const secret = (r: Row) => [...r.values.map((v) => v.value), ...r.inherited.map((i) => i.value)].some((v) => hasSecret(r.key, v)) || hasSecret(r.key, '')
  const name = (key: string) => splitKey(key)[2]
  const last = (r: Row) => r.inherited[r.inherited.length - 1]
</script>

<div class="table" bind:this={tableEl}>
  <div class="toolbar">
    <label class="filter">
      <Icon name="search" size={15} />
      <input type="search" placeholder="Filter keys and values" aria-label="Filter keys and values" bind:value={filter} spellcheck="false" />
    </label>
    <span class="segmented" role="group" aria-label="Which keys to show">
      <button aria-pressed={showInherited} onclick={() => (showInherited = true)}>everything git reads</button>
      <button aria-pressed={!showInherited} onclick={() => (showInherited = false)}>only this file</button>
    </span>
    <span class="spacer"></span>
    <button class="add-toggle" aria-expanded={addOpen} onclick={() => (addOpen = !addOpen)}><span class="plus" aria-hidden="true">+</span> Add a setting</button>
  </div>

  {#if addOpen}
    <form class="add" onsubmit={addKey}>
      <label class="form-field">
        <span>Key</span>
        <input class="field" list="gitconfig-keys" placeholder="e.g. pull.rebase" bind:value={newKey} spellcheck="false" autocomplete="off" />
      </label>
      <datalist id="gitconfig-keys">
        {#each suggestions as s (s)}<option value={s}>{keyInfo(s)?.description ?? ''}</option>{/each}
      </datalist>
      <label class="form-field">
        <span>Value</span>
        {#if newInfo?.values || newInfo?.type === 'bool'}
          <select class="field" bind:value={newKeyValue}>
            <option value="" disabled>Pick one</option>
            {#each newInfo.values ?? ['true', 'false'] as v (v)}<option value={v}>{v}</option>{/each}
          </select>
        {:else}
          <input class="field" bind:value={newKeyValue} spellcheck="false" autocomplete="off" />
        {/if}
      </label>
      <div class="add-actions">
        <button class="btn text" type="button" onclick={() => (addOpen = false)}>Cancel</button>
        <InkButton type="submit" disabled={!newKey.trim() || gitConfig.busy}>Add</InkButton>
      </div>
      {#if newInfo}<p class="hint">{newInfo.description}</p>{/if}
    </form>
  {/if}

  {#each shown as s (s.name)}
    {@const open = !collapsed[s.name]}
    {@const own = s.rows.filter((r) => r.values.length > 0).length}
    <section class="section">
      <h2>
        <button class="toggle" aria-expanded={open} onclick={() => (collapsed[s.name] = open)}>
          <Icon name={open ? 'chevron' : 'right'} size={14} />
          <code class:quiet={own === 0}>[{maskUrls(s.name)}]</code>
        </button>
        <span class="meta">{own === 0 ? 'inherited' : own === 1 ? '1 key' : `${own} keys`}</span>
        <span class="spacer"></span>
        {#if scope?.kind === 'local' && s.name.startsWith('remote "')}
          <button class="btn text" onclick={() => manageRemotes(scope.repo)}>also in Remotes →</button>
        {:else if scope?.kind === 'local' && s.name.startsWith('branch "')}
          <span class="sub">Upstreams are also set from the branch menu.</span>
        {/if}
      </h2>
      {#if open}
        {#each s.rows as r (canonicalKey(r.key))}
          {@const info = keyInfo(r.key)}
          {@const id = canonicalKey(r.key)}
          {@const own = r.values.length > 0}
          {@const hidden = r.values.some((v) => maskValue(r.key, v.value) !== v.value) && !revealed[id]}
          {@const changed = change(r)}
          <div class="row" class:inherited={!own} class:changed data-key={id}>
            <div class="key">
              <code>{name(r.key)}</code>
              {#if r.key.split('.').length > 2}<span class="full">{shownKey(r)}</span>{/if}
              {#if info}<span class="sub">{info.description}</span>{/if}
            </div>
            <div class="values">
              {#if own}
                {#each r.values as v, i (i)}
                  <div class="value">
                    {#if hidden}
                      <span class="masked" aria-label="hidden value">••••••••</span>
                      <span class="hand secret">a secret, kept hidden</span>
                    {:else}
                      <ValueEditor label={r.key} value={v.value} implicit={v.implicit} {info} disabled={gitConfig.busy} onCommit={(nv) => setValue(r, v.value, nv)} />
                    {/if}
                    {#if r.multi}
                      <button class="round" title="Remove this value" aria-label="Remove {shownValue(r, v.value)} from {r.key}" onclick={() => remove(r, v.value)}><Icon name="minus" size={14} /></button>
                    {/if}
                  </div>
                {/each}
                {#if r.multi}
                  {#if adding === id}
                    <form class="value" onsubmit={(e) => (e.preventDefault(), addValue(r))}>
                      <input class="field grow" aria-label="Another value for {r.key}" bind:value={newValue} spellcheck="false" autocomplete="off" />
                      <button class="btn text" type="button" onclick={() => (adding = '')}>Cancel</button>
                      <button class="btn text" type="submit" disabled={!newValue}>Add</button>
                    </form>
                  {:else}
                    <button class="btn text start" onclick={() => ((adding = id), (newValue = ''))}>+ add a value</button>
                  {/if}
                {/if}
                {#if changed || r.inherited.length > 0}
                  <p class="sub note">
                    {#if changed}<span>{changed}</span>{/if}
                    {#if changed && r.inherited.length > 0}<span aria-hidden="true">·</span>{/if}
                    {#if r.inherited.length > 0 && r.multi}
                      <span>adds to</span>
                      {#each [...new Map(r.inherited.map((i) => [source(i).text, source(i)])).values()] as t (t.text)}<span class="tag {t.tone}">{t.text}</span>{/each}
                    {:else if r.inherited.length > 0}
                      <span>overrides</span>
                      <span class="tag {source(last(r)).tone}" title={last(r).file}>{source(last(r)).text}</span>
                      <code>{last(r).implicit ? 'true' : shownValue(r, last(r).value)}</code>
                    {/if}
                  </p>
                {/if}
              {:else}
                {#each r.inherited as i, n (n)}
                  <div class="value from">
                    <code>{i.implicit ? 'true' : shownValue(r, i.value)}</code>
                    <span class="tag {source(i).tone}" title={i.file || 'set on the command line'}>{source(i).text}</span>
                    {#if editableSource(i)}
                      <button class="btn text" title={i.file} onclick={() => gitConfig.showFile(i.file, scope?.repo ?? '')}>edit that file</button>
                    {/if}
                  </div>
                {/each}
              {/if}
            </div>
            <div class="actions">
              {#if secret(r)}
                <button class="btn text" aria-pressed={!!revealed[id]} aria-label="{revealed[id] ? 'Hide' : 'Show'} the value of {r.key}" onclick={() => (revealed[id] = !revealed[id])}>
                  {revealed[id] ? 'Hide' : 'Show'}
                </button>
              {/if}
              {#if own}
                <button class="btn text" onclick={() => remove(r)} title={r.inherited.length ? 'Remove from this file, so the inherited value applies again' : 'Remove from this file'}>
                  {r.inherited.length ? `Use ${source(last(r)).text}` : r.multi ? 'Remove all' : 'Remove'}
                </button>
              {:else}
                <button class="btn text" onclick={() => override(r)}>{r.multi ? 'Add here' : 'Override here'}</button>
              {/if}
            </div>
          </div>
        {/each}
      {/if}
    </section>
  {:else}
    <p class="empty">
      {#if filter}Nothing matches “{filter}”.{:else}<span class="hand">a blank page</span> This file has no settings yet. Add one above.{/if}
    </p>
  {/each}
</div>

<style>
  .table {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px 16px;
  }

  .spacer {
    flex: 1;
  }

  .filter {
    flex: 1 1 260px;
    max-width: 400px;
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 38px;
    padding: 0 14px;
    border: 1px solid var(--paper-edge);
    border-radius: 999px;
    background: var(--paper);
    color: var(--ink-mute);
  }

  .filter:focus-within {
    border-color: var(--link);
    box-shadow: 0 0 0 3px var(--selection);
  }

  .filter input {
    flex: 1;
    min-width: 0;
    border: none;
    background: none;
    font-size: 15px;
    outline: none;
  }

  .add-toggle {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 38px;
    padding: 0 16px;
    background: none;
    border: 1px solid var(--paper-edge);
    border-radius: 999px;
    font-size: 15px;
  }

  .add-toggle:hover:not(:disabled),
  .add-toggle[aria-expanded='true'] {
    background: var(--wash-hover);
  }

  .plus {
    font-family: var(--font-hand);
    font-size: 24px;
    line-height: 0;
    color: var(--link);
  }

  .add {
    display: grid;
    grid-template-columns: minmax(200px, 1fr) minmax(200px, 1.4fr) auto;
    gap: 12px 16px;
    align-items: end;
    padding: 18px 20px;
    background: var(--paper-warm);
    border-radius: 10px;
  }

  .add-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .hint {
    grid-column: 1 / -1;
    margin: 0;
    font-size: 14px;
    color: var(--ink-mute);
  }

  .section {
    display: flex;
    flex-direction: column;
  }

  h2 {
    display: flex;
    align-items: baseline;
    gap: 12px;
    margin: 0 0 4px;
    font-size: 15px;
    font-weight: 400;
  }

  .toggle {
    display: flex;
    align-items: baseline;
    gap: 6px;
    min-width: 0;
    padding: 2px 4px 2px 0;
    text-align: left;
    background: none;
    border: none;
    color: var(--ink-mute);
  }

  .toggle:hover:not(:disabled) {
    background: none;
    color: var(--ink);
  }

  .toggle code {
    font-family: var(--font-mono);
    font-size: 14px;
    color: var(--accent-a);
    overflow-wrap: anywhere;
  }

  .toggle > :global(svg) {
    flex-shrink: 0;
    align-self: center;
  }

  h2 .meta {
    flex-shrink: 0;
  }

  .toggle code.quiet {
    color: var(--ink-mute);
  }

  .row {
    position: relative;
    display: grid;
    grid-template-columns: minmax(180px, 1fr) minmax(240px, 1.7fr) 150px;
    gap: 20px;
    align-items: start;
    padding: 14px 8px 14px 22px;
    border-top: 1px dotted var(--paper-edge);
    transition: background-color var(--dur-base);
  }

  .row:hover {
    background: var(--wash-hover);
  }

  .row.inherited {
    color: var(--ink-mute);
  }

  /* A row another part of the app sent us to. */
  .row:global(.flash) {
    animation: flash 1.6s var(--ease-ink);
  }

  @keyframes flash {
    from {
      background: color-mix(in srgb, var(--link) 18%, transparent);
    }
  }

  /* An edited row: a rose dot in the margin, like a pen mark. */
  .row.changed::before {
    content: '';
    position: absolute;
    left: 6px;
    top: 22px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--link);
  }

  .key {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .key code {
    font-family: var(--font-mono);
    font-size: 14px;
    color: var(--accent-b);
  }

  .row.inherited .key code {
    color: var(--ink-mute);
  }

  .full {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-faint);
    overflow-wrap: anywhere;
  }

  .sub {
    font-size: 13px;
    line-height: 1.45;
    color: var(--ink-mute);
  }

  .values {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
  }

  .value {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    min-height: 34px;
  }

  .value code {
    font-family: var(--font-mono);
    font-size: 13px;
    overflow-wrap: anywhere;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  .masked {
    font-family: var(--font-mono);
    font-size: 15px;
    letter-spacing: 0.2em;
    color: var(--ink-soft);
  }

  .secret {
    font-size: 19px;
    color: var(--accent-c);
  }

  .note {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    margin: 0;
  }

  .note code {
    font-family: var(--font-mono);
    font-size: 12px;
    overflow-wrap: anywhere;
  }

  .start {
    align-self: flex-start;
  }

  .round {
    display: grid;
    place-items: center;
    width: 34px;
    height: 34px;
    flex-shrink: 0;
    padding: 0;
    background: none;
    border: 1px solid var(--paper-edge);
    border-radius: 50%;
    color: var(--ink-soft);
  }

  .round:hover:not(:disabled) {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 2px;
  }

  .empty {
    display: flex;
    align-items: baseline;
    gap: 14px;
    margin: 0;
    padding: 24px 0;
    color: var(--ink-mute);
  }

  .empty .hand {
    color: var(--accent-c);
  }

  @media (prefers-reduced-motion: reduce) {
    .row {
      transition: none;
    }

    .row:global(.flash) {
      animation: none;
      background: color-mix(in srgb, var(--link) 12%, transparent);
    }
  }
</style>
