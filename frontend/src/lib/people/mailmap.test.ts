import { describe, expect, it } from 'vitest'
import type { app, git } from '../../../wailsjs/go/models'
import { canTakeApart, changedLines, describeNote, joined, linesText, sourceNote, summary, wrote } from './mailmap'

const note = (over: Partial<git.MailmapNote>) => ({ line: 1, kind: 'map', identities: 0, commits: 0, coAuthored: 0, replaced: false, ...over }) as git.MailmapNote
const identity = (name: string, email: string, lines: git.MailmapRef[] = []) => ({ name, email, commits: 1, coAuthored: 0, lines }) as app.PersonIdentity

describe('wrote', () => {
  it('says commits, credits, or both', () => {
    expect(wrote(71, 0)).toBe('71 commits')
    expect(wrote(1, 0)).toBe('1 commit')
    expect(wrote(2, 6)).toBe('2 commits, co-author on 6')
    expect(wrote(0, 4)).toBe('co-author on 4')
    expect(wrote(0, 0)).toBe('0 commits')
  })
})

describe('joined', () => {
  it('leaves out the identity the person is shown as', () => {
    const p = { name: 'Priya Raman', email: 'priya@tidewater.example', identities: [identity('Priya Raman', 'priya@tidewater.example'), identity('Priya R', 'priya@oldmill.example')] } as app.Person
    expect(joined(p).map((i) => i.name)).toEqual(['Priya R'])
  })

  it('keeps an identity that differs only by address', () => {
    const p = { name: 'Sam', email: 'sam@new.example', identities: [identity('Sam', 'sam@old.example')] } as app.Person
    expect(joined(p)).toHaveLength(1)
  })
})

describe('linesText', () => {
  it('names one line, several, and lines of another file', () => {
    expect(linesText([])).toBe('')
    expect(linesText([{ source: '.mailmap', line: 2 }])).toBe('line 2')
    expect(
      linesText([
        { source: '.mailmap', line: 2 },
        { source: '.mailmap', line: 5 },
      ]),
    ).toBe('lines 2 and 5')
    expect(
      linesText([
        { source: '.mailmap', line: 1 },
        { source: 'mailmap.file', line: 3 },
      ]),
    ).toBe('line 1, line 3 of your mailmap file')
  })
})

describe('canTakeApart', () => {
  it('needs every line to be in the .mailmap', () => {
    expect(canTakeApart(identity('a', 'a@x'))).toBe(false)
    expect(canTakeApart(identity('a', 'a@x', [{ source: '.mailmap', line: 2 }]))).toBe(true)
    expect(
      canTakeApart(
        identity('a', 'a@x', [
          { source: '.mailmap', line: 2 },
          { source: 'mailmap.file', line: 1 },
        ]),
      ),
    ).toBe(false)
  })
})

describe('describeNote', () => {
  it('says nothing for comments and blanks', () => {
    expect(describeNote(note({ kind: 'comment' })).text).toBe('')
    expect(describeNote(note({ kind: 'blank' })).text).toBe('')
  })

  it('says what a line joins', () => {
    expect(describeNote(note({ identities: 1, commits: 9 }))).toEqual({ text: 'joins 9 commits', tone: 'joins' })
    expect(describeNote(note({ identities: 2, commits: 10 }))).toEqual({ text: 'joins 2 names · 10 commits', tone: 'joins' })
    expect(describeNote(note({ identities: 1, commits: 0, coAuthored: 6 }))).toEqual({ text: 'joins 6 co-author credits', tone: 'joins' })
  })

  it('says when a line does nothing, and why', () => {
    expect(describeNote(note({}))).toEqual({ text: 'matches nobody now', tone: 'idle' })
    expect(describeNote(note({ replaced: true, identities: 0 }))).toEqual({ text: 'a later line replaces this', tone: 'idle' })
    expect(describeNote(note({ kind: 'skipped', why: 'no address in < >' }))).toEqual({ text: 'git skips this: no address in < >', tone: 'bad' })
  })
})

describe('changedLines', () => {
  it('counts lines that differ', () => {
    expect(changedLines('a\nb\n', 'a\nb\n')).toBe(0)
    expect(changedLines('a\nb\n', 'a\nc\n')).toBe(1)
    expect(changedLines('a\n', 'a\nb\nc\n')).toBe(2)
    expect(changedLines('', 'a\n')).toBe(1)
  })
})

describe('summary and sourceNote', () => {
  it('counts names apart from bots', () => {
    expect(summary({ everyone: 8, bots: 1, commits: 212 })).toBe('7 names and 1 bot on 212 commits')
    expect(summary({ everyone: 1, bots: 0, commits: 1 })).toBe('1 name on 1 commit')
  })

  it('says a repository has no mailmap without calling it a problem', () => {
    expect(sourceNote({ source: '.mailmap', exists: false })).toBe('none in this repository')
    expect(sourceNote({ source: '.mailmap', exists: true })).toBe('shared · committed with the repository')
    expect(sourceNote({ source: 'mailmap.file', exists: true })).toContain('read only')
  })
})
