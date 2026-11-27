<script lang="ts">
  import { patchLines } from './amPlan'
  import Modal from './ui/Modal.svelte'

  // Show the patch (#1023): the patch git am couldn't apply, as git keeps it. There's no
  // version here to set it beside, so it reads as the patch itself, its lines in the diff's
  // colours.
  let { title, text, onClose }: { title: string; text: string; onClose: () => void } = $props()

  let read = $derived(patchLines(text))
</script>

<Modal title="The patch that didn't apply" width={760} {onClose}>
  {#snippet subtitle()}{title}{/snippet}
  <!-- Its classes are prefixed: the app has a global .meta (uppercase mono labels). -->
  <pre class="patch" aria-label="The patch">{#each read.lines as l, i (i)}<span class="l-{l.kind}">{l.text || ' '}</span>{/each}</pre>
  {#if read.more}
    <p class="more">{read.more.toLocaleString('en-US')} more lines aren't shown.</p>
  {/if}
  <div class="foot">
    <button class="btn secondary" onclick={onClose}>Close</button>
  </div>
</Modal>

<style>
  .patch {
    max-height: min(60vh, 560px);
    margin: 0;
    padding: 6px 0;
    overflow: auto;
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.55;
    background: var(--paper);
    border: 1px solid var(--paper-edge);
    border-radius: 6px;
  }

  .patch span {
    display: block;
    padding: 0 12px;
    white-space: pre;
  }

  .l-file {
    margin-top: 6px;
    font-weight: 600;
    color: var(--ink);
  }

  .l-file:first-child {
    margin-top: 0;
  }

  .l-meta {
    color: var(--ink-mute);
  }

  .l-hunk {
    color: var(--syn-keyword);
    background: var(--paper-warm);
  }

  .l-add {
    background: var(--success-wash);
  }

  .l-del {
    background: var(--danger-wash);
  }

  .more {
    margin: 8px 0 0;
    font-size: 12px;
    color: var(--ink-mute);
  }

  .foot {
    display: flex;
    justify-content: flex-end;
    margin-top: 14px;
  }
</style>
