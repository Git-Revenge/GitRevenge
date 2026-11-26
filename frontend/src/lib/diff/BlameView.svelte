<script lang="ts">
  import Icon from '../ui/Icon.svelte'
  import { Blame, BlameDownloading, BlameNeeds, CancelDownload, DownloadFiles } from '../../../wailsjs/go/app/App.js'
  import type { git } from '../../../wailsjs/go/models'
  import { errorText } from '../errors'
  import Busy from '../ui/Busy.svelte'
  import { formatTime, laneVar } from '../graph/draw'
  import { formatShortcut, provideCommands } from '../commands.svelte'
  import { session } from '../session.svelte'
  import type { DiffTarget } from './DiffView.svelte'
  import { highlightLines, languageFor, type Token } from './highlight'
  import { toggleWrap, wrap, wrapCommand } from './wrap.svelte'
  import Loading from '../ui/Loading.svelte'

  let {
    path,
    file,
    rev = '',
    line = 0,
    onClose,
    onOpen,
    onReveal,
  }: {
    path: string
    file: string
    rev?: string
    /** A line (1-based) to wash and bring into view, as Find in files opens one (#980). */
    line?: number
    onClose: () => void
    onOpen: (t: DiffTarget) => void
    /** Shows a commit in the graph. */
    onReveal: (commit: string) => void
  } = $props()

  let lines = $state<git.BlameLine[] | null>(null)
  let tokens = $state<Token[][]>([])

  /** A stable color per commit, so a commit's blocks read as one. */
  let colorOf = $derived.by(() => {
    const m: Record<string, number> = {}
    let n = 0
    for (const l of lines ?? []) if (!(l.commit in m)) m[l.commit] = n++
    return m
  })

  // In a partial clone, blame reads versions that were never downloaded: it asks first, and
  // they come in one download (#1003). Git would fetch them one at a time.
  let ask = $state<{ versions: number; missing: string[] } | null>(null)
  let downloading = $state(false)
  let failed = $state('')
  let token = ''

  async function show(f: string, res: git.BlameLine[] | undefined) {
    lines = res ?? []
    const text = lines.map((l) => l.line).join('\n')
    tokens = highlightLines(text, null)
    const lang = await languageFor(f)
    if (lang && f === file) tokens = highlightLines(text, lang)
  }

  $effect(() => {
    const [f, r] = [file, rev]
    lines = null
    ask = null
    failed = ''
    void (async () => {
      const needs = await session.run(() => BlameNeeds(path, f, r))
      if (f !== file || r !== rev) return
      if (needs?.missing.length) {
        ask = { versions: needs.versions, missing: needs.missing }
        return
      }
      const res = await session.run(() => Blame(path, f, r))
      if (f === file && r === rev) await show(f, res)
    })()
  })

  async function downloadAndBlame() {
    if (!ask) return
    const [f, r, want] = [file, rev, ask.missing]
    const mine = `${Date.now()}-${Math.random().toString(36).slice(2)}`
    token = mine
    downloading = true
    failed = ''
    try {
      await DownloadFiles(path, mine, want)
      // Anything blame still finds missing, such as a version from before a rename, it may
      // fetch now: the person said to download.
      const res = await BlameDownloading(path, f, r)
      if (f !== file || r !== rev) return
      ask = null
      await show(f, res)
    } catch (err) {
      if (token === mine) failed = errorText(err)
    } finally {
      if (token === mine) downloading = false
    }
  }

  function stop() {
    void CancelDownload(token)
  }

  /** How many of the file's versions are missing, in words. */
  let missingText = $derived(!ask ? '' : ask.missing.length >= ask.versions ? `any of its ${ask.versions.toLocaleString()} versions` : `${ask.missing.length.toLocaleString()} of its ${ask.versions.toLocaleString()} versions`)

  provideCommands(() => [wrapCommand()])

  const uncommitted = (c: string) => /^0+$/.test(c)

  /** Brings the line asked for into view once it's drawn. */
  const scrollHere = (row: HTMLElement) => row.scrollIntoView({ block: 'center' })
</script>

