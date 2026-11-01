/** The rules behind the sidebar's Repository panel. */

/** The fields of git.Worktree these rules use. */
export interface WorktreeLike {
  path: string
  head: string
  branch: string
  main: boolean
  current: boolean
  locked: boolean
  prunable: boolean
}

export interface RepoStatus {
  text: string
  tone: 'warning' | 'muted'
}

/** The last folder of a path, on any platform. */
export function folderName(path: string): string {
  return path.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || path
}

/** The one thing a worktree's row shows: a problem if it has one, otherwise its branch. */
export function worktreeStatus(w: WorktreeLike): RepoStatus {
  if (w.prunable) return { text: 'missing', tone: 'warning' }
  if (w.locked) return { text: 'locked', tone: 'muted' }
  return { text: w.branch || 'detached', tone: 'muted' }
}

/** Everything known about a worktree, one fact per line. */
export function worktreeTooltip(w: WorktreeLike): string {
  const lines = [w.path, w.branch ? `On ${w.branch}` : `Detached at ${w.head.slice(0, 7)}`]
  if (w.main) lines.push('The main worktree')
  if (w.current) lines.push('Open in this window')
  if (w.locked) lines.push('Locked')
  if (w.prunable) lines.push('Missing on disk')
  return lines.join('\n')
}

/** Another worktree that's still on disk can open as a tab. */
export const canOpenWorktree = (w: WorktreeLike) => !w.current && !w.prunable

/** Git keeps the main worktree, and this window is using the current one. */
export const canRemoveWorktree = (w: WorktreeLike) => !w.main && !w.current

const STATE_WORDS: Record<string, string> = {
  uninitialized: 'not initialized',
  modified: 'different commit',
  conflict: 'conflict',
}

/** A submodule's state in words; '' when there's nothing to say. */
export function submoduleStatus(state: string): string {
  return STATE_WORDS[state] ?? ''
}

export function matchesFilter(query: string, ...names: string[]): boolean {
  const q = query.trim().toLowerCase()
  return !q || names.some((n) => n.toLowerCase().includes(q))
}

/** The names the filter searches for a worktree. */
export const worktreeNames = (w: WorktreeLike) => [w.path, w.branch]

/** The names the filter searches for the LFS row: its own, its patterns and its files. */
export function lfsNames(patterns: { pattern: string }[], files: { name: string }[]): string[] {
  return ['LFS', ...patterns.map((p) => p.pattern), ...files.map((f) => f.name)]
}
