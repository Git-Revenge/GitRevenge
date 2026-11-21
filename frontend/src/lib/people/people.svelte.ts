import { MailmapNotes, MailmapSources, People, ReadMailmapFile, SaveMailmap, SetPeopleTab, WhoIs } from '../../../wailsjs/go/app/App.js'
import type { app, git } from '../../../wailsjs/go/models'
import { errorText } from '../errors'
import { session } from '../session.svelte'
import { WORK_TREE, removeLines } from './mailmap'

/** How many people are asked for at a time. */
export const PAGE = 100

/** The work tree's .mailmap being edited: its text as last read from disk, and the working copy. */
interface Doc {
  base: string
  text: string
}

/**
 * The People tab: who one repository's history names, how its mailmap reads them, and the
 * mailmap itself. It shows a repository open as a tab; if that tab closes, it moves to another.
 *
 * A repository needs no mailmap. Everything here works without one, and nothing is written
 * until the user saves. Where there's no .mailmap, saving first asks whether to create it.
 */
class PeopleTab {
  open = $state(false)
  /** The repository the tab was opened for. */
  wanted = $state('')
  sources = $state<git.MailmapSource[]>([])
  /** The source showing in Text. */
  selected = $state(WORK_TREE)
  view = $state<'people' | 'text'>('people')
  doc = $state<Doc | null>(null)
  /** The sources that are only read, by name, as last read. */
  others = $state<Record<string, string>>({})
  /** The first page's answer, for its totals; people holds every page asked for so far. */
  list = $state<app.PeopleList | null>(null)
  people = $state<app.Person[]>([])
  query = $state('')
  notes = $state<git.MailmapNote[]>([])
  error = $state('')
  /** Why the last save failed, until the next save, revert or reload. */
  saveError = $state('')
  busy = $state(false)
  /** True while the history is being read for the first time: on a long one that takes seconds. */
  reading = $state(false)
  loadingMore = $state(false)
  confirmClose = $state(false)
  /** Set while the user is asked whether to create the repository's first .mailmap. */
  confirmCreate = $state(false)

  /** The repository shown: the one asked for while it's open, else the first open one. */
  repo = $derived(session.tabs.find((t) => t.path === this.wanted)?.path ?? session.tabs[0]?.path ?? '')
  repoName = $derived(session.tabs.find((t) => t.path === this.repo)?.name ?? '')
  /** The work tree's .mailmap, which is always listed, whether or not it exists. */
  shared = $derived(this.sources.find((s) => s.source === WORK_TREE))
  exists = $derived(this.shared?.exists ?? false)
  source = $derived(this.sources.find((s) => s.source === this.selected) ?? this.shared)
  dirty = $derived(!!this.doc && this.doc.text !== this.doc.base)
  hasMore = $derived(!!this.list && this.people.length < this.list.matching)

  #loadedFor = ''
  #asked = 0
  #timer: ReturnType<typeof setTimeout> | undefined

