<script lang="ts">
  import { CancelDownload, CompareCommits, DownloadFiles } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import type { DiffTarget } from './diff/DiffView.svelte'
  import { errorText } from './errors'
  import Busy from './ui/Busy.svelte'
  import Loading from './ui/Loading.svelte'

  /** What changed from one commit's files to another's, as a list that opens each file's diff
   * (#1019): two commits compared, or a line of them as one change. from "" is nothing. In a
   * partial clone the names come at once and the line counts download by themselves, as a
   * commit's do (#1010). */
  let {
    path,
    from,
    to,
    title,
    openFile,
    comparison = $bindable(null),
  }: {
    path: string
    from: string
    to: string
    title: string
    openFile: (target: DiffTarget) => void
    comparison?: app.Comparison | null
  } = $props()

  /** How many files show before "Show all". */
  const FIRST_FILES = 40

  let error = $state('')
  let all = $state(false)
  let counts = $state<'idle' | 'downloading' | 'stopped' | 'failed'>('idle')
  let countsError = $state('')
  let token = ''

  // Read again whenever the two commits change; a download once for each pair.
  $effect(() => {
    const [a, b] = [from, to]
    comparison = null
    error = ''
    all = false
    counts = 'idle'
    token = ''
    void CompareCommits(path, a, b)
      .then((c) => {
        if (a !== from || b !== to) return
        comparison = c
        if (c.countsMissing?.length) void download(a, b, [...c.countsMissing])
      })
      .catch((e) => {
        if (a === from && b === to) error = errorText(e)
      })
  })

  async function download(a: string, b: string, ids: string[]) {
    const mine = `${Date.now()}-${Math.random().toString(36).slice(2)}`
    token = mine
    counts = 'downloading'
    countsError = ''
    try {
      await DownloadFiles(path, mine, ids)
      if (token !== mine) return
      const c = await CompareCommits(path, a, b)
      if (token !== mine || a !== from || b !== to) return
      comparison = c
      counts = c.countsMissing?.length ? 'failed' : 'idle'
    } catch (err) {
      if (token !== mine) return
      const text = errorText(err)
      counts = /cancel|stopp?ed|killed|interrupt/i.test(text) ? 'stopped' : 'failed'
      countsError = counts === 'failed' ? text : ''
    }
  }

  function stop() {
    counts = 'stopped'
    const t = token
    token = ''
    void CancelDownload(t)
  }

  let files = $derived(comparison ? (all ? comparison.files : comparison.files.slice(0, FIRST_FILES)) : [])
  const STATUS: Record<string, string> = { A: 'added', D: 'deleted', R: 'renamed', C: 'copied', T: 'type changed' }
</script>

<section aria-label={title}>
  <h3 class="meta">{comparison ? `${title} · ${comparison.files.length.toLocaleString()}` : title}</h3>
  {#if error}
    <p class="say">{error}</p>
  {:else if !comparison}
    <Loading label="Reading what changed…" />
  {:else if !comparison.files.length}
    <p class="say">No files differ.</p>
  {:else}
    {#if counts === 'downloading'}
      <p class="note" role="status"><Busy /> Downloading the files for their line counts… <button class="btn text" onclick={stop}>Stop</button></p>
    {:else if (counts === 'stopped' || counts === 'failed') && comparison.countsMissing?.length}
      <p class="note" role="status">
        {counts === 'failed' && countsError ? `The line counts couldn’t be downloaded: ${countsError}` : 'Line counts need these files, which this partial clone hasn’t got yet.'}
        <button class="btn text" onclick={() => comparison && void download(from, to, [...(comparison.countsMissing ?? [])])}>Download</button>
      </p>
    {/if}
    <ul class="files">
      {#each files as f (f.path)}
        <li>
          <button class="file" title={f.oldPath ? `${f.oldPath} → ${f.path}` : f.path} onclick={() => openFile({ mode: 'compare', commit: `${from}..${to}`, file: f.path, oldPath: f.oldPath })}>
            <span class="mono path">{f.path}</span>
            {#if STATUS[f.status]}<span class="status">{STATUS[f.status]}</span>{/if}
            {#if !f.binary && (f.added || f.deleted)}<span class="mono added">+{f.added}</span><span class="mono deleted">−{f.deleted}</span>{/if}
          </button>
        </li>
      {/each}
    </ul>
    {#if comparison.files.length > FIRST_FILES && !all}
      <button class="btn sm text" onclick={() => (all = true)}>Show all {comparison.files.length.toLocaleString()}</button>
    {/if}
  {/if}
</section>

<style>
  h3 {
    margin: 0 0 6px;
    font-weight: 400;
  }

  .say,
  .note {
    margin: 0;
    color: var(--ink-soft);
  }

  .note {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-bottom: 6px;
    font-size: 13px;
  }

  .files {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .files li {
    border-top: 1px dashed var(--hairline);
  }

  .files li:first-child {
    border-top: none;
  }

  .file {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    min-height: 32px;
    padding: 0;
    border: 0;
    background: none;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .file:hover .path {
    color: var(--link);
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .path {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .status {
    flex: none;
    font-size: 12px;
    color: var(--ink-mute);
  }

  .added {
    color: var(--success);
  }

  .deleted {
    color: var(--danger);
  }
</style>
