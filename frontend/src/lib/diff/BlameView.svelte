<script lang="ts">
  import Icon from '../ui/Icon.svelte'
  import { Blame } from '../../../wailsjs/go/app/App.js'
  import type { git } from '../../../wailsjs/go/models'
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
    onClose,
    onOpen,
    onReveal,
  }: {
    path: string
    file: string
    rev?: string
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

  $effect(() => {
    const [f, r] = [file, rev]
    lines = null
    void session.run(() => Blame(path, f, r)).then(async (res) => {
      if (f !== file || r !== rev) return
      lines = res ?? []
      const text = lines.map((l) => l.line).join('\n')
      tokens = highlightLines(text, null)
      const lang = await languageFor(f)
      if (lang && f === file) tokens = highlightLines(text, lang)
    })
  })

  provideCommands(() => [wrapCommand()])

  const uncommitted = (c: string) => /^0+$/.test(c)
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
    {#if !lines}
      <Loading />
    {:else}
      <table class:wrap={wrap.on}>
        <tbody>
          {#each lines as l, i (i)}
            {@const start = i === 0 || lines[i - 1].commit !== l.commit}
            <tr class:start>
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

  table {
    border-collapse: collapse;
    font-size: 13px;
    line-height: 1.6;
  }

  tr.start td {
    border-top: 1px dotted var(--paper-edge);
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
