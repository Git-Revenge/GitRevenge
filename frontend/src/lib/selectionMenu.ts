import { Integrate } from '../../wailsjs/go/app/App.js'
import type { app, git } from '../../wailsjs/go/models'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime.js'
import type { MenuItem } from './ContextMenu.svelte'
import { errorText } from './errors'
import type { MenuContext } from './menus'
import { copyAsPatches, EDGE_REASON, type PatchStart } from './patches'
import { session } from './session.svelte'
import { toasts } from './toasts.svelte'

// What can be done with several commits selected in the graph (#1018). Items for one commit,
// such as checkout, a branch or a tag there, or a reset, aren't among them.

/** Above this many commits, an action asks before it runs. */
export const ASK_ABOVE = 100

/** "4 commits", with a thousands separator. */
export const commitCount = (n: number) => `${n.toLocaleString()} ${n === 1 ? 'commit' : 'commits'}`

/** Whether a line can be squashed (#1020): 'checking' while the backend is asked, then its
 * answer. */
export type SquashState = 'checking' | app.SquashOffer

/** Squash into one, for a line of commits on the checked-out branch: it opens the interactive
 * rebase editor with the line marked to squash. Anything else shows it off, with the reason. */
function squashItem(sel: app.Selection, ctx: MenuContext, cur: string, squash: SquashState | undefined): MenuItem {
  const item: MenuItem = { label: 'Squash into one…', icon: 'rebase' }
  const off = (hint: string): MenuItem => ({ ...item, disabled: true, hint })
  if (!sel.line) return off(`Only for one line on ${cur}.`)
  if (!sel.base) return off(sel.baseEdge ? 'It starts where this shallow clone’s history stops.' : 'It starts at the history’s first commit.')
  if (!squash || squash === 'checking') return off('Checking…')
  if (!squash.ok) return off(squash.reason ?? `Only for one line on ${cur}.`)
  return { ...item, run: () => ctx.openRebase(sel.base, sel.ids) }
}

/** What can be done with the selection, as the menu and the panel both offer it. Commits go
 * oldest first, as history has them, whatever order they were clicked in; revert goes newest
 * first, so each undoes cleanly. */
export function selectionActions(sel: app.Selection, ctx: MenuContext, squash?: SquashState): MenuItem[] {
  const n = sel.ids.length
  const cur = ctx.current()
  const items: MenuItem[] = []
  if (ctx.openPatches) {
    // A line with no merges goes as a range, so any number fits; others are named one by one.
    const start: PatchStart = sel.line && sel.base && !sel.merges ? { kind: 'range', since: sel.base, tip: sel.ids[n - 1] } : { kind: 'ids', ids: sel.ids }
    const revs = (start.kind === 'range' ? { since: start.since, tip: start.tip, last: 0, ids: [] } : { since: '', tip: '', last: 0, ids: sel.ids }) as git.PatchRevs
    const off = sel.commits.some((c) => c.edge) ? { disabled: true, hint: EDGE_REASON } : {}
    items.push(
      { label: 'Copy as patches', icon: 'copy', ...off, run: () => copyAsPatches(ctx.path, revs, commitCount(n), ctx.openDialog) },
      { label: 'Save as patches…', icon: 'file', ...off, run: () => ctx.openPatches?.(start) },
    )
  }
  if (cur) {
    items.push({
      label: `Cherry-pick ${n.toLocaleString()} onto ${cur}`,
      icon: 'cherry-pick',
      disabled: sel.head,
      hint: sel.head ? `The checked-out commit is one of them.` : undefined,
      run: () => askIfMany(ctx, n, `Cherry-pick ${commitCount(n)} onto ${cur}`, 'Each is applied in turn, oldest first. A conflict stops it there, to resolve or skip.', 'Cherry-pick', () => Integrate(ctx.path, 'cherry-pick', sel.ids)),
    })
  }
  items.push({
    label: `Revert ${n.toLocaleString()}…`,
    icon: 'revert',
    disabled: sel.merges > 0,
    hint: sel.merges > 0 ? 'A merge is among them: revert merges one at a time, from their own menu.' : undefined,
    run: () =>
      ctx.openDialog({
        title: `Revert ${commitCount(n)}`,
        message: `Each gets a revert commit${cur ? ` on ${cur}` : ''}, newest first, so each undoes cleanly. A conflict stops it there, to resolve or skip.`,
        submit: 'Revert',
        run: () => attemptRun(() => Integrate(ctx.path, 'revert-all', [...sel.ids].reverse())),
      }),
  })
  if (cur) items.push(squashItem(sel, ctx, cur, squash))
  items.push({
    label: 'Copy their ids',
    icon: 'copy',
    separator: true,
    run: async () => {
      if ((await ClipboardSetText(sel.ids.join('\n')).catch(() => false)) !== false) toasts.show(`Copied the ids of ${commitCount(n)}, oldest first.`)
    },
  })
  return items
}

/** The menu for the selection: its count, then what can be done. Until its commits are read,
 * it says so. */
export function selectionMenu(count: number, sel: app.Selection | null, ctx: MenuContext, squash?: SquashState): MenuItem[] {
  const heading: MenuItem = { label: commitCount(count), heading: true, disabled: true, hint: sel ? undefined : 'Reading them…' }
  return sel ? [heading, ...selectionActions(sel, ctx, squash)] : [heading]
}

/** Runs it, asking first when it's for more than ASK_ABOVE commits. */
function askIfMany(ctx: MenuContext, n: number, title: string, message: string, submit: string, fn: () => Promise<unknown>) {
  if (n <= ASK_ABOVE) return void session.run(fn)
  ctx.openDialog({ title, message: `${message} That's a lot to do at once.`, submit, run: () => attemptRun(fn) })
}

/** Runs a backend call for a dialog: an error message keeps the dialog open. */
async function attemptRun(fn: () => Promise<unknown>): Promise<string | void> {
  try {
    await fn()
  } catch (e) {
    return errorText(e)
  }
}
