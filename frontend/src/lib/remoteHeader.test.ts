import { describe, expect, it } from 'vitest'
import type { git } from '../../wailsjs/go/models'
import { fetchAge, fetchAgeShort, onRemotesTooltip, placeLook, remoteHeader, trackersByRemote, type URLPlace } from './remoteHeader'

const place = (p: Partial<URLPlace>) => ({ place: 'web', host: '', forge: '', web: '', ...p }) as URLPlace
const remote = (r: Partial<git.Remote>) => ({ name: 'origin', fetchUrl: '', pushUrl: '', pushUrls: [], fetchedAt: 0, ...r }) as git.Remote

describe('fetchAge', () => {
  const now = 1_800_000_000
  it.each([
    [0, ''],
    [now - 20, 'just now'],
    [now - 4 * 60, '4 min ago'],
    [now - 3600, '1 hour ago'],
    [now - 5 * 3600, '5 hours ago'],
    [now - 86400, '1 day ago'],
    [now - 9 * 86400, '9 days ago'],
    [now - 21 * 86400, '3 weeks ago'],
    [now - 150 * 86400, '5 months ago'],
    [now - 800 * 86400, '2 years ago'],
    [now + 30, 'just now'],
  ])('%i reads %s', (at, want) => expect(fetchAge(at, now)).toBe(want))
})

describe('fetchAgeShort', () => {
  const now = 1_800_000_000
  it.each([
    [0, ''],
    [now - 20, 'now'],
    [now - 4 * 60, '4m'],
    [now - 5 * 3600, '5h'],
    [now - 9 * 86400, '9d'],
    [now - 21 * 86400, '3w'],
    [now - 150 * 86400, '5mo'],
    [now - 800 * 86400, '2y'],
  ])('%i reads %s', (at, want) => expect(fetchAgeShort(at, now)).toBe(want))
})

describe('placeLook', () => {
  it('shows a mark only for a forge', () => {
    expect(placeLook(place({ forge: 'github', host: 'github.com' })).mark?.name).toBe('GitHub')
    expect(placeLook(place({ forge: 'bitbucket-server' })).mark?.name).toBe('Bitbucket')
    expect(placeLook(place({ forge: 'bitbucket-server' })).says).toBe('Bitbucket Data Center')
  })
  it('falls back to the app’s own icons', () => {
    expect(placeLook(place({ host: 'gitlab.example.com' }))).toEqual({ icon: 'remote', says: "A host we don't recognise" })
    expect(placeLook(place({ place: 'server', host: 'nas.local' }))).toEqual({ icon: 'server', says: 'SSH only, no web address' })
    expect(placeLook(place({ place: 'local' })).icon).toBe('folder')
    expect(placeLook(undefined).icon).toBe('folder')
  })
  it('keeps the cloud for a forge without a mark, but names it', () => {
    expect(placeLook(place({ forge: 'azure' }))).toEqual({ mark: undefined, icon: 'remote', says: 'Azure DevOps' })
  })
})

describe('remoteHeader', () => {
  const now = 1_800_000_000
  const gh = 'git@github.com:ris/tidewater.git'
  const places = {
    [gh]: place({ place: 'server', host: 'github.com', forge: 'github', web: 'https://github.com/ris/tidewater' }),
    'https://gitlab.com/t/t.git': place({ host: 'gitlab.com', forge: 'gitlab' }),
    'git@nas.local:srv/t.git': place({ place: 'server', host: 'nas.local' }),
    '/srv/git/t.git': place({ place: 'local' }),
  }

  it('says where it is, how fresh, and that push is the same', () => {
    const h = remoteHeader(remote({ fetchUrl: gh, pushUrl: gh, pushUrls: [gh], fetchedAt: now - 240 }), places, now, 4, ['main', 'fix/x'])
    expect(h.host).toBe('github.com')
    expect(h.look.mark?.name).toBe('GitHub')
    expect(h.age).toBe('4 min ago')
    expect(h.stale).toBe(false)
    expect(h.fan).toEqual([])
    expect(h.tooltip).toBe(`origin · GitHub\nFetch  ${gh}\nPush   same as fetch\nFetched 4 min ago\n4 branches\nTracked by main, fix/x`)
  })

  it('turns stale after a week', () => {
    expect(remoteHeader(remote({ fetchUrl: gh, fetchedAt: now - 8 * 86400 }), places, now, 1, []).stale).toBe(true)
    expect(remoteHeader(remote({ fetchUrl: gh, fetchedAt: now - 6 * 86400 }), places, now, 1, []).stale).toBe(false)
  })

  it("says so when it hasn't seen a fetch, without calling it stale", () => {
    const h = remoteHeader(remote({ fetchUrl: gh }), places, now, 1, [])
    expect(h.age).toBe('')
    expect(h.stale).toBe(false)
    expect(h.tooltip).toContain("Git Revenge hasn't seen it fetched")
    expect(h.tooltip).toContain('1 branch\n'.trim())
  })

  it('fans out the places a push goes', () => {
    const pushUrls = [gh, 'https://gitlab.com/t/t.git', 'git@nas.local:srv/t.git']
    const h = remoteHeader(remote({ fetchUrl: gh, pushUrl: gh, pushUrls }), places, now, 2, [])
    expect(h.fan.map((f) => f.mark?.name ?? f.icon)).toEqual(['GitHub', 'GitLab', 'server'])
    expect(h.tooltip).toContain('Push to origin: GitHub, GitLab, nas.local')
  })

  it('has no host chip for a folder', () => {
    const h = remoteHeader(remote({ name: 'backup', fetchUrl: '/srv/git/t.git' }), places, now, 1, [])
    expect(h.host).toBe('')
    expect(h.look.icon).toBe('folder')
  })

  it('cuts a long list of trackers short', () => {
    const h = remoteHeader(remote({ fetchUrl: gh }), places, now, 9, ['a', 'b', 'c', 'd', 'e', 'f', 'g'])
    expect(h.tooltip).toContain('Tracked by a, b, c, d, e and 2 more')
  })
})

describe('trackersByRemote', () => {
  it('files each tracking branch under its remote, the longest name first', () => {
    const refs = [
      { kind: 'local', short: 'main', upstream: 'origin/main' },
      { kind: 'local', short: 'x', upstream: 'origin/team/x' },
      { kind: 'local', short: 'y', upstream: 'origin/team-b/y' },
      { kind: 'local', short: 'lone', upstream: '' },
      { kind: 'remote', short: 'origin/main', upstream: '' },
    ]
    expect(trackersByRemote(refs, ['origin', 'origin/team'])).toEqual({ origin: ['main', 'y'], 'origin/team': ['x'] })
  })
})

describe('onRemotesTooltip', () => {
  it('lists each copy and says whether they agree', () => {
    expect(onRemotesTooltip('main', [{ short: 'origin/main', commit: 'aaaaaaaaa' }, { short: 'upstream/main', commit: 'aaaaaaaaa' }])).toBe(
      'main\norigin/main  aaaaaaa\nupstream/main  aaaaaaa\nThe same commit on every remote',
    )
    expect(onRemotesTooltip('fix', [{ short: 'origin/fix', commit: 'aaaaaaaaa' }, { short: 'upstream/fix', commit: 'bbbbbbbbb' }])).toContain('≠ marks a remote at another commit than origin/fix')
    expect(onRemotesTooltip('solo', [{ short: 'origin/solo', commit: 'aaaaaaaaa' }])).toBe('solo\norigin/solo  aaaaaaa')
  })
})
