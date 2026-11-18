import {
  ChooseAndOpenRepo,
  CloseRepo,
  OpenRepo,
  RecentRepos,
  RemoveRecentRepo,
  RepoInfo,
  RestoreSession,
  SetActiveRepo,
  TakeProblem,
  TakeTrustQuestion,
  TrustRepo,
} from '../../wailsjs/go/app/App.js'
import type { app } from '../../wailsjs/go/models'
import { errorText } from './errors'
import { onBackend, type BackendEvents } from './events'
import { fromText, type Problem } from './problem'
import { forgetRepoReads } from './repoReads'

/** Open tabs, the active tab and recent repos, mirrored from the Go session manager. */
class Session {
  tabs = $state<app.RepoTab[]>([])
  /** Path of the active tab, or '' for the start page. */
  active = $state('')
  recent = $state<app.RecentRepo[]>([])
  /** The error the banner is showing, or null. */
  problem = $state<Problem | null>(null)

  /** The banner's error as text, or ''. Setting it shows that text, and '' clears the banner. */
  get error(): string {
    return this.problem?.text ?? ''
  }

  set error(text: string) {
    this.problem = text ? fromText(text) : null
  }
  /** Per-repo counter bumped on every change on disk; views re-fetch when it moves. Only the
   * repository on screen is watched, so it only moves for that one: a tab that's switched to
   * reads everything as its view opens. */
  revision = $state<Record<string, number>>({})
  /** The same, for a view that only cares about the repository's git config. Most changes
   * are saves in the working tree, which that view can sit out. */
  configRevision = $state<Record<string, number>>({})
  /** Per-repo counter bumped when its undo or redo history changes. */
  undoRevision = $state<Record<string, number>>({})
  /** Branches whose work is already on another branch, by repository and name, with that
   * branch. It fills in after the branches load, since checking takes a while. */
  merged = $state<Record<string, Record<string, string>>>({})
  /** The active value for the Git Config tab; never a repository path. */
  configTabId = $state('gitrevenge:gitconfig')
  /** The config file the Git Config tab had selected last session, or '' if it was closed. */
  savedConfigTab = ''
  /** The active value for the Insights tab; never a repository path. */
  insightsTabId = $state('gitrevenge:insights')
  /** The Insights tab's scope last session ("repo:…" or "group:…"), or '' if it was closed. */
  savedInsightsTab = ''
  /** The active value for the Ignore rules tab; never a repository path. */
  ignoreTabId = $state('gitrevenge:ignore')
  /** The repository the Ignore rules tab showed last session, or '' if it was closed. */
  savedIgnoreTab = ''

  activeTab = $derived(this.tabs.find((t) => t.path === this.active))

  async restore() {
    onBackend('repo:changed', (c) => void this.onChange(c))
    // The watcher can report an operation before its undo entry exists, so undo history
    // has its own event.
    onBackend('undo:changed', (c) => {
      this.undoRevision[c.path] = (this.undoRevision[c.path] ?? 0) + 1
    })
    // Global and included config apply to every repository, so a change refreshes them all:
    // signing, identity, and pull and merge defaults are read again.
    onBackend('gitconfig:changed', () => {
      for (const t of this.tabs) {
        this.revision[t.path] = (this.revision[t.path] ?? 0) + 1
        this.configRevision[t.path] = (this.configRevision[t.path] ?? 0) + 1
      }
    })
    const st = await this.run(() => RestoreSession())
    if (!st) return
    this.tabs = st.tabs
    this.configTabId = st.configTabId
    this.savedConfigTab = st.configTab
    this.insightsTabId = st.insightsTabId
    this.savedInsightsTab = st.insightsTab
    this.ignoreTabId = st.ignoreTabId
    this.savedIgnoreTab = st.ignoreTab
    this.active = st.active
    if (st.failed.length > 0) {
      this.error = `Couldn't reopen: ${st.failed.map((f) => f.path).join(', ')}`
    }
    await this.refreshRecent()
  }

  async choose() {
    const tab = await this.opening(() => ChooseAndOpenRepo())
    if (tab) this.add(tab)
  }

  async open(path: string) {
    const tab = await this.opening(() => OpenRepo(path))
    if (tab) this.add(tab)
  }

  /**
   * A repository that wasn't opened because its own git config or hooks would run programs,
   * and what they are, for TrustDialog to ask about. Nothing has run in it yet.
   */
  trustQuestion = $state<app.TrustQuestion | null>(null)

