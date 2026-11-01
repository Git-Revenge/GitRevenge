import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Command } from './commands.svelte'

vi.mock('../../wailsjs/go/app/App.js', () => ({ Keybindings: vi.fn(async () => ({})), SetKeybindings: vi.fn(async () => {}) }))
vi.stubGlobal('document', { querySelector: () => null })

const { commands } = await import('./commands.svelte')

const key = (k: string, extra: Partial<KeyboardEvent> = {}) =>
  ({ key: k, ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, defaultPrevented: false, target: null, preventDefault: () => {}, ...extra }) as unknown as KeyboardEvent

function command(id: string, shortcut?: string) {
  const run = vi.fn<() => unknown>()
  return { id, title: id, group: 'Test', shortcut, run } satisfies Command
}

describe('commands only the palette needs', () => {
  let removers: (() => void)[] = []
  const add = (...args: Parameters<typeof commands.add>) => removers.push(commands.add(...args))

  beforeEach(() => {
    removers.forEach((r) => r())
    removers = []
    commands.overrides = {}
  })

  it('are listed for the palette', () => {
    add(() => [command('goto.main')], { paletteOnly: true })
    expect(commands.all().map((c) => c.id)).toContain('goto.main')
  })

  // A key press used to build every ref's commands: 61ms with 50,000 branches (#360).
  it('are not built for a key press', () => {
    const provider = vi.fn(() => [command('goto.main')])
    add(provider, { paletteOnly: true })
    add(() => [command('other', 'k')])
    commands.dispatch(key('z'))
    expect(provider).not.toHaveBeenCalled()
  })

  it('leave ordinary shortcuts working', () => {
    const c = command('other', 'k')
    add(() => [command('goto.main')], { paletteOnly: true })
    add(() => [c])
    expect(commands.dispatch(key('k'))).toBe(true)
    expect(c.run).toHaveBeenCalled()
  })

  it('still run when someone has given one a shortcut', () => {
    const c = command('goto.main')
    add(() => [c], { paletteOnly: true })
    commands.overrides = { 'goto.main': 'g' }
    expect(commands.dispatch(key('g'))).toBe(true)
    expect(c.run).toHaveBeenCalled()
  })

  it('are skipped again when a shortcut was only unbound', () => {
    const provider = vi.fn(() => [command('goto.main')])
    add(provider, { paletteOnly: true })
    commands.overrides = { 'goto.main': '' }
    commands.dispatch(key('z'))
    expect(provider).not.toHaveBeenCalled()
  })

  it('are skipped when a saved shortcut is for an ordinary command', () => {
    const provider = vi.fn(() => [command('goto.main')])
    const c = command('other', 'k')
    add(provider, { paletteOnly: true })
    add(() => [c])
    commands.overrides = { other: 'j' }
    expect(commands.dispatch(key('j'))).toBe(true)
    expect(c.run).toHaveBeenCalled()
    expect(provider).not.toHaveBeenCalled()
  })

  it('are removed with their provider', () => {
    const remove = commands.add(() => [command('goto.main')], { paletteOnly: true })
    remove()
    expect(commands.all().map((c) => c.id)).not.toContain('goto.main')
  })
})
