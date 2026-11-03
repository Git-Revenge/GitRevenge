import { describe, expect, it } from 'vitest'
import { fileFolder, fileGroups, fileName, fileRows } from './fileList'

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

describe('fileRows', () => {
  const files = [f('README.md'), f('src/a.ts'), f('src/lib/b.ts'), f('src/lib/c.ts')]
  const said = (rows: ReturnType<typeof fileRows<{ path: string }>>) => rows.map((r) => (r.type === 'file' ? `${' '.repeat(r.depth)}${r.file.path}` : `${' '.repeat(r.depth)}[${r.name}]`))

  it('lists each folder once, with its whole path', () => {
    expect(said(fileRows(files))).toEqual(['README.md', '[src/]', 'src/a.ts', '[src/lib/]', 'src/lib/b.ts', 'src/lib/c.ts'])
  })

  it('nests folders in a tree', () => {
    expect(said(fileRows(files, { tree: true }))).toEqual(['README.md', '[src]', ' src/a.ts', ' [lib]', '  src/lib/b.ts', '  src/lib/c.ts'])
  })

  it('hides what is inside a closed folder', () => {
    expect(said(fileRows(files, { tree: true, closed: new Set(['src/lib']) }))).toEqual(['README.md', '[src]', ' src/a.ts', ' [lib]'])
    expect(said(fileRows(files, { tree: true, closed: new Set(['src']) }))).toEqual(['README.md', '[src]'])
  })

  it('filters by any part of the path, ignoring case', () => {
    expect(said(fileRows(files, { query: 'LIB/B' }))).toEqual(['[src/lib/]', 'src/lib/b.ts'])
    expect(fileRows(files, { query: 'nothing' })).toEqual([])
  })

  it('opens closed folders while filtering, so a match shows', () => {
    expect(said(fileRows(files, { tree: true, query: 'b.ts', closed: new Set(['src']) }))).toEqual(['[src]', ' [lib]', '  src/lib/b.ts'])
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
