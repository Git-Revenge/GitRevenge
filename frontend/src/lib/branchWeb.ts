/** What a branch's menu needs beyond the ref itself (#922): its pages on the forge, how far a
 * remote branch is from its remote's default branch, and which local branches track it. It's
 * read from the backend as the menu opens, so the menus themselves stay plain functions. */

import { BranchWebLinks, PushPlan, Reflog, RemoteBranchStatus, Remotes } from '../../wailsjs/go/app/App.js'
import type { app, git } from '../../wailsjs/go/models'
import type { Ref } from './graph/rowCache'
import { repoReads } from './repoReads'

export interface RefExtras {
  /** The remote branch these are about: the ref itself, or a local branch's upstream. */
  remote: string
  branch: string
  /** Its pages, when its remote has a web address. */
  web?: app.BranchWeb
  /** The host the pages are on, for the group's heading. */
  host: string
  /** A remote branch's distance from its remote's default branch. */
  status?: app.RemoteBranchStatus
  /** Local branches whose upstream is this remote branch. */
  trackedBy: string[]
  /** Whether a local branch of the same name is there already. */
  localExists: boolean
  /** Every remote, for Always push … to ▸. */
  remotes: string[]
  /** Where Push sends a local branch, by git's rules and the Pushing settings. */
  plan?: git.PushPlan
  /** A local branch's reflog, newest first, for Compare with an earlier version ▸ (#1040). */
  reflog?: git.ReflogEntry[]
}

/** Splits a remote branch's short name at its remote, the longest remote name first. */
export function splitRemote(short: string, remotes: string[]): { remote: string; branch: string } {
  const remote = [...remotes].sort((a, b) => b.length - a.length).find((r) => short.startsWith(`${r}/`)) ?? short.slice(0, short.indexOf('/'))
  return { remote, branch: short.slice(remote.length + 1) }
}

/** The remote branch a ref's web items are about, or null: a remote branch is its own, a
 * local branch has its upstream's. */
export function remoteBranchOf(ref: Ref): string | null {
  if (ref.kind === 'remote') return ref.short
  if (ref.kind === 'local' && ref.upstream && !ref.gone) return ref.upstream
  return null
}

const hostOf = (u: string) => {
  try {
    return new URL(u).host
  } catch {
    return ''
  }
}

/** Reads the extras for some refs, by short name. A ref with nothing to add is left out, and
 * a read that fails leaves its ref out rather than holding up the menu. */
export async function loadRefExtras(path: string, refs: Ref[]): Promise<Map<string, RefExtras>> {
  const out = new Map<string, RefExtras>()
  // Plans read before a branch's extras exist, joined to them at the end.
  const pending = new Map<string, git.PushPlan>()
  if (!refs.some((r) => r.kind === 'local' || remoteBranchOf(r))) return out
  const wanted = refs.filter((r) => remoteBranchOf(r))
  const all = ((await repoReads(path).refs().catch(() => [])) ?? []) as Ref[]
  const remoteRefs = all.filter((r) => r.kind === 'remote').map((r) => r.short)
  const listed = await Remotes(path).catch(() => null)
  const remotes = listed ? listed.map((r) => r.name) : [...new Set([...remoteRefs, ...wanted.map((r) => remoteBranchOf(r)!)].map((s) => s.slice(0, s.indexOf('/'))))]
  // A local branch with no upstream still gets the remotes, for Always push … to ▸.
  for (const ref of refs) {
    if (ref.kind === 'local' && !remoteBranchOf(ref)) out.set(ref.short, { remote: '', branch: ref.short, host: '', trackedBy: [], localExists: true, remotes })
  }
  // And every local branch, where Push sends it, and its reflog.
  const logs = new Map<string, git.ReflogEntry[]>()
  await Promise.all(
    refs
      .filter((r) => r.kind === 'local')
      .map(async (r) => {
        const [plan, log] = await Promise.all([PushPlan(path, r.short).catch(() => null), Reflog(path, r.short).catch(() => null)])
        if (log) logs.set(r.short, log)
        if (!plan) return
        const e = out.get(r.short)
        if (e) e.plan = plan
        else pending.set(r.short, plan)
      }),
  )
  await Promise.all(
    wanted.map(async (ref) => {
      const short = remoteBranchOf(ref)!
      const { remote, branch } = splitRemote(short, remotes)
      const [web, status] = await Promise.all([
        BranchWebLinks(path, remote, branch).catch(() => null),
        ref.kind === 'remote' ? RemoteBranchStatus(path, remote, branch).catch(() => null) : Promise.resolve(null),
      ])
      out.set(ref.short, {
        remote,
        branch,
        web: web?.branch ? web : undefined,
        host: web?.branch ? hostOf(web.branch) : '',
        status: status ?? undefined,
        trackedBy: all.filter((r) => r.kind === 'local' && r.upstream === short).map((r) => r.short),
        localExists: all.some((r) => r.kind === 'local' && r.short === branch),
        remotes,
      })
    }),
  )
  for (const [short, plan] of pending) {
    const e = out.get(short)
    if (e) e.plan = plan
  }
  for (const [short, log] of logs) {
    const e = out.get(short)
    if (e) e.reflog = log
  }
  return out
}

/** The heading's second line for a remote branch: how far from the default branch, and who
 * tracks it. */
export function remoteBranchHint(short: string, e: RefExtras | undefined): string {
  if (!e) return ''
  const parts: string[] = []
  const s = e.status
  if (s?.base && s.base !== e.branch) {
    const base = `${e.remote}/${s.base}`
    if (!s.ahead && !s.behind) parts.push(`Even with ${base}`)
    else parts.push([s.ahead ? `${s.ahead} ahead` : '', s.behind ? `${s.behind} behind` : ''].filter(Boolean).join(', ') + ` ${base}`)
  } else if (s?.base === e.branch) {
    parts.push(`${e.remote}'s default branch`)
  }
  if (e.trackedBy.length) parts.push(`Tracked by ${e.trackedBy.join(', ')}`)
  else if (short) parts.push('No local branch tracks it')
  return parts.join(' · ')
}
