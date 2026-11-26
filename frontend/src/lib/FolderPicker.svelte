<script lang="ts">
  // The clone dialog's second step (#1004): which folders to write, from the clone's own tree.
  // Each folder says how many files it holds; the line under the list says how many the
  // checkout will write, by cone mode's rule, so it's what you'll get.
  import { childrenOf, filesWritten, nameOf, outermost, within, type Folder } from './clone'

  let {
    counts,
    busy = false,
    onCheckout,
    onEvery,
  }: {
    counts: Folder[]
    /** The checkout is running. */
    busy?: boolean
    onCheckout: (folders: string[]) => void
    /** Every folder after all. */
    onEvery: () => void
  } = $props()

  let chosen = $state<string[]>([])
  let open = $state<string[]>([])
  let filter = $state('')

  let total = $derived(counts.find((c) => c.path === '')?.files ?? 0)
  let topFiles = $derived(counts.find((c) => c.path === '')?.direct ?? 0)
  let picked = $derived(outermost(chosen))
  let written = $derived(filesWritten(counts, picked))
  let checkoutLabel = $derived(picked.length === 1 ? 'Check out 1 folder' : picked.length ? `Check out ${picked.length} folders` : 'Check out folders')

  /** The rows shown: the tree as far as it's opened, or every folder matching the filter. */
  let rows = $derived.by(() => {
    const f = filter.trim().toLowerCase()
    if (f) return counts.filter((c) => c.path !== '' && c.path.toLowerCase().includes(f)).map((c) => ({ folder: c, depth: 0, kids: false, flat: true }))
    const out: { folder: Folder; depth: number; kids: boolean; flat: boolean }[] = []
    const walk = (parent: string, depth: number) => {
      for (const c of childrenOf(counts, parent)) {
        const kids = childrenOf(counts, c.path).length > 0
        out.push({ folder: c, depth, kids, flat: false })
        if (kids && open.includes(c.path)) walk(c.path, depth + 1)
      }
    }
    walk('', 0)
    return out
  })

  function toggle(path: string, on: boolean) {
    chosen = on ? [...chosen.filter((p) => p !== path), path] : chosen.filter((p) => p !== path)
  }

  function expand(path: string) {
    open = open.includes(path) ? open.filter((p) => p !== path) : [...open, path]
  }

  const count = (n: number) => n.toLocaleString()
</script>

<div class="picker">
  <input class="filter" placeholder="Filter folders" aria-label="Filter folders" spellcheck="false" bind:value={filter} />
  <ul class="folders" aria-label="Folders">
    {#each rows as r (r.folder.path)}
      {@const inside = within(r.folder.path, picked) && !picked.includes(r.folder.path)}
      <li class="row" class:picked={within(r.folder.path, picked)} style:padding-left="{8 + r.depth * 20}px">
        {#if r.kids}
          <button class="twisty" aria-label="{open.includes(r.folder.path) ? 'Hide' : 'Show'} the folders in {r.folder.path}" aria-expanded={open.includes(r.folder.path)} onclick={() => expand(r.folder.path)}
            >{open.includes(r.folder.path) ? '▾' : '▸'}</button
          >
        {:else}
          <span class="twisty" aria-hidden="true"></span>
        {/if}
        <label>
          <input
            type="checkbox"
            checked={within(r.folder.path, picked)}
            disabled={inside || busy}
            onchange={(e) => toggle(r.folder.path, e.currentTarget.checked)}
          />
          <span class="name">{r.flat ? r.folder.path : nameOf(r.folder.path)}/</span>
        </label>
        <span class="files">{count(r.folder.files)} {r.folder.files === 1 ? 'file' : 'files'}</span>
      </li>
    {:else}
      <li class="none">No folder matches “{filter}”.</li>
    {/each}
  </ul>
  <p class="sum" role="status">
    {#if picked.length}
      <strong>{count(written)} of {count(total)} files</strong> will be written: the {picked.length === 1 ? 'folder' : `${picked.length} folders`}, the {count(topFiles)}
      files at the top, and any directly in the folders above them, which every sparse checkout keeps.
    {:else}
      Choose the folders to check out. The {count(topFiles)} files at the top come with any of them.
    {/if}
  </p>
  <div class="form-actions">
    <button type="button" class="btn secondary" disabled={busy} onclick={onEvery}>Every folder after all</button>
    <button type="button" class="btn primary" disabled={busy || !picked.length} onclick={() => onCheckout(picked)}>{checkoutLabel}</button>
  </div>
</div>

<style>
  .picker {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .filter {
    font-family: var(--font-mono);
    font-size: 12.5px;
    padding: 6px 8px;
    border: 1px solid var(--paper-edge);
    border-radius: 4px;
    background: var(--paper);
    color: var(--ink);
  }

  .folders {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 320px;
    overflow: auto;
    border-top: 1px solid var(--paper-edge);
    border-bottom: 1px solid var(--paper-edge);
  }

  .row {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 3px 8px;
    border-bottom: 1px dashed var(--hairline);
    font-size: 14px;
  }

  .row.picked {
    background: color-mix(in srgb, var(--accent) 8%, transparent);
  }

  .row label {
    display: flex;
    align-items: center;
    gap: 6px;
    flex: 1;
    min-width: 0;
  }

  .row input {
    accent-color: var(--accent);
    margin: 0;
  }

  .twisty {
    width: 18px;
    flex: none;
    border: 0;
    background: none;
    padding: 0;
    color: var(--ink-mute);
    cursor: pointer;
  }

  .name {
    font-family: var(--font-mono);
    font-size: 13px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .files {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    white-space: nowrap;
  }

  .none {
    padding: 8px;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .sum {
    margin: 0;
    font-size: 13.5px;
    line-height: 1.5;
    color: var(--ink-soft);
  }

  .sum strong {
    font-weight: 500;
    color: var(--ink);
  }
</style>
