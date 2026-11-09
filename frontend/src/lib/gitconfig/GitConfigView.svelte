<script lang="ts">
  // The Git Config tab (#195): the config files and views down the side, the selected file's
  // working copy beside them, saved or discarded from the header.
  import { formatShortcut } from '../commands.svelte'
  import Icon from '../ui/Icon.svelte'
  import InkButton from '../ui/InkButton.svelte'
  import Modal from '../ui/Modal.svelte'
  import AliasList from './AliasList.svelte'
  import ConfigTable from './ConfigTable.svelte'
  import HistoryView from './HistoryView.svelte'
  import { gitConfig, problemText, VIEWS } from './config.svelte'
  import RawEditor from './RawEditor.svelte'
  import { baseName, type Scope } from './scopes'

  let scope = $derived(gitConfig.scope)
  let doc = $derived(gitConfig.doc)
  let outside = $derived(scope ? gitConfig.outside[scope.path] : false)
  /** Aliases set in the selected file, for the rail. */
  let aliasCount = $derived(doc?.entries.filter((e) => /^alias\.[^.]+$/i.test(e.key)).length ?? 0)

  /** The handwritten note over the title, saying what the file applies to. */
  function eyebrow(s: Scope): string {
    switch (s.kind) {
      case 'global':
        return 'every repository on this machine'
      case 'local':
        return `only in ${gitConfig.label(s).replace(/^Local · /, '')}`
      case 'file':
        return 'read in by an include'
    }
  }

  function title(s: Scope): string {
    switch (s.kind) {
      case 'global':
        return 'Global config'
      case 'local':
        return 'Repository config'
      case 'file':
        return baseName(s.path)
    }
  }

  /** The file as the rail shows it under the scope's name. */
  function shortPath(s: Scope): string {
    if (s.kind === 'local') return '.git/config'
    if (s.kind === 'file') return 'included file'
    const parts = s.path.split(/[\\/]/)
    return parts.at(-2) === 'git' ? 'git/config' : `~/${parts.at(-1)}`
  }

  function onKey(e: KeyboardEvent) {
    // Mod+S saves, as in any editor.
    if ((e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 's') {
      e.preventDefault()
      // The raw editor may still hold keystrokes, so this saves whenever there's a file; a
      // clean one saves as a no-op.
      if (doc) void gitConfig.save()
    }
  }
</script>

<svelte:window onkeydown={onKey} />

<div class="config">
  <nav class="rail" aria-label="Config files">
    <span class="hand brand">git config</span>
    <div class="group">
      <h2 class="meta">Files</h2>
      {#each gitConfig.scopes as s (s.path)}
        {@const on = s.path === scope?.path}
        <button class="item" class:on aria-current={on ? 'page' : undefined} title={s.path} onclick={() => gitConfig.select(s.path)}>
          <span class="names">
            <span>{gitConfig.label(s).replace(/^File · /, '')}</span>
            <span class="path">{shortPath(s)}</span>
          </span>
          {#if gitConfig.docs[s.path]?.dirty}<span class="tag rose">edited</span>{/if}
        </button>
      {/each}
    </div>
    <div class="group">
      <h2 class="meta">View</h2>
      {#each VIEWS as v (v.id)}
        <button class="item" class:on={gitConfig.view === v.id} aria-current={gitConfig.view === v.id ? 'page' : undefined} onclick={() => (gitConfig.view = v.id)}
          >{v.label}{#if v.id === 'aliases' && aliasCount > 0}<span class="count">{aliasCount}</span>{/if}</button
        >
      {/each}
    </div>
    <p class="aside">Saving backs up the file first, so a change is never lost for good.</p>
  </nav>

  <section class="main" aria-label="Git config">
    {#if scope}
      <header class="head">
        <div class="titles">
          <span class="hand eyebrow">{eyebrow(scope)}</span>
          <h1>{title(scope)}</h1>
          <span class="path" title={scope.path}>{scope.path}{#if doc && !doc.exists}<span class="tag sepia">new file</span>{/if}</span>
        </div>
        <div class="actions">
          {#if doc?.problem}
            <span class="unsaved bad">{doc.problem.line ? `fix line ${doc.problem.line} to save` : 'fix the file to save'}</span>
          {:else if doc?.dirty}
            <span class="unsaved">unsaved changes</span>
          {/if}
          {#if doc?.dirty}
            <InkButton variant="secondary" disabled={gitConfig.busy} onclick={() => gitConfig.reload()}>Discard</InkButton>
          {/if}
          <InkButton disabled={!doc?.dirty || gitConfig.busy || !!doc?.problem} onclick={() => gitConfig.save()}>
            Save <kbd>{formatShortcut('Mod+S')}</kbd>
          </InkButton>
        </div>
      </header>
    {/if}

    {#if scope?.kind === 'global' && gitConfig.globalAlt}
      <p class="note"><Icon name="info" size={14} />Git also reads <code>{gitConfig.globalAlt}</code>. Changes are saved to the file above.</p>
    {/if}
    {#if outside}
      <div class="banner caution" role="alert">
        <Icon name="warning" size={16} />
        <span class="message"><strong>Something else changed this file</strong> while you were editing it. Keep your version and save it over theirs, or reload and lose your edits.</span>
        <button class="btn secondary" onclick={() => gitConfig.reload()}>Reload theirs</button>
        <button class="btn secondary" onclick={() => gitConfig.keepMine()}>Keep mine</button>
      </div>
    {/if}
    {#if gitConfig.error}
      <div class="banner danger" role="alert">
        <Icon name="warning" size={16} />
        <span class="message">{gitConfig.error}</span>
        <button class="btn secondary" onclick={() => (gitConfig.error = '')}>Dismiss</button>
      </div>
    {:else if doc?.problem && gitConfig.view !== 'raw'}
      <div class="banner danger" role="alert">
        <Icon name="warning" size={16} />
        <span class="message">Git can't read this file ({problemText(doc.problem)}), so its settings can't be shown or saved.</span>
        <button class="btn secondary" onclick={() => (gitConfig.view = 'raw')}>Fix it in raw text</button>
      </div>
    {/if}

    {#if doc && scope}
      {#key scope.path}
        {#if gitConfig.view === 'raw'}
          <RawEditor />
        {:else if gitConfig.view === 'aliases'}
          <AliasList />
        {:else if gitConfig.view === 'history'}
          <HistoryView />
        {:else}
          <ConfigTable />
        {/if}
      {/key}
    {/if}
  </section>
</div>

{#if gitConfig.conflict}
  <Modal title="Save over the changed file?" onClose={() => (gitConfig.conflict = null)} width={640}>
    <p class="form-note">
      <code>{gitConfig.conflict.path}</code> changed on disk since you opened it. Saving replaces it with your version, and the file as it is now is backed up first.
    </p>
    <pre class="disk" aria-label="The file on disk now">{gitConfig.conflict.disk}</pre>
    <div class="form-actions">
      <button class="btn text" onclick={() => gitConfig.reload()}>Discard mine and reload</button>
      <InkButton onclick={() => gitConfig.save(true)}>Save mine</InkButton>
    </div>
  </Modal>
{/if}

<style>
  /* The rail stays put while the file scrolls beside it. */
  .config {
    display: flex;
    height: 100%;
  }

  .rail {
    flex: 0 0 248px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 24px;
    padding: 26px 14px 22px;
    background: var(--paper-warm);
    border-right: 1px solid var(--paper-edge);
  }

  .brand {
    margin-left: 12px;
    font-size: 26px;
    line-height: 1;
    transform-origin: left;
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .group h2 {
    margin: 0;
    padding: 0 12px 6px;
    font-weight: 400;
  }

  .item {
    position: relative;
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 40px;
    padding: 8px 12px;
    background: none;
    border: none;
    border-radius: var(--r-md);
    color: var(--ink-soft);
    font-size: 15px;
    text-align: left;
  }

  .item:hover:not(:disabled) {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .item.on {
    color: var(--ink);
  }

  /* The active item wears the tab bar's torn rose mark, upright. */
  .item.on::before {
    content: '';
    position: absolute;
    left: 0;
    top: 8px;
    bottom: 8px;
    width: 4px;
    background: var(--link);
    filter: url(#wc-edge);
    border-radius: 45% 55% 50% 50% / 55% 45%;
    opacity: 0.9;
  }

  .names {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .names > span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .path {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-faint);
  }

  .count {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-faint);
  }

  .aside {
    margin: auto 12px 0;
    font-size: 13px;
    line-height: 1.5;
    color: var(--ink-mute);
  }

  .main {
    flex: 1;
    min-width: 0;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 20px;
    padding: 30px 40px 48px;
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 16px 24px;
  }

  .titles {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .eyebrow {
    margin-bottom: 6px;
    color: var(--link);
    transform: rotate(-1.5deg);
    transform-origin: left;
    align-self: flex-start;
  }

  h1 {
    margin: 0;
    font-size: 34px;
    font-weight: 400;
    letter-spacing: -0.02em;
    line-height: 1.1;
    font-variation-settings:
      'SOFT' 50,
      'WONK' 1;
  }

  .titles .path {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-top: 8px;
    font-size: 13px;
    color: var(--ink-mute);
    overflow-wrap: anywhere;
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 14px;
  }

  .unsaved {
    font-style: italic;
    color: var(--ink-mute);
  }

  .unsaved.bad {
    color: var(--danger);
  }

  kbd {
    font-family: var(--font-mono);
    font-size: 12px;
    opacity: 0.8;
  }

  .note,
  .banner {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px 14px;
    margin: 0;
    padding: 10px 16px;
    border-radius: var(--r-md);
  }

  .note {
    padding: 0;
    color: var(--ink-mute);
  }

  .caution {
    background: color-mix(in srgb, var(--ink-ochre) 20%, transparent);
  }

  .caution > :global(svg) {
    color: var(--warning);
  }

  .danger {
    background: var(--danger-wash);
  }

  .danger > :global(svg) {
    color: var(--danger);
  }

  .message {
    flex: 1 1 320px;
    min-width: 0;
  }

  .message strong {
    font-weight: 600;
  }

  code,
  .disk {
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .disk {
    max-height: 40vh;
    overflow: auto;
    margin: 12px 0;
    padding: 12px 14px;
    background: var(--code-bg);
    color: var(--code-text);
    border-radius: var(--r-sm);
    white-space: pre;
  }
</style>
