<script lang="ts">
  // Rebinds command shortcuts: every command the palette offers, its keys, and a way to
  // record new ones. Changes save to the settings file as they're made.
  import { commands, formatShortcut, isMac, type Command } from './commands.svelte'
  import { errorText } from './errors'
  import { eventToShortcut } from './shortcuts'
  import Icon from './ui/Icon.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  let { onClose }: { onClose: () => void } = $props()

  let query = $state('')
  /** The command whose new shortcut is being recorded, or "". */
  let recording = $state('')
  /** A recorded shortcut another command already uses, waiting for a decision. */
  let pending = $state<{ id: string; keys: string; others: Command[] } | null>(null)
  let error = $state('')

  // Per-tab commands name a repository path; tab numbers and close cover them generally.
  const bindable = (c: Command) => !c.id.startsWith('tabs.') || /^tabs\.(n\d|close|home)$/.test(c.id)

  let list = $derived.by(() => {
    void commands.overrides
    const q = query.trim().toLowerCase()
    return commands
      .all()
      .filter(bindable)
      .filter((c) => !q || `${c.group} ${c.title} ${c.shortcut ?? ''}`.toLowerCase().includes(q))
      .sort((a, b) => a.group.localeCompare(b.group) || a.title.localeCompare(b.title))
  })

  async function save(id: string, keys: string | null) {
    error = ''
    try {
      await commands.rebind(id, keys)
    } catch (e) {
      error = errorText(e)
    }
  }

  async function assign(id: string, keys: string, others: Command[]) {
    // The shortcut moves here: the commands that had it lose it.
    for (const o of others) await save(o.id, '')
    await save(id, keys)
    pending = null
  }

  /** While recording, every key goes to the recorder, not to commands or the dialog. */
  function onKey(e: KeyboardEvent) {
    if (!recording) return
    e.preventDefault()
    e.stopImmediatePropagation()
    if (e.key === 'Escape' && !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey) {
      recording = ''
      return
    }
    if (e.key === 'Backspace' && !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey) {
      const id = recording
      recording = ''
      void save(id, '')
      return
    }
    const keys = eventToShortcut(e, isMac)
    if (!keys) return
    const id = recording
    recording = ''
    const others = commands.conflicts(keys, id)
    if (others.length) pending = { id, keys, others }
    else void save(id, keys)
  }

  const changed = (c: Command) => c.id in commands.overrides
</script>

<svelte:window onkeydowncapture={onKey} />

<Modal title="Keyboard shortcuts" eyebrow="your own keys" width={760} {onClose}>
  <div class="bar">
    <label class="search">
      <Icon name="search" size={14} />
      <input type="search" placeholder="Find a command" aria-label="Find a command" bind:value={query} />
    </label>
    {#if Object.keys(commands.overrides).length}
      <button class="btn quiet" onclick={() => Object.keys(commands.overrides).forEach((id) => void save(id, null))}>Reset all</button>
    {/if}
  </div>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if pending}
    <div class="conflict" role="alert">
      <Icon name="warning" size={16} />
      <span>
        <kbd>{formatShortcut(pending.keys)}</kbd> is already used by {pending.others.map((o) => o.title).join(', ')}.
      </span>
      <button class="btn quiet" onclick={() => pending && assign(pending.id, pending.keys, pending.others)}>Use it here</button>
      <button class="btn quiet" onclick={() => (pending = null)}>Cancel</button>
    </div>
  {/if}
  <p class="form-note">Choose Change, then press the keys. Escape cancels; Backspace removes the shortcut.</p>
  <div class="list">
    <table>
      <tbody>
        {#each list as c (c.id)}
          <tr class:recording={recording === c.id}>
            <td class="group meta">{c.group}</td>
            <td class="title">{c.title}</td>
            <td class="keys">
              {#if recording === c.id}
                <span class="listening" role="status">Press the keys…</span>
              {:else if c.shortcut}
                <kbd>{formatShortcut(c.shortcut)}</kbd>
              {:else}
                <span class="none">none</span>
              {/if}
            </td>
            <td class="actions">
              <button class="btn sm quiet" onclick={() => (recording = recording === c.id ? '' : c.id)}>{recording === c.id ? 'Cancel' : 'Change'}</button>
              {#if changed(c)}
                <button class="btn sm quiet" onclick={() => save(c.id, null)} title={c.defaultShortcut ? `Back to ${formatShortcut(c.defaultShortcut)}` : 'Back to no shortcut'}>Reset</button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  <InkButton onclick={onClose}>Done</InkButton>
</Modal>

<style>
  .bar {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 8px;
  }

  .search {
    flex: 1;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 0 8px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-sm);
    color: var(--ink-mute);
  }

  .search input {
    flex: 1;
    border: none;
    background: none;
    padding: 6px 0;
  }

  .list {
    max-height: 55vh;
    overflow: auto;
    margin-bottom: 14px;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13.5px;
  }

  td {
    padding: 4px 6px;
    border-top: 1px dotted var(--paper-edge);
    vertical-align: middle;
  }

  .group {
    width: 1%;
    white-space: nowrap;
  }

  .keys {
    width: 1%;
    white-space: nowrap;
  }

  .actions {
    width: 1%;
    white-space: nowrap;
    text-align: right;
  }

  kbd {
    font-family: var(--font-mono);
    font-size: 12px;
    padding: 1px 6px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-sm);
    background: var(--paper-warm);
  }

  .none {
    color: var(--ink-faint);
    font-style: italic;
  }

  tr.recording {
    background: var(--selected);
  }

  .listening {
    color: var(--link);
    font-style: italic;
  }

  .conflict {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
    padding: 6px 10px;
    background: var(--danger-wash);
    border-radius: var(--r-sm);
    color: var(--ink);
  }

  .conflict :global(svg) {
    color: var(--warning);
  }

  .error {
    color: var(--danger);
  }
</style>
