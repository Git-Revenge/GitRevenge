<script lang="ts">
  // Settings › Links: the smart links that apply to a repository, its own and the global
  // ones. Changes are written to git config straight away, not on the dialog's Save. The
  // repository is the active tab's, or, when Settings opens from another tab, one picked
  // from the repositories open, so a link can always go in a repository's own config.
  import { ChooseFile, ImportLinkRules, LinkRules, RemoveLinkRule, ReorderLinkRules, SaveFile, SaveLinkRule, SetWebLinks } from '../../../wailsjs/go/app/App.js'
  import { links } from '../../../wailsjs/go/models'
  import { errorText } from '../errors'
  import { session } from '../session.svelte'
  import { toasts } from '../toasts.svelte'
  import Icon from '../ui/Icon.svelte'
  import LinkEditor from './LinkEditor.svelte'
  import { linkStore } from './links.svelte'

  let { active, onShowConfig }: { active: string; onShowConfig: (repo: string) => void } = $props()

  // svelte-ignore state_referenced_locally
  let picked = $state(active || session.tabs[0]?.path || '')
  let repo = $derived(session.tabs.some((t) => t.path === picked) ? picked : (session.tabs[0]?.path ?? ''))
  let repoName = $derived(session.tabs.find((t) => t.path === repo)?.name ?? '')

  let cfg = $state<links.Config | null>(null)
  let error = $state('')
  let editing = $state<links.Rule | null | undefined>(undefined)
  let confirmDelete = $state('')

  async function load() {
    try {
      cfg = await LinkRules(repo)
      error = ''
    } catch (e) {
      error = errorText(e)
    }
    void linkStore.load(repo)
  }
  $effect(() => {
    void repo
    void load()
  })

  async function act(fn: () => Promise<unknown>) {
    try {
      await fn()
    } catch (e) {
      error = errorText(e)
    }
    await load()
  }

  const key = (r: links.Rule) => `${r.scope}:${r.name}`
  const inScope = (scope: string) => (cfg?.rules ?? []).filter((r) => r.scope === scope)

  function move(r: links.Rule, d: -1 | 1) {
    const names = inScope(r.scope).map((x) => x.name)
    const i = names.indexOf(r.name)
    const j = i + d
    if (j < 0 || j >= names.length) return
    ;[names[i], names[j]] = [names[j], names[i]]
    void act(() => ReorderLinkRules(repo, r.scope, names))
  }

  function toggle(r: links.Rule, on: boolean) {
    void act(() => SaveLinkRule(repo, links.Rule.createFrom({ ...r, enabled: on }), r.name))
  }

  function copyTo(r: links.Rule) {
    const scope = r.scope === 'local' ? 'global' : 'local'
    const exists = inScope(scope).some((x) => x.name === r.name)
    void act(async () => {
      await SaveLinkRule(repo, links.Rule.createFrom({ ...r, scope, shadowed: false, problem: '' }), exists ? r.name : '')
      toasts.show(`${r.name} copied to ${scope === 'global' ? 'your global config' : `${repoName}'s config`}.`)
    })
  }

  function remove(r: links.Rule) {
    confirmDelete = ''
    void act(() => RemoveLinkRule(repo, r.scope, r.name))
  }

  async function exportRules() {
    const list = (cfg?.rules ?? []).filter((r) => !r.shadowed).map(({ name, pattern, url, in: places, shipped, enabled }) => ({ name, pattern, url, in: places, shipped, enabled }))
    const json = JSON.stringify(list, null, 2) + '\n'
    try {
      const where = await SaveFile('Export links', 'gitrevenge-links.json', btoa(unescape(encodeURIComponent(json))))
      if (where) toasts.show(`Exported ${list.length === 1 ? '1 link' : `${list.length} links`} to ${where}.`)
    } catch (e) {
      error = errorText(e)
    }
  }

  let importScope = $state('global')
  async function importRules() {
    const file = await ChooseFile('Import links').catch(() => '')
    if (!file) return
    await act(async () => {
      const n = await ImportLinkRules(repo, file, importScope)
      toasts.show(`Imported ${n === 1 ? '1 link' : `${n} links`} into ${importScope === 'global' ? 'your global config' : `${repoName}'s config`}.`)
    })
  }
</script>

<fieldset class="links">
  <legend class="meta">Links</legend>
  <p class="hint">
    Text in commit messages, trailers and branch names that matches a pattern becomes a link that opens in your browser. Git Revenge never contacts these sites. Links live in git config: {repo
      ? `${repoName}'s .git/config, or`
      : ''} your global config.
  </p>
  {#if session.tabs.length > 1 || (!active && repo)}
    <label class="form-field pick">
      <span>Repository</span>
      <select bind:value={picked}>
        {#each session.tabs as t (t.path)}<option value={t.path}>{t.name}</option>{/each}
      </select>
    </label>
  {/if}
  {#if error}<p class="form-error" role="alert">{error}</p>{/if}
  {#if cfg}
    <div class="rule">
      <input type="checkbox" checked={cfg.web} aria-label="Link web addresses" onchange={(e) => act(() => SetWebLinks(repo, e.currentTarget.checked))} />
      <span class="t"><span class="n">Web addresses</span><span class="d">https://… and www.… written out in full</span></span>
      <span class="pill">built in</span>
    </div>
    {#each cfg.rules as r (key(r))}
      {@const siblings = inScope(r.scope)}
      {@const at = siblings.findIndex((x) => x.name === r.name)}
      <div class="rule" class:off={!r.enabled || r.shadowed}>
        <input type="checkbox" checked={r.enabled} disabled={r.shadowed} aria-label="{r.name} on" onchange={(e) => toggle(r, e.currentTarget.checked)} />
        <span class="t">
          <span class="n">{r.name}</span>
          <span class="d" title="{r.pattern} → {r.url}">{r.pattern} → {r.url}</span>
          {#if r.problem}<span class="warn">{r.problem}</span>{/if}
          {#if r.shadowed}<span class="d">Replaced here by {repoName}'s own {r.name}.</span>{/if}
        </span>
        <span class="pill" class:repo={r.scope === 'local'}>{r.scope === 'local' ? repoName || 'this repo' : 'global config'}</span>
        <span class="acts">
          <button type="button" class="btn sm quiet icon" aria-label="Move {r.name} up" title="Move up" disabled={at <= 0} onclick={() => move(r, -1)}><span class="up"><Icon name="chevron" size={14} /></span></button>
          <button type="button" class="btn sm quiet icon" aria-label="Move {r.name} down" title="Move down" disabled={at < 0 || at === siblings.length - 1} onclick={() => move(r, 1)}><Icon name="chevron" size={14} /></button>
          {#if repo}
            <button type="button" class="btn sm quiet icon" aria-label={r.scope === 'local' ? `Copy ${r.name} to your global config` : `Copy ${r.name} to ${repoName}'s config`} title={r.scope === 'local' ? 'Copy to global config' : `Copy to ${repoName}'s config`} onclick={() => copyTo(r)}><Icon name="copy" size={14} /></button>
          {/if}
          <button type="button" class="btn sm quiet" onclick={() => (editing = r)}>Edit</button>
          {#if confirmDelete === key(r)}
            <button type="button" class="btn sm danger" onclick={() => remove(r)}>Delete</button>
          {:else}
            <button type="button" class="btn sm quiet icon del" aria-label="Delete {r.name}" title="Delete" onclick={() => (confirmDelete = key(r))}><Icon name="trash" size={14} /></button>
          {/if}
        </span>
      </div>
    {/each}
    <div class="row">
      <button type="button" class="btn" onclick={() => (editing = null)}><Icon name="plus" size={14} />Add a link…</button>
      <span class="spacer"></span>
      {#if repo}<button type="button" class="btn quiet" onclick={() => onShowConfig(repo)} title="Show these links in the Git Config tab">Git Config</button>{/if}
      <button type="button" class="btn quiet" onclick={exportRules} disabled={!cfg.rules.length}>Export</button>
      <button type="button" class="btn quiet" onclick={importRules}>Import…</button>
      {#if repo}
        <select bind:value={importScope} aria-label="Import into">
          <option value="global">to global</option>
          <option value="local">to {repoName}</option>
        </select>
      {/if}
    </div>
  {/if}
</fieldset>

{#if editing !== undefined}
  <LinkEditor {repo} {repoName} rule={editing} onClose={() => (editing = undefined)} onSaved={load} />
{/if}

<style>
  /* As the dialog's other sections. */
  fieldset {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    border: none;
    min-width: 0;
  }

  legend {
    margin-bottom: 6px;
  }

  .pick {
    margin-bottom: 8px;
  }

  .hint {
    margin: 0 0 6px;
    font-size: 13px;
    line-height: 1.5;
    color: var(--ink-soft);
  }

  .rule {
    display: grid;
    grid-template-columns: 20px minmax(0, 1fr) auto auto;
    gap: 10px;
    align-items: center;
    padding: 8px 0;
    border-bottom: 1px dashed var(--hairline);
  }

  .rule input[type='checkbox'] {
    accent-color: var(--accent);
    margin: 0;
  }

  .rule.off .n {
    color: var(--ink-mute);
  }

  .t {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .n {
    font-size: 14px;
  }

  .d {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .warn {
    font-size: 12px;
    color: var(--danger);
  }

  .pill {
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    padding: 2px 8px;
    border-radius: 999px;
    background: var(--paper-deep);
    color: var(--ink-soft);
    white-space: nowrap;
  }

  .pill.repo {
    background: var(--selected);
    color: var(--ink);
  }

  .acts {
    display: flex;
    align-items: center;
    gap: 2px;
  }

  .up {
    display: inline-flex;
    transform: rotate(180deg);
  }

  .del {
    color: var(--danger);
  }

  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding-top: 10px;
  }

  .row select {
    font-size: 13px;
  }

  .spacer {
    flex: 1;
  }
</style>
