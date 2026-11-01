<script lang="ts" module>
  import type { IconName } from './ui/Icon.svelte'

  export interface MenuItem {
    label: string
    /** Shown before the label at 16px. */
    icon?: IconName
    /** Destructive items ask for a second click or Enter before running. */
    danger?: boolean
    /** Shown but not selectable; skipped by the arrow keys and type-ahead. */
    disabled?: boolean
    /** Draws a rule above this item, starting a new group. */
    separator?: boolean
    /** Shown beside the label, in Mod+Shift+K form. */
    shortcut?: string
    run: () => void | Promise<void>
  }
</script>

<script lang="ts">
  import { onMount } from 'svelte'
  import { formatShortcut } from './commands.svelte'
  import { edge, step, typeahead } from './menuKeys'
  import Icon from './ui/Icon.svelte'
  import { menuOut } from './ui/motion'

  let { x, y, items, onClose }: { x: number; y: number; items: MenuItem[]; onClose: () => void } = $props()

  let menu: HTMLDivElement
  /** The highlighted item, which also holds DOM focus; -1 for none. */
  let active = $state(-1)
  let armed = $state(-1)
  // Keep the menu on screen near the window edges.
  let left = $state(0)
  let top = $state(0)

  // Reserve the icon column when any item has one, so labels line up.
  let hasIcons = $derived(items.some((i) => i.icon))

  let query = ''
  let queryTimer: ReturnType<typeof setTimeout> | undefined

  /** Highlights and focuses item i. Moving off an armed item disarms it. */
  function activate(i: number) {
    if (i < 0) return
    active = i
    if (armed !== i) armed = -1
    menu.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')[i]?.focus()
  }

  onMount(() => {
    const before = document.activeElement as HTMLElement | null
    const r = menu.getBoundingClientRect()
    left = Math.max(4, Math.min(x, window.innerWidth - r.width - 4))
    top = Math.max(4, Math.min(y, window.innerHeight - r.height - 4))
    const first = edge(items, false)
    if (first < 0) menu.focus()
    else activate(first)
    return () => {
      clearTimeout(queryTimer)
      before?.focus?.()
    }
  })

  async function pick(i: number) {
    const item = items[i]
    if (item.disabled) return
    if (item.danger && armed !== i) {
      armed = i
      return
    }
    onClose()
    await item.run()
  }

  /** Arrows, Home, End and type-ahead move between items; Tab closes, as in native menus. */
  function onKey(e: KeyboardEvent) {
    const go = (j: number) => {
      e.preventDefault()
      activate(j)
    }
    if (e.key === 'ArrowDown') go(step(items, active, 1))
    else if (e.key === 'ArrowUp') go(step(items, active < 0 ? items.length : active, -1))
    else if (e.key === 'Home' || e.key === 'PageUp') go(edge(items, false))
    else if (e.key === 'End' || e.key === 'PageDown') go(edge(items, true))
    else if (e.key === 'Tab') {
      e.preventDefault()
      onClose()
    } else if (e.key.length === 1 && e.key !== ' ' && !e.ctrlKey && !e.metaKey && !e.altKey) {
      e.preventDefault()
      clearTimeout(queryTimer)
      query += e.key
      queryTimer = setTimeout(() => (query = ''), 600)
      activate(typeahead(items, active, query))
    }
  }
</script>

<svelte:window
  onmousedown={(e) => {
    if (!menu.contains(e.target as Node)) onClose()
  }}
  onkeydown={(e) => e.key === 'Escape' && onClose()}
  onblur={onClose}
/>

<div class="menu" bind:this={menu} out:menuOut|global role="menu" tabindex="-1" style:left="{left}px" style:top="{top}px" onkeydown={onKey}>
  {#each items as item, i (i)}
    {#if item.separator && i > 0}<div class="rule" role="separator"></div>{/if}
    <button
      role="menuitem"
      tabindex="-1"
      class="item"
      class:active={active === i}
      class:danger={item.danger}
      class:armed={armed === i}
      aria-disabled={item.disabled || undefined}
      onclick={() => pick(i)}
      onmousedown={(e) => item.disabled && e.preventDefault()}
      onmousemove={() => !item.disabled && active !== i && activate(i)}
    >
      {#if hasIcons}<span class="icon">{#if item.icon}<Icon name={item.icon} />{/if}</span>{/if}
      <span class="label">{armed === i ? `Press again to ${item.label.toLowerCase()}` : item.label}</span>
      {#if armed === i}<Icon name="warning" size={14} />
      {:else if item.shortcut}<kbd>{formatShortcut(item.shortcut)}</kbd>{/if}
    </button>
  {/each}
</div>

<style>
  .menu {
    position: fixed;
    z-index: 100;
    min-width: 220px;
    max-width: 380px;
    /* A commit's menu combines every ref on it and can outgrow the window. */
    max-height: calc(100vh - 8px);
    overflow-y: auto;
    box-sizing: border-box;
    padding: 4px;
    background: var(--paper);
    border: 1px solid var(--paper-edge);
    border-radius: 8px;
    box-shadow: var(--shadow);
    outline: none;
    animation: drop-in var(--dur-base) var(--ease-ink);
  }

  .item {
    display: flex;
    align-items: center;
    gap: 20px;
    width: 100%;
    min-height: 30px;
    padding: 4px 12px;
    background: none;
    border: none;
    border-radius: 5px;
    font-size: 14px;
    text-align: left;
    white-space: nowrap;
  }

  /* The active item carries the highlight, so mouse hover and focus rings add nothing. */
  .item:hover,
  .item:focus-visible {
    background: none;
    outline: none;
  }

  /* A solid fill plus a bar at the leading edge, so the highlight doesn't rest on hue alone. */
  .item.active {
    background: var(--accent);
    color: var(--on-accent);
    box-shadow: inset 3px 0 0 var(--ink);
  }

  .item[aria-disabled='true'] {
    color: var(--ink-faint);
    cursor: default;
  }

  .icon {
    display: inline-flex;
    flex: none;
    width: 16px;
    margin-right: -10px;
  }

  .label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  kbd {
    flex: none;
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--ink-mute);
  }

  .active kbd {
    color: inherit;
  }

  .danger {
    color: var(--danger);
  }

  .item.danger.active {
    background: var(--danger);
  }

  .armed .label {
    font-style: italic;
  }

  .item.armed:not(.active) {
    background: var(--danger-wash);
  }

  .rule {
    height: 1px;
    margin: 4px 8px;
    background: var(--paper-edge);
  }

  @media (prefers-reduced-motion: reduce) {
    .menu {
      animation: none;
    }
  }

  @media (forced-colors: active) {
    .item.active,
    .item.danger.active {
      background: Highlight;
      color: HighlightText;
      forced-color-adjust: none;
    }
  }
</style>
