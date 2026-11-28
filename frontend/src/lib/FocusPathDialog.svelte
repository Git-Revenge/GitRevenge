<script lang="ts">
  // "Focus the timeline on a folder or file…" from the palette (#1060): a box that suggests the
  // repository's folders and files as you type, read from the checked-out commit's listing.
  import { FocusSuggestions, SetGraphPath } from '../../wailsjs/go/app/App.js'
  import { errorText } from './errors'
  import { showTimeline } from './focusPathItem'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  let { path, onClose }: { path: string; onClose: () => void } = $props()
  const id = $props.id()

  let typed = $state('')
  let offered = $state<string[]>([])
  /** The suggestion the arrow keys are on, or -1 for what's typed. */
  let at = $state(-1)
  let error = $state('')
  let busy = $state(false)

  // A short pause first, so typing doesn't ask on every key; a stale answer is dropped.
  $effect(() => {
    const now = typed
    let stale = false
    const timer = setTimeout(async () => {
      const list = (await FocusSuggestions(path, now).catch(() => null)) ?? []
      if (stale) return
      offered = list
      at = -1
    }, 80)
    return () => {
      stale = true
      clearTimeout(timer)
    }
  })

  /** What a submit focuses on: the suggestion picked, or what's typed, with a folder's slash
   * when it names one that's offered. */
  function chosen(): string {
    if (at >= 0 && offered[at]) return offered[at]
    const t = typed.trim().replaceAll('\\', '/')
    return offered.includes(`${t}/`) ? `${t}/` : t
  }

  async function focusOn(target: string) {
    if (!target) {
      error = 'Type a folder or file in the repository.'
      return
    }
    busy = true
    error = ''
    try {
      await SetGraphPath(path, target)
      await showTimeline()
      onClose()
    } catch (e) {
      error = errorText(e)
    } finally {
      busy = false
    }
  }

  function key(e: KeyboardEvent) {
    if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && offered.length) {
      e.preventDefault()
      const n = offered.length
      at = e.key === 'ArrowDown' ? (at + 1 >= n ? -1 : at + 1) : at <= -1 ? n - 1 : at - 1
    } else if (e.key === 'Tab' && !e.shiftKey && offered.length) {
      // Tab completes with the suggestion the keys are on, or the first: a folder's then
      // offers what's in it.
      e.preventDefault()
      typed = offered[Math.max(0, at)]
    }
  }
</script>

<Modal title="Focus the timeline on a folder or file" {onClose} dismissable={!busy}>
  <form
    onsubmit={(e) => {
      e.preventDefault()
      void focusOn(chosen())
    }}
  >
    <p class="message">Only the commits that changed something there show, with their branches. It lasts while the repository is open.</p>
    <label class="form-field">
      <span>Folder or file</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input
        bind:value={typed}
        onkeydown={key}
        placeholder="src/alerts/"
        autocomplete="off"
        spellcheck="false"
        autofocus
        role="combobox"
        aria-expanded={offered.length > 0}
        aria-controls="{id}-list"
        aria-autocomplete="list"
        aria-activedescendant={at >= 0 ? `${id}-${at}` : undefined}
      />
    </label>
    {#if offered.length}
      <ul class="offered" id="{id}-list" role="listbox" aria-label="Folders and files">
        {#each offered as p, i (p)}
          <li id="{id}-{i}" role="option" aria-selected={i === at}>
            <!-- The keys reach it through the box: the arrows pick it, Enter takes it. -->
            <span class="path" class:folder={p.endsWith('/')} onclick={() => void focusOn(p)} role="presentation">{p}</span>
          </li>
        {/each}
      </ul>
    {:else if typed.trim()}
      <p class="none" role="status">No folder or file at the checked-out commit is called that.</p>
    {/if}
    {#if error}<p class="form-error" role="alert">{error}</p>{/if}
    <div class="form-actions">
      <InkButton variant="secondary" onclick={onClose} disabled={busy}>Cancel</InkButton>
      <InkButton type="submit" variant="primary" disabled={busy}>Focus</InkButton>
    </div>
  </form>
</Modal>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .message,
  .none {
    margin: 0;
    font-size: 14px;
    line-height: 1.55;
    color: var(--ink-soft);
  }

  input {
    font-family: var(--font-mono);
  }

  .offered {
    margin: -8px 0 0;
    padding: 4px 0;
    list-style: none;
    max-height: 264px;
    overflow-y: auto;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-sm);
  }

  .path {
    display: block;
    padding: 6px 12px;
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--ink-soft);
    cursor: pointer;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .path.folder {
    color: var(--ink);
  }

  .path:hover,
  [aria-selected='true'] .path {
    background: var(--selection);
    color: var(--ink);
  }
</style>
