<script lang="ts">
  // The settings panel: appearance, git, repositories and updates, saved to the local
  // settings file (ADR 0005).
  import { ChooseFile, ChooseFolder, Preferences, RepoPreferences, SetPreferences, SetRepoPreferences } from '../../wailsjs/go/app/App.js'
  import { commands } from './commands.svelte'
  import { app } from '../../wailsjs/go/models'
  import { errorText } from './errors'
  import { gitConfig } from './gitconfig/config.svelte'
  import { session } from './session.svelte'
  import { prefs } from './prefs.svelte'
  import { theme } from './theme.svelte'
  import Icon from './ui/Icon.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'
  import { update } from './update.svelte'
  import Loading from './ui/Loading.svelte'

  let { onClose }: { onClose: () => void } = $props()

  let p = $state<app.Preferences | null>(null)
  let initialGit = ''
  let error = $state('')
  let saving = $state(false)
  let restartNote = $state(false)

  // This repository's own settings (ADR 0009), when a repository tab is active. Patterns are
  // edited as text, comma or space separated.
  const repoPath = session.activeTab?.path ?? ''
  const repoName = session.activeTab?.name ?? ''
  const KIND_LABELS: Record<string, { label: string; hint: string }> = {
    trunk: { label: 'Trunk', hint: 'The main line' },
    release: { label: 'Release', hint: 'Releases in progress' },
    environment: { label: 'Environment', hint: 'Deploy branches such as staging' },
    docs: { label: 'Docs', hint: 'Published sites such as gh-pages' },
    bot: { label: 'Bot', hint: 'Branches a bot opens' },
    feature: { label: 'Feature', hint: 'Work in progress' },
  }
  let repo = $state<app.RepoPreferences | null>(null)
  let kindText = $state<Record<string, string>>({})
  let hideText = $state('')

  const splitPatterns = (text: string) => text.split(/[\s,]+/).filter(Boolean)
  const joinPatterns = (list: string[]) => list.join(', ')
  const sameList = (a: string[], b: string[]) => a.length === b.length && a.every((x, i) => x === b[i])

  $effect(() => {
    if (!repoPath) return
    void RepoPreferences(repoPath)
      .then((res) => {
        repo = res
        kindText = Object.fromEntries(res.kinds.map((k) => [k.kind, joinPatterns(k.patterns)]))
        hideText = joinPatterns(res.hide)
      })
      .catch((e) => (error = errorText(e)))
  })

  function repoToSave(r: app.RepoPreferences): app.RepoPreferences {
    return app.RepoPreferences.createFrom({
      ...r,
      kinds: r.kinds.map((k) => {
        const patterns = splitPatterns(kindText[k.kind] ?? '')
        return { ...k, patterns, custom: !sameList(patterns, k.defaults) }
      }),
      hide: splitPatterns(hideText),
    })
  }

  $effect(() => {
    void Preferences()
      .then((res) => {
        p = res
        initialGit = res.gitPath
      })
      .catch((e) => (error = errorText(e)))
  })

  async function chooseGit() {
    const f = await ChooseFile('Choose the git executable')
    if (f && p) p.gitPath = f
  }

  async function chooseCloneDir() {
    const d = await ChooseFolder('Folder to clone repositories into')
    if (d && p) p.cloneDir = d
  }

  async function save() {
    if (!p) return
    error = ''
    saving = true
    try {
      await SetPreferences(p)
      if (repo) await SetRepoPreferences(repoPath, repoToSave(repo))
      // Apply what can change now; the git executable waits for a restart.
      theme.mode = p.theme as typeof theme.mode
      theme.density = p.graphDensity === 'compact' ? 'compact' : 'comfortable'
      update.checksOn = p.updateChecks
      prefs.zoom = p.zoom
      prefs.cloneDir = p.cloneDir
      prefs.apply()
      if (p.gitPath !== initialGit) {
        restartNote = true
        initialGit = p.gitPath
      } else onClose()
    } catch (e) {
      error = errorText(e)
    } finally {
      saving = false
    }
  }
</script>

