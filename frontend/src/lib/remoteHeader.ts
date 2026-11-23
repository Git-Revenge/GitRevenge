/** What a remote's folder header in the Remotes panel says (#919): where the remote lives,
 * with the forge's mark only where Open on the web works, how long since it was fetched, and
 * a tooltip with its addresses and the local branches that track it. */

import type { git } from '../../wailsjs/go/models'
import type { IconName } from './ui/Icon.svelte'
import { FORGE_NAMES, forgeMark, type ForgeMark } from './ui/forgeMarks'

/** Where one remote URL points, as RemotePlaces returns it (app.URLPlace in Go; Wails makes
 * no model for a struct that's only a map's value). forge is '' unless it opens on the web. */
export interface URLPlace {
  place: string
  host: string
  forge: string
  web: string
}

/** Older than this, the fetch age turns ochre: a stale mirror stops looking current. */
export const STALE_AFTER = 7 * 86400

/** How a place is drawn: a forge's mark, or one of the app's own icons. */
export interface PlaceLook {
  mark?: ForgeMark
  icon: IconName
  /** What the mark or icon means, for its tooltip and screen readers. */
  says: string
}

export interface RemoteHeader {
  /** The host chip's text, or '' for a folder on this machine. */
  host: string
  look: PlaceLook
  /** "4 min ago", or '' when the app hasn't seen it fetched. */
  age: string
  stale: boolean
  /** Where pushes go, when there's more than one place. */
  fan: PlaceLook[]
  tooltip: string
}

/** How a URL's place looks. A forge without a mark (Azure DevOps) keeps the cloud. */
export function placeLook(p: URLPlace | undefined): PlaceLook {
  if (!p || p.place === 'local') return { icon: 'folder', says: 'A folder on this machine' }
  if (p.forge) {
    const name = FORGE_NAMES[p.forge] ?? p.forge
    return { mark: forgeMark(p.forge), icon: 'remote', says: name }
  }
  if (p.place === 'server') return { icon: 'server', says: 'SSH only, no web address' }
  return { icon: 'remote', says: "A host we don't recognise" }
}

/** "just now", "4 min ago", "3 hours ago", "2 days ago", "3 weeks ago", "5 months ago",
 * "2 years ago": rounded down, so a week turns into weeks only when it's a whole one. */
export function fetchAge(at: number, now: number): string {
  if (!at) return ''
  const s = Math.max(0, now - at)
  const plural = (n: number, unit: string) => `${n} ${unit}${n === 1 ? '' : 's'} ago`
  if (s < 60) return 'just now'
  if (s < 3600) return `${Math.floor(s / 60)} min ago`
  if (s < 86400) return plural(Math.floor(s / 3600), 'hour')
  if (s < 14 * 86400) return plural(Math.floor(s / 86400), 'day')
  if (s < 60 * 86400) return plural(Math.floor(s / (7 * 86400)), 'week')
  if (s < 365 * 86400) return plural(Math.floor(s / (30 * 86400)), 'month')
  return plural(Math.floor(s / (365 * 86400)), 'year')
}

/** A push destination's name in the fan's tooltip: its forge, or its host. */
function pushName(p: URLPlace | undefined, url: string): string {
  if (p?.forge) return FORGE_NAMES[p.forge] ?? p.forge
  return p?.host || url
}

/** The header for one remote. branches is how many of its branches the panel holds, and
 * trackers the local branches whose upstream is on it. */
export function remoteHeader(r: git.Remote, places: Record<string, URLPlace>, now: number, branches: number, trackers: string[]): RemoteHeader {
  const place = places[r.fetchUrl]
  const look = placeLook(place)
  const age = fetchAge(r.fetchedAt, now)
  const stale = !!r.fetchedAt && now - r.fetchedAt > STALE_AFTER
  const pushUrls = r.pushUrls?.length ? r.pushUrls : [r.pushUrl || r.fetchUrl]
  const fan = pushUrls.length > 1 ? pushUrls.map((u) => placeLook(places[u])) : []

  const lines = [`${r.name} · ${look.says}`, `Fetch  ${r.fetchUrl}`]
  if (pushUrls.length === 1) {
    lines.push(pushUrls[0] === r.fetchUrl ? 'Push   same as fetch' : `Push   ${pushUrls[0]}`)
  } else {
    lines.push(`Push to ${r.name}: ${pushUrls.map((u) => pushName(places[u], u)).join(', ')}`)
    for (const u of pushUrls) lines.push(`       ${u}`)
  }
  lines.push(age ? `Fetched ${age}` : "Git Revenge hasn't seen it fetched")
  lines.push(`${branches.toLocaleString()} ${branches === 1 ? 'branch' : 'branches'}`)
  if (trackers.length) {
    const shown = trackers.slice(0, 5).join(', ')
    lines.push(`Tracked by ${shown}${trackers.length > 5 ? ` and ${trackers.length - 5} more` : ''}`)
  }
  return { host: place?.place === 'local' ? '' : (place?.host ?? ''), look, age, stale, fan, tooltip: lines.join('\n') }
}

/** The local branches tracking each remote, by remote name. A branch's upstream is
 * "remote/branch"; with remote names that contain slashes the longest match wins. */
export function trackersByRemote(refs: { kind: string; short: string; upstream: string }[], remotes: string[]): Record<string, string[]> {
  const out: Record<string, string[]> = {}
  const byLength = [...remotes].sort((a, b) => b.length - a.length)
  for (const ref of refs) {
    if (ref.kind !== 'local' || !ref.upstream) continue
    const remote = byLength.find((n) => ref.upstream.startsWith(`${n}/`))
    if (remote) (out[remote] ??= []).push(ref.short)
  }
  return out
}
