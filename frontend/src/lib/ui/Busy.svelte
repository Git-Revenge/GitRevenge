<script lang="ts">
  /** Three ink dots that pulse in turn: the app's one busy mark, the same as a button's. Put it
   * beside a label that says what's happening; it says nothing on its own. */
  let { color = 'var(--link)' }: { color?: string } = $props()
</script>

<span class="busy" aria-hidden="true" style:color><i></i><i></i><i></i></span>

<style>
  .busy {
    display: inline-flex;
    flex: none;
    align-items: center;
    gap: 3px;
  }

  .busy i {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: currentColor;
    animation: busy 1.2s var(--ease-ink) infinite;
  }

  .busy i:nth-child(2) {
    animation-delay: 0.15s;
  }

  .busy i:nth-child(3) {
    animation-delay: 0.3s;
  }

  @keyframes busy {
    0%,
    100% {
      opacity: 0.35;
    }

    40% {
      opacity: 1;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .busy i {
      animation: none;
      opacity: 0.7;
    }
  }
</style>
