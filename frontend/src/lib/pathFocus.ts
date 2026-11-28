// A timeline focused on a folder or file (#1059): what's under the path, for the working
// tree row and anything else that counts only what the focus covers.

/** Whether file is under the focus: in the folder (which ends in a slash), or the file itself. */
export function underPath(file: string, focus: string): boolean {
  if (!focus) return true
  return focus.endsWith('/') ? file.startsWith(focus) : file === focus || file.startsWith(`${focus}/`)
}

/** The working tree row's words under a path focus: how many of the changed files are under
 * it, or that none are. Without a focus, what the row says today. */
export function workingTreeSaid(files: readonly string[] | null, focus: string): { changes: number | null; title: string } {
  if (files === null) return { changes: null, title: 'Working tree' }
  const n = files.filter((f) => underPath(f, focus)).length
  const counted = `${n.toLocaleString()} ${n === 1 ? 'file' : 'files'} changed`
  if (!focus) return { changes: n, title: n ? counted : 'Working tree' }
  return { changes: n, title: n ? `${counted} in ${focus}` : `Nothing changed in ${focus}` }
}
