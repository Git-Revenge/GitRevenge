import { describe, expect, it } from 'vitest'
import type { links } from '../../../wailsjs/go/models'
import { expand, makeLinker, patternProblem, webOnly } from './links'

const rule = (o: Partial<links.Rule>): links.Rule => ({ name: 'R', pattern: 'x', url: 'https://x/{0}', in: ['message', 'trailers', 'branches'], shipped: false, enabled: true, scope: 'global', ...o })
const jira = rule({ name: 'Jira', pattern: '\\b(ABC|OPS)-\\d+\\b', url: 'https://acme.atlassian.net/browse/{0}' })

describe('expand', () => {
  it('escapes each value like Go does', () => {
    expect(expand('https://x/{1}?q={0}', ['a b/c', 'é'])).toBe('https://x/%C3%A9?q=a%20b%2Fc')
    expect(expand('https://x/{3}', ['a'])).toBe('https://x/{3}')
  })
})

describe('makeLinker', () => {
  it('links web addresses by default, without trailing punctuation', () => {
    expect(webOnly.parts('see https://example.com/a?b=1. or www.example.org', 'message')).toEqual([
      { text: 'see ' },
      { text: 'https://example.com/a?b=1', href: 'https://example.com/a?b=1', rule: '' },
      { text: '. or ' },
      { text: 'www.example.org', href: 'https://www.example.org', rule: '' },
    ])
  })

  it('leaves a bare #123 alone unless a rule says what it means', () => {
    expect(webOnly.parts('#123', 'trailers')).toEqual([{ text: '#123' }])
    const l = makeLinker({ web: true, rules: [rule({ name: 'Num', pattern: '#(\\d+)\\b', url: 'https://t.example/i/{1}' })] })
    expect(l.find('#123', 'trailers')).toEqual([{ text: '#123', href: 'https://t.example/i/123', rule: 'Num' }])
  })

  it('takes the earliest match, then the earlier rule, and respects places', () => {
    const wide = rule({ name: 'Keys', pattern: '\\b[A-Z]+-\\d+\\b', url: 'https://k.example/{0}' })
    const branchOnly = rule({ name: 'B', pattern: 'XYZ-\\d+', url: 'https://b.example/{0}', in: ['branches'] })
    const l = makeLinker({ web: true, rules: [jira, wide, branchOnly] })
    expect(l.find('Fix ABC-12 and XYZ-9', 'message')).toEqual([
      { text: 'ABC-12', href: 'https://acme.atlassian.net/browse/ABC-12', rule: 'Jira' },
      { text: 'XYZ-9', href: 'https://k.example/XYZ-9', rule: 'Keys' },
    ])
    expect(makeLinker({ web: true, rules: [branchOnly] }).find('XYZ-9', 'message')).toEqual([])
    expect(makeLinker({ web: true, rules: [branchOnly] }).find('feature/XYZ-9-x', 'branches')[0].text).toBe('XYZ-9')
  })

  it('lets a web address win over a key inside it', () => {
    const l = makeLinker({ web: true, rules: [jira] })
    expect(l.find('https://wiki.example/ABC-1', 'message')).toEqual([{ text: 'https://wiki.example/ABC-1', href: 'https://wiki.example/ABC-1', rule: '' }])
  })

  it('skips switched-off, replaced and broken rules, and web addresses when off', () => {
    const off = makeLinker({
      web: false,
      rules: [rule({ ...jira, enabled: false }), rule({ ...jira, shadowed: true }), rule({ ...jira, problem: 'bad' }), rule({ name: 'Bad', pattern: '(' })],
    })
    expect(off.any).toBe(false)
    expect(off.find('ABC-1 https://x.example', 'message')).toEqual([])
  })

  it('never links to anything but http and https', () => {
    const l = makeLinker({ web: false, rules: [rule({ pattern: 'run-(\\w+)', url: 'https://x/{1}' }), rule({ name: 'Evil', pattern: 'evil', url: 'javascript:alert(1)' })] })
    expect(l.find('evil run-me', 'message')).toEqual([{ text: 'run-me', href: 'https://x/me', rule: 'R' }])
  })
})

describe('patternProblem', () => {
  it('flags broken and empty-matching patterns', () => {
    expect(patternProblem('\\bABC-\\d+')).toBe('')
    expect(patternProblem('(')).toMatch(/regular expression/)
    expect(patternProblem('x*')).toMatch(/empty text/)
  })
})
