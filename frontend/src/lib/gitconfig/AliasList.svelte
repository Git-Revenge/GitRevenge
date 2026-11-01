<script lang="ts">
  // The Aliases view of the Git Config tab (#227): the selected file's aliases and the ones
  // it inherits, edited in place. Aliases are shown, never run: a shell alias is code.
  import { GitCommands, OpenGitConfig, RenameGitConfigKey, SetGitConfig, UnsetGitConfig } from '../../../wailsjs/go/app/App.js'
  import Icon from '../ui/Icon.svelte'
  import InkButton from '../ui/InkButton.svelte'
  import { buildAliases, NAME, type Alias } from './aliases'
  import { gitConfig } from './config.svelte'

  let builtins = $state<string[]>([])
  /** The alias open in the form: its name, '+' for a new one, or '' for none. */
  let editing = $state('')
  let name = $state('')
  let shell = $state(false)
  let command = $state('')
  let formError = $state('')
  /** A short note after copying an alias to another file. */
  let copied = $state('')

  let doc = $derived(gitConfig.doc)
  let scope = $derived(gitConfig.scope)
  let aliases = $derived(doc ? buildAliases(doc.entries, gitConfig.effective, doc.path, builtins) : [])
  /** The other files an alias can be copied to. */
  let targets = $derived(gitConfig.scopes.filter((s) => s.path !== scope?.path && s.kind !== 'file'))

  $effect(() => {
    GitCommands()
      .then((c) => (builtins = c ?? []))
      .catch(() => {})
  })

  function open(a?: Alias) {
    editing = a?.name ?? '+'
    name = a?.name ?? ''
    shell = a?.shell ?? false
    command = a ? (a.shell ? a.value.slice(1) : a.value) : ''
    formError = ''
    copied = ''
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    const n = name.trim()
    if (!NAME.test(n)) {
      formError = 'Use letters, digits and dashes, starting with a letter.'
      return
    }
    if (!command.trim()) {
      formError = 'Say what the alias runs.'
      return
    }
    const old = editing
    const taken = aliases.some((a) => a.own && a.name === n.toLowerCase() && a.name !== old)
    if (taken) {
      formError = `${n} is already an alias in this file.`
      return
    }
    const value = shell ? `!${command}` : command.trim()
    if (old !== '+' && old !== n.toLowerCase() && !(await gitConfig.apply((p) => RenameGitConfigKey(p, `alias.${old}`, `alias.${n}`)))) return
    if (await gitConfig.apply((p) => SetGitConfig(p, `alias.${n}`, value))) editing = ''
  }

  function remove(a: Alias) {
    if (editing === a.name) editing = ''
    return gitConfig.apply((p) => UnsetGitConfig(p, `alias.${a.name}`, ''))
  }

  /** Copies an alias into this file, or into another one, where it's saved separately. */
  async function copy(a: Alias, path = doc?.path ?? '') {
    if (!path) return
    copied = ''
    if (path === doc?.path) {
      await gitConfig.apply((p) => SetGitConfig(p, `alias.${a.name}`, a.value))
      return
    }
    const opened = await gitConfig.call(() => OpenGitConfig(path))
    if (!opened) return
    const d = await gitConfig.call(() => SetGitConfig(path, `alias.${a.name}`, a.value))
    if (!d) return
    gitConfig.docs[path] = d
    const target = targets.find((t) => t.path === path)
    copied = `Copied ${a.name} to ${target ? gitConfig.label(target) : path}. Save it there to keep it.`
  }

  const destination = (path: string) => {
    const t = targets.find((x) => x.path === path)
    return t ? gitConfig.label(t).replace(/^Local · /, '') : path
  }
</script>

