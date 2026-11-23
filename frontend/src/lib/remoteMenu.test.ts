import { describe, expect, it, vi } from 'vitest'
import type { app, git } from '../../wailsjs/go/models'
import type { DialogSpec } from './Dialog.svelte'
import { remoteMenuItems, type RemoteMenuActions } from './remoteMenu'

const now = 1_800_000_000
const gh = 'git@github.com:ris/tidewater.git'
const remote = (r: Partial<git.Remote> = {}) => ({ name: 'origin', fetchUrl: gh, pushUrl: gh, pushUrls: [gh], fetchedAt: now - 240, ...r }) as git.Remote
const web = {
  remote: 'origin',
  forge: 'github',
  repo: 'https://github.com/ris/tidewater',
  branches: 'https://github.com/ris/tidewater/branches',
  cloneHttps: 'https://github.com/ris/tidewater.git',
  cloneSsh: gh,
} as app.RemoteWeb
const ghPlace = { place: 'server', host: 'github.com', forge: 'github', web: web.repo }

function actions() {
  const dialogs: DialogSpec[] = []
  const a = {
    open: vi.fn(),
    copy: vi.fn(),
    fetch: vi.fn(),
    setPushDefault: vi.fn(),
    hide: vi.fn(),
    showFolder: vi.fn(),
    nameForge: vi.fn(),
    openDialog: (d: DialogSpec) => void dialogs.push(d),
    act: vi.fn(async () => ''),
  } satisfies RemoteMenuActions
  return { a, dialogs }
}

const labels = (items: { label: string }[]) => items.map((i) => i.label)
const item = (items: { label: string; run?: () => void }[], label: string) => {
  const found = items.find((i) => i.label === label)
  if (!found) throw new Error(`no item ${label} in ${labels(items).join(', ')}`)
  return found
}

describe("a remote's menu", () => {
  it('heads with the remote, then opens it on its forge', () => {
    const { a } = actions()
    const items = remoteMenuItems({ remote: remote(), place: ghPlace, web, pushDefault: '', now }, a)
    expect(items[0]).toMatchObject({ label: 'origin', heading: true, disabled: true, hint: `github.com · ${gh}` })
    expect(labels(items)).toEqual([
      'origin',
      'Open ris/tidewater on github.com',
      'Open its branches on github.com',
      'Copy the URL',
      'Copy the HTTPS URL',
      'Fetch origin',
      'Push here by default',
      "Hide origin's branches",
      'Rename origin…',
      'Change URL…',
      'Remove origin…',
    ])
    item(items, 'Open ris/tidewater on github.com').run?.()
    expect(a.open).toHaveBeenCalledWith('https://github.com/ris/tidewater')
    item(items, 'Copy the HTTPS URL').run?.()
    expect(a.copy).toHaveBeenCalledWith('https://github.com/ris/tidewater.git', 'HTTPS URL')
  })

  it('offers SSH when the remote is HTTPS', () => {
    const { a } = actions()
    const https = remote({ fetchUrl: 'https://github.com/ris/tidewater.git' })
    const items = remoteMenuItems({ remote: https, place: { ...ghPlace, place: 'web' }, web, pushDefault: '', now }, a)
    expect(labels(items)).toContain('Copy the SSH URL')
    expect(labels(items)).not.toContain('Copy the HTTPS URL')
  })

  it('says why there is no web address, and offers to name the forge', () => {
    const { a } = actions()
    const items = remoteMenuItems({ remote: remote({ fetchUrl: 'git@nas.local:srv/t.git' }), place: { place: 'server', host: 'nas.local', forge: '', web: '' }, pushDefault: '', now }, a)
    const open = item(items, 'Open on the web…') as { hint?: string; run?: () => void }
    expect(open.hint).toBe('No web address for nas.local. Say what forge it runs.')
    open.run?.()
    expect(a.nameForge).toHaveBeenCalledWith('nas.local')
  })

  it('shows a folder on this machine in the file manager instead', () => {
    const { a } = actions()
    const items = remoteMenuItems({ remote: remote({ name: 'disk', fetchUrl: '/srv/git/t.git' }), place: { place: 'local', host: '', forge: '', web: '' }, pushDefault: '', now }, a)
    expect(labels(items)).toContain('Show in the file manager')
    expect(labels(items).some((l) => l.startsWith('Open'))).toBe(false)
    expect(items[0].hint).toBe('/srv/git/t.git')
  })

  it('checks Push here by default on the remote that has it, and unsets it from there', () => {
    const { a } = actions()
    const on = item(remoteMenuItems({ remote: remote(), place: ghPlace, web, pushDefault: 'origin', now }, a), 'Push here by default') as { icon?: string; run?: () => void }
    expect(on.icon).toBe('check')
    on.run?.()
    expect(a.setPushDefault).toHaveBeenCalledWith('')
    const off = item(remoteMenuItems({ remote: remote(), place: ghPlace, web, pushDefault: 'upstream', now }, a), 'Push here by default') as { icon?: string; hint?: string; run?: () => void }
    expect(off.icon).toBeUndefined()
    expect(off.hint).toBe('remote.pushDefault is upstream now')
    off.run?.()
    expect(a.setPushDefault).toHaveBeenCalledWith('origin')
  })

  it('says when it was fetched', () => {
    const { a } = actions()
    expect(item(remoteMenuItems({ remote: remote(), place: ghPlace, web, pushDefault: '', now }, a), 'Fetch origin')).toMatchObject({ hint: 'Last fetched 4 min ago' })
    expect(item(remoteMenuItems({ remote: remote({ fetchedAt: 0 }), place: ghPlace, web, pushDefault: '', now }, a), 'Fetch origin')).toMatchObject({ hint: "Git Revenge hasn't seen it fetched" })
  })

  it('shows a hidden remote’s branches again', () => {
    const { a } = actions()
    const items = remoteMenuItems({ remote: remote({ hidden: true }), place: ghPlace, web, pushDefault: '', now }, a)
    item(items, "Show origin's branches").run?.()
    expect(a.hide).toHaveBeenCalledWith('origin', false)
  })

  it('renames, repoints and removes through dialogs, keeping git’s error in the dialog', async () => {
    const { a, dialogs } = actions()
    const items = remoteMenuItems({ remote: remote(), place: ghPlace, web, pushDefault: '', now }, a)
    item(items, 'Rename origin…').run?.()
    expect(await dialogs[0].run({ name: 'origin' })).toBe('Give it a new name.')
    await dialogs[0].run({ name: 'mine' })
    expect(a.act).toHaveBeenCalledWith('rename', ['origin', 'mine'])

    item(items, 'Change URL…').run?.()
    a.act.mockResolvedValueOnce('That URL is no good.')
    expect(await dialogs[1].run({ url: 'https://example.com/x.git' })).toBe('That URL is no good.')

    item(items, 'Remove origin…').run?.()
    expect(dialogs[2].danger).toBe(true)
    await dialogs[2].run({})
    expect(a.act).toHaveBeenLastCalledWith('remove', ['origin'])
  })
})
