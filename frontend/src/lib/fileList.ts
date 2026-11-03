/** A run of files in one folder, in the order they came. */
export interface FileGroup<F> {
  /** The folder's path, or '' for the top of the repository. */
  folder: string
  files: F[]
}

export const fileName = (path: string) => path.slice(path.lastIndexOf('/') + 1)

export const fileFolder = (path: string) => path.slice(0, Math.max(0, path.lastIndexOf('/')))

/** Groups files by folder, keeping their order, so a list reads by filename. */
export function fileGroups<F extends { path: string }>(files: F[]): FileGroup<F>[] {
  const groups: FileGroup<F>[] = []
  for (const f of files) {
    const folder = fileFolder(f.path)
    const last = groups.at(-1)
    if (last?.folder === folder) last.files.push(f)
    else groups.push({ folder, files: [f] })
  }
  return groups
}
