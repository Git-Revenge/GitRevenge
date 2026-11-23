<script lang="ts">
  // One row of the sidebar's list, inside its treeitem: a folder, or a branch, tag or stash.
  import { kindInfo } from './branchKinds'
  import type { Ref } from './graph/rowCache'
  import PlaceGlyph from './PlaceGlyph.svelte'
  import { onRemotesTooltip, type PlaceLook, type RemoteHeader } from './remoteHeader'
  import { tidyStatus } from './remoteTidy'
  import { matchParts, refStatus, refTooltip, stackPlace, type FolderRow, type RefRow, type TreeRow } from './sidebarTree'
  import Icon from './ui/Icon.svelte'

  const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1)

  // For screen readers. Written in the markup, it would lose its space.
  const STACK_PREFIX = 'Stack: '
  // A row grouped by name shows the first few remotes' chips that fit, and how many remotes
  // have it in all.
  const CHIPS = 4

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
    onRemoteContext,
    onTidy,
    tidyMark,
    remote,
    hostLook,
    remoteTones = {},
    brand = false,
  }: {
    row: TreeRow
    /** For a remote's own folder: where it lives and how fresh it is. */
    remote?: RemoteHeader
    /** For a host's folder: its mark. */
    hostLook?: PlaceLook
    /** Each remote's chip tone, 0 to 5. */
    remoteTones?: Record<string, number>
    /** Forge marks in their brands' colours. */
    brand?: boolean
    filter: string
    selectedCommit?: string
    /** The branch already holding this row's work, if any. */
    mergedInto?: string
    onFolderClick: (row: FolderRow) => void
    onRefClick: (row: RefRow) => void
    onActivate?: (ref: Ref) => void
    onContext?: (ref: Ref, x: number, y: number) => void
    onStackContext?: (members: Ref[], x: number, y: number) => void
    /** A remote's own menu, opened on its folder. */
    onRemoteContext?: (remote: string, x: number, y: number) => void
    /** Opens Tidy for a remote, from the row at its foot. */
    onTidy?: (remote: string) => void
    /** A remote branch's Tidy mark: gone, merged or quiet. */
    tidyMark?: string
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
      if (row.remote && onRemoteContext) {
        e.preventDefault()
        onRemoteContext(row.label, e.clientX, e.clientY)
        return
      }
      if (!row.members || !onStackContext) return
      e.preventDefault()
      onStackContext(row.members, e.clientX, e.clientY)
    }}
    title={remote ? remote.tooltip : row.remotes !== undefined ? `${row.label}: ${row.remotes} ${row.remotes === 1 ? 'remote' : 'remotes'}, ${row.count.toLocaleString()} ${row.count === 1 ? 'branch' : 'branches'}` : row.stack ? `A stack of ${row.count} branches${row.note ? `, all ${row.note}` : ''}` : `${row.label}/`}
    onclick={() => onFolderClick(row)}
  >
    <span class="caret" class:open={row.open}><Icon name="right" size={10} /></span>
    {#if k}
      <span class="kind-icon" style:color="var({k.token})" title={k.label}><Icon name={k.icon} size={14} /><span class="visually-hidden">{k.label}</span></span>
    {:else if hostLook}
      <span class="kind-icon" title={hostLook.says}><PlaceGlyph look={hostLook} size={14} {brand} /></span>
    {:else if !remote}
      <span class="kind-icon"><Icon name={row.stack ? 'stack' : row.remote ? 'remote' : row.open ? 'folder-open' : 'folder'} size={14} /></span>
    {/if}
    <span class="name">{#if row.stack}<span class="visually-hidden">{STACK_PREFIX}</span>{/if}{row.label}</span>
    {#if remote}
      <span class="host" class:bare={!remote.host}>
        <PlaceGlyph look={remote.look} size={13} {brand} />{#if remote.host}<span class="host-name">{remote.host}</span>{/if}<span class="visually-hidden">, {remote.look.says}</span>
      </span>
      {#if remote.fan.length}
        <span class="fan" aria-hidden="true">{#each remote.fan as f, i (i)}<span class="fan-mark"><PlaceGlyph look={f} size={11} {brand} /></span>{/each}</span>
        <span class="visually-hidden">, pushes to {remote.fan.length} places</span>
      {/if}
    {/if}
    {#if row.holdsHead && !row.open}<span class="dot" title="The checked-out branch is in here"><span class="visually-hidden">Holds the checked-out branch</span></span>{/if}
    {#if row.note}<span class="meta note">{row.note}</span>{/if}
    {#if remote?.age}
      <span class="meta age" class:stale={remote.stale}><span aria-hidden="true">{remote.ageShort}</span><span class="visually-hidden">{remote.stale ? 'Stale, fetched ' : 'Fetched '}{remote.age}</span></span>
      <span class="visually-hidden">, {row.count.toLocaleString()} {row.count === 1 ? 'branch' : 'branches'}</span>
    {:else if row.remotes !== undefined}
      <span class="meta note">{row.remotes} {row.remotes === 1 ? 'remote' : 'remotes'}</span>
    {:else if remote}
      <span class="visually-hidden">, not fetched yet that Git Revenge has seen, {row.count.toLocaleString()} {row.count === 1 ? 'branch' : 'branches'}</span>
    {:else}
      <span class="meta count">{row.count.toLocaleString()}</span>
    {/if}
  </div>
  {#if row.remote && onRemoteContext}
    <button
      class="more"
      tabindex="-1"
      aria-hidden="true"
      aria-label="More for {row.label}"
      aria-haspopup="menu"
      title="More"
      onclick={(e) => {
        const box = e.currentTarget.getBoundingClientRect()
        onRemoteContext(row.label, box.left, box.bottom)
      }}><Icon name="more" size={14} /></button
    >
  {:else if row.members && onStackContext}
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
{:else if row.type === 'tidy'}
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
  <div class="row tidy" style:--depth={row.depth} title="Open Tidy for {row.remote}" onclick={() => onTidy?.(row.remote)}>
    <span class="tidy-text">{row.label}</span><span class="tidy-arrow" aria-hidden="true">→</span>
  </div>
{:else}
  {@const r = row.ref}
  {@const k = kindInfo(row.mark)}
  {@const status = (r.kind === 'remote' ? tidyStatus(tidyMark) : undefined) ?? refStatus(r, mergedInto)}
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
    class:gone={status?.text === 'gone'}
    style:--depth={row.depth}
    onclick={() => onRefClick(row)}
    ondblclick={() => onActivate?.(r)}
    oncontextmenu={(e) => {
      if (!onContext) return
      e.preventDefault()
      onContext(r, e.clientX, e.clientY)
    }}
    title={row.onRemotes ? onRemotesTooltip(row.label, row.onRemotes) : refTooltip(r, mergedInto) + (r.kind === 'remote' && tidyStatus(tidyMark) ? `
${capitalize(tidyStatus(tidyMark)!.spoken)}: Tidy can delete it` : '')}
  >
    {#if r.head}<span class="here"><span class="visually-hidden">Checked out: </span></span>{/if}
    {#if k}<span class="kind-icon" style:color="var({k.token})" title={k.label}><Icon name={k.icon} size={13} /><span class="visually-hidden">{k.label}</span></span>{/if}
    <span class="name">{parts.pre}{#if parts.hit}<mark>{parts.hit}</mark>{/if}{parts.post}</span>
    {#if row.onRemotes}
      <!-- Only the first few fit; screen readers hear them all. -->
      <span class="chips" aria-hidden="true"
        >{#each row.onRemotes.slice(0, CHIPS) as o (o.short)}{@const name = o.short.slice(0, o.short.indexOf('/'))}<span class="chip t{remoteTones[name] ?? 0}" class:differs={o.commit !== r.commit}
            >{#if o.commit !== r.commit}≠{/if}{name}</span
          >{/each}</span
      >{#if row.onRemotes.length > 1}<span class="chip more-chips" aria-hidden="true">{row.onRemotes.length}</span>{/if}
      <span class="visually-hidden"
        >, on {row.onRemotes.map((o) => o.short.slice(0, o.short.indexOf('/')) + (o.commit !== r.commit ? ' at another commit' : '')).join(', ')}</span
      >
    {/if}
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

  /* Gone: struck through, and the mark says so in words. */
  .ref.gone .name {
    text-decoration: line-through;
    text-decoration-color: var(--ink-mute);
  }

  /* The row at a remote's foot that opens Tidy. */
  .tidy {
    gap: 6px;
    padding: 2px 10px 2px calc(28px + var(--depth, 0) * 14px);
    font-size: 13px;
    color: var(--link);
  }

  .tidy:hover .tidy-text {
    text-decoration: underline;
  }

  .tidy-text {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Grouped by name: a chip for each remote that has the branch. Tones differ in lightness as
     well as hue, and each chip carries the remote's name, so none relies on colour. */
  .ref:has(.chips) .name {
    flex: none;
    max-width: 55%;
  }

  /* Chips that don't fit wrap onto a second line, which is clipped, so a chip goes whole. */
  .chips {
    display: inline-flex;
    flex: 0 1 auto;
    flex-wrap: wrap;
    gap: 3px;
    height: 18px;
    min-width: 18px;
    overflow: hidden;
  }

  .chip {
    flex: none;
    padding: 0 5px;
    border: 1px solid currentColor;
    border-radius: 4px;
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 16px;
    font-weight: 400;
  }

  .chip.t0 {
    background: var(--ink-teal);
    border-color: var(--ink-teal);
    color: var(--paper);
  }

  .chip.t1 {
    color: var(--ink-rose);
  }

  .chip.t2 {
    background: var(--ink-plum);
    border-color: var(--ink-plum);
    color: var(--paper);
  }

  .chip.t3 {
    color: var(--ink-sepia);
  }

  .chip.t4 {
    background: var(--ink-soft);
    border-color: var(--ink-soft);
    color: var(--paper);
  }

  .chip.t5 {
    color: var(--ink-ochre);
    border-style: dashed;
  }

  .chip.more-chips {
    flex: none;
    margin-left: auto;
    border-color: transparent;
    color: var(--ink-mute);
  }

  /* On a commit other than the row's own: the same name, somewhere else. */
  .chip.differs {
    opacity: 0.55;
  }

  @media (forced-colors: active) {
    .chip {
      background: none !important;
      color: CanvasText !important;
      border-color: CanvasText !important;
    }
  }

  /* A remote's header: where it lives, as a chip, and how long since it was fetched. */
  .name:has(+ .host) {
    flex: none;
    max-width: 45%;
  }

  .host {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    min-width: 0;
    padding: 1px 6px 1px 5px;
    border-radius: 4px;
    background: var(--paper-deep);
    color: var(--ink-soft);
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.02em;
  }

  .host.bare {
    padding: 1px 4px;
  }

  .host-name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .fan {
    display: inline-flex;
    flex: none;
    color: var(--ink-soft);
  }

  .fan-mark {
    display: inline-grid;
    place-items: center;
    width: 15px;
    height: 15px;
    border-radius: 50%;
    background: var(--paper);
    box-shadow: 0 0 0 1px var(--hairline);
  }

  .fan-mark + .fan-mark {
    margin-left: -4px;
  }

  .age {
    margin-left: auto;
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0;
    text-transform: none;
    color: var(--ink-mute);
  }

  .age.stale {
    color: var(--warning);
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
