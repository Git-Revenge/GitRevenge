<script lang="ts">
  import Icon from '../ui/Icon.svelte'
  import { Blame, BlameDownloading, BlameNeeds, BlameStep, BlameTrack, CancelDownload, DownloadFiles, FileDiff } from '../../../wailsjs/go/app/App.js'
  import VersionTrack from './VersionTrack.svelte'
  import { blameBackward, blameForward, changedLines, fillBlame, textLines, trackSteps, type Blamed, type Step } from './track'
  import type { git } from '../../../wailsjs/go/models'
  import { errorText } from '../errors'
  import Busy from '../ui/Busy.svelte'
  import { formatTime, laneVar } from '../graph/draw'
  import { day } from '../thisClone'
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

  /** A stable color per commit, so a commit's blocks read as one: the newest version's
   * commits first, so they keep their colours at an older one. */
  let colorOf = $derived.by(() => {
    const m: Record<string, number> = {}
    let n = 0
    for (const l of [...(lines ?? []), ...(olderBlame ?? [])]) if (!(l.commit in m)) m[l.commit] = n++
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

  // The track through the file's versions (#1078). The newest step is the blame above; any
  // other shows that version's text, and what it changed, from FileDiff.
  let steps = $state<Step[]>([])
  let renamedFrom = $state('')
  let at = $state(0)
  let newest = $derived(steps.length === 0 || at === steps.length - 1)
  /** An older version, as read: its lines and the ones it changed, or why there's no text. */
  type Older = { at: number; kind: string; lines: string[]; tokens: Token[][]; changed: Set<number>; first: number }
  let older = $state<Older | null>(null)
  /** Set once the person moves along the track, so a step brings its change into view, and
   * opening the view still goes to the line asked for. */
  let stepped = $state(false)
  let want = -1
  let reading = false

  /** Reads the step wanted, and once it's in, the one wanted since: while dragging, only the
   * latest is read. */
  async function readSteps() {
    if (reading) return
    reading = true
    try {
      while (want >= 0 && want < steps.length - 1 && older?.at !== want) {
        const i = want
        const s = steps[i]
        const [f, r] = [file, rev]
        const res = await FileDiff(path, 'commit', s.id, file, '', false, 'lines').catch((err) => {
          session.error = errorText(err)
          return null
        })
        if (f !== file || r !== rev || !res) return
        if (i !== want) continue
        const kind = res.contents.kind
        const text = kind === 'text' ? res.contents.new.text : ''
        const changed = changedLines(res.diff?.blocks ?? [])
        const view: Older = { at: i, kind, lines: textLines(text), tokens: highlightLines(text, null), changed, first: changed.size ? Math.min(...changed) : -1 }
        blocksOf.set(s.id, res.diff?.blocks ?? [])
        older = view
        olderBlame = kind === 'text' ? (blameOf.get(s.id) ?? null) : null
        if (kind === 'text' && !olderBlame) settle(i)
        const lang = await languageFor(file)
        // older is a proxy of view now, never the same object: the step says it's still this one.
        if (lang && older?.at === i && f === file && r === rev) older = { ...view, tokens: highlightLines(text, lang) }
      }
    } finally {
      reading = false
    }
  }

  // Who wrote each line at an older version (#1079): read once the handle has settled, never
  // on each step of a drag, and kept, so going back to one is instant. A version next to one
  // already read is worked out from it: forward with no git at all, back with git blame of
  // only the lines the step changed.
  /** Each version's blame, by commit ("" for the work tree), and its change against the one
   * before, from FileDiff. Plain maps: what shows is olderBlame. */
  // eslint-disable-next-line svelte/prefer-svelte-reactivity
  const blameOf = new Map<string, Blamed[]>()
  // eslint-disable-next-line svelte/prefer-svelte-reactivity
  const blocksOf = new Map<string, { oldStart: number; oldEnd: number; newStart: number; newEnd: number }[]>()
  let olderBlame = $state<Blamed[] | null>(null)
  let settleTimer: ReturnType<typeof setTimeout> | undefined
  /** How long the handle has to rest on a version before its blame is read. */
  const SETTLE_MS = 250

  /** Keeps a version's blame, letting the longest-kept go past a few dozen: on a big file
   * each is thousands of lines. */
  function keep(id: string, blamed: Blamed[]) {
    blameOf.delete(id)
    blameOf.set(id, blamed)
    while (blameOf.size > 40) blameOf.delete(blameOf.keys().next().value!)
  }

  function settle(i: number) {
    clearTimeout(settleTimer)
    settleTimer = setTimeout(() => void blameAt(i), SETTLE_MS)
  }

  /** The blame of step i, and the change of the step after it, read if need be. */
  async function blocksAt(i: number) {
    const s = steps[i]
    const known = blocksOf.get(s.id)
    if (known) return known
    const res = await FileDiff(path, s.id ? 'commit' : 'worktree', s.id, file, '', false, 'lines').catch(() => null)
    if (!res?.diff) return null
    blocksOf.set(s.id, res.diff.blocks)
    return res.diff.blocks
  }

  async function blameAt(i: number) {
    const [f, r] = [file, rev]
    const s = steps[i]
    const view = older
    if (!s?.id || !view || view.at !== i || view.kind !== 'text') return
    const still = () => f === file && r === rev && at === i && older?.at === i
    let got: Blamed[] | null = blameOf.get(s.id) ?? null
    try {
      const before = i > 0 ? blameOf.get(steps[i - 1].id) : undefined
      const after = i < steps.length - 1 ? blameOf.get(steps[i + 1].id) : undefined
      if (!got && before && !s.merge) got = blameForward(before, blocksOf.get(s.id) ?? [], view.lines, s)
      if (!got && after) {
        const blocks = await blocksAt(i + 1)
        const back = blocks && blameBackward(after, blocks, view.lines.length)
        if (back && still()) got = back.ranges.length ? fillBlame(back.known, await BlameStep(path, file, s.id, back.ranges)) : fillBlame(back.known, [])
      }
      if (!got && still()) got = await BlameStep(path, file, s.id, [])
    } catch {
      // Not downloaded, or git failed: the text stands without who wrote it.
      got = null
    }
    if (!got || got.length !== view.lines.length) return
    keep(s.id, got)
    if (still()) olderBlame = got
  }

  // The newest version's blame is the view's own: the track starts from it.
  $effect(() => {
    const newestStep = steps.at(-1)
    if (lines && newestStep && lines.length) keep(newestStep.id, lines)
  })

  function goTo(i: number) {
    if (!steps.length) return
    at = Math.max(0, Math.min(steps.length - 1, i))
    stepped = true
    want = at
    if (!newest) void readSteps()
  }

  /** The lines the newest step changed: the newest commit's, or those not committed yet. */
  let newestChanged = $derived.by(() => {
    const s = steps.at(-1)
    // Made afresh each time and never changed after: nothing watches it change.
    // eslint-disable-next-line svelte/prefer-svelte-reactivity
    const out = new Set<number>()
    if (!s || !lines) return out
    lines.forEach((l, i) => {
      if (s.id ? l.commit === s.id : uncommitted(l.commit)) out.add(i)
    })
    return out
  })
  let newestFirst = $derived(newestChanged.size ? Math.min(...newestChanged) : -1)

  /** Brings a step's change into view, once for each step. */
  const toChange = (step: number) => (row: HTMLElement) => {
    void step
    row.scrollIntoView({ block: 'center' })
  }

  function carryOn() {
    if (!renamedFrom || !steps[0]) return
    onOpen({ mode: 'blame', file: renamedFrom, commit: `${steps[0].id}^` })
  }

  $effect(() => {
    const [f, r] = [file, rev]
    steps = []
    blameOf.clear()
    blocksOf.clear()
    olderBlame = null
    clearTimeout(settleTimer)
    renamedFrom = ''
    older = null
    stepped = false
    want = -1
    void BlameTrack(path, f, r)
      .then((t) => {
        if (f !== file || r !== rev) return
        steps = trackSteps(t)
        renamedFrom = t.renamedFrom
        at = steps.length - 1
      })
      .catch(() => {
        // The track is extra: blame shows without it.
      })
  })

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

  provideCommands(() => [
    wrapCommand(),
    ...(steps.length > 1
      ? [
          { id: 'blame.older', group: 'Blame', title: 'The version before', keywords: 'blame history older earlier previous track', shortcut: '[', run: () => goTo(at - 1) },
          { id: 'blame.newer', group: 'Blame', title: 'The version after', keywords: 'blame history newer later next track', shortcut: ']', run: () => goTo(at + 1) },
        ]
      : []),
  ])

  const uncommitted = (c: string) => /^0+$/.test(c)

  /** Brings the line asked for into view once it's drawn. */
  const scrollHere = (row: HTMLElement) => row.scrollIntoView({ block: 'center' })
</script>

{#snippet gutter(l: Blamed, start: boolean)}
  <td class="gutter" style:border-left-color={uncommitted(l.commit) ? 'transparent' : laneVar(colorOf[l.commit] ?? 0)}>
    {#if start}
      {#if uncommitted(l.commit)}
        <span class="muted">Not committed yet</span>
      {:else}
        <span class="who-when">
          <button class="sha" onclick={() => onReveal(l.commit)} title="{l.summary}&#10;Show in graph">{l.commit.slice(0, 7)}</button>
          {#if l.edge}
            <!-- A shallow clone's edge (#1006): written there or earlier, in history it hasn't got. -->
            <span class="who before" title="This clone's history stops at {l.commit.slice(0, 7)}: these lines are as old as it, or older">before {day(l.time)}</span>
          {:else}
            <span class="who" title={l.author}>{l.author}</span>
            <span class="muted when">{formatTime(l.time)}</span>
          {/if}
        </span>
      {/if}
    {/if}
  </td>
{/snippet}

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
  {#if steps.length > 1 && !ask}
    <VersionTrack {steps} {at} onAt={goTo} {renamedFrom} onCarryOn={carryOn} {onReveal} />
  {/if}
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
    {:else if !newest}
      {#if !older}
        <Loading />
      {:else if older.kind !== 'text'}
        <p class="no-text" role="status">
          {older.kind === 'binary' || older.kind === 'image'
            ? 'Not text at this version: the file was binary.'
            : older.kind === 'large'
              ? 'Too big to show at this version.'
              : older.kind === 'missing'
                ? 'This version hasn’t been downloaded to this clone.'
                : 'The file isn’t at this version.'}
        </p>
      {:else}
        <!-- An older version: its text, the lines it changed, and once the handle has settled,
             who wrote each line (#1079). -->
        {#if !olderBlame && older.at === at}<p class="reading" role="status">Reading who wrote each line…</p>{/if}
        <table class:wrap={wrap.on} class:dim={older.at !== at}>
          <tbody>
            {#each older.lines as text, i (i)}
              {@const changed = older.changed.has(i)}
              {@const b = olderBlame?.[i]}
              <tr class:changed class:start={!!b && (i === 0 || olderBlame?.[i - 1]?.commit !== b.commit)} {@attach stepped && i === older.first ? toChange(older.at) : undefined}>
                {#if b}{@render gutter(b, i === 0 || olderBlame?.[i - 1]?.commit !== b.commit)}{:else}<td class="gutter"></td>{/if}
                <td class="num">{i + 1}</td>
                <td class="code"
                  >{#if older.tokens[i]}{#each older.tokens[i] as t, k (k)}<span class={t.cls}>{t.text}</span>{/each}{:else}{text}{/if}</td
                >
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    {:else if !lines}
      <Loading />
    {:else}
      <table class:wrap={wrap.on}>
        <tbody>
          {#each lines as l, i (i)}
            {@const start = i === 0 || lines[i - 1].commit !== l.commit}
            <tr
              class:start
              class:at={i + 1 === line}
              class:changed={stepped && newestChanged.has(i)}
              {@attach i + 1 === line ? scrollHere : stepped && i === newestFirst ? toChange(at) : undefined}
            >
              {@render gutter(l, start)}
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

  /* What the step shown changed (#1078). */
  tr.changed td.code,
  tr.changed td.num {
    background: color-mix(in srgb, var(--accent) 16%, transparent);
  }

  /* While the next step is read, the last one stays, faded, rather than blanking. */
  table.dim {
    opacity: 0.55;
  }

  .reading {
    position: sticky;
    top: 0;
    z-index: 1;
    margin: 0;
    padding: 4px 12px;
    font-size: 12px;
    color: var(--ink-mute);
    background: var(--paper-warm);
  }

  .no-text {
    margin: 32px auto;
    max-width: 560px;
    font-size: 14px;
    color: var(--ink-mute);
    text-align: center;
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

  .who.before {
    font-style: italic;
    color: var(--ink-mute);
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
