<script lang="ts">
  // The graph bar's view menu: one button that shows the current view and opens the list. It
  // took the place of a two-icon switch, so experimental views have somewhere to go without
  // the bar growing.
  import { tick } from 'svelte'
  import { edge, step } from '../menuKeys'
  import Icon from '../ui/Icon.svelte'
  import type { GraphView, ViewInfo } from './views'

  let {
    views,
    current,
    onPick,
    iconOnly = false,
  }: {
    /** The views on offer, in menu order. */
    views: ViewInfo[]
    current: GraphView
    onPick: (view: GraphView) => void
    /** No room for the view's name: the button is its icon. */
    iconOnly?: boolean
  } = $props()

  let open = $state(false)
  let root = $state<HTMLElement>()
  let button = $state<HTMLButtonElement>()
  let now = $derived(views.find((v) => v.id === current) ?? views[0])
  /** Where the experimental views start, for the rule and the label above them. */
  let firstExperiment = $derived(views.findIndex((v) => v.experimental))

  const items = () => [...(root?.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]') ?? [])]

  async function show() {
    open = true
    await tick()
    // The menu opens on what's chosen, as a radio group does.
    items()[Math.max(0, views.indexOf(now))]?.focus()
  }

  function close(refocus = true) {
    open = false
    if (refocus) button?.focus()
  }

  function pick(v: ViewInfo) {
    close()
    if (v.id !== current) onPick(v.id)
  }

  function onKey(e: KeyboardEvent) {
    if (!open) {
      if (e.key === 'ArrowDown' && document.activeElement === button) {
        e.preventDefault()
        void show()
      }
      return
    }
    const all = items()
    const at = all.indexOf(document.activeElement as HTMLButtonElement)
    if (e.key === 'Escape') {
      // Only this menu closes, not the screen behind it.
      e.stopPropagation()
      close()
      return
    }
    if (e.key === 'Tab') {
      // Tab leaves the menu, as it leaves any menu.
      close(false)
      return
    }
    const moves: Record<string, () => number> = {
      ArrowDown: () => step(views, at, 1),
      ArrowUp: () => step(views, at, -1),
      Home: () => edge(views, false),
      End: () => edge(views, true),
    }
    const move = moves[e.key]
    if (!move) return
    e.preventDefault()
    all[move()]?.focus()
  }

  $effect(() => {
    if (!open) return
    const outside = (e: PointerEvent) => {
      if (!root?.contains(e.target as Node)) close(false)
    }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  })
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="views" bind:this={root} onkeydown={onKey}>
  <button
    bind:this={button}
    class="view"
    class:icon-only={iconOnly}
    aria-haspopup="menu"
    aria-expanded={open}
    aria-label="Graph view: {now.label}"
    title="Graph view: {now.label}. Choose how the history is shown."
    onclick={() => (open ? close() : void show())}
  >
    <Icon name={now.icon} size={14} />{#if !iconOnly}<span>{now.label}</span>{/if}<span class="chevron"><Icon name="chevron" size={12} /></span>
  </button>
  {#if open}
    <div class="menu" role="menu" aria-label="Graph view">
      {#each views as v, i (v.id)}
        {#if i === firstExperiment}
          <div class="rule" role="separator"></div>
          <div class="hand" aria-hidden="true">experimental</div>
        {/if}
        <button role="menuitemradio" aria-checked={v.id === current} aria-label={v.experimental ? `${v.label}, experimental` : v.label} title={v.title} tabindex="-1" onclick={() => pick(v)}>
          <span class="mark"><Icon name={v.icon} size={15} /></span>
          {v.label}
          {#if v.id === current}<span class="tick"><Icon name="check" size={14} /></span>{/if}
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .views {
    position: relative;
    flex: none;
  }

  .view {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: var(--btn-h-sm);
    padding: 0 8px;
    border: none;
    border-radius: var(--btn-r-sm);
    background: var(--paper-deep);
    font-family: var(--font-serif);
    font-size: var(--btn-fs-sm);
    color: var(--ink);
    white-space: nowrap;
    cursor: pointer;
  }

  .chevron {
    display: inline-flex;
    color: var(--ink-mute);
  }

  .menu {
    position: absolute;
    z-index: 20;
    top: calc(100% + 4px);
    left: 0;
    display: flex;
    flex-direction: column;
    min-width: 200px;
    box-sizing: border-box;
    padding: 4px;
    background: var(--paper);
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    box-shadow: var(--shadow);
    animation: drop-in var(--dur-base) var(--ease-ink);
  }

  .menu button {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 32px;
    padding: 0 8px;
    border: none;
    border-radius: var(--r-sm);
    background: none;
    font-family: var(--font-serif);
    font-size: 14px;
    color: var(--ink);
    text-align: left;
    white-space: nowrap;
    cursor: pointer;
  }

  .menu button:hover,
  .menu button:focus-visible {
    background: var(--wash-hover);
  }

  .menu button[aria-checked='true'] {
    background: var(--selected);
  }

  .mark {
    display: inline-flex;
    color: var(--ink-mute);
  }

  .tick {
    display: inline-flex;
    margin-left: auto;
    padding-left: 12px;
    color: var(--link);
  }

  .rule {
    margin: 5px 2px;
    border-top: 1px dashed var(--hairline);
  }

  .hand {
    align-self: flex-start;
    padding: 2px 8px 4px;
    font-family: var(--font-hand);
    font-size: 18px;
    line-height: 1;
    color: var(--accent-a);
    transform: rotate(-2deg);
  }

  @media (prefers-reduced-motion: reduce) {
    .menu {
      animation: none;
    }
  }
</style>
