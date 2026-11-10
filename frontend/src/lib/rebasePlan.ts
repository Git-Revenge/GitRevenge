// The plan an interactive rebase is edited as (issue #596): what each action means, what the
// branch will look like afterwards, and the moves the editor makes. Plain functions over a
// plain list, oldest first, as git applies it.

export type Action = 'pick' | 'reword' | 'squash' | 'fixup' | 'drop'

export interface PlanItem {
  commit: string
  /** The commit's subject as it is now. */
  subject: string
  /** Its whole message as it is now. */
  original: string
  /** The message it will have: edited for a reword, and for a squash the combined message. */
  message: string
  action: Action
}

/** What each action is called, what it does, and the key that sets it. */
export const ACTIONS: { action: Action; label: string; does: string; key: string }[] = [
  { action: 'pick', label: 'Keep', does: 'as it is', key: 'P' },
  { action: 'reword', label: 'Reword', does: 'keep the changes, change the message', key: 'R' },
  { action: 'squash', label: 'Squash', does: 'fold into the one above, keep both messages', key: 'S' },
  { action: 'fixup', label: 'Fixup', does: 'fold into the one above, drop this message', key: 'F' },
  { action: 'drop', label: 'Drop', does: "throw this commit's changes away", key: 'D' },
]

export const actionLabel = (a: Action) => ACTIONS.find((x) => x.action === a)!.label
export const actionForKey = (key: string): Action | undefined => ACTIONS.find((x) => x.key === key.toUpperCase())?.action

const folds = (a: Action) => a === 'squash' || a === 'fixup'
const subjectOf = (message: string) => message.split('\n', 1)[0].trim()

/** A fresh plan from the backend's list: every commit kept. */
export function newPlan(commits: { commit: string; subject: string; message: string }[]): PlanItem[] {
  return commits.map((c) => ({ commit: c.commit, subject: c.subject, original: c.message, message: c.message, action: 'pick' }))
}

/** The index of the commit a squash or fixup at i folds into: the nearest one above that is
 * kept or reworded. -1 when there's none. */
export function foldTarget(items: PlanItem[], i: number): number {
  for (let j = i - 1; j >= 0; j--) if (items[j].action === 'pick' || items[j].action === 'reword') return j
  return -1
}

/** Sets what happens to the commit at i, starting its message where git would. */
export function setAction(items: PlanItem[], i: number, action: Action): PlanItem[] {
  const out = items.map((it) => ({ ...it }))
  const it = out[i]
  if (!it || it.action === action) return out
  it.action = action
  const into = foldTarget(out, i)
  // A squash starts from both messages, as git's does; anything else from the commit's own.
  it.message = action === 'squash' && into >= 0 ? `${out[into].message.trim()}\n\n${it.original.trim()}` : it.original
  return out
}

/** Moves the commit at from to sit at to. */
export function move(items: PlanItem[], from: number, to: number): PlanItem[] {
  if (from === to || from < 0 || to < 0 || from >= items.length || to >= items.length) return items
  const out = [...items]
  const [it] = out.splice(from, 1)
  out.splice(to, 0, it)
  return out
}

/** The subject a "fixup! x" or "squash! x" commit names, and which of the two it is. */
export function autosquashOf(subject: string): { action: 'fixup' | 'squash'; target: string } | null {
  let rest = subject
  let action: 'fixup' | 'squash' | null = null
  for (;;) {
    const m = /^(fixup|squash|amend)! (.+)$/.exec(rest)
    if (!m) break
    action ??= m[1] === 'squash' ? 'squash' : 'fixup'
    rest = m[2]
  }
  return action ? { action, target: rest } : null
}

/** How many commits are a fixup! or squash! for another commit above them in the list. */
export function foldable(items: PlanItem[]): number {
  return items.filter((it, i) => !folds(it.action) && it.action !== 'drop' && autosquashTarget(items, i) >= 0).length
}

function autosquashTarget(items: PlanItem[], i: number): number {
  const named = autosquashOf(items[i].subject)
  if (!named) return -1
  for (let j = i - 1; j >= 0; j--) {
    if (autosquashOf(items[j].subject)) continue
    if (items[j].subject === named.target || items[j].commit.startsWith(named.target)) return j
  }
  return -1
}

/** Moves every fixup! and squash! commit to sit under the commit it names, set to fold into
 * it, as `git rebase --autosquash` would. */
export function foldIn(items: PlanItem[]): PlanItem[] {
  let out = items.map((it) => ({ ...it }))
  for (const commit of items.map((it) => it.commit)) {
    const i = out.findIndex((it) => it.commit === commit)
    if (folds(out[i].action) || out[i].action === 'drop') continue
    const named = autosquashOf(out[i].subject)
    const target = autosquashTarget(out, i)
    if (!named || target < 0) continue
    // After the target and anything already folded into it.
    let at = target + 1
    while (at < i && folds(out[at].action)) at++
    out = move(out, i, at)
    out = setAction(out, at, named.action)
  }
  return out
}

/** A commit the branch will have afterwards. */
export interface Outcome {
  commit: string
  subject: string
  /** How many commits it's made of: more than one when others fold into it. */
  made: number
  /** Its subject before, when it was reworded or a squash changed it. */
  was: string
}

/** What the branch will look like: the commits that will exist, oldest first, and those dropped. */
export function outcome(items: PlanItem[]): { commits: Outcome[]; dropped: PlanItem[] } {
  const commits: Outcome[] = []
  const dropped: PlanItem[] = []
  for (const it of items) {
    if (it.action === 'drop') dropped.push(it)
    else if (folds(it.action) && commits.length) {
      const into = commits[commits.length - 1]
      into.made++
      // A squash's message is the folded commit's new message.
      if (it.action === 'squash') {
        const subject = subjectOf(it.message)
        if (subject && subject !== into.subject) {
          into.was ||= into.subject
          into.subject = subject
        }
      }
    } else {
      const subject = it.action === 'reword' ? subjectOf(it.message) || it.subject : it.subject
      commits.push({ commit: it.commit, subject, made: 1, was: subject !== it.subject ? it.subject : '' })
    }
  }
  return { commits, dropped }
}

/** Why the plan can't run as it is, or "". */
export function problem(items: PlanItem[]): string {
  if (items.length && items.every((it) => it.action === 'drop')) return 'Every commit is dropped. Use Reset instead.'
  for (let i = 0; i < items.length; i++) {
    const it = items[i]
    if (folds(it.action) && foldTarget(items, i) < 0) return `${it.subject} has nothing above it to fold into. Move it down, or keep it.`
    if ((it.action === 'reword' || it.action === 'squash') && !it.message.trim()) return `${it.subject} needs a message.`
  }
  return ''
}

/** Whether anything differs from a fresh plan of the same commits. */
export function changed(items: PlanItem[], original: string[]): boolean {
  return items.some((it, i) => it.action !== 'pick' || it.commit !== original[i])
}

/** The plan as the backend runs it. */
export function toRun(items: PlanItem[]): { commit: string; subject: string; message: string; action: string }[] {
  return items.map((it) => ({ commit: it.commit, subject: it.subject, message: it.message, action: it.action }))
}
