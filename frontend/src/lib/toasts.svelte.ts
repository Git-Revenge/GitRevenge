import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { Undo, UndoState } from '../../wailsjs/go/app/App.js'
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
EventsOn('notice', (e: { path: string; message: string }) => {
  toasts.show(e.message)
})

/** Cleanup on pull: say what was deleted, with Undo while it's still the next undo. */
EventsOn('branches:cleaned', (e: { path: string; branches: string[]; undoLabel: string; merged: boolean }) => {
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
