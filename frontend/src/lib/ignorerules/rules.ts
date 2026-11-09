/** Reading and editing an ignore file as a list of lines, for the Ignore rules tab. Edits
 * work on the text, so the Rules and Text views always agree. */

import type { git } from '../../../wailsjs/go/models'

export type LineKind = 'blank' | 'comment' | 'rule' | 'negation'

export interface RuleLine {
  /** 1-based, as git numbers lines. */
  n: number
  text: string
  kind: LineKind
}

/** Splits text into lines. A final newline doesn't make an extra empty line. */
export function splitLines(text: string): string[] {
  if (text === '') return []
  return text.replace(/\r\n/g, '\n').replace(/\n$/, '').split('\n')
}

export const joinLines = (lines: string[]) => (lines.length ? `${lines.join('\n')}\n` : '')

export function lineKind(text: string): LineKind {
  const t = text.replace(/[ \t]+$/, '')
  if (t === '') return 'blank'
  if (t.startsWith('#')) return 'comment'
  return t.startsWith('!') ? 'negation' : 'rule'
}

export const parseLines = (text: string): RuleLine[] => splitLines(text).map((t, i) => ({ n: i + 1, text: t, kind: lineKind(t) }))

/** Replaces line n (1-based). A value with newlines is cut at the first. */
export function setLine(text: string, n: number, value: string): string {
  const lines = splitLines(text)
  if (n < 1 || n > lines.length) return text
  lines[n - 1] = value.split(/\r?\n/)[0]
  return joinLines(lines)
}

export function deleteLine(text: string, n: number): string {
  const lines = splitLines(text)
  if (n < 1 || n > lines.length) return text
  lines.splice(n - 1, 1)
  return joinLines(lines)
}

/** Moves line n up (-1) or down (1). Order matters: a later rule wins over an earlier one. */
export function moveLine(text: string, n: number, d: -1 | 1): string {
  const lines = splitLines(text)
  const to = n - 1 + d
  if (n < 1 || n > lines.length || to < 0 || to >= lines.length) return text
  ;[lines[n - 1], lines[to]] = [lines[to], lines[n - 1]]
  return joinLines(lines)
}

export const appendLine = (text: string, value: string) => joinLines([...splitLines(text), value])

/** Each rule's counts in one file, by line number. */
export function statsFor(stats: git.IgnoreStats | null, source: string): Map<number, git.IgnoreRuleStats> {
  const m = new Map<number, git.IgnoreRuleStats>()
  for (const s of stats?.rules ?? []) if (s.source === source) m.set(s.line, s)
  return m
}

const count = (n: number, one: string, many = `${one}s`) => `${n.toLocaleString()} ${n === 1 ? one : many}`

/** What a rule does now, in words; '' for nothing to say. */
export function describe(line: RuleLine, s: git.IgnoreRuleStats | undefined, truncated: boolean): string {
  if (line.kind === 'blank' || line.kind === 'comment') return ''
  const more = truncated ? '+' : ''
  if (line.kind === 'negation') return s?.restores ? `brings back ${count(s.restores, 'file')}` : 'brings nothing back now'
  if (!s?.ignores && !s?.tracked) return 'ignores nothing now'
  const bits: string[] = []
  if (s.ignores) bits.push(`ignores ${count(s.ignores, 'path')}${more}`)
  if (s.tracked) bits.push(`${count(s.tracked, 'tracked file')} still tracked`)
  return bits.join(' · ')
}

/** Whether a source names the same file, however git spelled it. */
export function sameSource(a: string, b: string): boolean {
  const k = (s: string) => s.replace(/\\/g, '/').toLowerCase()
  return k(a) === k(b)
}
