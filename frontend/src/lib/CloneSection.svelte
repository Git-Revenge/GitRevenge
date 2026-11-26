<script lang="ts">
  // The Repository panel's This clone section (#1005): shown for a clone that is shallow,
  // partial, sparse or one branch, whoever made it. Each row says what the clone has and
  // offers the rest. Every action runs as a fetch does: progress and Stop in the toolbar, and
  // the graph and lists updating when it lands.
  import { Backfill, BackfillCost, FetchEveryBranch, FetchMoreHistory, RepoFolders, SetSparseFolders, StopBackfillCost } from '../../wailsjs/go/app/App.js'
  import type { git } from '../../wailsjs/go/models'
  import type { MenuItem } from './ContextMenu.svelte'
  import type { DialogSpec } from './Dialog.svelte'
  import { errorText } from './errors'
  import FolderPicker from './FolderPicker.svelte'
  import { network } from './network.svelte'
  import { folderNames, moreHistory, partialWords, shallowLines, sizeWords, sparseLines, type Shape } from './thisClone'
  import Busy from './ui/Busy.svelte'
  import Icon from './ui/Icon.svelte'
  import Modal from './ui/Modal.svelte'

  let {
    path,
    shape,
    openDialog,
    onMenu,
  }: {
    path: string
    shape: Shape
    openDialog: (spec: DialogSpec) => void
    onMenu?: (items: MenuItem[], x: number, y: number) => void
  } = $props()

  let busy = $derived(!!network.running[path])

  /** How long the count runs before the dialog says it's a long one. */
  const SLOW_COUNT = 8000

  function moreMenu(e: MouseEvent) {
    if (!shape.shallow) return
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    const items: MenuItem[] = moreHistory(shape.shallow.since).map((c) => ({
      label: c.label,
      icon: 'fetch',
      run: () => void network.run(path, 'history', () => FetchMoreHistory(path, c.more as git.MoreHistory)),
    }))
    onMenu?.(items, r.left, r.bottom + 2)
  }

  // Download them all: says what it costs first. Counting walks the whole history's folder
  // listings, so the count arrives while the dialog is open, and stops when it closes. On the
  // kernel it hadn't finished in ten minutes: after a while the dialog says so, and the
  // download needn't wait for it.
  let backfill = $state<{ cost: git.BackfillCost | null; error: string; slow: boolean } | null>(null)
  let slowTimer: ReturnType<typeof setTimeout> | undefined
  function askBackfill() {
    backfill = { cost: null, error: '', slow: false }
    clearTimeout(slowTimer)
    slowTimer = setTimeout(() => {
      if (backfill && !backfill.cost && !backfill.error) backfill = { ...backfill, slow: true }
    }, SLOW_COUNT)
    BackfillCost(path)
      .then((cost) => {
        if (backfill) backfill = { cost, error: '', slow: false }
      })
      .catch((e) => {
        if (backfill) backfill = { cost: null, error: errorText(e), slow: false }
      })
  }
  function closeBackfill() {
    clearTimeout(slowTimer)
    if (backfill && !backfill.cost) void StopBackfillCost(path)
    backfill = null
  }
  function runBackfill() {
    closeBackfill()
    void network.run(path, 'backfill', () => Backfill(path))
  }

  // Change folders: the folder list from the clone's second step, the current ones ticked.
  let folders = $state<{ counts: git.FolderCount[] | null; error: string } | null>(null)
  function changeFolders() {
    folders = { counts: null, error: '' }
    RepoFolders(path)
      .then((counts) => {
        if (folders) folders = { counts, error: '' }
      })
      .catch((e) => {
        if (folders) folders = { counts: null, error: errorText(e) }
      })
  }
  function checkOut(chosen: string[]) {
    folders = null
    void network.run(path, 'folders', () => SetSparseFolders(path, chosen))
  }

  function everyFolder() {
    const s = shape.sparse
    if (!s) return
    const left = s.total - s.files
    openDialog({
      title: 'Check out every folder?',
      message: `This writes the ${left.toLocaleString('en-US')} ${left === 1 ? 'file' : 'files'} the sparse checkout leaves out${shape.partial ? ', downloading the ones this partial clone hasn’t got' : ''}. On a big repository that takes a while; Stop is in the toolbar.`,
      submit: 'Check out every folder',
      run: async () => {
        void network.run(path, 'folders', () => SetSparseFolders(path, []))
      },
    })
  }

  function everyBranch() {
    const b = shape.oneBranch
    if (b) void network.run(path, 'branches', () => FetchEveryBranch(path, b.remote))
  }
</script>

