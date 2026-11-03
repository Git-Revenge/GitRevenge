import { describe, expect, it } from 'vitest'
import { fileFolder, fileGroups, fileName } from './fileList'

const f = (path: string) => ({ path })

describe('fileGroups', () => {
  it('groups runs of files by folder, in order', () => {
    const groups = fileGroups([f('a/one.ts'), f('a/two.ts'), f('b/three.ts')])
    expect(groups.map((g) => [g.folder, g.files.map((x) => x.path)])).toEqual([
      ['a', ['a/one.ts', 'a/two.ts']],
      ['b', ['b/three.ts']],
    ])
  })

  it('keeps a folder together when a subfolder sorts among its files', () => {
    const groups = fileGroups([f('src/a.ts'), f('src/lib/b.ts'), f('src/z.ts')])
    expect(groups.map((g) => [g.folder, g.files.length])).toEqual([
      ['src', 2],
      ['src/lib', 1],
    ])
  })

  it('keeps the top of the repository apart', () => {
    expect(fileGroups([f('README.md'), f('a/one.ts')]).map((g) => g.folder)).toEqual(['', 'a'])
  })

  it('has nothing for no files', () => {
    expect(fileGroups([])).toEqual([])
  })
})

describe('file paths', () => {
  it('splits a path into its folder and name', () => {
    expect(fileName('a/b/c.ts')).toBe('c.ts')
    expect(fileFolder('a/b/c.ts')).toBe('a/b')
  })

  it('handles a file at the top', () => {
    expect(fileName('c.ts')).toBe('c.ts')
    expect(fileFolder('c.ts')).toBe('')
  })
})
