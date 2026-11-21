import { KeepApart, MailmapNotes, MailmapSources, People, PeopleSuggestions, PlanSamePerson, ReadMailmapFile, SaveMailmap, WhoIs } from '../../../wailsjs/go/app/App.js'
import type { app, git } from '../../../wailsjs/go/models'
import { errorText } from '../errors'
import { WORK_TREE, removeLines } from './mailmap'
import { peopleTab } from './tab.svelte'

/** How many people are asked for at a time. */
export const PAGE = 100

/** The work tree's .mailmap being edited: its text as last read from disk, and the working copy. */
interface Doc {
  base: string
  text: string
}

/**
 * What the People tab shows: who one repository's history names, how its mailmap reads
 * them, and the mailmap itself. It loads with the view; the tab itself (open or not, which
 * repository) is peopleTab, which the app shell holds. It outlives the view, so an edit is
 * still there after a look at another tab.
 *
 * A repository needs no mailmap. Everything here works without one, and nothing is written
 * until the user saves. Where there's no .mailmap, saving first asks whether to create it.
 */
class PeopleState {
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
  /** Who looks like the same person: what's offered, what's left out, and what was answered. */
  suggestions = $state<app.Suggestions | null>(null)
  error = $state('')
  /** Why the last save failed, until the next save, revert or reload. */
  saveError = $state('')
  busy = $state(false)
  /** True while the history is being read for the first time: on a long one that takes seconds. */
  reading = $state(false)
  loadingMore = $state(false)
  /** Set while the user is asked whether to create the repository's first .mailmap. */
  confirmCreate = $state(false)

  repo = $derived(peopleTab.repo)
  /** The work tree's .mailmap, which is always listed, whether or not it exists. */
  shared = $derived(this.sources.find((s) => s.source === WORK_TREE))
  exists = $derived(this.shared?.exists ?? false)
  source = $derived(this.sources.find((s) => s.source === this.selected) ?? this.shared)
  dirty = $derived(!!this.doc && this.doc.text !== this.doc.base)
  hasMore = $derived(!!this.list && this.people.length < this.list.matching)

  #loadedFor = ''
  #closes = 0
  #asked = 0
  #timer: ReturnType<typeof setTimeout> | undefined

  /** The mailmap text names are read through: the edit when there is one, else what's saved. */
  #mailmap(): app.MailmapText {
    return { text: this.doc?.text ?? '', edited: this.dirty } as app.MailmapText
  }

  #forget() {
    this.doc = null
    this.others = {}
    this.sources = []
    this.list = null
    this.people = []
    this.notes = []
    this.suggestions = null
    this.query = ''
    this.selected = WORK_TREE
    this.view = 'people'
    this.error = ''
    this.saveError = ''
    this.confirmCreate = false
    this.#loadedFor = ''
  }

  /** Reads the files and the people, keeping an unsaved edit. */
  async load() {
    const repo = this.repo
    // The tab was closed since this last showed, or shows another repository: start afresh.
    if (this.#closes !== peopleTab.closes || this.#loadedFor !== repo) {
      this.#forget()
      this.#closes = peopleTab.closes
      this.#loadedFor = repo
    }
    if (!repo) return
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
      const [list, notes, suggestions] = await Promise.all([
        People(repo, { mailmap: this.#mailmap(), query: this.query, offset: 0, limit: Math.max(PAGE, this.people.length) } as app.PeopleRequest),
        MailmapNotes(repo, this.#mailmap(), this.selected),
        PeopleSuggestions(repo, this.#mailmap()),
      ])
      if (n !== this.#asked || repo !== this.repo) return
      this.list = list
      this.people = list.people
      this.notes = notes
      this.suggestions = suggestions
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

  /** The lines that would make these identities one person, shown as name and address. Nothing is written. */
  plan(identities: git.Identity[], name: string, email: string) {
    return PlanSamePerson(this.repo, this.#mailmap(), { identities, name, email } as app.SamePersonRequest)
  }

  /** Remembers that a suggestion is of different people, or forgets it, and asks again. */
  async keepApart(key: string, apart: boolean) {
    try {
      await KeepApart(this.repo, key, apart)
    } catch (e) {
      this.error = errorText(e)
    }
    await this.refresh()
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
}

export const people = new PeopleState()
