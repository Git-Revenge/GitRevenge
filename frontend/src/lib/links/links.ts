/** Smart links: text in commit messages, trailers and branch names that a rule turns into a
 * link to another site. Matching mirrors internal/links in Go, which uses the same rules for
 * Insights › Shipped: rules in order, the earliest match wins, then the earlier rule. Web
 * addresses written out in full come first, when they're on. Links only: nothing here
 * contacts the linked site. */

import type { links } from '../../../wailsjs/go/models'

export type Place = 'message' | 'trailers' | 'branches'

export const PLACES: { id: Place; label: string }[] = [
  { id: 'message', label: 'Commit messages' },
  { id: 'trailers', label: 'Trailers' },
  { id: 'branches', label: 'Branch names' },
]

/** A run of text: plain, or a link. */
export interface LinkPart {
  text: string
  href?: string
  /** The rule's name, or "" for a web address. */
  rule?: string
}

export interface LinkMatch {
  text: string
  href: string
  rule: string
}

/** Web addresses: http(s)://… and www.…, without trailing punctuation. */
const WEB = /\b(?:https?:\/\/|www\.)[^\s<>]*[^\s<>.,;:!?)\]'"]/g

const SAFE = /^https?:\/\//i

/** Fills {0}…{9} with a match's groups, each escaped as a URL component. Matches Expand in Go. */
export function expand(template: string, groups: (string | undefined)[]): string {
  return template.replace(/\{([^}]*)\}/g, (m, n: string) => {
    const i = /^\d+$/.test(n) ? Number(n) : -1
    return i >= 0 && i < groups.length ? encodeURIComponent(groups[i] ?? '') : m
  })
}

/** Why a pattern can't be used in the app's own matching, or ''. Go checks its side. */
export function patternProblem(pattern: string): string {
  try {
    const re = new RegExp(pattern, 'g')
    if (re.test('')) return 'The pattern matches empty text, so it would match everywhere.'
    return ''
  } catch (e) {
    return `The pattern isn't a regular expression this app can read: ${(e as Error).message}`
  }
}

interface Compiled {
  name: string
  re: RegExp
  url: string
  in: string[]
}

export interface Linker {
  /** Whether anything can link: web addresses or at least one rule. */
  any: boolean
  find(text: string, place: Place): LinkMatch[]
  parts(text: string, place: Place): LinkPart[]
}

/** Builds the matcher for a repository's rules; null config means web addresses only. */
export function makeLinker(cfg: Pick<links.Config, 'web' | 'rules'> | null): Linker {
  const web = cfg ? cfg.web : true
  const rules: Compiled[] = []
  for (const r of cfg?.rules ?? []) {
    if (!r.enabled || r.shadowed || r.problem) continue
    try {
      rules.push({ name: r.name, re: new RegExp(r.pattern, 'g'), url: r.url, in: r.in })
    } catch {
      // Go accepted it but JavaScript can't read it: no links from this rule.
    }
  }

  function hits(text: string, place: Place) {
    const out: { start: number; end: number; order: number; match: LinkMatch }[] = []
    if (web) {
      for (const m of text.matchAll(WEB)) {
        const href = m[0].startsWith('www.') ? `https://${m[0]}` : m[0]
        out.push({ start: m.index, end: m.index + m[0].length, order: -1, match: { text: m[0], href, rule: '' } })
      }
    }
    rules.forEach((r, order) => {
      if (!r.in.includes(place)) return
      for (const m of text.matchAll(r.re)) {
        if (!m[0]) continue
        const href = expand(r.url, [...m])
        if (SAFE.test(href)) out.push({ start: m.index, end: m.index + m[0].length, order, match: { text: m[0], href, rule: r.name } })
      }
    })
    out.sort((a, b) => a.start - b.start || a.order - b.order)
    const kept: typeof out = []
    let end = 0
    for (const h of out) {
      if (h.start < end) continue
      kept.push(h)
      end = h.end
    }
    return kept
  }

  return {
    any: web || rules.length > 0,
    find: (text, place) => hits(text, place).map((h) => h.match),
    parts(text, place) {
      const parts: LinkPart[] = []
      let at = 0
      for (const h of hits(text, place)) {
        if (h.start > at) parts.push({ text: text.slice(at, h.start) })
        parts.push({ text: h.match.text, href: h.match.href, rule: h.match.rule })
        at = h.end
      }
      if (at < text.length) parts.push({ text: text.slice(at) })
      return parts
    },
  }
}

/** The matcher with no rules: web addresses only. */
export const webOnly = makeLinker(null)

/** The tooltip for a link: what opens it and where. */
export const linkTitle = (p: LinkPart) => `${p.rule ? `${p.rule} · ` : ''}opens in your browser\n${p.href}`
