<script lang="ts">
  // The sidebar's Repository panel: worktrees, submodules and the way into LFS. It stays mounted
  // while other panels show, so the rail's badge and filter counts are always right.
  import { LFS, RemoveWorktree, Submodules, UpdateSubmodules, Worktrees } from '../../wailsjs/go/app/App.js'
  import type { app, git } from '../../wailsjs/go/models'
  import type { MenuItem } from './ContextMenu.svelte'
  import type { DialogSpec } from './Dialog.svelte'
  import { errorText } from './errors'
  import LFSDialog from './LFSDialog.svelte'
  import { canOpenWorktree, canRemoveWorktree, folderName, lfsNames, matchesFilter, submoduleStatus, worktreeNames, worktreeStatus, worktreeTooltip } from './repoPanel'
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'

  let {
    path,
    filter,
    openDialog,
    onMenu,
    onManageWorktrees,
    onStatus,
  }: {
    path: string
    filter: string
    openDialog: (spec: DialogSpec) => void
    onMenu?: (items: MenuItem[], x: number, y: number) => void
    onManageWorktrees?: () => void
    /** Reports how many submodules need updating and how many rows match the filter. */
    onStatus?: (status: { attention: number; matches: number }) => void
  } = $props()

  let trees = $state<git.Worktree[]>([])
  let subs = $state<git.Submodule[]>([])
  let lfs = $state<app.LFSInfo | null>(null)
  let busy = $state(false)
  let lfsOpen = $state(false)
  /** False until the first answer, so the count doesn't claim 0 worktrees. */
  let loaded = $state(false)

  /** Numbers each load, and remembers the newest each section has shown, so a slow answer
   * can't replace a newer one. */
  let loads = 0
  const newest = { trees: 0, subs: 0, lfs: 0 }

  /** Each section shows as soon as its own answer arrives: submodules and LFS can take seconds. */
  function load() {
    const n = ++loads
    const fresh = (section: keyof typeof newest) => {
      if (n < newest[section]) return false
      newest[section] = n
      return true
    }
    return Promise.all([
      session.run(() => Worktrees(path)).then((t) => {
        if (!fresh('trees')) return
        trees = t ?? []
        loaded = true
      }),
      session.run(() => Submodules(path)).then((s) => {
        if (fresh('subs')) subs = s ?? []
      }),
      session.run(() => LFS(path)).then((l) => {
        if (fresh('lfs')) lfs = l ?? null
      }),
    ])
  }

  $effect(() => {
    void session.revision[path]
    void load()
  })

  let shownTrees = $derived(trees.filter((w) => matchesFilter(filter, ...worktreeNames(w))))
  let shownSubs = $derived(subs.filter((s) => matchesFilter(filter, s.path)))
  let lfsFiles = $derived(lfs?.status.files.length ?? 0)
  let showLfs = $derived(!!lfs?.available && matchesFilter(filter, ...lfsNames(lfs.status.patterns, lfs.status.files)))
  let filtering = $derived(filter.trim().length > 0)
  let matches = $derived(shownTrees.length + shownSubs.length + (showLfs ? 1 : 0))

  $effect(() => {
    onStatus?.({ attention: subs.filter((s) => s.state !== 'ok').length, matches })
  })

  async function updateSubmodules() {
    busy = true
    await session.run(() => UpdateSubmodules(path))
    busy = false
    await load()
  }

  const join = (base: string, rel: string) => base.replace(/[\\/]+$/, '') + '/' + rel

  function askRemove(w: git.Worktree) {
    openDialog({
      title: `Remove the worktree ${folderName(w.path)}?`,
      message: `This deletes the folder ${w.path}. The repository and its branches stay.`,
      fields: [{ key: 'force', label: 'Remove it even with uncommitted changes', type: 'checkbox' }],
      submit: 'Remove worktree',
      danger: true,
      run: async (v) => {
        try {
          await RemoveWorktree(path, w.path, !!v.force)
          await load()
        } catch (e) {
          return errorText(e)
        }
      },
    })
  }

  function worktreeMenu(e: MouseEvent, w: git.Worktree) {
    e.preventDefault()
    const items: MenuItem[] = [
      { label: 'Open as a tab', icon: 'folder-open', disabled: !canOpenWorktree(w), run: () => session.open(w.path) },
      { label: 'Manage worktrees…', icon: 'worktree', run: () => onManageWorktrees?.() },
      { label: 'Remove worktree…', icon: 'trash', separator: true, disabled: !canRemoveWorktree(w), run: () => askRemove(w) },
    ]
    onMenu?.(items, e.clientX, e.clientY)
  }
</script>

