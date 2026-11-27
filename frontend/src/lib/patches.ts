import { CopyPatches, DownloadFiles, PatchSeries } from '../../wailsjs/go/app/App.js'
import type { git } from '../../wailsjs/go/models'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime.js'
import { errorText } from './errors'
import type { DialogSpec } from './Dialog.svelte'
import { session } from './session.svelte'
import { toasts } from './toasts.svelte'

// Saving commits as patches, and copying them (#1021).

/** Where the Save as patches dialog starts: a branch's commits since where it left its base,
 * a commit (up to here, alone at first), or commits picked in the graph. */
export type PatchStart =
  | { kind: 'branch'; branch: string; stackParent?: string }
  | { kind: 'commit'; tip: string; subject: string }
  | { kind: 'ids'; ids: string[] }
  | { kind: 'range'; since: string; tip: string }

/** Above this many commits, copying them asks first. */
export const COPY_ASK_ABOVE = 100

/** "1 file version", "12 file versions". */
export const versions = (n: number) => `${n.toLocaleString()} ${n === 1 ? 'file version' : 'file versions'}`

/** Why a commit can't be saved as a patch, or "": a shallow clone's edge commit has no parent
 * here, so its patch would add every file. */
export const EDGE_REASON = 'Its parent isn’t in this shallow clone, so its patch would add every file.'

const key = (path: string, what: string) => `gitrevenge.patches.${what}:${path}`

/** The folder patches were saved into last time in this repository; the first time, the
 * folder above the repository. */
export function patchFolder(path: string, store: Pick<Storage, 'getItem'> | null): string {
  try {
    const saved = store?.getItem(key(path, 'folder'))
    if (saved) return saved
  } catch {
    // No storage: the default stands.
  }
  const trimmed = path.replace(/[\\/]+$/, '')
  const cut = Math.max(trimmed.lastIndexOf('/'), trimmed.lastIndexOf('\\'))
  if (cut < 0) return trimmed
  const parent = trimmed.slice(0, cut)
  // A:\repo's parent is A:\, and /repo's is /.
  return parent === '' || /^[A-Za-z]:$/.test(parent) ? trimmed.slice(0, cut + 1) : parent
}

/** Whether Signed-off-by was ticked last time in this repository. */
export function patchSignoff(path: string, store: Pick<Storage, 'getItem'> | null): boolean {
  try {
    return store?.getItem(key(path, 'signoff')) === '1'
  } catch {
    return false
  }
}

/** Remembers the folder and the Signed-off-by choice for this repository. */
export function rememberPatchChoices(path: string, folder: string, signoff: boolean, store: Pick<Storage, 'setItem'> | null) {
  try {
    store?.setItem(key(path, 'folder'), folder)
    store?.setItem(key(path, 'signoff'), signoff ? '1' : '0')
  } catch {
    // Nothing to remember with: next time starts from the defaults.
  }
}

/** Copy as a patch, or patches: format-patch's text on the clipboard. In a partial clone, one
 * commit downloads what it needs by itself; more ask first, as Insights does (#1003). So does
 * copying more than COPY_ASK_ABOVE commits. */
export async function copyAsPatches(path: string, revs: git.PatchRevs, label: string, openDialog: (spec: DialogSpec) => void) {
  const series = await session.run(() => PatchSeries(path, revs))
  if (!series) return
  if (!series.count) return void toasts.show('There’s nothing to copy: merges are left out, as format-patch leaves them out.')
  if (series.edge) return void toasts.show(EDGE_REASON)
  const copy = async () => {
    if (series.missing?.length) await DownloadFiles(path, `copy-${Date.now()}`, series.missing)
    const text = await CopyPatches(path, revs, { prefix: '', version: 0, numbered: true, cover: false, coverSubject: '', coverText: '', signoff: false, base: '' } as git.PatchOptions)
    if ((await ClipboardSetText(text).catch(() => false)) === false) throw new Error('The clipboard didn’t take it.')
    toasts.show(series.count === 1 ? `Copied the patch for ${label}.` : `Copied ${series.count.toLocaleString()} patches.`)
  }
  const many = series.count > COPY_ASK_ABOVE
  const download = (series.missing?.length ?? 0) > 0 && series.count > 1
  if (!many && !download) return void session.run(copy)
  openDialog({
    title: `Copy ${series.count.toLocaleString()} patches`,
    message: [
      many ? `That’s ${series.count.toLocaleString()} commits’ patches, all at once.` : '',
      download ? `They need ${versions(series.missing!.length)} this partial clone hasn’t got yet, which come in one download first.` : '',
    ]
      .filter(Boolean)
      .join(' '),
    submit: download ? 'Download and copy' : 'Copy',
    run: async () => {
      try {
        await copy()
      } catch (e) {
        return errorText(e)
      }
    },
  })
}
