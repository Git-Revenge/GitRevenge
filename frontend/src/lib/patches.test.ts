import { describe, expect, it, vi } from 'vitest'

vi.mock('../../wailsjs/go/app/App.js', () => ({}))
vi.mock('../../wailsjs/runtime/runtime.js', () => ({}))
vi.mock('./session.svelte', () => ({ session: {} }))
vi.mock('./toasts.svelte', () => ({ toasts: {} }))

const { patchFolder, patchSignoff, rememberPatchChoices } = await import('./patches')

const memory = () => {
  const m = new Map<string, string>()
  return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v) }
}

describe('where patches go, and what was ticked', () => {
  it('starts in the folder above the repository', () => {
    expect(patchFolder('A:\\src\\tidewater', null)).toBe('A:\\src')
    expect(patchFolder('/home/ana/src/tidewater/', null)).toBe('/home/ana/src')
    expect(patchFolder('A:\\tidewater', null)).toBe('A:\\')
    expect(patchFolder('/tidewater', null)).toBe('/')
  })

  it('remembers the folder and Signed-off-by, each repository its own', () => {
    const store = memory()
    expect(patchSignoff('A:\\src\\a', store)).toBe(false)
    rememberPatchChoices('A:\\src\\a', 'A:\\outbox', true, store)
    expect(patchFolder('A:\\src\\a', store)).toBe('A:\\outbox')
    expect(patchSignoff('A:\\src\\a', store)).toBe(true)
    expect(patchFolder('A:\\src\\b', store)).toBe('A:\\src')
    expect(patchSignoff('A:\\src\\b', store)).toBe(false)
  })

  it('gets by without storage', () => {
    const broken = {
      getItem: () => {
        throw new Error('denied')
      },
      setItem: () => {
        throw new Error('denied')
      },
    }
    expect(patchFolder('A:\\src\\a', broken)).toBe('A:\\src')
    expect(patchSignoff('A:\\src\\a', broken)).toBe(false)
    expect(() => rememberPatchChoices('A:\\src\\a', 'x', true, broken)).not.toThrow()
  })
})
