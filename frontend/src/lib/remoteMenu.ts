/** The menu for a remote itself (#921), opened on a remote's folder in the Remotes panel. It
 * opens the remote on its forge, copies its address, fetches it, makes it where pushes go,
 * hides its branches, and renames, repoints or removes it. Nothing here starts or reads a pull
 * request (#916). */

import type { app, git } from '../../wailsjs/go/models'
import type { MenuItem } from './ContextMenu.svelte'
import type { DialogSpec } from './Dialog.svelte'
import { fetchAge, type URLPlace } from './remoteHeader'

export interface RemoteMenuData {
  remote: git.Remote
  /** Where its fetch URL points. */
  place?: URLPlace
  /** Its pages on the web, when it has them. */
  web?: app.RemoteWeb
  /** remote.pushDefault as git reads it now, or ''. */
  pushDefault: string
  now: number
}

/** What the menu's items do, given by the view that opens it. */
export interface RemoteMenuActions {
  open: (url: string) => void
  copy: (text: string, what: string) => void
  fetch: (remote: string) => void
  setPushDefault: (remote: string) => void
  hide: (remote: string, hide: boolean) => void
  showFolder: (remote: string) => void
  /** Opens Settings › Links › Forges with the host filled in. */
  nameForge: (host: string) => void
  openDialog: (spec: DialogSpec) => void
  /** Runs a Remotes dialog action: rename, seturl or remove. Resolves to what went wrong, in
   * words, or '' when it worked. */
  act: (action: 'rename' | 'seturl' | 'remove', args: string[]) => Promise<string>
}

/** The repository's path on its forge, "owner/repo", from its page's address. */
function repoPath(page: string): string {
  try {
    return decodeURIComponent(new URL(page).pathname.replace(/^\/+|\/+$/g, ''))
  } catch {
    return page
  }
}

/** Two URLs naming the same thing, but for a trailing ".git" or slash. */
const same = (a: string, b: string) => a.replace(/(\.git)?\/*$/, '') === b.replace(/(\.git)?\/*$/, '')

export function remoteMenuItems(d: RemoteMenuData, act: RemoteMenuActions): MenuItem[] {
  const r = d.remote
  const name = r.name
  const local = !d.place || d.place.place === 'local'
  const host = d.place?.host ?? ''
  const items: MenuItem[] = [{ label: name, hint: local ? r.fetchUrl : `${host} · ${r.fetchUrl}`, heading: true, disabled: true }]

  // Where it lives.
  if (local) {
    items.push({ label: 'Show in the file manager', icon: 'folder', separator: true, run: () => act.showFolder(name) })
  } else if (d.web) {
    const web = d.web
    items.push(
      { label: `Open ${repoPath(web.repo)} on ${host}`, icon: 'external', separator: true, run: () => act.open(web.repo) },
      { label: `Open its branches on ${host}`, icon: 'external', run: () => act.open(web.branches) },
    )
  } else {
    items.push({
      label: 'Open on the web…',
      icon: 'external',
      separator: true,
      hint: `No web address for ${host}. Say what forge it runs.`,
      run: () => act.nameForge(host),
    })
  }

  // Its address.
  items.push({ label: 'Copy the URL', icon: 'copy', separator: true, hint: r.fetchUrl, run: () => act.copy(r.fetchUrl, 'URL') })
  if (d.web?.cloneHttps && !same(d.web.cloneHttps, r.fetchUrl)) {
    const u = d.web.cloneHttps
    items.push({ label: 'Copy the HTTPS URL', icon: 'copy', hint: u, run: () => act.copy(u, 'HTTPS URL') })
  }
  if (d.web?.cloneSsh && !same(d.web.cloneSsh, r.fetchUrl)) {
    const u = d.web.cloneSsh
    items.push({ label: 'Copy the SSH URL', icon: 'copy', hint: u, run: () => act.copy(u, 'SSH URL') })
  }

  // Fetching and pushing.
  const age = fetchAge(r.fetchedAt, d.now)
  const isDefault = d.pushDefault === name
  items.push(
    { label: `Fetch ${name}`, icon: 'fetch', separator: true, hint: age ? `Last fetched ${age}` : "Git Revenge hasn't seen it fetched", run: () => act.fetch(name) },
    {
      label: 'Push here by default',
      icon: isDefault ? 'check' : undefined,
      hint: isDefault ? 'remote.pushDefault is this remote. Choose again to unset it.' : d.pushDefault ? `remote.pushDefault is ${d.pushDefault} now` : 'Sets remote.pushDefault',
      run: () => act.setPushDefault(isDefault ? '' : name),
    },
    { label: r.hidden ? `Show ${name}'s branches` : `Hide ${name}'s branches`, icon: 'eye', run: () => act.hide(name, !r.hidden) },
  )

  // Changing it.
  items.push(
    {
      label: `Rename ${name}…`,
      icon: 'edit',
      separator: true,
      run: () =>
        act.openDialog({
          title: `Rename ${name}`,
          message: 'Git renames its remote-tracking branches and the branches that track them too.',
          fields: [{ key: 'name', label: 'New name', value: name }],
          submit: 'Rename',
          run: async (v) => {
            const to = String(v.name ?? '').trim()
            if (!to || to === name) return 'Give it a new name.'
            return (await act.act('rename', [name, to])) || undefined
          },
        }),
    },
    {
      label: 'Change URL…',
      icon: 'edit',
      run: () =>
        act.openDialog({
          title: `Change ${name}'s URL`,
          message: 'Where fetches come from, and pushes go unless it has push URLs of its own.',
          fields: [{ key: 'url', label: 'URL', value: r.fetchUrl }],
          submit: 'Change',
          run: async (v) => {
            const url = String(v.url ?? '').trim()
            if (!url) return 'Give it a URL.'
            if (url !== r.fetchUrl) return (await act.act('seturl', [name, url])) || undefined
          },
        }),
    },
    {
      label: `Remove ${name}…`,
      icon: 'trash',
      // The dialog is the one confirmation: arming the item first would ask twice.
      run: () =>
        act.openDialog({
          title: `Remove ${name}`,
          message: `Removes ${name} from this repository, with its remote-tracking branches. Nothing on ${local ? 'that folder' : host} changes, and local branches that tracked it stop tracking anything.`,
          submit: `Remove ${name}`,
          danger: true,
          run: async () => (await act.act('remove', [name])) || undefined,
        }),
    },
  )
  return items
}
