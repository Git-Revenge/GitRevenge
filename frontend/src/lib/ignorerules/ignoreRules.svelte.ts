import { CheckIgnorePath, IgnoreSources, IgnoreStats, ReadIgnoreFile, SaveIgnoreFile, SetIgnoreTab } from '../../../wailsjs/go/app/App.js'
import type { git } from '../../../wailsjs/go/models'
import { errorText } from '../errors'
import { session } from '../session.svelte'
import { sameSource } from './rules'

/** One ignore file being edited: its text as last read from disk, and the working copy. */
interface Doc {
  base: string
  text: string
}

/** The Ignore rules tab: one repository's ignore files, the one selected, and the edits made
 * to each. It shows a repository open as a tab; if that tab closes, it moves to another. */
class IgnoreRulesTab {
  open = $state(false)
  /** The repository the tab was opened for. */
  wanted = $state('')
  sources = $state<git.IgnoreSource[]>([])
  selected = $state('')
  view = $state<'rules' | 'text'>('rules')
  docs = $state<Record<string, Doc>>({})
  stats = $state<git.IgnoreStats | null>(null)
  error = $state('')
  /** Why the last save failed, until the next save, revert or reload. A refresh from disk
   * leaves it, so a refused save stays explained. */
  saveError = $state('')
  busy = $state(false)
  /** A line to highlight once the file shows, then clear. */
  focusLine = $state(0)
  confirmClose = $state(false)

  /** The repository shown: the one asked for while it's open, else the first open one. */
  repo = $derived(session.tabs.find((t) => t.path === this.wanted)?.path ?? session.tabs[0]?.path ?? '')
  repoName = $derived(session.tabs.find((t) => t.path === this.repo)?.name ?? '')
  source = $derived(this.sources.find((s) => sameSource(s.source, this.selected)) ?? this.sources[0])
  doc = $derived(this.source ? this.docs[this.source.source] : undefined)
  dirty = $derived(Object.entries(this.docs).filter(([, d]) => d.text !== d.base).map(([s]) => s))

  #loadedFor = ''

  /** Opens the tab on a repository, optionally at one file and line, and selects it. */
  async show(repo: string, source = '', line = 0) {
    this.open = true
    if (repo && repo !== this.wanted) this.setRepo(repo)
    await this.load()
    if (source) {
      this.selected = this.sources.find((s) => sameSource(s.source, source))?.source ?? source
      await this.ensureDoc()
      this.view = 'rules'
      this.focusLine = line
    }
    await session.select(session.ignoreTabId)
  }

  /** Restores the tab from the last session. */
  async restore(saved: string) {
    if (!saved) return
    this.open = true
    this.wanted = saved
    await this.load()
  }

  setRepo(repo: string) {
    if (this.dirty.length) return
    this.wanted = repo
    this.docs = {}
    this.selected = ''
    this.stats = null
    this.#loadedFor = ''
    void SetIgnoreTab(repo).catch(() => {})
  }

  /** Reads the list of files, the selected file and the counts, keeping unsaved edits. */
  async load() {
    const repo = this.repo
    if (!repo) {
      this.sources = []
      return
    }
    if (this.#loadedFor !== repo) {
      this.#loadedFor = repo
      this.docs = {}
      void SetIgnoreTab(repo).catch(() => {})
    }
    this.error = ''
    try {
      const list = await IgnoreSources(repo)
      if (repo !== this.repo) return
      this.sources = list
      if (!this.sources.some((s) => sameSource(s.source, this.selected))) this.selected = this.sources[0]?.source ?? ''
      await this.ensureDoc()
      // Re-read files without edits, so changes made elsewhere show up.
      for (const [src, d] of Object.entries(this.docs)) {
        if (d.text !== d.base) continue
        const now = await ReadIgnoreFile(repo, src).catch(() => d.base)
        if (repo === this.repo && this.docs[src] && this.docs[src].text === this.docs[src].base) this.docs[src] = { base: now, text: now }
      }
    } catch (e) {
      this.error = errorText(e)
    }
    void this.loadStats()
  }

  async loadStats() {
    const repo = this.repo
    const res = await IgnoreStats(repo).catch(() => null)
    if (repo === this.repo) this.stats = res
  }

  async ensureDoc() {
    const src = this.source?.source
    if (!src || this.docs[src]) return
    const text = await ReadIgnoreFile(this.repo, src)
    this.docs[src] = { base: text, text }
  }

  async select(source: string) {
    this.selected = source
    this.saveError = ''
    try {
      await this.ensureDoc()
    } catch (e) {
      this.error = errorText(e)
    }
  }

  edit(text: string) {
    const src = this.source?.source
    if (!src || !this.docs[src]) return
    this.docs[src] = { ...this.docs[src], text }
  }

  revert() {
    const src = this.source?.source
    if (src && this.docs[src]) this.docs[src] = { ...this.docs[src], text: this.docs[src].base }
    this.saveError = ''
  }

  async save() {
    const src = this.source?.source
    const d = src ? this.docs[src] : undefined
    if (!src || !d) return
    this.busy = true
    this.saveError = ''
    try {
      await SaveIgnoreFile(this.repo, src, d.base, d.text)
      const now = await ReadIgnoreFile(this.repo, src)
      this.docs[src] = { base: now, text: now }
      await this.load()
    } catch (e) {
      this.saveError = errorText(e)
    }
    this.busy = false
  }

  /** Drops the selected file's edits and reads it again from disk. */
  async reload() {
    const src = this.source?.source
    if (!src) return
    const now = await ReadIgnoreFile(this.repo, src)
    this.docs[src] = { base: now, text: now }
    this.error = ''
    this.saveError = ''
    void this.loadStats()
  }

  check(path: string) {
    return CheckIgnorePath(this.repo, path)
  }

  /** Closes the tab, first asking if that would discard unsaved edits. */
  requestClose() {
    if (this.dirty.length) this.confirmClose = true
    else void this.close()
  }

  async close() {
    this.confirmClose = false
    if (session.active === session.ignoreTabId) await session.select(session.tabs.at(-1)?.path ?? '')
    this.open = false
    this.docs = {}
    this.sources = []
    this.stats = null
    this.selected = ''
    this.wanted = ''
    this.#loadedFor = ''
    void SetIgnoreTab('').catch(() => {})
  }
}

export const ignoreRules = new IgnoreRulesTab()
