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
    /** Opens these as a submenu instead of running. */
    items?: MenuItem[]
    run?: () => void | Promise<void>
  }
</script>

<script lang="ts">
  import { onMount } from 'svelte'
  import { formatShortcut } from './commands.svelte'
  import { edge, step, typeahead } from './menuKeys'
  import Icon from './ui/Icon.svelte'
  import { menuOut } from './ui/motion'
  import ContextMenu from './ContextMenu.svelte'

  let {
    x,
    y,
    items,
    onClose,
    flipX,
    onBack,
    focusFirst = true,
  }: {
    x: number
    y: number
    items: MenuItem[]
    onClose: () => void
    /** A submenu's other side of its parent item, for when there's no room to the right. */
    flipX?: number
    /** Set on a submenu: go back to the parent, leaving it open. */
    onBack?: () => void
    /** A submenu opened by hover leaves focus where it is. */
    focusFirst?: boolean
  } = $props()

  /** The open submenu: which item it belongs to, where it goes and whether it takes focus. */
  let sub = $state<{ index: number; x: number; y: number; flipX: number; focus: boolean } | null>(null)
  let hoverTimer: ReturnType<typeof setTimeout> | undefined

  function openSub(i: number, focus: boolean) {
    clearTimeout(hoverTimer)
    const el = menu.querySelectorAll<HTMLElement>('[role="menuitem"]')[i]
    const box = el.getBoundingClientRect()
    sub = { index: i, x: box.right - 4, y: box.top - 5, flipX: box.left + 4, focus }
  }

  function hover(i: number) {
    clearTimeout(hoverTimer)
    if (sub?.index === i) return
    // A short wait, so moving past a row on the way to another doesn't flash its submenu.
    // Read now: the parent may have closed this menu by the time the wait is over.
    const hasItems = !!items[i].items
    hoverTimer = setTimeout(() => {
      if (hasItems) openSub(i, false)
      else sub = null
    }, 120)
  }

  function closeSub() {
    const i = sub?.index ?? -1
    sub = null
    if (i >= 0) activate(i)
  }

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

  /** Whether the pointer's last move already dealt with this item. */
  let lastHover = -1
  function hovering(i: number) {
    const same = lastHover === i
    lastHover = i
    return same
  }

  /** A label cut off by the menu's width has its full text as a tooltip. */
  function showFullLabel(e: MouseEvent) {
    const item = e.currentTarget as HTMLElement
    const label = item.querySelector<HTMLElement>('.label')
    item.title = label && label.scrollWidth > label.clientWidth ? (label.textContent ?? '') : ''
  }

  /** Highlights and focuses item i. Moving off an armed item disarms it. */
  function activate(i: number) {
    if (i < 0) return
    active = i
    if (armed !== i) armed = -1
    menu.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')[i]?.focus()
  }

  onMount(() => {
    const before = document.activeElement as HTMLElement | null
    const el = menu
    const r = menu.getBoundingClientRect()
    const fits = x + r.width + 4 <= window.innerWidth
    left = Math.max(4, !fits && flipX !== undefined ? flipX - r.width : Math.min(x, window.innerWidth - r.width - 4))
    top = Math.max(4, Math.min(y, window.innerHeight - r.height - 4))
    if (focusFirst) {
      const first = edge(items, false)
      if (first < 0) menu.focus()
      else activate(first)
    }
    return () => {
      clearTimeout(queryTimer)
      clearTimeout(hoverTimer)
      // Give focus back, unless what was chosen has moved it somewhere on purpose.
      const now = document.activeElement
      if (focusFirst && (!now || now === document.body || el.contains(now))) before?.focus?.()
    }
  })

  async function pick(i: number) {
    const item = items[i]
    if (item.disabled) return
    clearTimeout(hoverTimer)
    if (item.items) {
      openSub(i, true)
      return
    }
    if (item.danger && armed !== i) {
      armed = i
      return
    }
    onClose()
    await item.run?.()
  }

  /** Arrows, Home, End and type-ahead move between items; Tab closes, as in native menus. */
  function onKey(e: KeyboardEvent) {
    const go = (j: number) => {
      e.preventDefault()
      activate(j)
    }
    if (onBack) {
      // Only this submenu hears its keys; Esc shouldn't close the parent too.
      e.stopPropagation()
      if (e.key === 'Escape' || e.key === 'ArrowLeft') {
        e.preventDefault()
        onBack()
        return
      }
    }
    if (e.key === 'ArrowRight' && active >= 0 && items[active].items && !items[active].disabled) {
      e.preventDefault()
      openSub(active, true)
    } else if (e.key === 'ArrowDown') go(step(items, active, 1))
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
    if (!onBack && !(e.target as Element).closest?.('[role="menu"]')) onClose()
  }}
  onkeydown={(e) => e.key === 'Escape' && !onBack && onClose()}
  onblur={() => !onBack && onClose()}
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
      aria-haspopup={item.items ? 'menu' : undefined}
      aria-expanded={item.items ? sub?.index === i : undefined}
      onclick={() => pick(i)}
      onmouseenter={showFullLabel}
      onmousedown={(e) => item.disabled && e.preventDefault()}
      onmousemove={() => {
        if (!item.disabled && active !== i) activate(i)
        if (!item.disabled && !hovering(i)) hover(i)
      }}
    >
      {#if hasIcons}<span class="icon">{#if item.icon}<Icon name={item.icon} />{/if}</span>{/if}
      <span class="label">{armed === i ? `Press again to ${item.label.toLowerCase()}` : item.label}</span>
      {#if armed === i}<Icon name="warning" size={14} />
      {:else if item.items}<span class="arrow" aria-hidden="true"><Icon name="right" size={14} /></span>
      {:else if item.shortcut}<kbd>{formatShortcut(item.shortcut)}</kbd>{/if}
    </button>
  {/each}
</div>
{#if sub && items[sub.index]?.items}
  <!-- Props are read once more as sub clears, before this block goes, so they allow for null. -->
  <ContextMenu x={sub?.x ?? 0} y={sub?.y ?? 0} flipX={sub?.flipX} items={(sub && items[sub.index]?.items) || []} {onClose} onBack={closeSub} focusFirst={sub?.focus ?? false} />
{/if}

<style>
  .menu {
    position: fixed;
    z-index: 100;
    min-width: 220px;
    max-width: min(480px, calc(100vw - 8px));
    /* A commit's menu combines every ref on it and can outgrow the window. */
    max-height: calc(100vh - 8px);
    overflow-y: auto;
    box-sizing: border-box;
    padding: 4px;
    background: var(--paper);
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
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
    border-radius: var(--r-sm);
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
    font-size: 12px;
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

  .arrow {
    display: inline-flex;
    margin-left: auto;
    color: var(--ink-mute);
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
