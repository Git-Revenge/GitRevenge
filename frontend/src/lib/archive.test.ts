import { describe, expect, it, vi } from 'vitest'

// The folder's default comes from patches.ts, which reaches the backend.
vi.mock('../../wailsjs/go/app/App.js', () => ({}))
vi.mock('../../wailsjs/runtime/runtime.js', () => ({}))
vi.mock('./session.svelte', () => ({ session: {} }))
vi.mock('./toasts.svelte', () => ({ toasts: {} }))

const { archiveBase, archiveCommand, archiveFile, archiveFolder, archiveFormat, folderOf, planSaid, rememberArchive } = await import('./archive')

function memory() {
  const m = new Map<string, string>()
  return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v) }
}

describe('Save as an archive', () => {
  it('names the archive for the repository and what is archived', () => {
    expect(archiveBase('T:\\tidewater', 'v1.1.0')).toBe('tidewater-v1.1.0')
    expect(archiveBase('/home/me/tidewater/', 'feature/offline-cache')).toBe('tidewater-feature-offline-cache')
    expect(archiveFile('tidewater-v1.1.0', 'tar.gz')).toBe('tidewater-v1.1.0.tar.gz')
  })

  it('says what goes in', () => {
    expect(planSaid({ files: 1234, bytes: 3 << 20, ignored: 0, subst: 0 })).toBe('1,234 files, 3 MB before compressing.')
    expect(planSaid({ files: 1, bytes: 10, ignored: 3, subst: 1 })).toBe('1 file, 1 KB before compressing; 3 left out by export-ignore; 1 with export-subst placeholders filled in.')
    // A partial clone doesn't know the sizes of files it hasn't got.
    expect(planSaid({ files: 12, bytes: -1, ignored: 0, subst: 0 })).toBe('12 files.')
    expect(planSaid({ files: 0, bytes: 0, ignored: 2, subst: 0 })).toBe('0 files; 2 left out by export-ignore.')
  })

  it('shows the command its choices make', () => {
    expect(archiveCommand({ format: 'zip', prefix: 'tidewater-v1.1.0', sub: '', file: 'tidewater-v1.1.0.zip', rev: 'v1.1.0' })).toBe(
      'git archive --format=zip --prefix=tidewater-v1.1.0/ -o tidewater-v1.1.0.zip v1.1.0',
    )
    expect(archiveCommand({ format: 'tar.gz', prefix: '', sub: '\\src\\app\\', file: 'my app.tar.gz', rev: 'a1b2c3d' })).toBe('git archive --format=tar.gz -o "my app.tar.gz" a1b2c3d -- src/app')
    expect(folderOf('/a\\b')).toBe('a/b/')
  })

  it('remembers the folder and format for each repository', () => {
    const store = memory()
    expect(archiveFormat('/r', store)).toBe('zip')
    expect(archiveFolder('/home/me/r', store)).toBe('/home/me')
    rememberArchive('/home/me/r', '/tmp/out', 'tar.gz', store)
    expect(archiveFormat('/home/me/r', store)).toBe('tar.gz')
    expect(archiveFolder('/home/me/r', store)).toBe('/tmp/out')
    expect(archiveFormat('/home/me/other', store)).toBe('zip')
  })
})
