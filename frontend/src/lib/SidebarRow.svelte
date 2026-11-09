<script lang="ts">
  // One row of the sidebar's list, inside its treeitem: a folder, or a branch, tag or stash.
  import { kindInfo } from './branchKinds'
  import type { Ref } from './graph/rowCache'
  import { matchParts, refStatus, refTooltip, stackPlace, type FolderRow, type RefRow, type TreeRow } from './sidebarTree'
  import Icon from './ui/Icon.svelte'

  // For screen readers. Written in the markup, it would lose its space.
  const STACK_PREFIX = 'Stack: '

  let {
    row,
    filter,
    selectedCommit,
    mergedInto,
    onFolderClick,
    onRefClick,
    onActivate,
    onContext,
    onStackContext,
  }: {
    row: TreeRow
    filter: string
    selectedCommit?: string
    /** The branch already holding this row's work, if any. */
    mergedInto?: string
    onFolderClick: (row: FolderRow) => void
    onRefClick: (row: RefRow) => void
    onActivate?: (ref: Ref) => void
    onContext?: (ref: Ref, x: number, y: number) => void
    onStackContext?: (members: Ref[], x: number, y: number) => void
  } = $props()
</script>

{#if row.type === 'folder'}
  {@const k = kindInfo(row.kind)}
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
  <div
    class="row folder"
    class:stack-row={row.stack}
    style:--depth={row.depth}
    oncontextmenu={(e) => {
      if (!row.members || !onStackContext) return
      e.preventDefault()
      onStackContext(row.members, e.clientX, e.clientY)
    }}
    title={row.stack ? `A stack of ${row.count} branches${row.note ? `, all ${row.note}` : ''}` : `${row.label}/`}
    onclick={() => onFolderClick(row)}
  >
    <span class="caret" class:open={row.open}><Icon name="right" size={10} /></span>
    {#if k}
      <span class="kind-icon" style:color="var({k.token})" title={k.label}><Icon name={k.icon} size={14} /><span class="visually-hidden">{k.label}</span></span>
    {:else}
      <span class="kind-icon"><Icon name={row.stack ? 'stack' : row.remote ? 'remote' : row.open ? 'folder-open' : 'folder'} size={14} /></span>
    {/if}
    <span class="name">{#if row.stack}<span class="visually-hidden">{STACK_PREFIX}</span>{/if}{row.label}</span>
    {#if row.holdsHead && !row.open}<span class="dot" title="The checked-out branch is in here"><span class="visually-hidden">Holds the checked-out branch</span></span>{/if}
    {#if row.note}<span class="meta note">{row.note}</span>{/if}
    <span class="meta count">{row.count.toLocaleString()}</span>
  </div>
  {#if row.members && onStackContext}
    {@const members = row.members}
    <button
      class="more"
      tabindex="-1"
      aria-hidden="true"
      aria-label="More for this stack"
      aria-haspopup="menu"
      title="More"
      onclick={(e) => {
        const box = e.currentTarget.getBoundingClientRect()
        onStackContext(members, box.left, box.bottom)
      }}><Icon name="more" size={14} /></button
    >
  {/if}
{:else}
  {@const r = row.ref}
  {@const k = kindInfo(row.mark)}
  {@const status = refStatus(r, mergedInto)}
  {@const parts = matchParts(row.label, filter)}
  {@const isSelected = !!selectedCommit && r.commit === selectedCommit}
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
  <div
    class="row ref"
    class:head={r.head}
    class:selected={isSelected}
    class:hidden-ref={r.hidden}
    class:stacked={row.stacked}
    class:railed={row.railed}
    class:context={row.context}
    style:--depth={row.depth}
    onclick={() => onRefClick(row)}
    ondblclick={() => onActivate?.(r)}
    oncontextmenu={(e) => {
      if (!onContext) return
      e.preventDefault()
      onContext(r, e.clientX, e.clientY)
    }}
    title={refTooltip(r, mergedInto)}
  >
    {#if r.head}<span class="here"><span class="visually-hidden">Checked out: </span></span>{/if}
    {#if k}<span class="kind-icon" style:color="var({k.token})" title={k.label}><Icon name={k.icon} size={13} /><span class="visually-hidden">{k.label}</span></span>{/if}
    <span class="name">{parts.pre}{#if parts.hit}<mark>{parts.hit}</mark>{/if}{parts.post}</span>
    {#if r.focused}<span class="focused" title="The graph is focused on this"><Icon name="focus" size={13} /><span class="visually-hidden">, focused</span></span>{/if}
    {#if r.worktree}<span class="worktree" title="Checked out in the worktree at {r.worktree}"><Icon name="worktree" size={13} /><span class="visually-hidden">, in a worktree</span></span>{/if}
    {#if r.hidden}<span class="visually-hidden">, hidden</span>{/if}
    {#if stackPlace(r)}<span class="visually-hidden">, {stackPlace(r)}</span>{/if}
    {#if status}<span class="track {status.tone}"><span aria-hidden="true">{status.text}</span><span class="visually-hidden">, {status.spoken}</span></span>{/if}
  </div>
  {#if onContext}
    <!-- The Menu key reaches this from the keyboard, so it isn't a Tab stop. -->
    <button
      class="more"
      tabindex="-1"
      aria-hidden="true"
      aria-label="More for {r.kind === 'stash' ? r.subject || r.short : r.short}"
      aria-haspopup="menu"
      title="More"
      onclick={(e) => {
        const box = e.currentTarget.getBoundingClientRect()
        onContext(r, box.left, box.bottom)
      }}><Icon name="more" size={14} /></button
    >
  {/if}
{/if}

<style>
  .row {
    position: relative;
    display: flex;
    align-items: center;
    width: 100%;
    height: 28px;
    box-sizing: border-box;
    background: none;
    border: none;
    border-radius: 0;
    color: var(--ink-soft);
    font-size: 14px;
    text-align: left;
    white-space: nowrap;
    cursor: pointer;
    user-select: none;
  }

  .row:hover {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .folder {
    gap: 6px;
    padding: 2px 10px 2px calc(10px + var(--depth, 0) * 14px);
  }

  /* Only the checked-out branch is at full strength; hover, focus and selection bring the
     others up to it. */
  .ref {
    gap: 7px;
    padding: 2px 10px 2px calc(28px + var(--depth, 0) * 14px);
    color: var(--ink-mute);
  }

  :global(li:hover) .ref,
  :global([role='tree']:focus-visible li.active) .ref {
    color: var(--ink);
  }

  :global(li:hover) .row {
    background: var(--wash-hover);
  }

  .ref.selected {
    background: var(--selected);
    color: var(--ink);
  }

  .ref.head {
    color: var(--ink);
    font-weight: 600;
  }

  /* While narrowed, what matched stands out from the parents kept only for context. */
  :global(.narrowed) .ref:not(.context, .head, .selected) {
    color: var(--ink-soft);
  }

  mark {
    background: var(--highlight);
    color: inherit;
  }

  .more {
    position: absolute;
    z-index: 2;
    top: 2px;
    right: 4px;
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    padding: 0;
    background: var(--paper-warm);
    border: none;
    border-radius: var(--r-md);
    color: var(--ink-soft);
    opacity: 0;
    transition: opacity var(--dur-quick);
  }

  .more:hover:not(:disabled) {
    background: var(--paper-deep);
    color: var(--ink);
  }

  :global(li:hover) .more,
  :global([role='tree']:focus-visible li.active) .more {
    opacity: 1;
  }

  /* More replaces the status; it stays readable to screen readers. */
  :global(li:hover) .track,
  :global([role='tree']:focus-visible li.active) .track,
  :global(li:hover) .stack-row .count,
  :global([role='tree']:focus-visible li.active) .stack-row .count,
  :global(li:hover) .stack-row .note,
  :global([role='tree']:focus-visible li.active) .stack-row .note {
    opacity: 0;
  }

  .here,
  .dot {
    flex: none;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--link);
  }

  .here {
    margin-left: -15px;
  }

  /* A stacked branch sits indented under its parent, with a thin elbow joining them. */
  .ref.stacked::before {
    content: '';
    position: absolute;
    left: calc(16px + var(--depth, 0) * 14px);
    top: 0;
    width: 7px;
    height: 50%;
    border-left: 1px solid var(--paper-edge);
    border-bottom: 1px solid var(--paper-edge);
    border-bottom-left-radius: 4px;
    pointer-events: none;
  }

  /* The branches of a stack's group share one line, since they're a sequence, not a hierarchy. */
  .ref.railed::before {
    content: '';
    position: absolute;
    left: calc(16px + var(--depth, 0) * 14px);
    top: 0;
    width: 0;
    height: 100%;
    border-left: 1px solid var(--paper-edge);
    border-bottom: none;
    border-radius: 0;
    pointer-events: none;
  }

  .caret {
    flex: none;
    display: grid;
    place-items: center;
    width: 10px;
    color: var(--ink-mute);
    transition: transform var(--dur-base) var(--ease-ink);
  }

  .caret.open {
    transform: rotate(90deg);
  }

  .kind-icon {
    display: inline-flex;
    flex: none;
    color: var(--ink-mute);
  }

  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .count,
  .note {
    margin-left: auto;
    flex: none;
  }

  .note + .count {
    margin-left: 0;
  }

  .note {
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0;
    text-transform: none;
    color: var(--ink-mute);
  }

  .track {
    margin-left: auto;
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
    font-weight: 400;
    color: var(--link);
  }

  .track.warning {
    color: var(--warning);
  }

  .track.muted {
    color: var(--ink-mute);
  }

  .focused {
    flex: none;
    display: inline-flex;
    margin-left: 4px;
    color: var(--accent-a);
  }

  .worktree {
    flex: none;
    display: inline-flex;
    margin-left: 4px;
    color: var(--ink-mute);
  }

  .hidden-ref .name {
    font-style: italic;
  }

  @media (prefers-reduced-motion: reduce) {
    .caret,
    .more {
      transition: none;
    }
  }
</style>
