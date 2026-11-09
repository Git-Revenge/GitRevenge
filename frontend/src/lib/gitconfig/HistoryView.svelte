<script lang="ts">
  // The History view of the Git Config tab (#228): the selected file's backups, one compared
  // with the file as it is now, and restoring it. A restore backs up the current file first.
  import { GitConfigBackups, GitConfigBackupText, GitConfigFileText, RestoreGitConfig } from '../../../wailsjs/go/app/App.js'
  import type { gitconfig } from '../../../wailsjs/go/models'
  import InkButton from '../ui/InkButton.svelte'
  import Modal from '../ui/Modal.svelte'
  import { gitConfig } from './config.svelte'
  import { changedLines, dayLabel, splitRows, unifiedRows } from './history'

  let backups = $state<gitconfig.Backup[]>([])
  let selected = $state('')
  let text = $state('')
  let current = $state('')
  let split = $state(false)
  let loaded = $state(false)
  let confirming = $state(false)

  let doc = $derived(gitConfig.doc)
  let choice = $derived(backups.find((b) => b.id === selected))
  let rows = $derived(choice ? (split ? splitRows(text, current) : unifiedRows(text, current)) : [])
  let differs = $derived(choice ? changedLines(text, current) : 0)
  let groups = $derived.by(() => {
    const out: { day: string; items: gitconfig.Backup[] }[] = []
    for (const b of backups) {
      const day = dayLabel(new Date(b.time))
      if (out.at(-1)?.day !== day) out.push({ day, items: [] })
      out.at(-1)!.items.push(b)
    }
    return out
  })

  const time = (b: gitconfig.Backup) => new Date(b.time).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
  const size = (n: number) => (n < 1024 ? `${n} bytes` : `${(n / 1024).toFixed(1)} KB`)

  // Reload whenever the file's state changes: a save adds a backup, a restore two.
  $effect(() => {
    const path = doc?.path
    void doc?.text
    if (!path) return
    void load(path)
  })

  async function load(path: string) {
    const [list, now] = await Promise.all([gitConfig.call(() => GitConfigBackups(path)), gitConfig.call(() => GitConfigFileText(path))])
    backups = list ?? []
    current = now ?? ''
    loaded = true
    if (!backups.some((b) => b.id === selected)) await pick(backups[0]?.id ?? '')
    else await pick(selected)
  }

  async function pick(id: string) {
    selected = id
    const path = doc?.path
    text = id && path ? ((await gitConfig.call(() => GitConfigBackupText(path, id))) ?? '') : ''
  }

  async function restore() {
    confirming = false
    const path = doc?.path
    if (!path || !selected) return
    const id = selected
    const d = await gitConfig.call(() => RestoreGitConfig(path, id))
    if (!d) return
    gitConfig.docs[path] = d
    await gitConfig.refreshEffective()
  }
</script>

