import { describe, expect, it } from 'vitest'
import { anchored, escapeName, extension, fileChoices, folderChoices, patternIn, sourceName } from './ignore'

describe('escaping', () => {
  it('anchors a path and escapes what git would read as a pattern', () => {
    expect(anchored('debug.log')).toBe('/debug.log')
    expect(anchored('what?[1]*.txt')).toBe('/what\\?\\[1]\\*.txt')
    expect(anchored('back\\slash')).toBe('/back\\\\slash')
    expect(anchored('trailing  ')).toBe('/trailing\\ \\ ')
    expect(anchored('#notes')).toBe('/#notes')
  })

  it('escapes a leading # or ! in a name', () => {
    expect(escapeName('#notes')).toBe('\\#notes')
    expect(escapeName('!bang')).toBe('\\!bang')
    expect(escapeName('a*b')).toBe('a\\*b')
  })
})

describe('choices', () => {
  it('reads an extension, but not from a dotfile', () => {
    expect(extension('a/debug.log')).toBe('log')
    expect(extension('.env')).toBe('')
    expect(extension('Makefile')).toBe('')
    expect(extension('archive.tar.gz')).toBe('gz')
  })

  it('offers the file, its name, its extension and two folders', () => {
    expect(fileChoices('build/cache/index.bin')).toEqual([
      { label: 'This file', pattern: '/build/cache/index.bin' },
      { label: 'Any file named index.bin', pattern: 'index.bin' },
      { label: 'Every .bin file', pattern: '*.bin' },
      { label: 'The folder cache/', pattern: '/build/cache/' },
      { label: 'The folder build/', pattern: '/build/' },
    ])
    expect(fileChoices('.env').map((c) => c.pattern)).toEqual(['/.env'])
  })

  it('offers a folder, its parents and its name anywhere', () => {
    expect(folderChoices('build/cache').map((c) => c.pattern)).toEqual(['/build/cache/', '/build/', 'cache/'])
    expect(folderChoices('dist').map((c) => c.pattern)).toEqual(['/dist/'])
  })
})

describe('patternIn', () => {
  it('leaves patterns alone outside a folder .gitignore', () => {
    expect(patternIn('/build/x', { scope: 'root' })).toBe('/build/x')
    expect(patternIn('/build/x', { scope: 'exclude' })).toBe('/build/x')
  })

  it('makes an anchored pattern relative to the folder, or refuses it', () => {
    expect(patternIn('/build/cache/', { scope: 'dir', dir: 'build' })).toBe('/cache/')
    expect(patternIn('*.bin', { scope: 'dir', dir: 'build' })).toBe('*.bin')
    expect(patternIn('/build/', { scope: 'dir', dir: 'build' })).toBeNull()
    expect(patternIn('/buildx/a', { scope: 'dir', dir: 'build' })).toBeNull()
  })
})

describe('sourceName', () => {
  it('shortens info/exclude and names the global file', () => {
    expect(sourceName('.gitignore')).toBe('.gitignore')
    expect(sourceName('C:\\repo\\.git\\worktrees\\..\\info\\exclude')).toBe('.git/info/exclude')
    expect(sourceName('C:\\Users\\me\\.config\\git\\ignore', 'C:/Users/me/.config/git/ignore')).toBe('your global excludes file')
  })
})