  /** The mailmap text names are read through: the edit when there is one, else what's saved. */
  #mailmap(): app.MailmapText {
    return { text: this.doc?.text ?? '', edited: this.dirty } as app.MailmapText
  }

  /** Opens the tab on a repository and selects it. */
  async show(repo: string) {
    this.open = true
    if (repo && repo !== this.wanted) this.setRepo(repo)
    await session.select(session.peopleTabId)
    await this.load()
  }

  /** Restores the tab from the last session. */
  async restore(saved: string) {
    if (!saved) return
    this.open = true
    this.wanted = saved
    await this.load()
  }

  setRepo(repo: string) {
    if (this.dirty) return
    this.wanted = repo
    this.#forget()
    void SetPeopleTab(repo).catch(() => {})
  }

  #forget() {
    this.doc = null
    this.others = {}
    this.sources = []
    this.list = null
    this.people = []
    this.notes = []
    this.query = ''
    this.selected = WORK_TREE
    this.error = ''
    this.saveError = ''
    this.#loadedFor = ''
  }

  /** Reads the files and the people, keeping an unsaved edit. */
  async load() {
    const repo = this.repo
    if (!repo) {
      this.#forget()
      return
    }
    if (this.#loadedFor !== repo) {
      this.#forget()
      this.#loadedFor = repo
      void SetPeopleTab(repo).catch(() => {})
    }
    this.error = ''
    try {
      const sources = await MailmapSources(repo)
      if (repo !== this.repo) return
      this.sources = sources
      if (!sources.some((s) => s.source === this.selected)) this.selected = WORK_TREE
      // The file is read again unless it's being edited, so a change made elsewhere shows.
      if (!this.dirty) {
        const text = await ReadMailmapFile(repo, WORK_TREE)
        if (repo !== this.repo) return
        if (!this.dirty) this.doc = { base: text, text }
      }
      const others: Record<string, string> = {}
      for (const s of sources) if (s.source !== WORK_TREE) others[s.source] = await ReadMailmapFile(repo, s.source).catch(() => '')
      if (repo !== this.repo) return
      this.others = others
    } catch (e) {
      this.error = errorText(e)
    }
    await this.refresh()
  }

  /** Asks again who the history names and what each line does, through the text as it stands. */
  async refresh() {
    const repo = this.repo
    if (!repo || !this.doc) return
    const n = ++this.#asked
    if (!this.list) this.reading = true
    try {
      const [list, notes] = await Promise.all([
        People(repo, { mailmap: this.#mailmap(), query: this.query, offset: 0, limit: Math.max(PAGE, this.people.length) } as app.PeopleRequest),
        MailmapNotes(repo, this.#mailmap(), this.selected),
      ])
      if (n !== this.#asked || repo !== this.repo) return
      this.list = list
      this.people = list.people
      this.notes = notes
      this.error = ''
    } catch (e) {
      if (n === this.#asked) this.error = errorText(e)
    }
    if (n === this.#asked) this.reading = false
  }

  /** Asks again once typing pauses. */
  #soon() {
    clearTimeout(this.#timer)
    this.#timer = setTimeout(() => void this.refresh(), 220)
  }

  /** Reads the next page of people. */
  async more() {
    const repo = this.repo
    if (!repo || !this.hasMore || this.loadingMore) return
    const n = this.#asked
    this.loadingMore = true
    try {
      const next = await People(repo, { mailmap: this.#mailmap(), query: this.query, offset: this.people.length, limit: PAGE } as app.PeopleRequest)
      // An answer to a question that has since changed would put the wrong people after these.
      if (n === this.#asked && repo === this.repo) this.people = [...this.people, ...next.people]
    } catch (e) {
      this.error = errorText(e)
    }
    this.loadingMore = false
  }

  search(q: string) {
    this.query = q
    this.#soon()
  }

  async select(source: string) {
    this.selected = source
    await this.refresh()
  }

  edit(text: string) {
    if (!this.doc) return
    this.doc = { ...this.doc, text }
    this.saveError = ''
    this.#soon()
  }

  /** Takes an identity away from the person it was joined to, by removing the lines that joined it. */
  takeApart(id: app.PersonIdentity) {
    if (!this.doc) return
    this.edit(removeLines(this.doc.text, id.lines.filter((l) => l.source === WORK_TREE).map((l) => l.line)))
  }

  revert() {
    if (!this.doc) return
    this.doc = { ...this.doc, text: this.doc.base }
    this.saveError = ''
    this.confirmCreate = false
    void this.refresh()
  }

  who(query: string) {
    return WhoIs(this.repo, this.#mailmap(), query)
  }

  /**
   * Saves the .mailmap. A repository that has none is asked about first: the file is only
   * created once the user says yes, which is what `create` records.
   */
  async save(create = false) {
    const d = this.doc
    if (!d || !this.dirty) return
    if (!this.exists && !create && d.text.trim() !== '') {
      this.confirmCreate = true
      return
    }
    this.confirmCreate = false
    this.busy = true
    this.saveError = ''
    try {
      await SaveMailmap(this.repo, d.base, d.text, create)
      const now = await ReadMailmapFile(this.repo, WORK_TREE)
      this.doc = { base: now, text: now }
      await this.load()
    } catch (e) {
      this.saveError = errorText(e)
    }
    this.busy = false
  }

  /** Drops the edit and reads the file again from disk. */
  async reload() {
    const now = await ReadMailmapFile(this.repo, WORK_TREE)
    this.doc = { base: now, text: now }
    this.error = ''
    this.saveError = ''
    await this.load()
  }

  /** Closes the tab, first asking if that would discard an unsaved edit. */
  requestClose() {
    if (this.dirty) this.confirmClose = true
    else void this.close()
  }

  async close() {
    this.confirmClose = false
    this.confirmCreate = false
    if (session.active === session.peopleTabId) await session.select(session.tabs.at(-1)?.path ?? '')
    this.open = false
    this.wanted = ''
    this.#forget()
    void SetPeopleTab('').catch(() => {})
  }
}

export const people = new PeopleTab()
