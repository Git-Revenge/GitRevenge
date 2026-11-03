/** A run of files in one folder, in the order they came. */
export interface FileGroup<F> {
  /** The folder's path, or '' for the top of the repository. */
  folder: string
  files: F[]
}

export const fileName = (path: string) => path.slice(path.lastIndexOf('/') + 1)

export const fileFolder = (path: string) => path.slice(0, Math.max(0, path.lastIndexOf('/')))

/** Groups files by folder, so a list reads by filename. Folders keep the order they first
 * appear in, and a folder's files stay together even where a subfolder sorts among them. */
export function fileGroups<F extends { path: string }>(files: F[]): FileGroup<F>[] {
  const groups = new Map<string, FileGroup<F>>()
  for (const f of files) {
    const folder = fileFolder(f.path)
    const group = groups.get(folder)
    if (group) group.files.push(f)
    else groups.set(folder, { folder, files: [f] })
  }
  return [...groups.values()]
}
