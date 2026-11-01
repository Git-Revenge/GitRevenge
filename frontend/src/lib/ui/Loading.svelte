<script lang="ts">
  import { onMount } from 'svelte'

  /** Shows that something is loading, but only once it's taken long enough to notice, so a fast
   * load doesn't flash. */
  let { label = 'Loading…' }: { label?: string } = $props()

  const DELAY_MS = 200
  let show = $state(false)

  onMount(() => {
    const timer = setTimeout(() => (show = true), DELAY_MS)
    return () => clearTimeout(timer)
  })
</script>

{#if show}
  <p class="loading" role="status">
    <span class="bead" aria-hidden="true"></span>
    {label}
  </p>
{/if}

<style>
  .loading {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0;
    padding: 12px;
    color: var(--ink-mute);
    font-size: 14px;
    font-style: italic;
    animation: arrive var(--dur-base) var(--ease-ink);
  }

  .bead {
    flex: none;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--link);
    animation: pulse 1.2s var(--ease-ink) infinite;
  }

  @keyframes arrive {
    from {
      opacity: 0;
    }
  }

  @keyframes pulse {
    50% {
      opacity: 0.25;
      transform: scale(0.7);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .loading,
    .bead {
      animation: none;
    }
  }
</style>