  /** Opens a repository. If the app refused it for want of trust, that's a question to ask,
   * not an error to show. */
  private async opening<T>(fn: () => Promise<T>): Promise<T | undefined> {
    try {
      return await fn()
    } catch (e) {
      const question = await TakeTrustQuestion().catch(() => null)
      if (!question) return this.run(() => Promise.reject(e))
      this.trustQuestion = question
      return undefined
    }
  }

  /** The user agrees: remember that, and open the repository. */
  async trustAndOpen() {
    const question = this.trustQuestion
    if (!question) return
    this.trustQuestion = null
    const trusted = await this.run(() => TrustRepo(question.path).then(() => true))
    if (trusted) await this.open(question.path)
  }

  /** The user doesn't: the repository stays closed, and nothing in it has run. */
  declineTrust() {
    this.trustQuestion = null
  }

  async close(path: string) {
    const i = this.tabs.findIndex((t) => t.path === path)
    if (i < 0) return
    await this.run(() => CloseRepo(path))
    forgetRepoReads(path)
    this.tabs = this.tabs.filter((t) => t.path !== path)
    if (this.active === path) {
      // Same rule as the backend, which is watching that one now: select the tab that slid
      // into this position.
      this.active = this.tabs[Math.min(i, this.tabs.length - 1)]?.path ?? ''
      if (this.active) void this.refreshTab(this.active)
    }
  }

  /** Closes the tab on a worktree that's been removed, if there is one, and says if its folder
   * had to stay. Git and the app can spell a path differently, so they're compared loosely. */
  async closeRemoved(dir: string, leftover = '') {
    const key = (p: string) => p.replaceAll('\\', '/').replace(/\/+$/, '').toLowerCase()
    const tab = this.tabs.find((t) => key(t.path) === key(dir))
    if (tab) await this.close(tab.path)
    if (leftover) this.error = `The worktree is gone, but its folder couldn't be deleted because something is using it: ${leftover}`
  }

  /** Switches of tab still on their way to the backend, in the order they were asked for. */
  private selecting: Promise<void> = Promise.resolve()

  /**
   * Switches to a tab. The backend watches only the repository on screen, so it's told first
   * and the view opens once it has started watching: everything the view reads as it opens is
   * then already covered, and nothing can change unseen in between. That's a few milliseconds.
   * Switches go to the backend one at a time, so the last tab asked for is the one it ends up
   * watching however fast they come.
   */
  select(path: string): Promise<void> {
    const switched = this.selecting.then(async () => {
      await this.run(() => SetActiveRepo(path))
      this.active = path
      // Its branch may have changed while it wasn't being watched.
      if (this.tabs.some((t) => t.path === path)) void this.refreshTab(path)
    })
    this.selecting = switched
    return switched
  }

  /** Reads a tab's label again. */
  private async refreshTab(path: string) {
    const tab = await RepoInfo(path).catch(() => null)
    if (!tab) return
    // The tabs may have moved or closed while that was being read.
    const i = this.tabs.findIndex((t) => t.path === path)
    if (i >= 0) this.tabs[i] = tab
  }

  async forget(path: string) {
    await this.run(() => RemoveRecentRepo(path))
    await this.refreshRecent()
  }

  async refreshRecent() {
    this.recent = (await this.run(() => RecentRepos())) ?? []
  }

  /** Handles a change detected on disk by the backend watcher. */
  private async onChange(c: BackendEvents['repo:changed']) {
    this.revision[c.path] = (this.revision[c.path] ?? 0) + 1
    if (c.kinds.includes('config')) this.configRevision[c.path] = (this.configRevision[c.path] ?? 0) + 1
    if (c.kinds.includes('head') || c.kinds.includes('refs')) {
      if (!this.tabs.some((t) => t.path === c.path)) return
      const tab = await this.run(() => RepoInfo(c.path))
      // Found again by its path: a tab closed while that was being read moves the ones after
      // it, and the answer would land on a neighbour.
      const i = this.tabs.findIndex((t) => t.path === c.path)
      if (tab && i >= 0) this.tabs[i] = tab
    }
  }

  private add(tab: app.RepoTab) {
    if (!this.tabs.some((t) => t.path === tab.path)) this.tabs.push(tab)
    this.active = tab.path
    this.error = ''
    void this.refreshRecent()
  }

  /** Runs a backend call, turning a rejection into the error banner. The backend keeps more
   * than the message for failures it understands: a headline, next steps and git's output. */
  async run<T>(fn: () => Promise<T>): Promise<T | undefined> {
    try {
      return await fn()
    } catch (e) {
      const text = errorText(e)
      const known = await TakeProblem(text).catch(() => null)
      this.problem = known ? { ...known, actions: known.actions ?? [], text, retry: fn } : fromText(text)
      return undefined
    }
  }
}

export const session = new Session()
