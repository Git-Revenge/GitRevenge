/** Saying what a push to several places did (#926): every destination, in words. Git goes
 * through a remote's push URLs in turn, carries on past one that's rejected, and stops at one
 * it can't reach, so the ones after that weren't tried. */

export interface PushDestView {
  url: string
  state: string
  detail?: string
  /** The host, or a folder's path. */
  name: string
}

/** The result in one message, and the URLs worth trying again: those that failed or weren't
 * tried. A rejection isn't worth retrying until the branch takes in the remote's work. */
export function pushSummary(dests: PushDestView[]): { message: string; retry: string[] } {
  const list = (ds: PushDestView[]) => {
    const names = ds.map((d) => d.name)
    return names.length <= 1 ? names.join('') : `${names.slice(0, -1).join(', ')} and ${names[names.length - 1]}`
  }
  const went = dests.filter((d) => d.state === 'pushed')
  const same = dests.filter((d) => d.state === 'up to date')
  const rejected = dests.filter((d) => d.state === 'rejected')
  const failed = dests.filter((d) => d.state === 'failed')
  const untried = dests.filter((d) => d.state === 'not tried')
  const parts: string[] = []
  if (went.length) parts.push(`Pushed to ${list(went)}.`)
  if (same.length) parts.push(`${list(same)} ${same.length === 1 ? 'was' : 'were'} already up to date.`)
  for (const d of rejected) parts.push(`${d.name} refused it${d.detail ? ` (${d.detail})` : ''}: it has work this branch doesn't. Pull first.`)
  for (const d of failed) parts.push(`${d.name} FAILED: ${d.detail || "couldn't be reached"}.`)
  if (untried.length) parts.push(`Not tried: ${list(untried)}, since git stops at the first place it can't reach.`)
  return { message: parts.join(' '), retry: [...failed, ...untried].map((d) => d.url) }
}
