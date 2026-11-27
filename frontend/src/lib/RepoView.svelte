<script lang="ts">
  import { onDestroy, untrack } from 'svelte'
  import { CarryOnHistorySearch, ChoosePatchFile, CloneShape, CommitRow, CompareVersions, Reflog, Refs, VersionBase, ReadClipboardPatches, ReadPatches, ReadPatchFiles, SetHistorySearch, SquashOffer, SetTrailerFilter, SetTrailerPin, StopHistorySearch, TrailerFilter, TrailerFilterRows, TrailerFilterStep, TrailerPins } from '../../wailsjs/go/app/App.js'
  import ShallowBand from './ShallowBand.svelte'
  import type { app } from '../../wailsjs/go/models'
  import { onBackend } from './events'
  import FilterStrip from './filters/FilterStrip.svelte'
  import { hasPick, loadOnly, saveOnly, togglePick } from './filters/filters'
  import { historyKey, historyOptions, type HistoryOptions } from './filters/history'
  import { theme as colorTheme } from './theme.svelte'
  import { repoReads } from './repoReads'
  import { provideCommands } from './commands.svelte'
  import ContextMenu, { type MenuItem } from './ContextMenu.svelte'
  import Dialog, { type DialogSpec } from './Dialog.svelte'
  import Toolbar from './Toolbar.svelte'
  import Icon from './ui/Icon.svelte'
  import { DEFAULT_WIDTH, fitWidth, loadRightPanel, maxWidth, MIN_WIDTH, saveRightPanel } from './rightPanel'
  import { loadRefExtras } from './branchWeb'
  import { whileBusy } from './busy'
  import { commitMenuItems, branchAtDialog, checkoutRef, dropItems, newBranchDialog, refItems, stackItems, type MenuContext } from './menus'
  import CommitPanel from './CommitPanel.svelte'
  import SelectionPanel from './SelectionPanel.svelte'
  import { selectionMenu, type SquashState } from './selectionMenu'
  import type { Selected } from './graph/selection'
  import type { PatchStart } from './patches'
  import { toasts } from './toasts.svelte'
  import { earlierTips } from './versions'
  import { versionsAsk, type VersionsAskFor } from './versionsAsk.svelte'
  import type { DiffTarget } from './diff/DiffView.svelte'
  import { ofCommits } from './diff/target'
  import Lazy from './ui/Lazy.svelte'
  import { findAsk } from './find/findAsk.svelte'
  import GraphView from './graph/GraphView.svelte'
  import type { RefLabel } from './graph/draw'
  import type { Ref, Row } from './graph/rowCache'
  import { session } from './session.svelte'
  import Sidebar from './Sidebar.svelte'
  import { linkStore } from './links/links.svelte'
  import WorkingTreePanel from './WorkingTreePanel.svelte'
  import { bisect, marksOf } from './bisect.svelte'
  import { rebaseMarks } from './operation'
  import { SHARED_STEPS, squeezeFor } from './barSqueeze'
  import { operations } from './operation.svelte'
  import { Integrate } from '../../wailsjs/go/app/App.js'

  let { path }: { path: string } = $props()

  const storage = (): Pick<Storage, 'getItem' | 'setItem'> => localStorage
  let right = $state(loadRightPanel(untrack(() => path), storage()))
  $effect(() => saveRightPanel(path, $state.snapshot(right), storage()))

  // Smart links come from git config: read again whenever the repository's config changes.
  $effect(() => {
    void session.configRevision[path]
    void linkStore.load(path)
  })
  let windowWidth = $state(0)
  /** The panel's width now. Before the first measure there's nothing to fit it to. */
  let rightWidth = $derived(windowWidth ? fitWidth(right.width, windowWidth) : right.width)

  // Dragging the panel's edge, or the arrow keys on it, resizes the panel.
  let rightDrag: { x: number; width: number } | null = null
  function startRightResize(e: PointerEvent) {
    rightDrag = { x: e.clientX, width: rightWidth }
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }
  function rightResize(e: PointerEvent) {
    if (rightDrag) right.width = fitWidth(rightDrag.width + rightDrag.x - e.clientX, windowWidth)
  }
  function rightResizeKey(e: KeyboardEvent) {
    const step = e.key === 'ArrowLeft' ? 16 : e.key === 'ArrowRight' ? -16 : 0
    if (step) right.width = fitWidth(rightWidth + step, windowWidth)
    else if (e.key === 'Home') right.width = DEFAULT_WIDTH
    else return
    e.preventDefault()
  }

  let graph: GraphView
  /** Counts clicks on a note mark in the graph, each one asking the commit panel for its notes. */
  let noteAsk = $state(0)
  /** The bar stays on one line: as it narrows, its parts give way in steps. */
  let barWidth = $state(0)
  let barSqueeze = $derived(squeezeFor(barWidth, SHARED_STEPS))
  /** Where the graph puts its header tools: in the bar, beside the repository's actions. */
  let toolsHost = $state<HTMLElement>()
  let selected = $state<Row | undefined>()
  /** Several commits selected in the graph (#1018), or null while one is. */
  let several = $state<Selected | null>(null)
  /** Whether the selected line can be squashed into one (#1020): asked once it's known to be
   * a line. */
  let squashState = $state<SquashState | undefined>()
  $effect(() => {
    const info = several?.info
    squashState = undefined
    if (!info?.line || !info.base || !ctx.current()) return
    squashState = 'checking'
    void SquashOffer(path, info.ids[0], info.ids[info.ids.length - 1])
      .then((o) => {
        if (several?.info === info) squashState = o
      })
      .catch(() => {
        if (several?.info === info) squashState = { ok: false, reason: 'Couldn’t tell whether they can be squashed.' }
      })
  })
  /** Which panel is on the right: uncommitted changes, or the selected commit. */
  let view = $state<'wip' | 'commit'>('commit')
  /** Files with uncommitted changes; null until the first answer, so nothing claims "clean". */
  let changes = $state<number | null>(null)

  /** Where a shallow clone's history stops, Unix seconds; 0 for a whole history (#1006). The
   * graph ends with a band saying so, and offering the rest. */
  let shallowSince = $state(0)
  $effect(() => {
    const p = path
    void session.revision[p]
    void CloneShape(p)
      .then((s) => {
        if (p === path) shallowSince = s?.shallow?.since ?? 0
      })
      .catch(() => {})
  })

  /** Shows the working tree's panel: from its row in the graph, or its column in a view that
   * runs across. */
  function openWip() {
    view = 'wip'
    right.hidden = false
    if (ofCommits(diff?.mode)) diff = null
  }
  let menu = $state<{ x: number; y: number; items: MenuItem[] } | null>(null)
  let sidebar = $state<Sidebar>()

  // The smart filter: the values picked in the sidebar's panel, which narrow the graph. It
  // lasts until it's cleared or the tab closes, and the backend holds it; which of its two
  // modes this repository last used is remembered here.
  const noFilter = { picks: [], only: true, kept: 0, total: 0 } as unknown as app.TrailerFilterState
  let filter = $state<app.TrailerFilterState>(noFilter)
  let filterOnly = $state(loadOnly(untrack(() => path), localStorage))
  // Taking commits out would break the Horizontal view's lanes: there a filter highlights.
  let onlyOffered = $derived(colorTheme.view !== 'horizontal')
  let only = $derived(filterOnly && onlyOffered)
  /** The rows a highlighting filter keeps in ink. */
  let highlight = $state<number[] | null>(null)
  let filterAsk = 0
  /** Whether a filter is on: values picked, a history search (#982), or both. */
  let filterOn = $derived(filter.picks.length > 0 || !!filter.history)

  async function setFilter(picks: { key: string; value: string }[]) {
    const n = ++filterAsk
    // Only what names a pick goes back: the labels that came with it are the backend's to give.
    const asked = picks.map((p) => ({ key: p.key, value: p.value }))
    const next = await session.run(() => SetTrailerFilter(path, asked as never, only))
    if (n !== filterAsk || !next) return
    // Under Only these the graph is other commits now: where it was scrolled to means nothing.
    if (only || filter.only) graph?.top()
    filter = next
    await loadHighlight(n)
  }

  // The history search (#981, #982): the commits that add or remove a text, as part of the
  // filter. Empty text takes it out; the picks stay either way.
  async function setHistory(o: HistoryOptions | null) {
    const n = ++filterAsk
    let next = await session.run(() => SetHistorySearch(path, (o ?? historyOptions('')) as never))
    if (n !== filterAsk || !next) return
    // A search with no filter on starts as Only these: the mode chosen here goes with it.
    if (next.history && next.only !== only) {
      next = await session.run(() => SetTrailerFilter(path, next!.picks.map((p) => ({ key: p.key, value: p.value })) as never, only))
      if (n !== filterAsk || !next) return
    }
    if (only || filter.only) graph?.top()
    filter = next
    await loadHighlight(n)
  }

  /** Stops the history search, keeping what it found, or carries on from where it stopped. */
  async function pauseHistory(stop: boolean) {
    const n = ++filterAsk
    const next = await session.run(() => (stop ? StopHistorySearch(path) : CarryOnHistorySearch(path)))
    if (n !== filterAsk || !next) return
    filter = next
    await loadHighlight(n)
  }

  /** Takes the whole filter off: the history search, then the picks. */
  async function clearFilter() {
    if (filter.history) await setHistory(null)
    if (filter.picks.length) await setFilter([])
  }

  async function loadHighlight(n = ++filterAsk) {
    if (!filterOn || only) {
      highlight = null
      return
    }
    const rows = await session.run(() => TrailerFilterRows(path))
    if (n === filterAsk) highlight = rows?.rows ?? []
  }

  // The trailer keys pinned to the graph's rows. They live in git config, the repository's
  // or the global one, so they're read again when either changes.
  let pins = $state<{ key: string; label: string }[]>([])
  let pinnedKeys = $derived(pins.map((p) => p.key))
  $effect(() => {
    const repo = path
    void session.configRevision[repo]
    void TrailerPins(repo)
      .then((got) => {
        if (repo === path) pins = (got ?? []).map((p) => ({ key: p.key, label: p.label }))
      })
      .catch(() => {})
  })
  async function setPin(key: string, on: boolean) {
    const got = await session.run(() => SetTrailerPin(path, key, on))
    if (got) pins = got.map((p) => ({ key: p.key, label: p.label }))
  }

  /** Backspace in an empty search box takes out the strip's last chip: the last value picked,
   * then the history search, which comes first. */
  function dropLastFilter() {
    if (filter.picks.length) void setFilter(filter.picks.slice(0, -1))
    else if (filter.history) void setHistory(null)
  }

  /** A value picked in the search box joins the filter; one that's there already stays. */
  function addPick(key: string, value: string) {
    if (!hasPick(filter.picks, key, value)) void setFilter([...filter.picks, { key, value }])
  }

  // How far the trailers are read: under a filter, the ruler's foot is hatched for the rest.
  let trailersRead = $state<{ reading: boolean; read: number; total: number } | null>(null)
  $effect(() =>
    onBackend('trailers:progress', (p) => {
      if (p.path === path) trailersRead = p
    }),
  )
  let trailersUnread = $derived(filter.picks.length && trailersRead?.reading && trailersRead.total > 0 ? Math.max(0, 1 - trailersRead.read / trailersRead.total) : 0)
  /** Names the filter that's on, for whoever shows what it picked. */
  let filterSig = $derived(filter.picks.map((p) => `${p.key}=${p.value}`).join('|'))
  /** The history search in the filter, for the marks beside the commits it found and the
   * files where its text changed: its options, a key naming them, and how far it has got,
   * since a commit asked about before it was found has its mark once it is. */
  let historyOn = $derived(
    filter.history ? { key: historyKey(filter.history.options), found: filter.history.running || filter.history.stopped ? filter.history.found : -1, options: filter.history.options } : null,
  )

  /** The next commit a highlighting filter keeps after the selected one, or the one before. */
  async function step(dir: 1 | -1) {
    const row = await session.run(() => TrailerFilterStep(path, graph?.selectedRow() ?? -1, dir))
    if (row !== undefined && row >= 0) graph.reveal(row)
  }

  function setMode(next: boolean) {
    filterOnly = next
    saveOnly(path, next, localStorage)
  }

  // The mode in force changed, by a click or by a switch of view: tell the backend.
  $effect(() => {
    void only
    untrack(() => {
      if (filterOn && filter.only !== only) void setFilter(filter.picks)
    })
  })

  // A tab shows the filter it had: the backend keeps it while the tab is open.
  $effect(() => {
    const repo = path
    void TrailerFilter(repo)
      .then((f) => {
        if (repo !== path) return
        filter = f ?? noFilter
        void loadHighlight()
      })
      .catch(() => {})
  })

  // New commits, or more of the history read, change what a filter keeps and where its rows are.
  $effect(() =>
    onBackend('graph:progress', (p) => {
      if (p.path !== path || !p.done || !filterOn) return
      void TrailerFilter(path)
        .then((f) => {
          filter = f ?? noFilter
          void loadHighlight()
        })
        .catch(() => {})
    }),
  )
  // A history search finds more as it goes, and says so at most once a second.
  $effect(() =>
    onBackend('history:progress', (p) => {
      if (p.path !== path) return
      filter = p.filter ?? noFilter
      // Without taking a turn of its own: a click still waiting on the backend keeps its answer.
      void loadHighlight(filterAsk)
    }),
  )
  /** The card about a commit's author, while one is open: whose, and where it goes. */
  let authorCard = $state<{ sha: string; name: string; x: number; y: number; above: number; opener?: HTMLElement } | null>(null)
  // The card is about a commit in this repository: it goes when another one shows.
  $effect(() => {
    void path
    authorCard = null
  })
  /** Where the graph's columns are, so the row of changes above it lines up with them. */

  let dialog = $state<DialogSpec | null>(null)
  let remotesOpen = $state(false)
  let worktreesOpen = $state(false)
  /** Commit after which the interactive rebase editor rewrites history, or "" when closed. */
  let rebaseBase = $state('')
  /** Commits the editor starts with squashed into the oldest of them (#1020). */
  let rebaseSquash = $state<string[]>([])
  let preview = $state<{ kind: 'merge' | 'rebase'; target: string; label: string } | null>(null)
  /** The ref whose reflog is open, or "" when closed. */
  let reflogRef = $state('')
  /** Where Save as patches starts, while it's open (#1021). */
  let patchesStart = $state<PatchStart | null>(null)
  /** What Save as an archive is saving, while it's open (#936). */
  let archiving = $state<{ rev: string; label: string } | null>(null)
  /** A file of patches being previewed to apply (#1022), or a plain diff (#1024). */
  let patchFile = $state<app.PatchFile | null>(null)
  /** Two versions of a branch compared (#1038), in the panel on the right; and the branch the
   * Compare two versions dialog starts on, while it's open. */
  let versions = $state<{ compared: app.VersionsCompared; branch: string; oldLabel: string; anyway: boolean } | null>(null)
  let comparing = $state<{ branch: string; oldTip: string } | null>(null)

  // A comparison asked for from a toast, the reflog or a menu (#1040): each version's start
  // is worked out here, and when it can't be, Compare two versions opens to ask.
  let askedSeq = 0
  $effect(() => {
    const a = versionsAsk.ask
    if (!a || a.path !== path || a.seq === askedSeq) return
    askedSeq = a.seq
    void untrack(() => compareAsked(a))
  })
  async function compareAsked(a: VersionsAskFor) {
    let oldTip = a.oldTip
    let oldLabel = a.oldLabel
    if (!oldTip) {
      const tip = earlierTips((await Reflog(path, a.branch).catch(() => null)) ?? [])[0]
      if (!tip) {
        comparing = { branch: a.branch, oldTip: '' }
        return
      }
      oldTip = tip.commit
      oldLabel = tip.said
    }
    const refs = ((await Refs(path).catch(() => null)) ?? []) as Ref[]
    const stack = refs.find((r) => r.kind === 'local' && r.short === a.branch)?.stackParent ?? ''
    const [oldBase, newBase] = await Promise.all([VersionBase(path, a.branch, stack, oldTip).catch(() => ''), VersionBase(path, a.branch, stack, a.branch).catch(() => '')])
    if (!oldBase || !newBase) {
      comparing = { branch: a.branch, oldTip }
      return
    }
    const got = await session.run(() => CompareVersions(path, { base: oldBase, tip: oldTip }, { base: newBase, tip: a.branch }, false, false))
    if (got) showVersions(got, a.branch, oldLabel)
  }

  /** The interactive rebase editor ran to the end: what became of the branch's commits. */
  function rebased() {
    const branch = ctx.current()
    if (branch) toasts.show(`Rebased ${branch}.`, { label: 'What changed?', run: () => versionsAsk.show({ path, branch, oldTip: '', oldLabel: '' }) })
  }
  function showVersions(compared: app.VersionsCompared, branch: string, oldLabel: string, anyway = false) {
    comparing = null
    versions = { compared, branch, oldLabel, anyway }
    pairAt = -1
    view = 'commit'
  }
  /** Compares the same two versions again (#1041): matched loosely or as git does, a big one
   * anyway, or once what a partial clone needed is here. Anyway sticks for the comparison. */
  async function recompare(o: { loose?: boolean; anyway?: boolean }) {
    const v = versions
    if (!v) return
    const loose = o.loose ?? v.compared.loose
    const anyway = o.anyway ?? v.anyway
    const got = await session.run(() => CompareVersions(path, v.compared.old, v.compared.new, loose, anyway))
    if (got) showVersions(got, v.branch, v.oldLabel, anyway)
  }
  /** The pair of the comparison open in the diff area (#1039), or -1. */
  let pairAt = $state(-1)
  function closeVersions() {
    versions = null
    pairAt = -1
  }
  let diffFile = $state<app.PatchFile | null>(null)

  /** Apply patches or a diff…: a file picked, read, and shown for what it is. */
  async function applyPatches() {
    const file = await session.run(() => ChoosePatchFile())
    if (!file) return
    openPatches(await session.run(() => ReadPatches(path, file)))
  }
  /** Opens what was read in its preview: mail in the series', a plain diff in the diff's.
   * Anything else says what it is (#1025). */
  function openPatches(read: app.PatchFile | undefined) {
    if (!read) return
    if (read.kind === 'mail') patchFile = read
    else if (read.kind === 'diff' && read.diff) diffFile = read
    else if (read.name === 'the clipboard') toasts.show('The clipboard doesn’t hold a patch or a diff.')
    else toasts.show(`Not a patch or a diff: ${read.name}. Nothing was opened.`)
  }
  /** Apply the patch on the clipboard (#1025): the pair to Copy as a patch. */
  async function applyClipboard() {
    openPatches(await session.run(() => ReadClipboardPatches(path)))
  }
  // Files dropped on the window, when this repository is the one on screen (#1025).
  onDestroy(
    onBackend('patches:dropped', (d) => {
      if (session.active !== path || !d.paths.length) return
      void session.run(() => ReadPatchFiles(path, d.paths)).then(openPatches)
    }),
  )
  const ctx: MenuContext = {
    get path() {
      return path
    },
    current: () => {
      const head = session.activeTab?.head ?? ''
      return head.startsWith('detached') ? '' : head
    },
    openDialog: (spec) => (dialog = spec),
    // From a row's menu, the card goes where the menu was.
    openAuthor: (row) => (authorCard = { sha: row.id, name: row.author, x: menuAt.x, y: menuAt.y, above: menuAt.y }),
    // One bisect at a time: the entry goes while one is running.
    get openBisect() {
      return running
        ? undefined
        : (bad: string) => {
            bisect.starting = { path, bad }
          }
    },
    openRebase: (base, squash = []) => {
      diff = null
      rebaseBase = base
      rebaseSquash = squash
    },
    openPreview: (kind, target, label) => (preview = { kind, target, label }),
    openReflog: (ref) => (reflogRef = ref),
    openPatches: (start) => (patchesStart = start),
    openArchive: (rev, label) => (archiving = { rev, label }),
    compareVersions: (branch, oldTip, oldLabel) => versionsAsk.show({ path, branch, oldTip, oldLabel }),
    pickVersions: (branch) => (comparing = { branch, oldTip: '' }),
  }

  provideCommands(() => [
    { id: 'branch.new', group: 'Branch', title: 'New branch…', keywords: 'create', shortcut: 'Mod+B', run: () => (dialog = newBranchDialog(ctx)) },
    // The rebase editor is left out: Escape shouldn't throw away a plan being edited.
    ...(diff ? [{ id: 'view.close', group: 'View', title: 'Back to the graph', keywords: 'close diff', shortcut: 'Escape', run: () => (diff = null) }] : []),
    { id: 'repo.remotes', group: 'Repository', title: 'Manage remotes…', keywords: 'origin url', run: () => (remotesOpen = true) },
    { id: 'repo.reflog', group: 'Repository', title: 'Show the reflog…', keywords: 'history head moves recover lost commit', run: () => (reflogRef = 'HEAD') },
    { id: 'repo.applyPatches', group: 'Repository', title: 'Apply patches or a diff…', keywords: 'am mbox mailbox format-patch import mail', run: () => void applyPatches() },
    { id: 'repo.compareVersions', group: 'Repository', title: 'Compare two versions of a branch…', keywords: 'range-diff rebase before after versions what changed', run: () => (comparing = { branch: ctx.current(), oldTip: '' }) },
    { id: 'repo.applyClipboard', group: 'Repository', title: 'Apply the patch on the clipboard', keywords: 'paste am diff git apply clipboard', run: () => void applyClipboard() },
    { id: 'repo.worktrees', group: 'Repository', title: 'Manage worktrees…', keywords: 'worktree checkout folder parallel', run: () => (worktreesOpen = true) },
    {
      id: 'view.rightPanel',
      group: 'View',
      title: right.hidden ? 'Show the right panel' : 'Hide the right panel',
      keywords: 'details commit changes collapse graph width',
      shortcut: 'Mod+Alt+B',
      run: () => (right.hidden = !right.hidden),
    },
    ...(running ? [] : [{ id: 'repo.bisect', group: 'Repository', title: 'Find the commit that broke it…', keywords: 'bisect regression bad good', run: () => (bisect.starting = { path, bad: 'HEAD' }) }]),
    {
      id: 'view.changes',
      group: 'View',
      title: 'Show uncommitted changes',
      keywords: 'working tree stage commit',
      run: () => {
        view = 'wip'
        right.hidden = false
        if (ofCommits(diff?.mode)) diff = null
      },
    },
  ])

  /** Where the last menu opened, for what one of its items puts in its place. */
  let menuAt = { x: 0, y: 0 }
  function openMenu(items: MenuItem[], x: number, y: number) {
    menuAt = { x, y }
    menu = items.length ? { x, y, items } : null
  }

  /** A file diff shown in place of the graph, or null for the graph. */
  let diff = $state<DiffTarget | null>(null)
  let checkedInitial = false

  async function refreshCount() {
    const wt = await session.run(() => repoReads(path).workingTree())
    if (!wt) return
    changes = wt.status.files.length
    // Open on the changes when there are any.
    if (!checkedInitial) {
      checkedInitial = true
      if (changes > 0) view = 'wip'
    }
  }

  $effect(() => {
    void session.revision[path]
    void refreshCount()
  })

  /** Labels for the branches and tags on the selected commit. */
  let selectedRefs = $state<RefLabel[]>([])

  // A bisect in progress (issue #592). The banner keeps the store up to date; here it picks
  // what the graph marks and what the right panel shows.
  let running = $derived(bisect.of(path))
  // A rebase in progress marks the commits it has written and the one in hand (issue #593).
  let bisectMarks = $derived(marksOf(running) ?? rebaseMarks(operations.of(path)))

  // A conflict tool left open on a file that's no longer in conflict (resolved elsewhere, or the
  // operation moved on or ended) has nothing true to show.
  // It only closes once the strip has seen that file in conflict, so a tool opened a moment
  // before the strip catches up isn't shut again.
  let wasConflicted = ''
  $effect(() => {
    const state = operations.state[path]
    if (diff?.mode !== 'conflict' || !state) return
    if (state.conflicts.includes(diff.file)) wasConflicted = diff.file
    else if (wasConflicted === diff.file) {
      wasConflicted = ''
      diff = null
    }
  })

  // The operation strip asks for a conflicted file to be opened.
  $effect(() => {
    const ask = operations.resolve
    if (ask?.path !== path) return
    operations.resolve = null
    rebaseBase = ''
    diff = { mode: 'conflict', file: ask.file }
  })
  /** The commit a bisect is about right now: the one under test, or the answer. */
  let bisectSubject = $derived(running ? running.firstBad || running.current : '')
  /** Set when someone asks for the commit's own details instead of the bisect's panel. */
  let bisectAside = $state(false)

  // Each step checks out another commit: bring it into view and select it.
  let lastSubject = ''
  $effect(() => {
    const id = bisectSubject
    if (id === lastSubject) return
    lastSubject = id
    bisectAside = false
    if (id) {
      view = 'commit'
      right.hidden = false
      void revealCommit(id)
    }
  })

  /** Stops the bisect, then starts a branch for the fix from where it began. */
  async function fixBranch() {
    await bisect.stop(path)
    dialog = newBranchDialog(ctx)
  }

  /** Stops the bisect, then reverts the commit it found on the branch it began from. */
  async function revertFound(id: string) {
    await bisect.stop(path)
    await session.run(() => Integrate(path, 'revert', [id, '']))
  }

  function onSelect(row: Row | undefined, auto: boolean) {
    selected = row
    if (!auto) diff = null
    // The automatic HEAD selection on load must not pull focus away from the changes.
    if (!auto) view = 'commit'
  }

  /** Shows a commit in the graph (from blame), closing the file view. */
  async function revealCommit(commit: string) {
    const row = await session.run(() => CommitRow(path, commit))
    if (row === undefined) return
    if (row < 0) {
      session.error = graph.focused() ? 'That commit isn’t part of what the graph is focused on. Stop focusing to see it.' : 'That commit isn’t in the graph: it isn’t on any branch. Start a branch there to bring it back.'
      return
    }
    diff = null
    graph.reveal(row)
  }

  async function pick(ref: Ref) {
    const row = await session.run(() => CommitRow(path, ref.commit))
    if (row === undefined) return
    if (row < 0) {
      session.error = graph.focused() ? `${ref.short} isn't part of what the graph is focused on. Stop focusing to see it.` : `${ref.short} points at a commit that isn't in the graph yet.`
      return
    }
    graph.reveal(row)
  }
