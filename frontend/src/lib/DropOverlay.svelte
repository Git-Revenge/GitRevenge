<script lang="ts">
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'

  // While files are dragged over the window (#1025): what letting go will do, and to which
  // repository. A web view doesn't say a dragged file's name until it's dropped, so this
  // counts them; the preview that opens names them.
  let count = $state(0)
  let shown = $state(false)
  let hideTimer: ReturnType<typeof setTimeout> | undefined

  const isFiles = (e: DragEvent) => !!e.dataTransfer?.types.includes('Files')

  function over(e: DragEvent) {
    if (!isFiles(e)) return
    shown = true
    count = [...(e.dataTransfer?.items ?? [])].filter((i) => i.kind === 'file').length
    // Leaving the window sends no last event in every web view, so the overlay goes when the
    // drag stops coming.
    clearTimeout(hideTimer)
    hideTimer = setTimeout(() => (shown = false), 400)
  }
  function leave(e: DragEvent) {
    if (e.relatedTarget === null) shown = false
  }
  function done() {
    clearTimeout(hideTimer)
    shown = false
  }

  let tab = $derived(session.activeTab)
  let branch = $derived(tab?.head && !tab.head.startsWith('detached') ? tab.head : '')
  const files = (n: number) => (n === 1 ? 'the file' : n > 1 ? `the ${n} files` : 'it')
</script>

<svelte:window ondragenter={over} ondragover={over} ondragleave={leave} ondrop={done} />

{#if shown}
  <div class="drop" role="status" aria-live="polite">
    <div class="card">
      <Icon name="pull" size={28} />
      {#if tab}
        <strong>Let go to preview {files(count)}</strong>
        <span>as patches or a diff for <span class="mono">{tab.name}</span>{#if branch}, on <span class="mono">{branch}</span>{/if}. Nothing changes until you apply.</span>
      {:else if session.tabs.length}
        <strong>Switch to a repository’s tab first</strong>
        <span>Patches and diffs dropped here go to the repository on screen.</span>
      {:else}
        <strong>Open a repository first</strong>
        <span>Patches and diffs dropped here go to the repository on screen.</span>
      {/if}
    </div>
  </div>
{/if}

<style>
  /* Over everything, but never in the way: the drop lands on the window either way. */
  .drop {
    position: fixed;
    inset: 0;
    z-index: 900;
    display: grid;
    place-items: center;
    pointer-events: none;
    background: color-mix(in srgb, var(--paper) 72%, transparent);
    outline: 2px dashed var(--accent);
    outline-offset: -10px;
  }

  .card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    max-width: 420px;
    padding: 22px 28px;
    text-align: center;
    background: var(--paper);
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow);
    color: var(--ink);
  }

  .card :global(svg) {
    color: var(--accent);
  }

  strong {
    font-size: 17px;
    font-weight: 600;
  }

  span {
    font-size: 14px;
    line-height: 1.5;
    color: var(--ink-soft);
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--ink);
  }
</style>
