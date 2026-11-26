<script lang="ts">
  import { SelectionFiles } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import type { DiffTarget } from './diff/DiffView.svelte'
  import type { Selected } from './graph/selection'
  import type { MenuContext } from './menus'
  import { commitCount, selectionActions } from './selectionMenu'
  import Icon from './ui/Icon.svelte'
  import Loading from './ui/Loading.svelte'

  /** The right-hand panel with several commits selected in the graph (#1018): who and when,
   * the files they change, the commits, and what can be done with them all. */
  let {
    path,
    selected,
    ctx,
    onUnpick,
    onShow,
    openFile,
  }: {
    path: string
    selected: Selected
    ctx: MenuContext
    /** Takes a commit out of the selection, by its graph row. */
    onUnpick: (row: number) => void
    /** Shows one commit alone, by its graph row. */
    onShow: (row: number) => void
    /** Opens a file's diff in one of the commits. */
    openFile: (target: DiffTarget) => void
  } = $props()

  /** How many files show before "Show all". */
  const FIRST_FILES = 12

  let info = $derived(selected.info)
  let files = $state<app.SelectionFiles | null>(null)
  let filesError = $state('')
  let allFiles = $state(false)
  /** The file whose commits are open under it, or "". */
  let opened = $state('')

  // The files are read once the commits are, and again whenever they change.
  $effect(() => {
    const sel = info
    files = null
    filesError = ''
    allFiles = false
    opened = ''
    if (!sel) return
    void SelectionFiles(path, sel.ids)
      .then((f) => {
        if (sel === info) files = f
      })
      .catch((e) => {
        if (sel === info) filesError = String(e?.message ?? e)
      })
  })

  let subjects = $derived(new Map((info?.commits ?? []).map((c) => [c.id, c])))
  let actions = $derived(info ? selectionActions(info, ctx) : [])
  let shownFiles = $derived(files ? (allFiles ? files.files : files.files.slice(0, FIRST_FILES)) : [])

  const short = (id: string) => id.slice(0, 7)
  const date = (s: number) => new Date(s * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
  function when(sel: app.Selection): string {
    const [a, b] = [date(sel.first), date(sel.last)]
    return a === b ? `on ${a}` : `${a} to ${b}`
  }
</script>

<aside class="panel" aria-label="Several commits selected">
  <header>
    <span class="hand">several at once</span>
    <h2>{commitCount(info?.ids.length ?? selected.count)}</h2>
    {#if info}
      <p class="who">{info.authors === 1 ? info.author : `${info.authors.toLocaleString()} authors`} · {when(info)}{#if info.merges} · {info.merges === 1 ? '1 merge' : `${info.merges.toLocaleString()} merges`}{/if}</p>
    {/if}
  </header>

  {#if !info}
    <Loading label="Reading them…" />
  {:else}
    <section aria-label="Files they change">
      <h3 class="meta">{files ? `Files they change · ${files.total.toLocaleString()}` : 'Files they change'}</h3>
      {#if filesError}
        <p class="say">{filesError}</p>
      {:else if !files}
        <Loading label="Reading their files…" />
      {:else if !files.files.length}
        <p class="say">None that can be shown: merges and a history's first commit have nothing to compare with here.</p>
      {:else}
        <ul class="files">
          {#each shownFiles as f (f.path)}
            <li>
              <button class="file" aria-expanded={opened === f.path} onclick={() => (opened = opened === f.path ? '' : f.path)}>
                <Icon name="chevron" size={12} />
                <span class="mono path" title={f.path}>{f.path}</span>
                <span class="mono in" title="Changed by {commitCount(f.commits.length)} of the {files.read.toLocaleString()} read">{f.commits.length} of {files.read.toLocaleString()}</span>
              </button>
              {#if opened === f.path}
                <ul class="by">
                  {#each f.commits as id (id)}
                    <li>
                      <button class="show" title="Open {f.path} in {short(id)}" onclick={() => openFile({ mode: 'commit', commit: id, file: f.path })}>
                        <span class="mono">{short(id)}</span>
                        <span class="subject">{subjects.get(id)?.subject ?? ''}</span>
                      </button>
                    </li>
                  {/each}
                </ul>
              {/if}
            </li>
          {/each}
        </ul>
        {#if files.files.length > FIRST_FILES && !allFiles}
          <button class="btn sm text" onclick={() => (allFiles = true)}>Show all {files.files.length.toLocaleString()}</button>
        {:else if files.total > files.files.length}
          <p class="more">and {(files.total - files.files.length).toLocaleString()} more</p>
        {/if}
        {#if files.read < info.ids.length}
          <p class="more">From the oldest {files.read.toLocaleString()} of them.</p>
        {/if}
      {/if}
    </section>

    <section aria-label="The commits">
      <h3 class="meta">The commits, oldest first</h3>
      <ol class="commits">
        {#each info.commits as c (c.id)}
          <li>
            <button class="show" title="Show {short(c.id)} alone" onclick={() => onShow(c.index)}>
              <span class="mono">{short(c.id)}</span>
              <span class="subject">{c.subject}</span>
            </button>
            <button class="btn sm quiet icon" aria-label="Take {c.subject} out of the selection" title="Take it out of the selection" onclick={() => onUnpick(c.index)}><Icon name="close" size={13} /></button>
          </li>
        {/each}
      </ol>
      {#if info.ids.length > info.commits.length}
        <p class="more">and {(info.ids.length - info.commits.length).toLocaleString()} more</p>
      {/if}
    </section>

    <section class="next" aria-label="What to do with them">
      {#each actions as a (a.label)}
        <button class="btn secondary" disabled={a.disabled} title={a.hint} onclick={() => a.run?.()}>
          {#if a.icon}<Icon name={a.icon} size={15} />{/if}
          {a.label}
        </button>
      {/each}
    </section>

    <p class="foot">Esc in the graph goes back to one commit.</p>
  {/if}
</aside>

<style>
  .panel {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 22px 24px;
    overflow-y: auto;
  }

  header {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 500;
    line-height: 1.25;
    letter-spacing: -0.01em;
  }

  h3 {
    margin: 0 0 6px;
    font-weight: 400;
  }

  .who,
  .foot,
  .more {
    margin: 0;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .foot {
    margin-top: auto;
  }

  .say {
    margin: 0;
    color: var(--ink-soft);
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .files,
  .by,
  .commits {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .files > li,
  .commits > li {
    border-top: 1px dashed var(--hairline);
  }

  .files > li:first-child,
  .commits > li:first-child {
    border-top: none;
  }

  .file,
  .show {
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

  .file :global(svg) {
    flex: none;
    color: var(--ink-mute);
    /* The chevron points down; closed, it points at the file instead. */
    transform: rotate(-90deg);
    transition: transform var(--dur-quick, 0.2s);
  }

  .file[aria-expanded='true'] :global(svg) {
    transform: none;
  }

  .file:hover .path,
  .show:hover .subject {
    color: var(--link);
  }

  .path,
  .subject {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .in {
    flex: none;
    color: var(--ink-mute);
  }

  .by {
    padding: 0 0 6px 20px;
  }

  .by .show {
    min-height: 28px;
    font-size: 14px;
  }

  .commits > li {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .show .mono {
    flex: none;
    color: var(--accent-b);
  }

  .next {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 14px 16px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
  }

  .next .btn {
    justify-content: flex-start;
  }

  @media (prefers-reduced-motion: reduce) {
    .file :global(svg) {
      transition: none;
    }
  }
</style>
