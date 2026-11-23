import { onBackend } from './events'
import { PushToURLs, Undo, UndoState } from '../../wailsjs/go/app/App.js'
import { pushSummary } from './pushResult'
import { session } from './session.svelte'

/** A short message in the corner, with at most one action, such as Undo. */
export interface Toast {
  id: number
  message: string
  action?: { label: string; run: () => void | Promise<void> }
}

/** How long a toast stays, in milliseconds. Hovering or focusing it holds it. */
export const TOAST_MS = 8000

class Toasts {
  list = $state<Toast[]>([])
  #next = 1

  show(message: string, action?: Toast['action']): number {
    const id = this.#next++
    this.list = [...this.list.slice(-2), { id, message, action }]
    return id
  }

  dismiss(id: number) {
    this.list = this.list.filter((t) => t.id !== id)
  }
}

export const toasts = new Toasts()

/** Something worth knowing that isn't an error in what the user asked for, such as notes
 * that couldn't sync. */
onBackend('notice', (e) => {
  toasts.show(e.message)
})

/** A push to several places: every destination, and pushing on to those it didn't reach. */
onBackend('push:result', (e) => {
  const { message, retry } = pushSummary(e.dests)
  toasts.show(
    message,
    retry.length
      ? {
          label: retry.length === 1 ? `Retry ${e.dests.find((d) => d.url === retry[0])?.name ?? ''}`.trim() : 'Push to the rest',
          run: () => void session.run(() => PushToURLs(e.path, e.remote, e.refspec, retry)),
        }
      : undefined,
  )
})

/** Cleanup on pull: say what was deleted, with Undo while it's still the next undo. */
onBackend('branches:cleaned', (e) => {
  const n = e.branches.length
  const names = n <= 3 ? e.branches.join(', ') : `${e.branches.slice(0, 2).join(', ')} and ${n - 2} more`
  const kind = e.merged ? 'merged branch' : 'branch'
  toasts.show(`Deleted ${n === 1 ? `a ${kind}` : `${n} ${kind}es`}: ${names}`, {
    label: 'Undo',
    run: async () => {
      const state = await session.run(() => UndoState(e.path))
      if (state?.undo !== e.undoLabel) {
        toasts.show('Something else happened since, so this can no longer be undone here. The toolbar Undo steps back one change at a time.')
        return
      }
      await session.run(() => Undo(e.path))
    },
  })
})