{#if shownTrees.length || !filtering}
  <section>
    <div class="head">
      <h3 class="meta">Worktrees {#if loaded}<span class="count">{shownTrees.length.toLocaleString()}</span>{/if}</h3>
      <button class="btn sm quiet icon" onclick={onManageWorktrees} title="Add or remove worktrees" aria-label="Manage worktrees"><Icon name="plus" size={14} /></button>
    </div>
    <ul>
      {#each shownTrees as w (w.path)}
        {@const status = worktreeStatus(w)}
        <li>
          <button class="row" class:current={w.current} disabled={!canOpenWorktree(w) && !w.current} title={worktreeTooltip(w)} onclick={() => canOpenWorktree(w) && session.open(w.path)} oncontextmenu={(e) => worktreeMenu(e, w)}>
            {#if w.current}<span class="here"><span class="visually-hidden">This window: </span></span>{/if}
            <span class="name">{folderName(w.path)}</span>
            <span class="status {status.tone}">{status.text}</span>
          </button>
          {#if onMenu}
            <button class="more" aria-label="More for {folderName(w.path)}" aria-haspopup="menu" title="More" onclick={(e) => worktreeMenu(e, w)}><Icon name="more" size={14} /></button>
          {/if}
        </li>
      {/each}
    </ul>
  </section>
{/if}

{#if shownSubs.length}
  <section>
    <div class="head">
      <h3 class="meta">Submodules <span class="count">{shownSubs.length.toLocaleString()}</span></h3>
      <button class="btn sm quiet" disabled={busy} onclick={updateSubmodules} title="Initialize and update all submodules">Update</button>
    </div>
    <ul>
      {#each shownSubs as s (s.path)}
        <li>
          <button class="row" onclick={() => session.open(join(path, s.path))} title="Open {s.path} as a tab ({s.commit.slice(0, 7)})">
            <span class="name">{s.path}</span>
            {#if submoduleStatus(s.state)}<span class="status {s.state === 'conflict' ? 'danger' : 'warning'}">{submoduleStatus(s.state)}</span>{/if}
          </button>
        </li>
      {/each}
    </ul>
  </section>
{/if}

{#if showLfs}
  <section>
    <ul>
      <li>
        <button class="row lfs" onclick={() => (lfsOpen = true)} title="Patterns, files and locks" aria-haspopup="dialog">
          <span class="icon"><Icon name="lfs" size={14} /></span>
          <span class="name">LFS</span>
          <span class="status muted">{lfsFiles.toLocaleString()} {lfsFiles === 1 ? 'file' : 'files'}</span>
        </button>
      </li>
    </ul>
  </section>
{/if}

{#if filtering && matches === 0}
  <p class="empty">Nothing matches that.</p>
{/if}

{#if lfsOpen}
  <LFSDialog {path} onClose={() => (lfsOpen = false)} />
{/if}

<style>
  section + section {
    border-top: 1px dotted var(--paper-edge);
    margin-top: 4px;
    padding-top: 4px;
  }

  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: 32px;
    padding: 0 6px 0 10px;
  }

  h3 {
    margin: 0;
    font-weight: 400;
    color: var(--ink-soft);
  }

  .count {
    margin-left: 4px;
    color: var(--ink-mute);
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  li {
    position: relative;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 7px;
    width: 100%;
    height: 28px;
    box-sizing: border-box;
    padding: 2px 10px 2px 24px;
    background: none;
    border: none;
    border-radius: 0;
    color: var(--ink-mute);
    font-size: 14px;
    text-align: left;
    white-space: nowrap;
  }

  .row.lfs {
    padding-left: 10px;
    color: var(--ink-soft);
  }

  .row:focus-visible {
    outline: 2px solid var(--link);
    outline-offset: -2px;
    color: var(--ink);
  }

  li:hover .row:not(:disabled) {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .row.current {
    color: var(--ink);
    font-weight: 600;
  }

  .here {
    flex: none;
    width: 7px;
    height: 7px;
    margin-left: -14px;
    border-radius: 50%;
    background: var(--link);
  }

  .icon {
    display: inline-flex;
    flex: none;
    color: var(--ink-mute);
  }

  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* A long branch gives way to the folder's name, but a warning stays whole. */
  .status {
    flex: 0 100 auto;
    min-width: 0;
    margin-left: auto;
    overflow: hidden;
    text-overflow: ellipsis;
    font-family: var(--font-mono);
    font-size: 12px;
    font-weight: 400;
  }

  .status.muted {
    color: var(--ink-mute);
  }

  .status.warning {
    flex: none;
    color: var(--warning);
  }

  .status.danger {
    flex: none;
    color: var(--danger);
  }

  .more {
    position: absolute;
    top: 2px;
    right: 4px;
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    padding: 0;
    background: var(--paper-warm);
    border: none;
    border-radius: 7px;
    color: var(--ink-soft);
    opacity: 0;
    transition: opacity var(--dur-quick);
  }

  .more:hover:not(:disabled) {
    background: var(--paper-deep);
    color: var(--ink);
  }

  li:hover .more,
  li:focus-within .more {
    opacity: 1;
  }

  li:hover:has(.more) .status,
  li:focus-within:has(.more) .status {
    opacity: 0;
  }

  .empty {
    margin: 16px 12px;
    color: var(--ink-mute);
    font-style: italic;
  }

  @media (prefers-reduced-motion: reduce) {
    .more {
      transition: none;
    }
  }
</style>
