import { tick, untrack } from 'svelte'
import type { FileRow } from './fileList'
import { drawnRows, moveTarget, rowTops, visibleRows } from './listWindow'

/** The nearest element that scrolls this one. */
function scrollerOf(el: HTMLElement): HTMLElement {
  for (let up = el.parentElement; up; up = up.parentElement) if (/auto|scroll/.test(getComputedStyle(up).overflowY)) return up
  return document.documentElement
}

/**
 * Keeps only the rows of a file list that are in view in the DOM, so a change of tens of
 * thousands of files stays quick to draw, scroll and tick.
 *
 * The list is as tall as all its rows, and each drawn row is placed at its own top (`tops`)
 * with the height the list was given for it, so those heights have one source. The element
 * with `follow` holds the rows: each row carries its index as data-row, and the control the
 * arrow keys move between is marked data-file-row.
 */
export class ListWindow<F> {
  #rows: () => FileRow<F>[]
  #list: HTMLElement | undefined
  #scroller: HTMLElement | undefined
  /** How far the list's top is above the top of its scroller's view, and how tall that view
   * is. A list shares its scroller with what's above it, so it isn't the scroller's scrollTop. */
  #view = $state.raw({ top: 0, height: 800 })
  /** The row with focus, by key. It stays drawn after it scrolls out of view: taking it out
   * of the DOM would drop the focus, and the arrow keys with it. */
  #kept = $state('')
  /** Where that row is now, or -1. Apart from the range, so that scrolling doesn't search
   * every row for it. */
  #keptAt: number
  /** Where each row starts. One more than there are rows: the last is the list's height. */
  readonly tops: number[]
  /** The indexes of the rows to draw. */
  readonly drawn: number[]

  constructor(rows: () => FileRow<F>[], heightOf: (row: FileRow<F>) => number) {
    this.#rows = rows
    this.tops = $derived(rowTops(rows(), heightOf))
    this.#keptAt = $derived(this.#kept ? rows().findIndex((r) => r.key === this.#kept) : -1)
    this.drawn = $derived(drawnRows(visibleRows(this.tops, this.#view.top, this.#view.height), this.#keptAt))
    // New rows often change what's above the list too, such as its filter appearing.
    $effect(() => {
      void rows()
      this.measure()
    })
  }

  get height(): number {
    return this.tops[this.tops.length - 1]
  }

  /** Reads where the list is in its scroller. Scrolling and resizing do this themselves; call
   * it when something else moves the list. */
  measure = () => {
    const list = this.#list
    const scroller = this.#scroller
    if (!list || !scroller) return
    const top = scroller.getBoundingClientRect().top + scroller.clientTop - list.getBoundingClientRect().top
    const height = scroller.clientHeight
    const view = untrack(() => this.#view)
    if (top !== view.top || height !== view.height) this.#view = { top, height }
  }

  /** An action for the element that holds the rows. */
  follow = (list: HTMLElement) => {
    const scroller = scrollerOf(list)
    this.#list = list
    this.#scroller = scroller
    const resized = new ResizeObserver(this.measure)
    resized.observe(scroller)
    scroller.addEventListener('scroll', this.measure, { passive: true })
    list.addEventListener('keydown', this.#key)
    list.addEventListener('focusin', this.#focusIn)
    list.addEventListener('focusout', this.#focusOut)
    this.measure()
    return {
      destroy: () => {
        resized.disconnect()
        scroller.removeEventListener('scroll', this.measure)
        // The list may already be drawn again somewhere else, and following that.
        if (this.#list === list) this.#list = this.#scroller = undefined
      },
    }
  }

  #index(el: EventTarget | null): number {
    return Number((el as HTMLElement | null)?.closest<HTMLElement>('[data-row]')?.dataset.row ?? -1)
  }

  #focusIn = (e: FocusEvent) => {
    this.#kept = this.#rows()[this.#index(e.target)]?.key ?? ''
  }

  // Focus that leaves the page altogether comes back to the same row, so that one is kept.
  #focusOut = (e: FocusEvent) => {
    if (e.relatedTarget && !this.#list?.contains(e.relatedTarget as Node)) this.#kept = ''
  }

  /** Arrow keys, Home and End move between the file rows, in the DOM or not. */
  #key = (e: KeyboardEvent) => {
    const target = e.target as HTMLElement
    if (e.altKey || !target.matches('[data-file-row]')) return
    const to = moveTarget(e.key, this.#rows(), this.#index(target), (r) => r.type === 'file')
    if (to === null) return
    e.preventDefault()
    void this.#focus(to)
  }

  async #focus(index: number) {
    const row = this.#rows()[index]
    if (!row) return
    // Kept first, which draws the row; focusing it then scrolls it into view.
    this.#kept = row.key
    await tick()
    this.#list?.querySelector<HTMLElement>(`[data-row="${index}"] [data-file-row]`)?.focus()
  }
}