<div class="blame">
  <header class="view-head">
    <button class="btn quiet" onclick={onClose} title="Back"><Icon name="back" />Back</button>
    <span class="view-badge">Blame</span>
    <span class="view-file" title={file}>{file}</span>
    {#if rev}<span class="meta">at {rev.slice(0, 7)}</span>{/if}
    <span class="spacer"></span>
    <button class="btn quiet" aria-pressed={wrap.on} onclick={toggleWrap} title="Wrap long lines ({formatShortcut('Alt+Z')})"
      ><Icon name="wrap" />Wrap</button
    >
    <button class="btn quiet" onclick={() => onOpen({ mode: 'history', file })}>History</button>
  </header>
  <div class="body code-ground">
    {#if ask}
      <div class="ask" role="group" aria-label="Blame {file}?">
        <h2>Blame <span class="mono">{file}</span>?</h2>
        <p>Blame reads every version of the file, and this partial clone hasn’t got {missingText} yet. They come from the remote in one download first.</p>
        {#if failed}<p class="failed" role="alert">The download failed: {failed}</p>{/if}
        <div class="actions">
          {#if downloading}
            <span class="busy" role="status"><Busy /> Downloading {ask.missing.length.toLocaleString()} versions…</span>
            <button class="btn secondary" onclick={stop}>Stop</button>
          {:else}
            <button class="btn secondary" onclick={onClose}>Not now</button>
            <button class="btn primary" onclick={downloadAndBlame}>Download and blame</button>
          {/if}
        </div>
      </div>
    {:else if !lines}
      <Loading />
    {:else}
      <table class:wrap={wrap.on}>
        <tbody>
          {#each lines as l, i (i)}
            {@const start = i === 0 || lines[i - 1].commit !== l.commit}
            <tr class:start class:at={i + 1 === line} {@attach i + 1 === line ? scrollHere : undefined}>
              <td class="gutter" style:border-left-color={uncommitted(l.commit) ? 'transparent' : laneVar(colorOf[l.commit] ?? 0)}>
                {#if start}
                  {#if uncommitted(l.commit)}
                    <span class="muted">Not committed yet</span>
                  {:else}
                    <span class="who-when">
                      <button class="sha" onclick={() => onReveal(l.commit)} title="{l.summary}&#10;Show in graph">{l.commit.slice(0, 7)}</button>
                      <span class="who" title={l.author}>{l.author}</span>
                      <span class="muted when">{formatTime(l.time)}</span>
                    </span>
                  {/if}
                {/if}
              </td>
              <td class="num">{i + 1}</td>
              <td class="code"
                >{#if tokens[i]}{#each tokens[i] as t, k (k)}<span class={t.cls}>{t.text}</span>{/each}{:else}{l.line}{/if}</td
              >
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>

<style>
  .blame {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }

  .spacer {
    flex: 1;
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }

  .ask {
    display: flex;
    flex-direction: column;
    gap: 10px;
    max-width: 560px;
    margin: 32px auto;
    padding: 18px 20px;
    border: 1px solid var(--paper-edge);
    border-radius: 10px;
    background: var(--paper);
    color: var(--ink);
  }

  .ask h2 {
    margin: 0;
    font-size: 19px;
    font-weight: 400;
  }

  .ask p {
    margin: 0;
    font-size: 14px;
    line-height: 1.5;
    color: var(--ink-soft);
  }

  .ask .failed {
    color: var(--danger);
  }

  .ask .actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
  }

  .ask .busy {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-right: auto;
    font-size: 13px;
    color: var(--ink-mute);
  }

  table {
    border-collapse: collapse;
    font-size: 13px;
    line-height: 1.6;
  }

  tr.start td {
    border-top: 1px dotted var(--paper-edge);
  }

  tr.at td {
    background: var(--highlight);
  }

  td {
    padding: 0 8px;
    vertical-align: top;
    white-space: pre;
  }

  /* Each commit's lines carry its lane colour, as in the graph. */
  .gutter {
    width: 340px;
    max-width: 340px;
    border-left: 3px solid transparent;
    background: var(--paper-warm);
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: 13px;
  }

  /* A long name gives up its width: the date is what's being looked for. */
  .who-when {
    display: flex;
    align-items: baseline;
    width: 320px;
  }

  .sha {
    flex: none;
    padding: 0;
    background: none;
    border: none;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-a);
  }

  .sha:hover:not(:disabled) {
    background: none;
    color: var(--link);
  }

  .who {
    flex: 1;
    min-width: 0;
    margin: 0 6px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .when {
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .num {
    text-align: right;
    color: var(--ink-faint);
    user-select: none;
  }

  .code {
    font-family: var(--font-mono);
    tab-size: 4;
  }

  /* Wrapped lines keep the gutter and line number on their first visual line. */
  table.wrap {
    width: 100%;
  }

  .wrap .code {
    width: 100%;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .muted {
    color: var(--ink-mute);
  }

</style>
