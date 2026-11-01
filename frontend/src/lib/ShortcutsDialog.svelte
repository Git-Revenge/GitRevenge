<script lang="ts">
  import { commands, formatShortcut } from './commands.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  let { onClose }: { onClose: () => void } = $props()

  /** Keys handled inside a view rather than by a command. */
  interface Row {
    group: string
    title: string
    keys: string[]
    /** Word between the keys: "or" for alternatives, "to" for a range. */
    joiner?: string
  }

  const FIXED: Row[] = [
    { group: 'View', title: 'Close a diff, blame or history view', keys: ['Escape'] },
    { group: 'Graph', title: 'Move the selection', keys: ['↑', '↓'] },
    { group: 'Graph', title: 'Move a page', keys: ['PageUp', 'PageDown'] },
    { group: 'Graph', title: 'First or last commit', keys: ['Home', 'End'] },
    { group: 'Graph', title: 'Next or previous search match', keys: ['Enter', 'Shift+Enter'] },
    { group: 'Sidebar', title: 'Move through the list', keys: ['↑', '↓'] },
    { group: 'Sidebar', title: 'Open or close a folder, or step in and out', keys: ['→', '←'] },
    { group: 'Sidebar', title: 'Show in the graph', keys: ['Enter'] },
    { group: 'Sidebar', title: 'Check out', keys: ['Space'] },
    { group: 'Sidebar', title: 'Open the menu', keys: ['Menu', 'Shift+F10'] },
    { group: 'Commit', title: 'Commit the staged changes', keys: ['Mod+Enter'] },
    { group: 'Dialogs and menus', title: 'Close', keys: ['Escape'] },
  ]

  const ORDER = ['App', 'Tabs', 'Repository', 'Branch', 'View', 'Graph', 'Sidebar', 'Commit', 'Dialogs and menus']

  // Read once on open; tab numbers collapse into one line rather than nine.
  const groups = (() => {
    const rows: Row[] = []
    let tabNumbers = false
    for (const c of commands.all()) {
      // Listed once among the fixed keys, whether or not a file view is open now.
      if (!c.shortcut || c.id === 'view.close') continue
      if (/^tabs\.n\d$/.test(c.id)) {
        if (!tabNumbers) rows.push({ group: 'Tabs', title: 'Go to a tab by number', keys: ['Mod+1', 'Mod+9'], joiner: 'to' })
        tabNumbers = true
        continue
      }
      // Commands named after the current state get a general name here.
      const title = c.id === 'tabs.close' ? 'Close the tab' : c.id === 'repo.undo' ? 'Undo' : c.id === 'repo.redo' ? 'Redo' : c.title
      rows.push({ group: c.group, title, keys: [c.shortcut, ...(c.aliases ?? [])] })
    }
    rows.push(...FIXED)
    const by: Record<string, typeof rows> = {}
    for (const r of rows) (by[r.group] ??= []).push(r)
    return Object.entries(by).sort((a, b) => (ORDER.indexOf(a[0]) + 1 || 99) - (ORDER.indexOf(b[0]) + 1 || 99))
  })()
</script>

<Modal title="Keyboard shortcuts" eyebrow="faster by hand" width={780} {onClose}>
  <p class="form-note">Repository shortcuts appear here while a repository is open.</p>
  <div class="columns">
    {#each groups as [group, rows] (group)}
      <section>
        <h3 class="meta">{group}</h3>
        <dl>
          {#each rows as r (r.title)}
            <dt>{r.title}</dt>
            <dd>
              {#each r.keys as k, i (i)}{#if i > 0}<span class="or">{r.joiner ?? 'or'}</span>{/if}<kbd>{formatShortcut(k)}</kbd>{/each}
            </dd>
          {/each}
        </dl>
      </section>
    {/each}
  </div>
  <div class="form-actions">
    <InkButton onclick={onClose}>Done</InkButton>
    <InkButton
      variant="secondary"
      onclick={() => {
        onClose()
        commands.keybindingsOpen = true
      }}>Customize…</InkButton
    >
  </div>
</Modal>

<style>
  .columns {
    columns: 2 300px;
    column-gap: 28px;
  }

  section {
    break-inside: avoid;
    margin-bottom: 16px;
  }

  h3 {
    margin: 0 0 6px;
    padding-bottom: 4px;
    border-bottom: 1px solid var(--paper-edge);
    font-weight: normal;
    color: var(--ink-soft);
  }

  dl {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 6px 12px;
    align-items: center;
    margin: 0;
  }

  dt {
    font-size: 14px;
  }

  dd {
    margin: 0;
    display: flex;
    align-items: center;
    gap: 4px;
    justify-content: flex-end;
  }

  kbd {
    padding: 1px 6px;
    border: 1px solid var(--paper-edge);
    border-bottom-width: 2px;
    border-radius: 4px;
    background: var(--paper-warm);
    font-family: var(--font-mono);
    font-size: 12px;
    white-space: nowrap;
  }

  .or {
    font-size: 12px;
    font-style: italic;
    color: var(--ink-mute);
  }
</style>
