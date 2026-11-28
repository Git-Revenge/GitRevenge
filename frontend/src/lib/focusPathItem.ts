// "Focus the timeline on src/alerts/" (#1060), for a folder's or file's menu: in a commit's
// files and in Changes.
import { SetGraphPath } from '../../wailsjs/go/app/App.js'
import type { MenuItem } from './ContextMenu.svelte'
import { isRows } from './graph/views'
import { session } from './session.svelte'
import { theme } from './theme.svelte'

/** Focuses the timeline of the repository at repo on target, a folder (ending in a slash) or a
 * file, and shows the timeline if another view covers it. */
export async function focusTimelineOn(repo: string, target: string): Promise<void> {
  await session.run(async () => {
    await SetGraphPath(repo, target)
    await showTimeline()
  })
}

/** Shows the timeline, in place of the Map or another view that covers its rows. */
export async function showTimeline(): Promise<void> {
  if (!isRows(theme.view)) await theme.setView(theme.density === 'compact' ? 'compact' : 'timeline')
}

/** The menu item: a folder by its whole path, a file by its name. */
export function focusPathItem(repo: string, target: string): MenuItem {
  const name = target.endsWith('/') ? target : target.slice(target.lastIndexOf('/') + 1)
  return { label: `Focus the timeline on ${name}`, icon: 'focus', run: () => void focusTimelineOn(repo, target) }
}
