import { SetPeopleTab } from '../../../wailsjs/go/app/App.js'
import type { app } from '../../../wailsjs/go/models'
import { session } from '../session.svelte'

/** Someone to show in the People view: found in the list, or, with `pick`, picked as the
 * first of two for "Same person as…". */
export interface PersonAsk {
  person: app.Person
  pick: boolean
}

/**
 * The People tab as the app shell knows it: whether it's open, on which repository, and
 * whether it holds an unsaved edit. That's all the tab bar and the menus need, so it's all
 * that loads with the app. What the tab shows lives in people.svelte.ts, which loads with
 * the view (the main chunk has a size budget; see scripts/bundle-budget.mjs).
 */
class PeopleTab {
  open = $state(false)
  /** The repository the tab was opened for. */
  wanted = $state('')
  /** Whether the view holds an edit that isn't saved. The view keeps this up to date. */
  dirty = $state(false)
  /** Set to ask, in the view, whether closing may discard that edit. */
  confirmClose = $state(false)
  /** Counts closes, so the view's state knows to start afresh the next time it opens. */
  closes = 0
  /** Someone the view should go to when it next shows; an author's card in the graph asks. */
  ask = $state<PersonAsk | null>(null)

  /** The repository shown: the one asked for while it's open, else the first open one. */
  repo = $derived(session.tabs.find((t) => t.path === this.wanted)?.path ?? session.tabs[0]?.path ?? '')
  repoName = $derived(session.tabs.find((t) => t.path === this.repo)?.name ?? '')

  /** Opens the tab on a repository and selects it, going to someone when one is asked for. */
  async show(repo: string, ask?: PersonAsk) {
    this.open = true
    if (repo && repo !== this.wanted) this.setRepo(repo)
    // An unsaved edit keeps the tab on its repository; the person asked for isn't in that one.
    this.ask = ask && this.repo === repo ? ask : null
    await session.select(session.peopleTabId)
  }

  /** Restores the tab from the last session. */
  restore(saved: string) {
    if (!saved) return
    this.open = true
    this.wanted = saved
  }

  /** Shows another repository, unless that would drop an unsaved edit. */
  setRepo(repo: string) {
    if (this.dirty) return
    this.wanted = repo
    void SetPeopleTab(repo).catch(() => {})
  }

  /** Closes the tab. With an unsaved edit it shows the tab first, where the view asks. */
  async requestClose() {
    if (!this.dirty) return this.close()
    await session.select(session.peopleTabId)
    this.confirmClose = true
  }

  async close() {
    this.confirmClose = false
    if (session.active === session.peopleTabId) await session.select(session.tabs.at(-1)?.path ?? '')
    this.open = false
    this.wanted = ''
    this.dirty = false
    this.ask = null
    this.closes++
    void SetPeopleTab('').catch(() => {})
  }
}

export const peopleTab = new PeopleTab()
