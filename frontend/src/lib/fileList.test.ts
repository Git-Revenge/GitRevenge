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
