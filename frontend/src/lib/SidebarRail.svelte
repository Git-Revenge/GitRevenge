<script lang="ts">
  // The sidebar's icon rail: one button per panel, with a badge. It's one Tab stop, and the arrow
  // keys move along it.
  import { railMove } from './sidebarKeys'
  import type { SidebarPanel } from './sidebarState'
  import Icon, { type IconName } from './ui/Icon.svelte'

  export interface RailTab {
    key: SidebarPanel
    label: string
    icon: IconName
    /** A number to show, what it means, and whether it's drawn as a dot (something needs
     * attention) rather than a count (how many there are). */
    badge: { count: number; text: string; dot?: boolean }
  }

  let { tabs, panel, collapsed, onPick, onToggle }: { tabs: RailTab[]; panel: SidebarPanel; collapsed: boolean; onPick: (key: SidebarPanel) => void; onToggle: () => void } = $props()

  let rail = $state<HTMLElement>()
  let stop = $derived(tabs.findIndex((t) => t.key === panel))

  function railKey(e: KeyboardEvent) {
    const buttons = [...(rail?.querySelectorAll('button') ?? [])]
    const next = railMove(e.key, buttons.indexOf(e.target as HTMLButtonElement), buttons.length)
    if (next === null) return
    e.preventDefault()
    buttons[next].focus()
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<nav class="rail" aria-label="Sidebar panels" bind:this={rail} onkeydown={railKey}>
  {#each tabs as t, i (t.key)}
    {@const b = t.badge}
    <button
      class="tab"
      class:on={t.key === panel && !collapsed}
      aria-pressed={t.key === panel && !collapsed}
      tabindex={i === stop ? 0 : -1}
      aria-label={b.count ? `${t.label}, ${b.text}` : t.label}
      title={b.count ? `${t.label}: ${b.text}` : t.label}
      onclick={() => onPick(t.key)}
    >
      <Icon name={t.icon} size={18} />
      {#if b.count}<span class="pip" class:dot={b.dot} aria-hidden="true">{b.dot ? '' : b.count > 99 ? '99+' : b.count}</span>{/if}
    </button>
  {/each}
  <button
    class="tab toggle"
    tabindex="-1"
    aria-expanded={!collapsed}
    aria-label={collapsed ? 'Show the sidebar panel' : 'Hide the sidebar panel'}
    title={collapsed ? 'Show the sidebar panel' : 'Hide the sidebar panel'}
    onclick={onToggle}
  >
    <Icon name="sidebar" size={18} />
  </button>
</nav>

<style>
  .rail {
    flex: none;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    width: 44px;
    padding: 8px 0;
    box-sizing: border-box;
    background: var(--paper-warm);
    border-right: 1px solid var(--paper-edge);
  }

  .tab {
    position: relative;
    display: grid;
    place-items: center;
    width: 36px;
    height: 36px;
    padding: 0;
    background: none;
    border: none;
    border-radius: var(--r-md);
    color: var(--ink-mute);
    transition:
      background-color var(--dur-quick),
      color var(--dur-quick);
  }

  .tab:hover:not(:disabled) {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .tab.on {
    background: var(--selected);
    color: var(--ink);
  }

  .toggle {
    margin-top: auto;
  }

  .pip {
    position: absolute;
    right: 0;
    top: 0;
    min-width: 15px;
    padding: 0 3px;
    box-sizing: border-box;
    border-radius: 999px;
    background: var(--paper-deep);
    color: var(--ink-soft);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 15px;
    text-align: center;
  }

  /* Attention, not a tally: a rose dot where the count would be. */
  .pip.dot {
    right: 3px;
    top: 3px;
    min-width: 0;
    width: 8px;
    height: 8px;
    padding: 0;
    border-radius: 50%;
    background: var(--link);
  }

  @media (prefers-reduced-motion: reduce) {
    .tab {
      transition: none;
    }
  }
</style>