<div class="aliases">
  <div class="intro">
    <p>
      Type <code>git co</code> and git runs <code>git checkout</code>. Aliases starting with <code>!</code> run in a shell, so read them before you keep them.
    </p>
    <button class="add-toggle" onclick={() => open()}><span class="plus" aria-hidden="true">+</span> New alias</button>
  </div>
  {#if copied}<p class="copied" role="status">{copied}</p>{/if}

  {#snippet form()}
    <form class="form" onsubmit={save}>
      <div class="form-top">
        <label class="form-field">
          <span>Name</span>
          <input class="field name" bind:value={name} spellcheck="false" autocomplete="off" />
        </label>
        <span class="segmented" role="group" aria-label="Kind of alias">
          <button type="button" aria-pressed={!shell} onclick={() => (shell = false)}>git command</button>
          <button type="button" aria-pressed={shell} onclick={() => (shell = true)}>shell command</button>
        </span>
        <span class="spacer"></span>
        {#if shell}<span class="tag sepia">runs in a shell</span>{/if}
      </div>
      <label class="form-field">
        <span>{shell ? 'Runs, in a shell' : 'Runs, after git'}</span>
        {#if shell}
          <textarea class="code code-ground" bind:value={command} rows="4" spellcheck="false"></textarea>
        {:else}
          <input class="field" bind:value={command} placeholder="e.g. log --oneline --graph" spellcheck="false" autocomplete="off" />
        {/if}
      </label>
      {#if formError}<p class="form-error" role="alert">{formError}</p>{/if}
      <div class="form-bottom">
        <span class="spacer"></span>
        <button class="btn text" type="button" onclick={() => (editing = '')}>Cancel</button>
        <InkButton type="submit" disabled={gitConfig.busy}>Done</InkButton>
      </div>
    </form>
  {/snippet}

  <div role="list" class="list">
    {#if editing === '+'}<div role="listitem">{@render form()}</div>{/if}
    {#each aliases as a (a.name + a.own)}
      <div role="listitem">
        {#if editing === a.name && a.own}
          {@render form()}
        {:else}
          <div class="alias" class:inherited={!a.own}>
            <code class="name">{a.name}</code>
            <div class="body">
              {#if a.shell}
                <pre class="code code-ground">{a.value.slice(1)}</pre>
              {:else}
                <code class="value">{a.value}</code>
              {/if}
              <div class="meta-line">
                {#if a.shell}<span class="tag sepia">shell</span>{/if}
                {#if !a.own}<span class="tag" title={a.file}>{a.scope}</span>{/if}
              </div>
              {#each a.warnings as w (w)}
                <p class="warning"><Icon name="warning" size={14} />{w}</p>
              {/each}
            </div>
            <div class="actions">
              {#if a.own}
                <button class="btn text" onclick={() => open(a)}>Edit</button>
                {#each targets as t (t.path)}
                  <button class="btn text" onclick={() => copy(a, t.path)}>Copy to {destination(t.path)}</button>
                {/each}
                <button class="btn text" onclick={() => remove(a)}>Delete</button>
              {:else}
                <button class="btn text" onclick={() => copy(a)}>Copy here</button>
              {/if}
            </div>
          </div>
        {/if}
      </div>
    {:else}
      {#if editing !== '+'}
        <p class="empty"><span class="hand">none yet</span> No aliases here or in the files git reads with it.</p>
      {/if}
    {/each}
  </div>

  <aside class="aside">
    <span class="hand">heads up</span>
    <p>Git Revenge shows aliases but never runs them. Shell aliases are code, and they run with your permissions when you type them.</p>
  </aside>
</div>

<style>
  .aliases {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .intro {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px 24px;
  }

  .intro p {
    flex: 1 1 360px;
    max-width: 68ch;
    margin: 0;
    color: var(--ink-soft);
  }

  code {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .intro code {
    padding: 1px 5px;
    border-radius: 3px;
    background: var(--paper-warm);
    color: var(--accent-b);
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

  .add-toggle:hover:not(:disabled) {
    background: var(--wash-hover);
  }

  .plus {
    font-family: var(--font-hand);
    font-size: 24px;
    line-height: 0;
    color: var(--link);
  }

  .copied {
    margin: 0;
    font-style: italic;
    color: var(--success);
  }

  .list {
    display: flex;
    flex-direction: column;
  }

  .alias {
    display: grid;
    grid-template-columns: 150px minmax(0, 1fr) auto;
    gap: 20px;
    align-items: start;
    padding: 14px 8px 14px 14px;
    border-top: 1px dotted var(--paper-edge);
    transition: background-color var(--dur-base);
  }

  .alias:hover {
    background: var(--wash-hover);
  }

  .alias.inherited {
    color: var(--ink-mute);
  }

  .alias .name {
    font-size: 16px;
    color: var(--accent-b);
    overflow-wrap: anywhere;
  }

  .alias.inherited .name {
    color: var(--ink-mute);
  }

  .body {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
  }

  .value {
    font-size: 14px;
    color: var(--ink-soft);
    overflow-wrap: anywhere;
  }

  .alias.inherited .value {
    color: var(--ink-mute);
  }

  .meta-line {
    display: flex;
    gap: 6px;
  }

  .meta-line:empty {
    display: none;
  }

  /* Shell commands sit on the dark code ground, with the brand's rose smear on top. */
  .code {
    position: relative;
    margin: 0;
    padding: 10px 14px;
    border: none;
    border-radius: 4px;
    background: var(--code-bg);
    color: var(--code-text);
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.6;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  pre.code::before {
    content: '';
    position: absolute;
    left: 18px;
    top: -4px;
    width: 46px;
    height: 8px;
    background: var(--accent);
    opacity: 0.75;
    filter: url(#wc-edge);
    border-radius: 45% 55% 50% 50% / 55% 45%;
  }

  textarea.code {
    resize: vertical;
  }

  textarea.code:focus {
    outline: 2px solid var(--link);
    outline-offset: 2px;
  }

  .warning {
    display: flex;
    align-items: baseline;
    gap: 8px;
    margin: 0;
    font-size: 14px;
    color: var(--warning);
  }

  .warning > :global(svg) {
    flex-shrink: 0;
    transform: translateY(2px);
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 2px;
    max-width: 300px;
  }

  .form {
    display: flex;
    flex-direction: column;
    gap: 14px;
    margin: 6px 0;
    padding: 20px 22px 18px;
    background: var(--paper-warm);
    border-radius: 10px;
  }

  .form-top,
  .form-bottom {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 12px 16px;
  }

  .form-bottom {
    align-items: center;
  }

  .field.name {
    width: 180px;
    font-size: 15px;
    color: var(--accent-b);
  }

  .spacer {
    flex: 1;
  }

  .empty {
    display: flex;
    align-items: baseline;
    gap: 14px;
    margin: 0;
    padding: 24px 0;
    color: var(--ink-mute);
  }

  .empty .hand,
  .aside .hand {
    color: var(--accent-c);
  }

  .aside {
    display: flex;
    align-items: flex-start;
    gap: 14px;
    max-width: 62ch;
  }

  .aside .hand {
    font-size: 24px;
    transform: rotate(-3deg);
    white-space: nowrap;
  }

  .aside p {
    margin: 0;
    font-size: 14px;
    color: var(--ink-mute);
  }

  @media (prefers-reduced-motion: reduce) {
    .alias {
      transition: none;
    }
  }
</style>
