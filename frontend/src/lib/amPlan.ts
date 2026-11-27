import type { git } from '../../wailsjs/go/models'

// A file of patches as a series to apply (#1022): which version, its cover letter, its patches
// in order, and what's left out.

type Message = Pick<git.MailMessage, 'subject' | 'number' | 'total' | 'version' | 'cover' | 'hasPatch'>

/** One version of a series: its cover letter's index, if it has one, and its patches' indexes
 * in the order to apply them. */
export interface Version {
  version: number
  cover: number
  patches: number[]
}

/** A file's series: its versions, oldest first, and how many messages hold no patch and no
 * cover letter (a thread's replies), which are left out. */
export interface Series {
  versions: Version[]
  replies: number
}

/** Sorts a file's messages into a series. Numbered patches go in [n/m] order; a version whose
 * patches aren't all numbered keeps the file's order. A reply to a patch, which quotes its
 * [PATCH n/m] but holds no patch, is a reply. */
export function seriesOf(messages: Message[]): Series {
  const by = new Map<number, Version>()
  let replies = 0
  messages.forEach((m, i) => {
    if (!m.hasPatch && !m.cover) return void replies++
    let v = by.get(m.version)
    if (!v) by.set(m.version, (v = { version: m.version, cover: -1, patches: [] }))
    if (m.cover) {
      if (v.cover < 0) v.cover = i
    } else v.patches.push(i)
  })
  const versions = [...by.values()].sort((a, b) => a.version - b.version)
  for (const v of versions) {
    if (v.patches.every((i) => messages[i].number > 0)) v.patches.sort((a, b) => messages[a].number - messages[b].number || a - b)
  }
  return { versions: versions.filter((v) => v.patches.length > 0), replies }
}

/** The placeholder format-patch leaves in a cover letter's subject. */
const PLACEHOLDER = '*** SUBJECT HERE ***'

/** What a version is called: its cover letter's subject, else its first patch's. */
export function titleOf(messages: Message[], v: Version): string {
  const cover = v.cover >= 0 ? messages[v.cover].subject.trim() : ''
  return cover && cover !== PLACEHOLDER ? cover : (messages[v.patches[0]]?.subject ?? '')
}

/** A branch's name from a series' title: "Wind speeds in knots" → "wind-speeds-in-knots". */
export function branchNameOf(title: string): string {
  const slug = title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48)
    .replace(/-+$/, '')
  return slug || 'patches'
}

const count = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

/** What a file holds, said: "7 messages: a cover letter, 5 patches, and 1 reply left out". */
export function fileSummary(messages: Message[], s: Series, v: Version | undefined): string {
  const parts: string[] = []
  if (v && v.cover >= 0) parts.push('a cover letter')
  if (v) parts.push(count(v.patches.length, 'patch', 'patches'))
  const others = s.versions.filter((x) => x.version !== v?.version).reduce((n, x) => n + x.patches.length + (x.cover >= 0 ? 1 : 0), 0)
  if (others) parts.push(`${count(others, 'message', 'messages')} of other versions`)
  if (s.replies) parts.push(`${count(s.replies, 'reply', 'replies')} left out`)
  const list = parts.length > 1 ? `${parts.slice(0, -1).join(', ')} and ${parts[parts.length - 1]}` : (parts[0] ?? 'nothing to apply')
  return `${count(messages.length, 'message', 'messages')}: ${list}`
}