</script>

<div class="repo" bind:clientWidth={windowWidth}>
  <Sidebar
    bind:this={sidebar}
    {path}
    filterPicks={filter.picks}
    onFilterPick={(key, value) => setFilter(togglePick(filter.picks, key, value))}
    {pinnedKeys}
    onPin={setPin}
    onSearch={(q) => graph?.search(q)}
    selectedCommit={selected?.id}
    onPick={pick}
    onActivate={(ref) => checkoutRef(path, ref)}
    onNewBranch={() => (dialog = newBranchDialog(ctx))}
    onManageRemotes={() => (remotesOpen = true)}
    onContext={(ref, x, y) => void whileBusy(loadRefExtras(path, [ref])).then((e) => openMenu(refItems(ref, ctx, e.get(ref.short)), x, y))}
    onStackContext={(members, x, y) => openMenu(stackItems(members, ctx), x, y)}
    onMenu={openMenu}
    onManageWorktrees={() => (worktreesOpen = true)}
    openDialog={ctx.openDialog}
    diffShown={diff}
    onOpenFile={(t) => (diff = t)}
    onHistorySearch={(o) => {
      // The graph shows what the search finds: a file open from Find steps aside for it.
      diff = null
      void setHistory(o)
    }}
  />
  <div class="main">
    <div class="bar" bind:clientWidth={barWidth}>
      <Toolbar {path} squeeze={barSqueeze} />
      <!-- The graph's own tools join the bar while the graph is showing; a file or the rebase
           editor has its own header, so they step aside for it. -->
      <div class="graph-tools" class:hidden={diff || rebaseBase} bind:this={toolsHost}></div>
      <button class="btn sm quiet icon panel-toggle" aria-pressed={!right.hidden} aria-label={right.hidden ? 'Show the right panel' : 'Hide the right panel'} title={right.hidden ? 'Show the right panel' : 'Hide the right panel'} onclick={() => (right.hidden = !right.hidden)}><Icon name="sidebar" size={14} /></button>
    </div>
    <!-- The graph stays mounted while a diff is open, so scroll position and selection
         survive returning to it. -->
    {#if filterOn && !diff && !rebaseBase}
      <FilterStrip
        {filter}
        {only}
        {onlyOffered}
        onRemove={(p) => setFilter(togglePick(filter.picks, p.key, p.value))}
        onAdd={() => sidebar?.show('filters')}
        onMode={setMode}
        onClear={clearFilter}
        onStep={step}
        onHistorySearch={setHistory}
        onHistoryStop={() => pauseHistory(true)}
        onHistoryCarryOn={() => pauseHistory(false)}
        onHistoryRemove={() => setHistory(null)}
      />
    {/if}
    <!-- Where a shallow clone's history stops: after the graph's last commit (#1006). -->
    {#snippet shallowTail()}
      <ShallowBand {path} since={shallowSince} />
    {/snippet}
    <div class="graph" class:hidden={diff || rebaseBase}>
      <GraphView bind:this={graph} {path} {toolsHost} tail={shallowSince ? shallowTail : undefined} historySince={shallowSince} wip={{ changes, active: view === 'wip', open: openWip }} squeeze={barSqueeze} bisect={bisectMarks} {onSelect} onNote={() => (noteAsk += 1)} sparse={only && filterOn} {highlight} {trailersUnread} {pins} filterPicks={filter.picks} onFilterPick={(key, value) => setFilter(togglePick(filter.picks, key, value))} onFilterAdd={addPick} onFindInFiles={(text) => findAsk.show(path, text)} onHistorySearch={(text) => setHistory(historyOptions(text))} history={historyOn} onFilterDrop={dropLastFilter} onAuthor={(row, x, y, above) => (authorCard = { sha: row.id, name: row.author, x, y, above })} onSelectedRefs={(r) => (selectedRefs = r)} onSelection={(s) => (several = s)} onSelectionMenu={(x, y) => openMenu(selectionMenu(several?.count ?? 0, several?.info ?? null, ctx, squashState), x, y)} onContext={(row, refs, x, y) =>
          void whileBusy(loadRefExtras(path, refs)).then((e) => openMenu(commitMenuItems(row, refs, ctx, e), x, y))}
        onDrop={(source, sourceRefs, target, targetRefs, x, y) => {
          const items = dropItems(source, sourceRefs, target, targetRefs, ctx)
          if (items.length) openMenu(items, x, y)
          else
            session.error =
              'Drop onto the checked-out branch to merge, rebase or cherry-pick into it, or drag the checked-out branch onto another commit.'
        }}
      >
        <!-- The working tree is the graph's first row: what isn't a commit yet, above the newest
             one, with its mark on the first lane and its words where the messages start. -->
        {#snippet lead(at: { nodeX: number; textX: number })}
          <button
            class="wip"
            class:active={view === 'wip'}
            aria-pressed={view === 'wip'}
            style:padding-left="{at.textX}px"
            onclick={openWip}
          >
            <span class="dot" class:dirty={!!changes} style:left="{at.nodeX - 8}px">{#if changes && changes > 0}<span class="ring"></span>{:else if changes !== null}<Icon name="check" />{/if}</span>
            <span class="wip-title">{changes ? `${changes} ${changes === 1 ? 'file' : 'files'} changed` : 'Working tree'}</span>
            {#if changes && session.activeTab?.head}
              <span class="where">{session.activeTab.head.startsWith('detached') ? session.activeTab.head : `on ${session.activeTab.head}`}</span>
            {:else if changes === 0}
              <span class="meta">clean</span>
            {/if}
            {#if changes}<span class="review">Review</span>{/if}
          </button>
        {/snippet}
      </GraphView>
    </div>
    {#if rebaseBase}
      <div class="graph">
        <Lazy load={() => import('./RebaseEditor.svelte')} {path} base={rebaseBase} squash={rebaseSquash} onClose={() => (rebaseBase = '')} onDone={rebased} />
      </div>
    {:else if versions && pairAt >= 0}
      <div class="graph">
        <Lazy load={() => import('./VersionPairView.svelte')} compared={versions.compared} index={pairAt} onClose={() => (pairAt = -1)} onStep={(i: number) => (pairAt = i)} />
      </div>
    {:else if diff}
      <div class="graph">
        {#if diff.mode === 'conflict'}
          {#key diff.file}
            <Lazy load={() => import('./diff/ConflictView.svelte')} {path} file={diff.file} onClose={() => (diff = null)} onOpen={(file: string) => (diff = { mode: 'conflict', file })} />
          {/key}
        {:else if diff.mode === 'history'}
          {#key diff.file}
            <Lazy load={() => import('./diff/FileHistoryView.svelte')} {path} file={diff.file} onClose={() => (diff = null)} onOpen={(t: DiffTarget) => (diff = t)} />
          {/key}
        {:else if diff.mode === 'blame'}
          {#key diff.file + (diff.commit ?? '')}
            <Lazy
              load={() => import('./diff/BlameView.svelte')}
              {path}
              file={diff.file}
              rev={diff.commit ?? ''}
              line={diff.line ?? 0}
              onClose={() => (diff = null)}
              onOpen={(t: DiffTarget) => (diff = t)}
              onReveal={revealCommit}
            />
          {/key}
        {:else if diff.mode === 'found'}
          <Lazy load={() => import('./diff/FoundFileView.svelte')} {path} target={diff} onClose={() => (diff = null)} onOpen={(t: DiffTarget) => (diff = t)} />
        {:else}
          <Lazy load={() => import('./diff/DiffView.svelte')} {path} target={diff} onClose={() => (diff = null)} onOpen={(t: DiffTarget) => (diff = t)} />
        {/if}
      </div>
    {/if}
  </div>
  <!-- The rebase editor and the conflict tool each have a panel of their own. -->
  {#if (view === 'wip' || selected || running) && !right.hidden && !rebaseBase && diff?.mode !== 'conflict'}
    <div class="right" style:width="{rightWidth}px">
      <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
      <div
        class="edge"
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize the right panel"
        aria-valuenow={rightWidth}
        aria-valuemin={MIN_WIDTH}
        aria-valuemax={maxWidth(windowWidth)}
        tabindex="0"
        title="Drag to resize. Double-click to reset."
        onpointerdown={startRightResize}
        onpointermove={rightResize}
        onpointerup={() => (rightDrag = null)}
        ondblclick={() => (right.width = DEFAULT_WIDTH)}
        onkeydown={rightResizeKey}
      ></div>
  {#if view === 'wip'}
    <WorkingTreePanel {path} current={diff} openFile={(t) => (diff = t)} onNewBranch={() => (dialog = newBranchDialog(ctx))} onMenu={openMenu} />
  {:else if running && !bisectAside && (!selected || selected.id === bisectSubject || running.candidates.length > 0)}
    <!-- Loaded when a bisect first runs: most sessions never need it. -->
    <Lazy
      load={() => import('./BisectPanel.svelte')}
      {path}
      info={running}
      onOpenCommit={(id: string) => {
        bisectAside = true
        void revealCommit(id)
      }}
      onFixBranch={fixBranch}
      onRevert={revertFound}
    />
  {:else if versions}
    <Lazy
      load={() => import('./VersionsPanel.svelte')}
      compared={versions.compared}
      branch={versions.branch}
      oldLabel={versions.oldLabel}
      current={pairAt}
      {path}
      onRecompare={recompare}
      onClose={closeVersions}
      onOpen={(i: number) => (pairAt = i)}
      onShow={(id: string) => {
        pairAt = -1
        void revealCommit(id)
      }}
    />
  {:else if several}
    <SelectionPanel {path} selected={several} {ctx} squash={squashState} onUnpick={(row) => graph.unpick(row)} onShow={(row) => graph.reveal(row)} openFile={(t) => (diff = t)} />
  {:else if selected}
    <CommitPanel {path} onMenu={openMenu} showNotes={noteAsk} {filterSig} {pinnedKeys} onPin={setPin} history={historyOn} commitId={selected.id} refs={selectedRefs} current={diff} openFile={(t) => (diff = t)} onSearch={(q) => graph?.search(q)} onAuthor={(sha, name, from) => {
        const at = from.getBoundingClientRect()
        authorCard = { sha, name, x: at.left, y: at.bottom + 4, above: at.top, opener: from }
      }} onReveal={revealCommit} onActions={(x, y) => {
        if (!selected) return
        const row = selected
        const refs = graph.refsOf(row.id)
        void whileBusy(loadRefExtras(path, refs)).then((e) => openMenu(commitMenuItems(row, refs, ctx, e), x, y))
      }} />
  {/if}
    </div>
  {/if}
</div>

{#if menu}
  <ContextMenu x={menu.x} y={menu.y} items={menu.items} onClose={() => (menu = null)} />
{/if}
{#if authorCard}
  {#key authorCard.sha}
    <Lazy load={() => import('./people/AuthorCard.svelte')} repo={path} {...authorCard} onSearch={(q: string) => graph?.search(q)} onClose={() => (authorCard = null)} />
  {/key}
{/if}
{#if dialog}
  <Dialog spec={dialog} onClose={() => (dialog = null)} />
{/if}
{#if preview}
  <Lazy load={() => import('./PreviewDialog.svelte')} {path} {...preview} current={ctx.current()} onClose={() => (preview = null)} onEdit={ctx.openRebase} />
{/if}
{#if patchesStart}
  <Lazy load={() => import('./SavePatchesDialog.svelte')} {path} start={patchesStart} onClose={() => (patchesStart = null)} />
{/if}
{#if archiving}
  <Lazy load={() => import('./ArchiveDialog.svelte')} {path} rev={archiving.rev} label={archiving.label} onClose={() => (archiving = null)} />
{/if}
{#if patchFile}
  <Lazy load={() => import('./ApplyPatchesDialog.svelte')} {path} file={patchFile} onClose={() => (patchFile = null)} />
{/if}
{#if diffFile}
  <Lazy load={() => import('./ApplyDiffDialog.svelte')} {path} file={diffFile} onClose={() => (diffFile = null)} />
{/if}
{#if comparing}
  <Lazy
    load={() => import('./CompareVersionsDialog.svelte')}
    {path}
    branch={comparing.branch}
    oldTip={comparing.oldTip}
    onClose={() => (comparing = null)}
    onCompared={(compared: app.VersionsCompared, oldLabel: string) => showVersions(compared, comparing?.branch ?? '', oldLabel)}
  />
{/if}
{#if bisect.starting?.path === path}
  <Lazy load={() => import('./BisectDialog.svelte')} {path} bad={bisect.starting.bad} onClose={() => (bisect.starting = null)} />
{/if}
{#if reflogRef}
  <Lazy
    load={() => import('./ReflogDialog.svelte')}
    {path}
    initialRef={reflogRef}
    onReveal={(commit: string) => {
      reflogRef = ''
      void revealCommit(commit)
    }}
    onCreateBranch={(commit: string) => {
      reflogRef = ''
      dialog = branchAtDialog(path, commit)
    }}
    onCompare={(branch: string, commit: string, label: string) => {
      reflogRef = ''
      versionsAsk.show({ path, branch: branch || ctx.current(), oldTip: commit, oldLabel: label })
    }}
    onClose={() => (reflogRef = '')}
  />
{/if}

{#if worktreesOpen}
  <Lazy load={() => import('./WorktreesDialog.svelte')} {path} onClose={() => (worktreesOpen = false)} />
{/if}

{#if remotesOpen}
  <Lazy load={() => import('./RemotesDialog.svelte')} {path} onClose={() => (remotesOpen = false)} />
{/if}

<style>
  .repo {
    display: flex;
    height: 100%;
    min-height: 0;
  }

  .main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .graph {
    flex: 1;
    min-height: 0;
  }

  /* The panels inside lay out by this width, not the window's: see their container queries. */
  .right {
    position: relative;
    flex: none;
    display: flex;
    min-height: 0;
    container: right-panel / inline-size;
  }

  .edge {
    position: absolute;
    top: 0;
    bottom: 0;
    left: -3px;
    width: 6px;
    z-index: 3;
    cursor: col-resize;
    touch-action: none;
    transition: background-color var(--dur-quick);
  }

  .edge:hover,
  .edge:focus-visible {
    background: var(--link);
    outline: none;
  }

  @media (prefers-reduced-motion: reduce) {
    .edge {
      transition: none;
    }
  }

  .hidden {
    display: none;
  }

  /* The repository's actions, with the right panel's toggle at the end. */
  .bar {
    position: relative;
    flex: none;
    display: flex;
    align-items: center;
    border-bottom: 1px solid var(--paper-edge);
  }

  /* Takes the room the actions leave. */
  .graph-tools {
    flex: 1 1 0;
    min-width: 0;
    /* Narrower than the last step allows for: cut, never wrapped. Kinds' list still drops below. */
    overflow-x: clip;
    display: flex;
    border-left: 1px solid var(--paper-edge);
  }

  .graph-tools.hidden {
    display: none;
  }

  .panel-toggle {
    flex: none;
    margin: 0 8px 0 auto;
    transform: scaleX(-1);
  }

  .wip {
    position: relative;
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 10px;
    height: 100%;
    /* The left padding comes from the graph: where its messages start. */
    padding: 0 14px 0 44px;
    background: none;
    border: none;
    border-bottom: 1px dotted var(--paper-edge);
    border-radius: 0;
    text-align: left;
    transition: background-color var(--dur-base);
  }

  .wip:hover:not(:disabled) {
    background: var(--wash-hover);
  }

  .wip.active {
    background: var(--paper-warm);
  }

  .wip-title {
    font-weight: 500;
    color: var(--ink);
    white-space: nowrap;
  }

  .where {
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .review {
    flex: none;
    margin-left: auto;
    padding: 3px 10px;
    border-radius: var(--r-sm);
    background: color-mix(in srgb, var(--link) 12%, transparent);
    font-size: 14px;
    color: var(--link);
  }

  .ring {
    box-sizing: border-box;
    width: 14px;
    height: 14px;
    border: 2px dashed var(--lane-0);
    border-radius: 50%;
  }

  .dot {
    position: absolute;
    left: 14px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    /* The icon's size, kept while there's no icon yet so the title doesn't shift. */
    min-width: 16px;
    min-height: 16px;
    color: var(--ink-faint);
  }

  .dot.dirty {
    color: var(--warning);
  }
</style>
