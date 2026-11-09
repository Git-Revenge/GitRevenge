/** Building ignore rules for the working tree's menus and the Ignore dialog. Patterns here
 * are written relative to the top of the repository; patternIn rewrites one for a folder's
 * own .gitignore. Paths use forward slashes, as git status gives them. */

import { fileFolder, fileName } from './fileList'

/** Where a rule is written: the scopes git.IgnoreTarget takes. */
export type IgnoreScope = 'root' | 'dir' | 'exclude' | 'global'

export interface IgnoreTarget {
  scope: IgnoreScope
  /** The folder, for 'dir'. */
  dir?: string
}

/** Escapes wildcards, backslashes and trailing spaces, so the text matches only itself.
 * A leading # or ! is escaped too, for a pattern that isn't anchored. */
export function escapeName(s: string): string {
  const trimmed = s.replace(/ +$/, '')
  let out = trimmed.replace(/[\\*?[]/g, (c) => `\\${c}`)
  if (out.startsWith('#') || out.startsWith('!')) out = `\\${out}`
  return out + '\\ '.repeat(s.length - trimmed.length)
}

/** A pattern matching exactly this path from the top of the repository. Matches
 * EscapeIgnorePath in Go. */
export function anchored(path: string): string {
  const trimmed = path.replace(/ +$/, '')
  return `/${trimmed.replace(/[\\*?[]/g, (c) => `\\${c}`)}${'\\ '.repeat(path.length - trimmed.length)}`
}

/** The extension that "every .x file" would use, or '' for none. Dotfiles like .env have
 * none: their whole name is the name. */
export function extension(path: string): string {
  const name = fileName(path)
  const dot = name.lastIndexOf('.')
  return dot > 0 && dot < name.length - 1 ? name.slice(dot + 1) : ''
}

/** The folders above a path, nearest first. */
export function folders(path: string): string[] {
  const out: string[] = []
  for (let d = fileFolder(path); d; d = fileFolder(d)) out.push(d)
  return out
}

export interface IgnoreChoice {
  /** Says what the rule covers, in words. */
  label: string
  /** Relative to the top of the repository. */
  pattern: string
}

/** What the Ignore dialog offers for a file: the file, its name anywhere, its extension,
 * and up to two folders above it. */
export function fileChoices(path: string): IgnoreChoice[] {
  const name = fileName(path)
  const ext = extension(path)
  const out: IgnoreChoice[] = [{ label: 'This file', pattern: anchored(path) }]
  if (path.includes('/')) out.push({ label: `Any file named ${name}`, pattern: escapeName(name) })
  if (ext) out.push({ label: `Every .${ext} file`, pattern: `*.${escapeName(ext)}` })
  for (const d of folders(path).slice(0, 2)) out.push({ label: `The folder ${fileName(d)}/`, pattern: `${anchored(d)}/` })
  return out
}

/** What the Ignore dialog offers for a folder heading: the folder, its parents, and any
 * folder with its name. */
export function folderChoices(dir: string): IgnoreChoice[] {
  const out: IgnoreChoice[] = [{ label: `The folder ${fileName(dir)}/`, pattern: `${anchored(dir)}/` }]
  for (const d of folders(dir).slice(0, 2)) out.push({ label: `The folder ${fileName(d)}/`, pattern: `${anchored(d)}/` })
  if (dir.includes('/')) out.push({ label: `Any folder named ${fileName(dir)}/`, pattern: `${escapeName(fileName(dir))}/` })
  return out
}

/** Rewrites a pattern for the given ignore file. A folder's .gitignore reads patterns
 * relative to that folder, so an anchored pattern must be under it; null when it isn't. */
export function patternIn(pattern: string, target: IgnoreTarget): string | null {
  if (target.scope !== 'dir' || !target.dir) return pattern
  if (!pattern.startsWith('/')) return pattern
  const prefix = `/${target.dir}/`
  return pattern.startsWith(prefix) && pattern.length > prefix.length ? `/${pattern.slice(prefix.length)}` : null
}

/** Names an ignore file in a sentence: "the .gitignore at the top". */
export function targetName(target: IgnoreTarget): string {
  switch (target.scope) {
    case 'root':
      return '.gitignore'
    case 'dir':
      return `${target.dir}/.gitignore`
    case 'exclude':
      return '.git/info/exclude'
    case 'global':
      return 'your global excludes file'
  }
}

/** Names a rule's file as check-ignore gives it: short for the ones in this repository. */
export function sourceName(source: string, global?: string): string {
  const s = source.replace(/\\/g, '/')
  if (global && s.toLowerCase() === global.replace(/\\/g, '/').toLowerCase()) return 'your global excludes file'
  return s.endsWith('/info/exclude') && !s.startsWith('.git/') ? '.git/info/exclude' : s
}

export const plural = (n: number, what: string, many = `${what}s`) => `${n.toLocaleString()} ${n === 1 ? what : many}`
