<script lang="ts" module>
  /** Ready-made links: fill in a few details and the pattern and address follow. */
  export type Preset = 'jira' | 'numbered' | 'github' | 'gitlab' | 'linear' | 'azure' | 'custom'

  export const PRESETS: { id: Preset; label: string }[] = [
    { id: 'jira', label: 'Jira' },
    { id: 'numbered', label: 'Numbered (#123)' },
    { id: 'github', label: 'GitHub issues' },
    { id: 'gitlab', label: 'GitLab issues' },
    { id: 'linear', label: 'Linear' },
    { id: 'azure', label: 'Azure Boards' },
    { id: 'custom', label: 'My own pattern' },
  ]

  const trimSlash = (s: string) => s.trim().replace(/\/+$/, '')
  const escapeRe = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

  /** The pattern and address a preset makes from its fields. */
  export function presetRule(p: Preset, f: Record<string, string>): { pattern: string; url: string } | null {
    switch (p) {
      case 'jira': {
        const keys = (f.keys ?? '').split(/[\s,]+/).filter(Boolean).map((k) => escapeRe(k.toUpperCase()))
        const pattern = keys.length ? `\\b(${keys.join('|')})-\\d+\\b` : '\\b[A-Z][A-Z0-9_]+-\\d+\\b'
        return { pattern, url: `${trimSlash(f.address ?? '')}/browse/{0}` }
      }
      case 'numbered':
        return { pattern: '#(\\d+)\\b', url: (f.address ?? '').trim() }
      case 'github': {
        const repo = (f.repo ?? '').trim().replace(/^https?:\/\/github\.com\//, '').replace(/\.git$/, '').replace(/\/+$/, '')
        return { pattern: '#(\\d+)\\b', url: `https://github.com/${repo}/issues/{1}` }
      }
      case 'gitlab':
        return { pattern: '#(\\d+)\\b', url: `${trimSlash(f.project ?? '')}/-/issues/{1}` }
      case 'linear':
        return { pattern: '\\b[A-Z][A-Z0-9]+-\\d+\\b', url: `https://linear.app/${(f.workspace ?? '').trim()}/issue/{0}` }
      case 'azure':
        return { pattern: '\\bAB#(\\d+)\\b', url: `https://dev.azure.com/${(f.org ?? '').trim()}/${(f.project ?? '').trim()}/_workitems/edit/{1}` }
    }
    return null
  }

  /** The git config section a rule is written as, for showing before it's saved. */
  export function configText(r: { name: string; pattern: string; url: string; in: string[]; shipped: boolean; enabled: boolean }): string {
    // As git writes it: backslashes and quotes escaped, quoted only for a # or ; or space at
    // either end.
    const q = (v: string) => {
      const e = v.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
      return /[#;]|^\s|\s$/.test(v) ? `"${e}"` : e
    }
    const lines = [`[gitrevenge-link "${r.name}"]`, `\tpattern = ${q(r.pattern)}`, `\turl = ${q(r.url)}`, `\tin = ${r.in.join(' ')}`, `\tshipped = ${r.shipped}`]
    if (!r.enabled) lines.push('\tenabled = false')
    return lines.join('\n')
  }
</script>

<script lang="ts">
  import { IssueURL, SaveLinkRule, RemoveLinkRule, TryLinkRule } from '../../../wailsjs/go/app/App.js'
  import { app, links } from '../../../wailsjs/go/models'
  import { errorText } from '../errors'
  import InkButton from '../ui/InkButton.svelte'
  import Modal from '../ui/Modal.svelte'
  import { makeLinker, patternProblem, PLACES } from './links'

  let {
    repo,
    repoName,
    rule,
    onClose,
    onSaved,
  }: {
    /** The repository open, or '' when only the global config can be edited. */
    repo: string
    repoName: string
    /** The rule being edited, or null for a new one. */
    rule: links.Rule | null
    onClose: () => void
    onSaved: () => void
  } = $props()
  const id = $props.id()

  // The dialog opens afresh for each rule, so these are read once.
  // svelte-ignore state_referenced_locally
  const original = rule
  let preset = $state<Preset>(original ? 'custom' : 'jira')
  let fields = $state<Record<string, string>>({})
  let name = $state(original?.name ?? 'Jira')
  let pattern = $state(original?.pattern ?? '')
  let url = $state(original?.url ?? '')
  let places = $state<string[]>(original ? [...original.in] : ['message', 'trailers', 'branches'])
  let shipped = $state(original?.shipped ?? true)
  let scope = $state(original?.scope ?? 'global')
  let sample = $state('feat: retry uploads (ABC-1234)\n\nRefs: OPS-77, #518')

  // A new link starts from the Jira preset.
  if (!original) apply()

  function pick(p: Preset) {
    const was = PRESETS.find((x) => x.id === preset)?.label
    preset = p
    if (p !== 'custom' && (!name.trim() || name === was)) name = PRESETS.find((x) => x.id === p)?.label.replace(' (#123)', ' issues') ?? name
    apply()
  }

  /** Recomputes the pattern and address from the preset's fields. */
  function apply() {
    const r = presetRule(preset, fields)
    if (r) [pattern, url] = [r.pattern, r.url]
  }

  async function fromOrigin() {
    const u = await IssueURL(repo).catch(() => '')
    if (!u) {
      error = "This repository's remotes don't look like GitHub, GitLab or similar, so there's nothing to fill in."
      return
    }
    preset = 'custom'
    pattern = '#(\\d+)\\b'
    url = u.replace('{n}', '{1}')
  }

  let draft = $derived(links.Rule.createFrom({ name: name.trim(), pattern, url: url.trim(), in: places, shipped, enabled: original?.enabled ?? true, scope }))
  let jsProblem = $derived(pattern ? patternProblem(pattern) : '')
  let tried = $state<app.LinkTry | null>(null)
  let asked = 0
  $effect(() => {
    const r = draft
    const n = ++asked
    const timer = setTimeout(async () => {
      const res = await TryLinkRule(repo, r).catch((e) => app.LinkTry.createFrom({ scanned: 0, commits: 0, matches: [], problem: errorText(e) }))
      if (n === asked) tried = res
    }, 250)
    return () => clearTimeout(timer)
  })
  let problem = $derived(jsProblem || tried?.problem || '')
  let preview = $derived(problem ? [] : makeLinker({ web: false, rules: [{ ...draft, enabled: true, in: ['message', 'trailers', 'branches'] }] }).parts(sample, 'message'))

  let error = $state('')
  let busy = $state(false)

  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (problem) return
    busy = true
    error = ''
    try {
      const sameScope = original && original.scope === scope
      await SaveLinkRule(repo, draft, sameScope ? original.name : '')
      // Moved to the other config: the old one goes.
      if (original && !sameScope) await RemoveLinkRule(repo, original.scope, original.name)
      onSaved()
      onClose()
    } catch (err) {
      error = errorText(err)
    }
    busy = false
  }

  const togglePlace = (p: string, on: boolean) => (places = on ? [...places, p] : places.filter((x) => x !== p))
</script>

<Modal title={original ? `Edit ${original.name}` : 'Add a link'} eyebrow="just a link" width={880} layer={300} {onClose} dismissable={!busy}>
  <form onsubmit={save}>
    {#if !original}
      <div class="presets" role="radiogroup" aria-label="Start from">
        {#each PRESETS as p (p.id)}
          <button type="button" class="chip" role="radio" aria-checked={preset === p.id} onclick={() => pick(p.id)}>{p.label}</button>
        {/each}
      </div>
    {/if}
    <div class="cols">
      <div class="col">
        <label class="form-field"><span>Name</span><input bind:value={name} required /></label>
        {#if preset === 'jira'}
          <label class="form-field"
            ><span>Your Jira address</span><input class="mono" bind:value={fields.address} placeholder="https://acme.atlassian.net" oninput={apply} /><span class="hint"
              >Links go to /browse/KEY-123 there.</span
            ></label
          >
          <label class="form-field"
            ><span>Project keys</span><input class="mono" bind:value={fields.keys} placeholder="ABC, OPS" oninput={apply} /><span class="hint"
              >Leave empty to match any KEY-123.</span
            ></label
          >
        {:else if preset === 'numbered'}
          <label class="form-field"
            ><span>Address for #123</span><input class="mono" bind:value={fields.address} placeholder="https://tracker.example/issues/{'{1}'}" oninput={apply} /><span class="hint"
              >Write <code>{'{1}'}</code> where the number goes. A bare number means different things in different places, so nothing is assumed.</span
            ></label
          >
        {:else if preset === 'github'}
          <label class="form-field"
            ><span>Repository on GitHub</span><input class="mono" bind:value={fields.repo} placeholder="owner/name" oninput={apply} /></label
          >
        {:else if preset === 'gitlab'}
          <label class="form-field"><span>Project address</span><input class="mono" bind:value={fields.project} placeholder="https://gitlab.com/group/project" oninput={apply} /></label>
        {:else if preset === 'linear'}
          <label class="form-field"><span>Workspace</span><input class="mono" bind:value={fields.workspace} placeholder="acme" oninput={apply} /></label>
        {:else if preset === 'azure'}
          <label class="form-field"><span>Organization</span><input class="mono" bind:value={fields.org} placeholder="acme" oninput={apply} /></label>
          <label class="form-field"><span>Project</span><input class="mono" bind:value={fields.project} placeholder="Web" oninput={apply} /></label>
        {/if}
        {#if repo && (preset === 'numbered' || preset === 'github' || preset === 'gitlab')}
          <span><button type="button" class="btn sm quiet" onclick={fromOrigin}>Fill in from {repoName}'s remote</button></span>
        {/if}
        <div class="raw">
          <label class="form-field"
            ><span>Pattern (regular expression)</span><input class="mono" bind:value={pattern} spellcheck="false" oninput={() => (preset = 'custom')} required /></label
          >
          <label class="form-field"
            ><span>Opens</span><input class="mono" bind:value={url} spellcheck="false" placeholder="https://…/{'{0}'}" oninput={() => (preset = 'custom')} required /><span class="hint"
              ><code>{'{0}'}</code> is the whole match, <code>{'{1}'}</code> the first group. Only http and https.</span
            ></label
          >
          {#if problem}<p class="form-error" role="alert">{problem}</p>{/if}
        </div>
      </div>
      <div class="col">
        <fieldset>
          <legend>Look in</legend>
          {#each PLACES as p (p.id)}
            <label class="form-check"><input type="checkbox" checked={places.includes(p.id)} onchange={(e) => togglePlace(p.id, e.currentTarget.checked)} /> {p.label}</label>
          {/each}
          <label class="form-check"><input type="checkbox" bind:checked={shipped} /> Counts as a reference in Insights › Shipped</label>
        </fieldset>
        <fieldset>
          <legend>Save in</legend>
          <label class="form-check"><input type="radio" name="{id}-scope" value="global" bind:group={scope} /> Global git config, for every repository</label>
          <label class="form-check" title={repo ? '' : 'Open a repository to keep a link in its own config'}
            ><input type="radio" name="{id}-scope" value="local" bind:group={scope} disabled={!repo} /> {repoName ? `${repoName}'s .git/config` : "A repository's .git/config"}</label
          >
          <pre class="code-ground config">{configText(draft)}</pre>
        </fieldset>
        <div class="try">
          <label class="form-field"><span>Try it</span><textarea bind:value={sample} rows="3" spellcheck="false"></textarea></label>
          <p class="sample">
            {#each preview as part, k (k)}{#if part.href}<span class="hit" title={part.href}>{part.text}</span>{:else}{part.text}{/if}{/each}
          </p>
          {#each preview.filter((x) => x.href) as part, k (k)}<code class="out">{part.text} → {part.href}</code>{/each}
          {#if tried && !problem && tried.scanned}
            <span class="hint">{tried.commits} of the last {tried.scanned} commits in {repoName} match{tried.matches.length ? `, such as ${tried.matches.map((m) => m.text).join(', ')}` : ''}.</span>
          {/if}
        </div>
      </div>
    </div>
    {#if error}<p class="form-error" role="alert">{error}</p>{/if}
    <div class="foot">
      <span class="hint">Opens in your default browser. Nothing is fetched from the linked site.</span>
      <span class="spacer"></span>
      <InkButton variant="secondary" onclick={onClose} disabled={busy}>Cancel</InkButton>
      <InkButton type="submit" variant="primary" disabled={busy || !!problem || !name.trim()}>{original ? 'Save link' : 'Add link'}</InkButton>
    </div>
  </form>
</Modal>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .presets {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .chip {
    height: 30px;
    padding: 0 12px;
    border-radius: 999px;
    border: 1px solid var(--paper-edge);
    background: var(--paper);
    font-size: 13px;
    color: var(--ink-soft);
  }

  .chip[aria-checked='true'] {
    background: var(--selected);
    border-color: var(--accent);
    color: var(--ink);
  }

  .cols {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 28px;
  }

  .col {
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-width: 0;
  }

  .mono,
  textarea {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .hint {
    font-size: 12px;
    line-height: 1.45;
    color: var(--ink-mute);
  }

  .raw {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 12px;
    border-radius: var(--r-md);
    background: var(--paper-warm);
  }

  fieldset {
    border: 0;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  legend {
    font-size: 13px;
    font-weight: 600;
    color: var(--ink-soft);
    margin-bottom: 6px;
  }

  .config {
    margin: 4px 0 0;
    padding: 10px 12px;
    border-radius: var(--r-md);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.55;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .try {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 12px;
    border-radius: var(--r-md);
    background: var(--paper-warm);
  }

  .sample {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 13px;
    white-space: pre-wrap;
  }

  .hit {
    color: var(--link);
    background: linear-gradient(transparent 62%, color-mix(in srgb, var(--link) 30%, transparent) 62%);
  }

  .out {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-soft);
    overflow-wrap: anywhere;
  }

  code {
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .foot {
    display: flex;
    align-items: center;
    gap: 10px;
    padding-top: 14px;
    border-top: 1px solid var(--paper-edge);
  }

  .spacer {
    flex: 1;
  }

  @media (max-width: 900px) {
    .cols {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
