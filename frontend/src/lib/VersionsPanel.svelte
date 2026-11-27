<script lang="ts">
  import type { app, git } from '../../wailsjs/go/models'
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime.js'
  import Icon from './ui/Icon.svelte'
  import { isSame, kindOf, mergesNote, noteOf, placesOf, summaryOf } from './versions'

  /** Two versions of a branch, compared (#1038): what became of each old commit. */
  let {
    compared,
    branch,
    oldLabel,
    onClose,
    onShow,
  }: {
    compared: app.VersionsCompared
    /** The branch, or "" when the versions aren't one branch's. */
    branch: string
    /** What the old version is: "before the rebase · 4 min ago", or its name. */
    oldLabel: string
    onClose: () => void
    /** Selects a commit in the graph. */
    onShow: (id: string) => void
  } = $props()

  let hideSame = $state(false)
  let picked = $state(-1)
  let copied = $state(false)
  let shown = $derived(compared.pairs.map((p, i) => ({ p, i })).filter(({ p }) => !(hideSame && isSame(p))))
  let hidden = $derived(compared.pairs.length - shown.length)
  const short = (id: string) => id.slice(0, 7)

  function pick(i: number, p: git.RangePair) {
    picked = i
    const id = p.newId || p.oldId
    if (id) onShow(id)
  }

  async function copy() {
    if ((await ClipboardSetText(compared.text).catch(() => false)) === false) return
    copied = true
    setTimeout(() => (copied = false), 1500)
  }
</script>

<aside class="versions" aria-label="Two versions{branch ? ` of ${branch}` : ''}">
  <header>
    <span class="hand">two versions</span>
    <div class="title">
      <h2>{branch || 'Two ranges'}</h2>
      <button class="btn sm quiet icon" aria-label="Close the comparison" title="Close the comparison" onclick={onClose}><Icon name="close" size={14} /></button>
    </div>
    <p class="sides">
      {oldLabel} <span class="mono id">{short(compared.old.tip)}</span> → now <span class="mono id">{short(compared.new.tip)}</span>
    </p>
    <p class="summary">{summaryOf(compared.pairs, compared.oldCount.commits, compared.newCount.commits)}</p>
    {#if mergesNote(compared.oldCount.merges, compared.newCount.merges)}
      <p class="summary">{mergesNote(compared.oldCount.merges, compared.newCount.merges)}</p>
    {/if}
  </header>

  <div class="tools">
    <label><input type="checkbox" bind:checked={hideSame} /> Hide the same</label>
    <span class="grow"></span>
    <button class="btn sm text" onclick={copy}>{copied ? 'Copied' : 'Copy as text'}</button>
  </div>

  {#if compared.pairs.length}
    <ol class="rows" aria-label="What became of each commit">
      {#each shown as { p, i } (i)}
        {@const k = kindOf(p)}
        <li>
          <button class={['row', picked === i && 'picked']} onclick={() => pick(i, p)}>
            <span class="places mono">{placesOf(p)}</span>
            <span class="what">
              <span class="subject">{p.subject}</span>
              {#if noteOf(p)}<span class="note">{noteOf(p)}</span>{/if}
            </span>
            <span class="pill {k.tone}">{k.word}</span>
          </button>
        </li>
      {/each}
    </ol>
    {#if hidden}<p class="foot">{hidden.toLocaleString()} the same, hidden.</p>{/if}
  {:else}
    <p class="foot">Neither version has any commits.</p>
  {/if}

  <p class="command">As <code>{compared.command}</code> pairs them.</p>
</aside>

<style>
  .versions {
    display: flex;
    flex-direction: column;
    gap: 10px;
    height: 100%;
    min-height: 0;
    padding: 16px 18px;
    box-sizing: border-box;
    overflow: auto;
    background: var(--paper);
  }

  header {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .hand {
    font-family: var(--font-hand);
    font-size: 20px;
    color: var(--accent-a);
    transform: rotate(-1.5deg);
    transform-origin: left;
  }

  .title {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  h2 {
    flex: 1;
    margin: 0;
    font-size: 19px;
    font-weight: 500;
    overflow-wrap: anywhere;
  }

  .sides,
  .summary {
    margin: 0;
    font-size: 13px;
    line-height: 1.5;
    color: var(--ink-soft);
  }

  .summary {
    color: var(--ink-mute);
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .id {
    color: var(--accent-b);
  }

  .tools {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13.5px;
  }

  .tools label {
    display: flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
  }

  .tools input {
    accent-color: var(--accent);
    margin: 0;
  }

  .grow {
    flex: 1;
  }

  .rows {
    margin: 0;
    padding: 0;
    list-style: none;
    border-top: 1px dashed var(--hairline);
  }

  .rows li {
    border-bottom: 1px dashed var(--hairline);
  }

  .row {
    width: 100%;
    display: grid;
    grid-template-columns: 52px minmax(0, 1fr) auto;
    gap: 10px;
    align-items: baseline;
    padding: 9px 6px;
    border: 0;
    border-radius: var(--r-sm, 6px);
    background: none;
    color: var(--ink);
    text-align: left;
    cursor: pointer;
  }

  .row:hover {
    background: var(--wash-hover);
  }

  .row.picked {
    background: color-mix(in srgb, var(--accent) 10%, transparent);
  }

  .places {
    color: var(--ink-mute);
    font-size: 12px;
  }

  .what {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .subject {
    font-size: 14px;
    line-height: 1.35;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .note {
    font-size: 12px;
    color: var(--ink-mute);
    overflow-wrap: anywhere;
  }

  .pill {
    padding: 2px 9px;
    border-radius: 10px;
    font-size: 12px;
    white-space: nowrap;
  }

  .pill.changed {
    background: color-mix(in srgb, var(--accent) 14%, transparent);
    color: var(--danger);
  }

  .pill.around {
    background: var(--wash-hover);
    color: var(--ink-soft);
  }

  .pill.same {
    background: var(--success-wash);
    color: var(--success);
  }

  .pill.gone {
    background: color-mix(in srgb, var(--accent-b) 14%, transparent);
    color: var(--accent-b);
  }

  .pill.new {
    background: color-mix(in srgb, var(--accent-a) 14%, transparent);
    color: var(--accent-a);
  }

  .foot,
  .command {
    margin: 0;
    font-size: 12.5px;
    color: var(--ink-mute);
  }

  .command {
    margin-top: auto;
    padding-top: 8px;
  }

  .command code {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-b);
  }
</style>
