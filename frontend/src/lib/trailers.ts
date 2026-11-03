/** A trailer value read as a person: "Name <email>", as in Co-authored-by or Signed-off-by. */
export interface Person {
  name: string
  email: string
}

const PERSON = /^\s*(.+?)\s*<([^<>\s]+@[^<>\s]+)>\s*$/

export function asPerson(value: string): Person | null {
  const m = PERSON.exec(value)
  return m ? { name: m[1], email: m[2] } : null
}

/** A run of a trailer value: plain text, or a link (an issue reference or a URL). */
export interface Part {
  text: string
  href?: string
}

const TOKEN = /(https?:\/\/[^\s<>]+[^\s<>.,;:!?)\]'"])|(#(\d+))/g

/**
 * Splits a trailer value into text and links. URLs link to themselves; "#123" links to the
 * issue when issueURL (a pattern with "{n}") is known, and stays text otherwise.
 */
export function valueParts(value: string, issueURL: string): Part[] {
  const parts: Part[] = []
  let at = 0
  for (const m of value.matchAll(TOKEN)) {
    if (m.index > at) parts.push({ text: value.slice(at, m.index) })
    if (m[1]) parts.push({ text: m[1], href: m[1] })
    else parts.push(issueURL ? { text: m[2], href: issueURL.replace('{n}', m[3]) } : { text: m[2] })
    at = m.index + m[0].length
  }
  if (at < value.length) parts.push({ text: value.slice(at) })
  return parts
}

/** A trailer as git stores it (git.Trailer in Go). */
export interface TrailerLine {
  key: string
  value: string
}

/** One kind of trailer in a section: its label, and its values in the message's order. */
export interface TrailerRow {
  key: string
  label: string
  values: string[]
}

export interface TrailerGroups {
  /** What the commit points at: Fixes, Closes, Link and the like, plus a stable backport. */
  related: TrailerRow[]
  /** Who was involved: reporters and reviewers first, sign-offs at the end. */
  people: TrailerRow[]
  /** Everything else, in the message's order. */
  other: TrailerRow[]
  /** Everyone copied in: usually the longest list, and the least urgent. */
  cc: string[]
}

const RELATED: Record<string, string> = {
  fixes: 'Fixes',
  closes: 'Closes',
  close: 'Closes',
  closed: 'Closes',
  fix: 'Fixes',
  fixed: 'Fixes',
  resolves: 'Resolves',
  refs: 'Refs',
  references: 'Refs',
  ref: 'Refs',
  link: 'Link',
  'see-also': 'See also',
}

const PEOPLE: Record<string, string> = {
  'reported-by': 'Reported by',
  'requested-by': 'Requested by',
  'suggested-by': 'Suggested by',
  'co-authored-by': 'Co-authored by',
  'co-developed-by': 'Co-developed by',
  'reviewed-by': 'Reviewed by',
  'acked-by': 'Acked by',
  'tested-by': 'Tested by',
  'signed-off-by': 'Signed off by',
}

/** A Cc to the kernel's stable list, optionally with versions: "<stable@…> # 7.3.x". */
const STABLE = /^\s*(?:.*<)?stable@[^>\s]+>?\s*(?:#\s*(.+?))?\s*$/i

/**
 * Sorts a commit's trailers by what they're for, since the order in a message is about who
 * added what when. Rows follow a fixed order, so commits read alike, but a row's values keep
 * the message's order: sign-offs in particular record a chain of custody.
 */
export function groupTrailers(trailers: TrailerLine[]): TrailerGroups {
  const groups: TrailerGroups = { related: [], people: [], other: [], cc: [] }
  const add = (rows: TrailerRow[], key: string, label: string, value: string) => {
    const row = rows.find((r) => r.key === key)
    if (row) row.values.push(value)
    else rows.push({ key, label, values: [value] })
  }
  for (const t of trailers) {
    const key = t.key.toLowerCase()
    const stable = key === 'cc' ? STABLE.exec(t.value) : null
    if (stable) add(groups.related, 'stable', 'Backport', stable[1] ? `Stable ${stable[1]}` : 'Stable')
    else if (key === 'cc') groups.cc.push(t.value)
    else if (key in RELATED) add(groups.related, RELATED[key].toLowerCase(), RELATED[key], t.value)
    else if (key in PEOPLE) add(groups.people, key, PEOPLE[key], t.value)
    else add(groups.other, key, t.key, t.value)
  }
  const order = (table: Record<string, string>) => Object.values(table).filter((v, i, all) => all.indexOf(v) === i)
  const relatedOrder = [...order(RELATED), 'Backport']
  groups.related.sort((a, b) => relatedOrder.indexOf(a.label) - relatedOrder.indexOf(b.label))
  const peopleOrder = order(PEOPLE)
  groups.people.sort((a, b) => peopleOrder.indexOf(a.label) - peopleOrder.indexOf(b.label))
  return groups
}