<section aria-labelledby="this-clone-{path}">
  <div class="head">
    <h3 class="meta" id="this-clone-{path}">This clone</h3>
  </div>
  <ul class="parts">
    {#if shape.shallow}
      {@const l = shallowLines(shape.shallow)}
      <li>
        <p class="what">{l.what}</p>
        <p class="kind">{l.tag}</p>
        <div class="acts">
          <button class="btn sm secondary" disabled={busy} aria-haspopup="menu" onclick={moreMenu}>Fetch more history <Icon name="chevron" size={12} /></button>
        </div>
      </li>
    {/if}
    {#if shape.partial}
      <li>
        <p class="what">{partialWords(shape.partial.filter)}</p>
        <p class="kind">partial · from {shape.partial.remote}</p>
        <div class="acts">
          <button class="btn sm secondary" disabled={busy} aria-haspopup="dialog" onclick={askBackfill}>Download them all…</button>
        </div>
      </li>
    {/if}
    {#if shape.sparse}
      {@const l = sparseLines(shape.sparse)}
      <li>
        <p class="what">{l.what}</p>
        <p class="kind">{l.tag}</p>
        {#if shape.sparse.folders.length}<p class="names" title={shape.sparse.folders.join('\n')}>{folderNames(shape.sparse.folders)}</p>{/if}
        <div class="acts">
          <button
            class="btn sm secondary"
            disabled={busy || !shape.sparse.cone}
            aria-haspopup="dialog"
            title={shape.sparse.cone ? 'Choose the folders to check out' : 'These are patterns, not folders: change them with git sparse-checkout'}
            onclick={changeFolders}>Change folders…</button
          >
          <button class="btn sm quiet" disabled={busy} aria-haspopup="dialog" onclick={everyFolder}>Every folder</button>
        </div>
      </li>
    {/if}
    {#if shape.oneBranch}
      <li>
        <p class="what">Only <span class="mono">{shape.oneBranch.branch}</span> from {shape.oneBranch.remote}</p>
        <p class="kind">one branch</p>
        <div class="acts">
          <button class="btn sm secondary" disabled={busy} onclick={everyBranch}>Fetch every branch</button>
        </div>
      </li>
    {/if}
  </ul>
</section>

{#if backfill}
  <Modal title="Download every file?" eyebrow="this clone" width={480} onClose={closeBackfill}>
    <div class="ask">
      <p>
        Every version of every file in this branch’s history{shape.sparse ? ', in the folders checked out,' : ''} that this partial clone hasn’t got, from {shape.partial?.remote ?? 'its remote'}. It comes
        in batches, and later views of old files open at once.
      </p>
      <p class="cost" role="status">
        {#if backfill.error}
          The size couldn’t be worked out: {backfill.error}
        {:else if !backfill.cost && backfill.slow}
          <span class="waiting"><Busy /> Still counting: a long history takes a while to count. The download needn’t wait for it.</span>
        {:else if !backfill.cost}
          <span class="waiting"><Busy /> Counting what it would download…</span>
        {:else if backfill.cost.missing === 0}
          Nothing is missing: this branch’s history has every file already.
        {:else}
          <strong>{backfill.cost.missing.toLocaleString('en-US')} {backfill.cost.missing === 1 ? 'version' : 'versions'}</strong> to download. The clone takes {sizeWords(backfill.cost.size)} now;
          they add to that.
        {/if}
      </p>
      <div class="form-actions">
        <button type="button" class="btn quiet" onclick={closeBackfill}>Not now</button>
        <button type="button" class="btn primary" disabled={backfill.cost?.missing === 0} onclick={runBackfill}>Download them all</button>
      </div>
    </div>
  </Modal>
{/if}

{#if folders}
  <Modal title="Change folders" eyebrow="this clone" width={560} onClose={() => (folders = null)}>
    {#if folders.error}
      <p class="cost" role="alert">The folders couldn’t be read: {folders.error}</p>
    {:else if !folders.counts}
      <p class="cost" role="status"><span class="waiting"><Busy /> Reading the folders…</span></p>
    {:else}
      <FolderPicker counts={folders.counts} initial={shape.sparse?.folders ?? []} everyLabel="Every folder" onCheckout={checkOut} onEvery={() => checkOut([])} />
    {/if}
  </Modal>
{/if}

<style>
  /* The panel's sections are ruled off from each other; this one comes first. */
  section {
    border-bottom: 1px dotted var(--paper-edge);
    margin-bottom: 4px;
    padding-bottom: 4px;
  }

  .head {
    display: flex;
    align-items: center;
    min-height: 32px;
    padding: 0 6px 0 10px;
  }

  h3 {
    margin: 0;
    font-weight: 400;
    color: var(--ink-soft);
  }

  .parts {
    list-style: none;
    margin: 0;
    padding: 0 10px 6px;
    display: flex;
    flex-direction: column;
  }

  .parts li {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 6px 0 9px;
  }

  .parts li + li {
    border-top: 1px dashed var(--paper-edge);
  }

  p {
    margin: 0;
  }

  .what {
    font-size: 14px;
    color: var(--ink);
    line-height: 1.35;
  }

  .kind {
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .names {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-soft);
    line-height: 1.5;
    overflow-wrap: anywhere;
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 4px;
  }

  .ask {
    display: flex;
    flex-direction: column;
    gap: 12px;
    line-height: 1.5;
  }

  .cost {
    color: var(--ink-soft);
  }

  .waiting {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
</style>
