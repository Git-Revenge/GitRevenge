import type { git } from '../../wailsjs/go/models'
import { patchFolder } from './patches'
import { sizeWords } from './thisClone'

// Save as an archive (#936): git archive's choices, the names they start with, and what the
// dialog says about what goes in.

export type ArchiveFormat = 'zip' | 'tar.gz' | 'tar'
export const FORMATS: ArchiveFormat[] = ['zip', 'tar.gz', 'tar']

/** What an archive is named for: the repository's folder and what's archived, as git's own
 * release tarballs are, "tidewater-v1.1.0". A branch's slashes, which a file name can't hold,
 * become dashes. */
export function archiveBase(path: string, label: string): string {
  const repo = path.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || 'archive'
  return `${repo}-${label}`.replace(/[\\/:*?"<>|]+/g, '-')
}

/** The file's name for a format: the base with its extension. */
export const archiveFile = (base: string, format: ArchiveFormat) => `${base}.${format}`

/** The line that says what goes in: how many files and how big, then what .gitattributes
 * changes. */
export function planSaid(plan: Pick<git.ArchivePlan, 'files' | 'bytes' | 'ignored' | 'subst'>): string {
  const files = `${plan.files.toLocaleString('en-US')} ${plan.files === 1 ? 'file' : 'files'}`
  const parts = [plan.bytes >= 0 && plan.files ? `${files}, ${sizeWords(plan.bytes)} before compressing` : files]
  if (plan.ignored) parts.push(`${plan.ignored.toLocaleString('en-US')} left out by export-ignore`)
  if (plan.subst) parts.push(`${plan.subst.toLocaleString('en-US')} with export-subst placeholders filled in`)
  return `${parts.join('; ')}.`
}

/** The git command the dialog's choices make. */
export function archiveCommand(o: { format: ArchiveFormat; prefix: string; sub: string; file: string; rev: string }): string {
  const prefix = o.prefix.trim() ? ` --prefix=${quote(folderOf(o.prefix))}` : ''
  const sub = o.sub.trim() ? ` -- ${quote(o.sub.trim().replace(/\\/g, '/').replace(/^\/+|\/+$/g, ''))}` : ''
  return `git archive --format=${o.format}${prefix} -o ${quote(o.file.trim())} ${o.rev}${sub}`
}

/** A prefix as git archive takes it here: a folder, so it ends in a slash. */
export function folderOf(prefix: string): string {
  const p = prefix.trim().replace(/\\/g, '/').replace(/^\/+/, '')
  return p && !p.endsWith('/') ? `${p}/` : p
}

const quote = (s: string) => (/[\s"'$`\\]/.test(s) ? `"${s.replace(/(["\\$`])/g, '\\$1')}"` : s)

const key = (path: string, what: string) => `gitrevenge.archive:${path}:${what}`

/** Where archives of this repository were saved last, else where patches were, else beside
 * the repository. */
export function archiveFolder(path: string, store: Pick<Storage, 'getItem'> | null): string {
  try {
    const saved = store?.getItem(key(path, 'folder'))
    if (saved) return saved
  } catch {
    // No storage: the default stands.
  }
  return patchFolder(path, store)
}

/** The format chosen last for this repository; zip, which every system opens, the first time. */
export function archiveFormat(path: string, store: Pick<Storage, 'getItem'> | null): ArchiveFormat {
  try {
    const saved = store?.getItem(key(path, 'format'))
    if (saved && (FORMATS as string[]).includes(saved)) return saved as ArchiveFormat
  } catch {
    // No storage: the default stands.
  }
  return 'zip'
}

export function rememberArchive(path: string, folder: string, format: ArchiveFormat, store: Pick<Storage, 'setItem'> | null) {
  try {
    store?.setItem(key(path, 'folder'), folder)
    store?.setItem(key(path, 'format'), format)
  } catch {
    // Nothing to remember with: next time starts from the defaults.
  }
}
