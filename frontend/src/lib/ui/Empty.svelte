<script lang="ts">
  import type { Snippet } from 'svelte'

  /** An empty list, the one way the app says so: a handwritten note that names the mood, then one
   * plain sentence (and, if useful, a text button) as the children. `compact` stacks the two for
   * narrow panels. Loading states use Loading instead; this is for when there's truly nothing. */
  let { note, compact = false, children }: { note: string; compact?: boolean; children: Snippet } = $props()
</script>

<p class="empty" class:compact>
  <span class="hand" aria-hidden="true">{note}</span>
  <span class="text">{@render children()}</span>
</p>

<style>
  .empty {
    display: flex;
    align-items: baseline;
    gap: 12px;
    margin: 0;
    padding: 16px 12px;
    color: var(--ink-mute);
    font-size: 14px;
    line-height: 1.5;
    animation: fade-in var(--dur-base) var(--ease-ink);
  }

  .empty.compact {
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
  }

  .hand {
    flex: none;
    font-family: var(--font-hand);
    font-size: 20px;
    line-height: 1;
    color: var(--accent-c);
    transform: rotate(-2deg);
    white-space: nowrap;
  }

  .text {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 2px 8px;
  }

  @media (prefers-reduced-motion: reduce) {
    .empty {
      animation: none;
    }
  }
</style>
