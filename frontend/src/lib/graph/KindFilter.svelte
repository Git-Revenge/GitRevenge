<script lang="ts">
  // The graph header's Kinds filter: a checkbox for each kind of branch in the repository.
  import type { KindFilterRow } from '../branchKinds'
  import Icon from '../ui/Icon.svelte'

  let {
    rows,
    onToggle,
  }: {
    rows: KindFilterRow[]
    onToggle: (kind: string, visible: boolean) => void
  } = $props()

  let open = $state(false)
  let root = $state<HTMLElement>()
  let button = $state<HTMLButtonElement>()
  let hiddenCount = $derived(rows.filter((r) => r.hidden).length)

  function onKey(e: KeyboardEvent) {
    if (e.key !== 'Escape' || !open) return
    // Only this popover closes, not the screen behind it.
    e.stopPropagation()
    open = false
    button?.focus()
  }

  $effect(() => {
    if (!open) return
    root?.querySelector('input')?.focus()
    const outside = (e: PointerEvent) => {
      if (!root?.contains(e.target as Node)) open = false
    }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  })
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="kinds" bind:this={root} onkeydown={onKey}>
  <button bind:this={button} class="btn sm quiet" aria-expanded={open} aria-haspopup="true" title="Choose which kinds of branch the graph shows" onclick={() => (open = !open)}>
    <Icon name="tag" size={14} />Kinds{#if hiddenCount}<span class="filtered">{hiddenCount} hidden</span>{/if}
  </button>
  {#if open}
    <div class="popover" role="group" aria-label="Kinds of branch to show">
      {#each rows as r (r.kind)}
        <label>
          <input type="checkbox" checked={!r.hidden} onchange={(e) => onToggle(r.kind, e.currentTarget.checked)} />
          <span class="icon" style:color="var({r.token})"><Icon name={r.icon} size={14} /></span>
          {r.label}
        </label>
      {/each}
    </div>
  {/if}
</div>

<style>
  .kinds {
    position: relative;
    flex: none;
  }

  .filtered {
    margin-left: 6px;
    color: var(--ink-mute);
    font-weight: 400;
  }

  .popover {
    position: absolute;
    z-index: 20;
    top: calc(100% + 4px);
    left: 0;
    min-width: 170px;
    box-sizing: border-box;
    padding: 4px;
    background: var(--paper);
    border: 1px solid var(--paper-edge);
    border-radius: 8px;
    box-shadow: var(--shadow);
    animation: drop-in var(--dur-base) var(--ease-ink);
  }

  label {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 5px 8px;
    border-radius: 4px;
    font-size: 13px;
    white-space: nowrap;
    cursor: pointer;
  }

  label:hover {
    background: var(--wash-hover);
  }

  .icon {
    display: inline-flex;
  }
</style>
