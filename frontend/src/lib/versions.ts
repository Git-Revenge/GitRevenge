import type { git } from '../../wailsjs/go/models'

// The words of two versions of a branch compared (#1038), kept apart from the panel so they
// can be tested as plain functions.

type Pair = Pick<git.RangePair, 'old' | 'new' | 'kind' | 'subject' | 'oldSubject' | 'into' | 'moved' | 'newId'>

export type Tone = 'changed' | 'around' | 'same' | 'gone' | 'new'

const count = (n: number, one: string, many: string) => `${n.toLocaleString()} ${n === 1 ? one : many}`
const ordinal = (n: number) => {
  const s = n % 100 >= 11 && n % 100 <= 13 ? 'th' : (['th', 'st', 'nd', 'rd'][n % 10] ?? 'th')
  return `${n}${s}`
}

/** A pair's kind as its pill says it, and the pill's tone. */
export function kindOf(p: Pair): { word: string; tone: Tone } {
  switch (p.kind) {
    case 'same':
      return { word: p.moved ? 'the same, moved' : 'the same', tone: 'same' }
    case 'changed':
      return { word: 'changed', tone: 'changed' }
    case 'around':
      return { word: 'changed only around it', tone: 'around' }
    case 'message':
      return { word: 'message changed', tone: 'changed' }
    case 'folded':
      return { word: `folded into ${p.into}`, tone: 'gone' }
    case 'dropped':
      return { word: 'dropped', tone: 'gone' }
    case 'new':
      return { word: 'new', tone: 'new' }
  }
  return { word: p.kind, tone: 'around' }
}

/** The line under a pair's subject: what's worth knowing beyond its kind. */
export function noteOf(p: Pair): string {
  const parts: string[] = []
  if (p.oldSubject) parts.push(`was “${p.oldSubject}”`)
  if (p.kind === 'dropped' || p.kind === 'folded') parts.push('only in the old version')
  else if (p.kind === 'new') parts.push('only in the new version')
  if (p.moved && p.old && p.new) parts.push(`was ${ordinal(p.old)}, now ${ordinal(p.new)}`)
  if (p.kind === 'around') parts.push('the same change; lines around it changed')
  return parts.join(' · ')
}

/** Both places, "4 → 3", with – for a side it isn't in. */
export const placesOf = (p: Pair) => `${p.old || '–'} → ${p.new || '–'}`

/** Whether Hide the same hides it. */
export const isSame = (p: Pair) => p.kind === 'same' || p.kind === 'around'

/** The line under the header: how many then and now, and what became of them. */
export function summaryOf(pairs: Pair[], oldCount: number, newCount: number): string {
  const by = (k: string) => pairs.filter((p) => p.kind === k).length
  const parts: string[] = []
  const changed = by('changed')
  if (changed) parts.push(`${changed} changed`)
  if (by('message')) parts.push(`${by('message')} with a new message`)
  if (by('around')) parts.push(`${by('around')} changed only around it`)
  if (by('same')) parts.push(`${by('same')} the same`)
  if (by('folded')) parts.push(`${by('folded')} folded in`)
  if (by('dropped')) parts.push(`${by('dropped')} dropped`)
  if (by('new')) parts.push(`${by('new')} new`)
  const head = `${count(oldCount, 'commit', 'commits')} then, ${newCount.toLocaleString()} now`
  return parts.length ? `${head}: ${parts.join(', ')}.` : `${head}.`
}

/** Merges each version leaves out, said once: "2 merges left out, as range-diff leaves them out." */
export function mergesNote(oldMerges: number, newMerges: number): string {
  const n = oldMerges + newMerges
  return n ? `${count(n, 'merge', 'merges')} left out, as range-diff leaves them out.` : ''
}

/** How long ago, for the header: "4 min ago". */
export function ago(seconds: number, now = Date.now() / 1000): string {
  const d = Math.max(0, now - seconds)
  if (d < 60) return 'just now'
  if (d < 3600) return `${Math.floor(d / 60)} min ago`
  if (d < 86400) return `${Math.floor(d / 3600)} h ago`
  const days = Math.floor(d / 86400)
  return days === 1 ? 'yesterday' : `${days} days ago`
}

/** A branch's earlier tip that a rewrite moved it away from (#1040). */
export interface EarlierTip {
  commit: string
  /** What moved it away, capitalized: "Before the rebase". */
  said: string
  /** When that happened, in seconds. */
  time: number
}

const REWRITES = new Set(['before the rebase', 'before the amend', 'before the reset'])

/** The branch's last few tips that a rebase, an amend or a reset moved it away from, newest
 * first, from its reflog (newest first too). Each is named and dated by the entry that moved
 * the branch away from it; the tip it's at now, and repeats, are left out. */
export function earlierTips(entries: { commit: string; action: string; time: number }[], most = 3): EarlierTip[] {
  const out: EarlierTip[] = []
  const now = entries[0]?.commit
  for (let i = 1; i < entries.length && out.length < most; i++) {
    const said = tipLabel(entries[i - 1].action)
    const commit = entries[i].commit
    if (!REWRITES.has(said) || commit === now || out.some((t) => t.commit === commit)) continue
    out.push({ commit, said: `${said[0].toUpperCase()}${said.slice(1)}`, time: entries[i - 1].time })
  }
  return out
}

/** A reflog entry's tip as a choice for the old version: what moved the branch from it. */
export function tipLabel(action: string): string {
  const a = action.toLowerCase()
  if (a.startsWith('rebase')) return 'before the rebase'
  if (a.startsWith('commit (amend)')) return 'before the amend'
  if (a.startsWith('reset')) return 'before the reset'
  if (a.startsWith('cherry-pick') || a.startsWith('pull')) return `before the ${a.split(/[ :]/)[0]}`
  if (a.startsWith('commit')) return 'before the next commit'
  return 'earlier'
}