<Modal title="Settings" eyebrow="make it yours" width={560} {onClose}>
  {#if !p}
    {#if error}<p class="muted">{error}</p>{:else}<Loading />{/if}
  {:else}
    <form
      onsubmit={(e) => {
        e.preventDefault()
        void save()
      }}
    >
      <fieldset>
        <legend class="meta">Appearance</legend>
        <label class="form-field">
          <span>Theme</span>
          <select bind:value={p.theme}>
            <option value="">Follow the system</option>
            <option value="light">Light (Ink Paper)</option>
            <option value="dark">Dark (Ink Parchment)</option>
          </select>
        </label>
        <label class="form-field">
          <span>Commit graph</span>
          <select bind:value={p.graphDensity}>
            <option value="">Timeline (two lines per commit)</option>
            <option value="compact">Compact rows</option>
          </select>
        </label>
        <label class="form-field">
          <span>Zoom: {p.zoom}%</span>
          <input type="range" min="80" max="150" step="5" bind:value={p.zoom} aria-valuetext="{p.zoom} percent" />
        </label>
      </fieldset>

      <fieldset>
        <legend class="meta">Git</legend>
        <label class="form-field">
          <span>Git executable (empty to find it automatically)</span>
          <span class="row">
            <input bind:value={p.gitPath} placeholder={p.gitInUse} />
            <button type="button" class="btn quiet" onclick={chooseGit}><Icon name="folder" size={14} />Choose…</button>
          </span>
        </label>
        <p class="hint">In use now: <code>{p.gitInUse || 'none found'}</code>. A change takes effect after restarting Git Revenge.</p>
        <span>
          <button
            type="button"
            class="btn quiet"
            onclick={() => {
              onClose()
              void gitConfig.show(session.activeTab?.path ?? '')
            }}><Icon name="gear" size={14} />Edit git config…</button
          >
        </span>
      </fieldset>

      <fieldset>
        <legend class="meta">Repositories</legend>
        <label class="form-field">
          <span>Clone into (empty for the folder of the last repository opened)</span>
          <span class="row">
            <input bind:value={p.cloneDir} />
            <button type="button" class="btn quiet" onclick={chooseCloneDir}><Icon name="folder" size={14} />Choose…</button>
          </span>
        </label>
      </fieldset>

      <fieldset>
        <legend class="meta">Keyboard</legend>
        <span>
          <button
            type="button"
            class="btn quiet"
            onclick={() => {
              onClose()
              commands.keybindingsOpen = true
            }}><Icon name="keyboard" size={14} />Customize keyboard shortcuts…</button
          >
        </span>
      </fieldset>

      {#if repo}
        <fieldset>
          <legend class="meta">This repository · {repoName}</legend>
          <p class="hint">
            Branch kinds change how branches look and behave. Patterns match the branch name without the remote, so <code>release/*</code> also
            matches <code>origin/release/1.4</code>; <code>*</code> matches any run of characters. These stay on this computer.
          </p>
          {#each repo.kinds as k (k.kind)}
            {@const custom = !sameList(splitPatterns(kindText[k.kind] ?? ''), k.defaults)}
            <label class="form-field">
              <span class="kind-label">
                {KIND_LABELS[k.kind]?.label ?? k.kind}
                <span class="kind-hint">{KIND_LABELS[k.kind]?.hint ?? ''}</span>
                {#if custom}
                  <button type="button" class="btn sm text" onclick={() => (kindText[k.kind] = joinPatterns(k.defaults))}>Use defaults</button>
                {/if}
              </span>
              <input bind:value={kindText[k.kind]} placeholder={k.defaults.length ? '' : 'None by default'} spellcheck="false" autocomplete="off" />
            </label>
          {/each}
          <label class="check">
            <input type="checkbox" bind:checked={repo.cleanupOnPull} />
            Clean up merged branches after each pull
          </label>
          <p class="hint">
            Deletes a local branch once its upstream is deleted, if it has nothing unpushed. Trunk, Release, Environment and Docs branches, the branch
            you're on and worktree branches are always kept. Undo brings them back.
          </p>
          <label class="check">
            <input type="checkbox" bind:checked={repo.syncNotes} />
            Sync notes on fetch, pull and push
          </label>
          <p class="hint">
            Brings the remote's notes in on fetch and pull, and sends yours when you push. Notes that have diverged are left alone, with a note saying so.
          </p>
          <label class="form-field">
            <span>Hidden branches</span>
            <input bind:value={hideText} placeholder="For example wip/*, gh-pages" spellcheck="false" autocomplete="off" />
          </label>
        </fieldset>
      {/if}

      <fieldset>
        <legend class="meta">Updates</legend>
        <label class="check"><input type="checkbox" bind:checked={p.updateChecks} /> Check GitHub for a newer version at startup</label>
      </fieldset>

      {#if error}<p class="error" role="alert">{error}</p>{/if}
      {#if restartNote}<p class="note" role="status">Saved. Restart Git Revenge to use the new git.</p>{/if}
      <span class="buttons">
        <InkButton type="submit" disabled={saving}>Save</InkButton>
        <InkButton variant="secondary" onclick={onClose}>{restartNote ? 'Close' : 'Cancel'}</InkButton>
      </span>
    </form>
  {/if}
</Modal>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  fieldset {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin: 0;
    padding: 0;
    border: none;
  }

  legend {
    margin-bottom: 6px;
  }

  .row {
    display: flex;
    gap: 6px;
  }

  .row input {
    flex: 1;
    min-width: 0;
  }

  .hint {
    margin: -4px 0 0;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .hint code {
    font-family: var(--font-mono);
    font-size: 12px;
    overflow-wrap: anywhere;
  }

  .check {
    font-size: 14px;
  }

  .kind-label {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 28px;
  }

  .kind-hint {
    flex: 1;
    color: var(--ink-mute);
    font-size: 12px;
  }

  .form-field input {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  input[type='range'] {
    accent-color: var(--accent);
  }

  .buttons {
    display: flex;
    gap: 8px;
  }

  .muted {
    color: var(--ink-mute);
    font-style: italic;
  }

  .error {
    color: var(--danger);
  }

  .note {
    color: var(--success);
  }
</style>
