import { DiffTool, OpenDiffTool } from '../../wailsjs/go/app/App.js'
import type { git } from '../../wailsjs/go/models'
import type { MenuItem } from './ContextMenu.svelte'
import type { DiffTarget } from './diff/DiffView.svelte'
import { errorText } from './errors'
import { onBackend } from './events'
import { toasts } from './toasts.svelte'

// Open in diff tool (#939): which tool git difftool opens for each repository, kept current as
// its config changes, and the item that opens a comparison in it.

class DiffTools {
  tools = $state<Record<string, git.DiffTool>>({})
  private asked = new Set<string>()
  private listening = false

  /** The tool for a repository, or null when none is set. The first ask reads it. */
  of(path: string): git.DiffTool | null {
    if (!this.asked.has(path)) {
      this.asked.add(path)
      this.listen()
      void this.load(path)
    }
    const t = this.tools[path]
    return t?.name ? t : null
  }

  private async load(path: string) {
    try {
      this.tools[path] = await DiffTool(path)
    } catch {
      // A repository that can't be read offers no tool.
      this.tools[path] = { name: '', key: '' } as git.DiffTool
    }
  }

  private listen() {
    if (this.listening) return
    this.listening = true
    // The global config sets it for every repository; a repository's own, for itself.
    onBackend('gitconfig:changed', () => {
      for (const path of this.asked) void this.load(path)
    })
    onBackend('repo:changed', (c) => {
      if (c.kinds.includes('config') && this.asked.has(c.path)) void this.load(c.path)
    })
  }
}

export const diffTools = new DiffTools()

/** The comparison a diff view shows, as git difftool takes it; null for a view it can't open
 * (a conflict, a file's history, a blame, a file found). */
export function toolTarget(t: Pick<DiffTarget, 'mode' | 'file' | 'oldPath' | 'commit'>): git.DiffToolTarget | null {
  if (t.mode !== 'commit' && t.mode !== 'compare' && t.mode !== 'staged' && t.mode !== 'unstaged') return null
  return { mode: t.mode, file: t.file, oldPath: t.oldPath ?? '', commit: t.commit ?? '' } as git.DiffToolTarget
}

/** Opens a comparison in the diff tool. It runs until the tool closes; a tool that won't open
 * says why. */
export async function openInDiffTool(path: string, target: git.DiffToolTarget) {
  try {
    await OpenDiffTool(path, target)
  } catch (e) {
    toasts.show(errorText(e))
  }
}

/** "Open in meld", for a menu: nothing when no tool is set or the view isn't one it opens. */
export function diffToolItems(path: string, t: Pick<DiffTarget, 'mode' | 'file' | 'oldPath' | 'commit'>, label?: string): MenuItem[] {
  const tool = diffTools.of(path)
  const target = toolTarget(t)
  if (!tool || !target) return []
  return [{ label: label ?? `Open in ${tool.name}`, icon: 'external', hint: tool.key, run: () => void openInDiffTool(path, target) }]
}
