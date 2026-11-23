<script lang="ts">
  // Settings › Links › Forges: what kind of forge a server of your own is, so its remotes
  // open on the web. Forges known by host (github.com, gitlab.com…) need nothing here, and a
  // host is never guessed from its name. Rules live in git config as gitrevenge-forge.<host>.
  import { RemoveForgeRule, SaveForgeRule, UnknownHosts } from '../../../wailsjs/go/app/App.js'
  import { links } from '../../../wailsjs/go/models'
  import { errorText } from '../errors'
  import Icon from '../ui/Icon.svelte'
  import { forgeAsk } from './forgeAsk.svelte'

  let { repo, repoName, rules, onChanged }: { repo: string; repoName: string; rules: links.ForgeRule[]; onChanged: () => void } = $props()

  const KINDS: { id: string; label: string }[] = [
    { id: 'gitlab', label: 'GitLab' },
    { id: 'gitea', label: 'Gitea' },
    { id: 'forgejo', label: 'Forgejo' },
    { id: 'github', label: 'GitHub Enterprise' },
    { id: 'bitbucket-server', label: 'Bitbucket Data Center' },
    { id: 'azure', label: 'Azure DevOps Server' },
  ]
  const kindLabel = (id: string) => KINDS.find((k) => k.id === id)?.label ?? id

  let unknown = $state<string[]>([])
  $effect(() => {
    void rules
    UnknownHosts(repo)
      .then((h) => (unknown = h ?? []))
      .catch(() => (unknown = []))
  })

  /** The rule being written: a new one, or an edit of was (its host and scope). */
  let draft = $state<{ host: string; forge: string; web: string; scope: string; was: string; wasScope: string } | null>(null)
  let error = $state('')
  let confirmDelete = $state('')

  function start(r?: links.ForgeRule, host = '') {
    error = ''
    draft = r
      ? { host: r.host, forge: r.forge, web: r.web ?? '', scope: r.scope, was: r.host, wasScope: r.scope }
      : { host, forge: 'gitlab', web: '', scope: repo ? 'local' : 'global', was: '', wasScope: '' }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (!draft) return
    const d = draft
    try {
      if (d.was && d.wasScope !== d.scope) {
        // Moving to the other config: write it there, then drop the old one.
        await SaveForgeRule(repo, links.ForgeRule.createFrom({ host: d.host, forge: d.forge, web: d.web, scope: d.scope }), '')
        await RemoveForgeRule(repo, d.wasScope, d.was)
      } else {
        await SaveForgeRule(repo, links.ForgeRule.createFrom({ host: d.host, forge: d.forge, web: d.web, scope: d.scope }), d.was)
      }
      draft = null
      onChanged()
    } catch (err) {
      error = errorText(err)
    }
  }

  async function remove(r: links.ForgeRule) {
    confirmDelete = ''
    try {
      await RemoveForgeRule(repo, r.scope, r.host)
    } catch (err) {
      error = errorText(err)
    }
    onChanged()
  }

  const key = (r: links.ForgeRule) => `${r.scope}:${r.host}`

  // Opened from a remote's menu for a host: start its rule, in view.
  let section = $state<HTMLElement>()
  $effect(() => {
    if (!forgeAsk.host) return
    const host = forgeAsk.host
    forgeAsk.host = ''
    const have = rules.find((r) => r.host === host && !r.shadowed)
    start(have, have ? '' : host)
    queueMicrotask(() => section?.scrollIntoView({ block: 'center' }))
  })
</script>

<div class="forges" bind:this={section}>
  <h4 class="meta">Forges</h4>
  <p class="hint">
    Remotes on GitHub, GitLab.com, Gitea.com, Codeberg, Bitbucket and Azure DevOps open on the web. For a server of your own, say what it runs: Git Revenge never guesses from a host's name.
  </p>
  {#each rules as r (key(r))}
    <div class="rule" class:off={r.shadowed}>
      <span class="t">
        <span class="n mono">{r.host}</span>
        <span class="d">{kindLabel(r.forge)}{r.web ? ` · pages at ${r.web}` : ''}</span>
        {#if r.problem}<span class="warn">{r.problem}</span>{/if}
        {#if r.shadowed}<span class="d">Replaced here by {repoName}'s own rule for {r.host}.</span>{/if}
      </span>
      <span class="pill" class:repo={r.scope === 'local'}>{r.scope === 'local' ? repoName || 'this repo' : 'global config'}</span>
      <span class="acts">
        <button type="button" class="btn sm quiet" onclick={() => start(r)}>Edit</button>
        {#if confirmDelete === key(r)}
          <button type="button" class="btn sm danger" onclick={() => remove(r)}>Delete</button>
        {:else}
          <button type="button" class="btn sm quiet icon del" aria-label="Delete the rule for {r.host}" title="Delete" onclick={() => (confirmDelete = key(r))}><Icon name="trash" size={14} /></button>
        {/if}
      </span>
    </div>
  {/each}

  {#if unknown.length && !draft}
    <p class="unknown">
      <span class="d">{repoName ? `${repoName}'s` : 'These'} remotes on servers with no forge named:</span>
      {#each unknown as h (h)}
        <button type="button" class="btn sm quiet mono" title="Say what forge {h} runs" onclick={() => start(undefined, h)}>{h}</button>
      {/each}
    </p>
  {/if}

  {#if error}<p class="form-error" role="alert">{error}</p>{/if}

  {#if draft}
    <form class="edit" onsubmit={save}>
      <label class="form-field"><span>Host, as in the remote's address</span><input class="mono" bind:value={draft.host} placeholder="git.example.com" required /></label>
      <label class="form-field">
        <span>Runs</span>
        <select bind:value={draft.forge}>
          {#each KINDS as k (k.id)}<option value={k.id}>{k.label}</option>{/each}
        </select>
      </label>
      <label class="form-field wide">
        <span>Its pages are at (only when not https:// and the host, such as for an SSH alias)</span>
        <input class="mono" bind:value={draft.web} placeholder="https://{draft.host || 'git.example.com'}" />
      </label>
      {#if repo}
        <label class="form-field">
          <span>Keep in</span>
          <select bind:value={draft.scope}>
            <option value="local">{repoName}'s config</option>
            <option value="global">your global config</option>
          </select>
        </label>
      {/if}
      <span class="row">
        <button type="submit" class="btn">Save</button>
        <button type="button" class="btn quiet" onclick={() => (draft = null)}>Cancel</button>
      </span>
    </form>
  {:else}
    <div class="row">
      <button type="button" class="btn" onclick={() => start()}><Icon name="plus" size={14} />Add a forge…</button>
    </div>
  {/if}
</div>

<style>
  .forges {
    display: flex;
    flex-direction: column;
    margin-top: 18px;
  }

  h4 {
    margin: 0 0 6px;
  }

  .hint {
    margin: 0 0 6px;
    font-size: 13px;
    line-height: 1.5;
    color: var(--ink-soft);
  }

  .rule {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    gap: 10px;
    align-items: center;
    padding: 8px 0;
    border-bottom: 1px dashed var(--hairline);
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

  .mono {
    font-family: var(--font-mono);
  }

  .d {
    font-size: 12px;
    color: var(--ink-mute);
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

  .del {
    color: var(--danger);
  }

  .unknown {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
    margin: 8px 0 0;
  }

  .edit {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 8px 12px;
    padding-top: 10px;
  }

  .edit .wide,
  .edit .row {
    grid-column: 1 / -1;
  }

  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding-top: 10px;
  }
</style>
