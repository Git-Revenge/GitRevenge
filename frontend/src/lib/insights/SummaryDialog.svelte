<script lang="ts">
  // Copy summary (#252): the Insights results as Markdown, with the inputs they came from at
  // the top, ready to paste into review notes or save.
  import { SaveFile } from '../../../wailsjs/go/app/App.js'
  import { ClipboardSetText } from '../../../wailsjs/runtime/runtime.js'
  import { errorText } from '../errors'
  import InkButton from '../ui/InkButton.svelte'
  import Modal from '../ui/Modal.svelte'
  import { textToBase64 } from './export'
  import { insights } from './insights.svelte'
  import { attention } from './model'
  import { exportName, summary } from './report'

  let { onClose }: { onClose: () => void } = $props()

  let shipped = $state(true)
  let needsAttention = $state(true)
  let hotspots = $state(false)
  let groupByReference = $state(true)
  let status = $state('')

  let text = $derived(
    summary(insights.reportInputs(), insights.items, attention(insights.branches), insights.hot, { shipped, attention: needsAttention, hotspots, groupByReference }, insights.repoName),
  )

  $effect(() => {
    if (hotspots) void insights.loadHotspots()
  })

  async function copy() {
    status = (await ClipboardSetText(text).catch(() => false)) ? 'Copied.' : "Couldn't copy to the clipboard."
  }

  async function save() {
    try {
      const path = await SaveFile('Save summary', exportName(insights.label(), 'summary', insights.reportInputs().windows[0]?.asOf ?? 0, 'md'), textToBase64(text))
      if (path) status = `Saved to ${path}.`
    } catch (e) {
      status = errorText(e)
    }
  }
</script>

<Modal title="Copy summary" eyebrow="for the sprint review" width={760} {onClose}>
  <div class="options">
    <span class="segmented" role="group" aria-label="Include">
      <button aria-pressed={shipped} onclick={() => (shipped = !shipped)}>shipped</button>
      <button aria-pressed={needsAttention} onclick={() => (needsAttention = !needsAttention)}>needs attention</button>
      <button aria-pressed={hotspots} onclick={() => (hotspots = !hotspots)}>hotspots</button>
    </span>
    <label class="form-check"><input type="checkbox" bind:checked={groupByReference} /> Group by reference</label>
  </div>

  <pre class="md code-ground" aria-label="Summary text">{text}</pre>

  {#if insights.pending}
    <p class="pending" role="status">
      {insights.pending}
      {insights.pending === 1 ? 'conflict check is' : 'conflict checks are'} still running. Copy now and {insights.pending === 1 ? "it's" : "they're"} listed as pending, or wait a moment.
    </p>
  {/if}

  <div class="form-actions">
    <span class="note">{status || 'The inputs at the top let anyone with these commits get the same summary.'}</span>
    <button class="btn text" onclick={save}>Save as .md</button>
    <InkButton onclick={copy}>Copy</InkButton>
  </div>
</Modal>

<style>
  .options {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px 18px;
    margin-bottom: 14px;
  }

  .segmented button {
    white-space: nowrap;
  }

  .md {
    max-height: 50vh;
    overflow: auto;
    margin: 0;
    padding: 16px 18px;
    border-radius: var(--r-sm);
    background: var(--code-bg);
    color: var(--code-text);
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.7;
    white-space: pre-wrap;
  }

  .pending {
    margin: 14px 0 0;
    padding: 10px 14px;
    border-radius: var(--r-md);
    background: color-mix(in srgb, var(--ink-ochre) 20%, transparent);
    font-size: 14px;
  }

  .note {
    flex: 1 1 260px;
    font-size: 13px;
    color: var(--ink-mute);
  }
</style>