<div class="history">
  <p class="lede">Each save keeps the file as it was, in your app data folder. The last 20 are kept.</p>

  {#if loaded && backups.length === 0}
    <p class="empty"><span class="hand">nothing yet</span> This file has no backups. Saving it keeps the version it replaces.</p>
  {:else}
    <div class="layout">
      <div class="list" role="listbox" aria-label="Backups">
        {#each groups as g (g.day)}
          <h2 class="meta">{g.day}</h2>
          {#each g.items as b (b.id)}
            <button role="option" aria-selected={b.id === selected} class="backup" class:on={b.id === selected} onclick={() => pick(b.id)}>
              <span>{time(b)}</span>
              <span class="detail">{b.id === selected ? (differs === 0 ? 'same as now' : `${differs} ${differs === 1 ? 'line differs' : 'lines differ'}`) : size(b.size)}</span>
            </button>
          {/each}
        {/each}
      </div>

      {#if choice}
        <section class="compare" aria-label="Backup from {time(choice)} compared with the file now">
          <div class="compare-head">
            <div class="title">
              <span class="when">{dayLabel(new Date(choice.time))}, {time(choice)}</span>
              <span class="against">compared with the file now</span>
            </div>
            <span class="segmented" role="group" aria-label="Diff layout">
              <button aria-pressed={!split} onclick={() => (split = false)}>unified</button>
              <button aria-pressed={split} onclick={() => (split = true)}>split</button>
            </span>
          </div>

          {#if differs === 0}
            <p class="same"><span class="hand">identical</span> This backup matches the file as it is now.</p>
          {:else}
            <div class="diff code-ground" class:split role="region" aria-label="Changes">
              {#each rows as r, i (i)}
                {#if r.kind === 'gap'}
                  <div class="gap" aria-hidden="true">⋯</div>
                {:else if split}
                  <div class="line {r.kind === 'add' && !r.text ? 'blank' : r.kind}"><span class="mark">{r.kind === 'del' ? '−' : ''}</span><span>{r.text}</span></div>
                  <div class="line {r.right === undefined ? 'blank' : r.kind === 'eq' ? 'eq' : 'add'}">
                    <span class="mark">{r.kind !== 'eq' && r.right !== undefined ? '+' : ''}</span><span>{r.right ?? ''}</span>
                  </div>
                {:else}
                  <div class="line {r.kind}">
                    <span class="mark" aria-label={r.kind === 'del' ? 'removed' : r.kind === 'add' ? 'added' : undefined}>{r.kind === 'del' ? '−' : r.kind === 'add' ? '+' : ''}</span><span
                      >{r.text}</span
                    >
                  </div>
                {/if}
              {/each}
            </div>
          {/if}

          <div class="restore">
            <p>Restoring replaces the file with this backup. The file as it is now is backed up first, so you can undo it from here.</p>
            <InkButton disabled={gitConfig.busy || differs === 0} onclick={() => (doc?.dirty ? (confirming = true) : restore())}>Restore {time(choice)}</InkButton>
          </div>
        </section>
      {/if}
    </div>
  {/if}
</div>

{#if confirming && choice}
  <Modal title="Restore and discard your edits?" onClose={() => (confirming = false)}>
    <p class="form-note">Restoring the {time(choice)} backup replaces the file and discards the changes you haven't saved.</p>
    <div class="form-actions">
      <button class="btn text" onclick={() => (confirming = false)}>Keep editing</button>
      <InkButton variant="danger" onclick={restore}>Discard and restore</InkButton>
    </div>
  </Modal>
{/if}

<style>
  .history {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .lede {
    margin: 0;
    color: var(--ink-soft);
  }

  .layout {
    display: flex;
    flex-wrap: wrap;
    gap: 28px;
    align-items: flex-start;
  }

  .list {
    flex: 1 1 220px;
    max-width: 260px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .list h2 {
    margin: 0;
    padding: 12px 14px 6px;
    font-weight: 400;
  }

  .list h2:first-child {
    padding-top: 0;
  }

  .backup {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-height: 44px;
    padding: 10px 14px 10px 18px;
    background: none;
    border: none;
    border-radius: var(--r-md);
    color: var(--ink-soft);
    font-size: 15px;
    text-align: left;
  }

  .backup:hover:not(:disabled) {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .backup.on {
    background: var(--paper-warm);
    color: var(--ink);
  }

  .backup.on::before {
    content: '';
    position: absolute;
    left: 0;
    top: 10px;
    bottom: 10px;
    width: 4px;
    background: var(--link);
    filter: url(#wc-edge);
    border-radius: 45% 55% 50% 50% / 55% 45%;
    opacity: 0.9;
  }

  .detail {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-faint);
  }

  .compare {
    flex: 999 1 480px;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .compare-head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px 20px;
  }

  .title {
    flex: 1;
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 10px;
  }

  .when {
    font-size: 20px;
    font-weight: 500;
    letter-spacing: -0.01em;
  }

  .against {
    color: var(--ink-mute);
  }

  .diff {
    position: relative;
    padding: 14px 0;
    border-radius: var(--r-sm);
    background: var(--code-bg);
    color: var(--code-text);
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.75;
    tab-size: 4;
    overflow-x: auto;
  }

  .diff::before {
    content: '';
    position: absolute;
    right: 36px;
    top: -5px;
    width: 58px;
    height: 10px;
    background: var(--accent);
    opacity: 0.8;
    filter: url(#wc-edge);
    border-radius: 45% 55% 50% 50% / 55% 45%;
  }

  .diff.split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }

  .gap {
    grid-column: 1 / -1;
    padding: 2px 28px;
    color: var(--code-comment);
  }

  .line {
    display: grid;
    grid-template-columns: 28px minmax(0, 1fr);
    padding-right: 16px;
    white-space: pre;
  }

  .mark {
    text-align: center;
    color: var(--code-comment);
    user-select: none;
  }

  /* Side by side, long lines wrap within their own column. */
  .split .line {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .line.add {
    background: color-mix(in srgb, var(--syn-string) 18%, transparent);
  }

  .line.add .mark {
    color: var(--syn-string);
  }

  .line.del {
    background: color-mix(in srgb, var(--syn-invalid) 17%, transparent);
  }

  .line.del .mark {
    color: var(--syn-invalid);
  }

  .line.blank {
    background: color-mix(in srgb, var(--code-text) 4%, transparent);
  }

  .same,
  .empty {
    display: flex;
    align-items: baseline;
    gap: 14px;
    margin: 0;
    color: var(--ink-mute);
  }

  .same .hand,
  .empty .hand {
    color: var(--accent-c);
  }

  .restore {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 14px 20px;
  }

  .restore p {
    flex: 1 1 300px;
    margin: 0;
    font-size: 14px;
    color: var(--ink-mute);
  }
</style>
