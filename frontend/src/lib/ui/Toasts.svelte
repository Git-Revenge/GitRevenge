<script lang="ts">
  // The toast stack in the lower right: parchment-dark like code blocks, ink-in on arrival,
  // gone faster than it came. Hover or focus holds a toast; Escape inside one dismisses it.
  import { TOAST_MS, toasts, type Toast } from '../toasts.svelte'
  import Icon from './Icon.svelte'
  import { inkIn, inkOut } from './motion'

  // Timers aren't state: nothing renders from them, and the effect below writes them.
  const timers: Record<number, ReturnType<typeof setTimeout>> = {}

  function arm(t: Toast) {
    clearTimeout(timers[t.id])
    timers[t.id] = setTimeout(() => toasts.dismiss(t.id), TOAST_MS)
  }

  function hold(t: Toast) {
    clearTimeout(timers[t.id])
  }

  $effect(() => {
    for (const t of toasts.list) if (!(t.id in timers)) arm(t)
    for (const id of Object.keys(timers).map(Number)) {
      if (!toasts.list.some((t) => t.id === id)) {
        clearTimeout(timers[id])
        delete timers[id]
      }
    }
  })

  async function act(t: Toast) {
    toasts.dismiss(t.id)
    await t.action?.run()
  }
</script>

<div class="stack" role="status" aria-live="polite">
  {#each toasts.list as t (t.id)}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
      class="toast code-ground"
      in:inkIn
      out:inkOut
      onmouseenter={() => hold(t)}
      onmouseleave={() => arm(t)}
      onfocusin={() => hold(t)}
      onfocusout={() => arm(t)}
      onkeydown={(e) => e.key === 'Escape' && toasts.dismiss(t.id)}
      role="group"
    >
      <span class="message">{t.message}</span>
      {#if t.action}
        <button class="btn sm text" onclick={() => act(t)}>{t.action.label}</button>
      {/if}
      <button class="btn sm quiet icon" aria-label="Dismiss" onclick={() => toasts.dismiss(t.id)}><Icon name="close" /></button>
    </div>
  {/each}
</div>

<style>
  .stack {
    position: fixed;
    right: 16px;
    bottom: 16px;
    z-index: 300;
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 8px;
    max-width: min(440px, calc(100vw - 32px));
    pointer-events: none;
  }

  .toast {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 8px 8px 14px;
    border-radius: var(--r-md);
    box-shadow: var(--shadow);
    pointer-events: auto;
  }

  .message {
    flex: 1;
    min-width: 0;
    font-size: 14px;
    line-height: 1.4;
  }
</style>
