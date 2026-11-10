import { describe, expect, it } from 'vitest'
import { fromText, hiddenNote, outputLine } from './problem'

describe('fromText', () => {
  it('shows a one-line error as it is, with nothing behind Details', () => {
    const p = fromText('Nothing is staged.')
    expect(p).toMatchObject({ title: 'Nothing is staged.', detail: '', output: '', actions: [], text: 'Nothing is staged.' })
  })

  it('keeps a long message to its first line, with all of it behind Details', () => {
    const text = 'Your local changes would be overwritten by checkout:\n\ta.txt\n\nCommit or stash them first.\n'
    const p = fromText(text)
    expect(p.title).toBe('Your local changes would be overwritten by checkout:')
    expect(p.output).toBe(text.trim())
    expect(p.full).toBe(text.trim())
    expect(p.text).toBe(text)
  })

  it('skips leading blank lines', () => {
    expect(fromText('\n\nfailed').title).toBe('failed')
  })
})

describe('outputLine', () => {
  it.each([
    ['error: failed to push some refs', 'error:', ' failed to push some refs'],
    ['fatal: not a git repository', 'fatal:', ' not a git repository'],
    ['remote: error: GH013: Repository rule violations', 'remote: error:', ' GH013: Repository rule violations'],
    [' ! [remote rejected] main -> main (declined)', ' ! [remote rejected]', ' main -> main (declined)'],
    [' ! [rejected]        main -> main (fetch first)', ' ! [rejected]', '        main -> main (fetch first)'],
  ])('marks the failing part of %j', (line, mark, rest) => {
    expect(outputLine(line)).toEqual({ mark, rest })
  })

  it.each(['To github.com:a/b.git', 'remote: Review all repository rules at https://example.com', 'hint: an error: in the middle'])('leaves %j alone', (line) => {
    expect(outputLine(line)).toEqual({ mark: '', rest: line })
  })
})

describe('hiddenNote', () => {
  it('counts the progress lines left out', () => {
    expect(hiddenNote(0)).toBe('What git said.')
    expect(hiddenNote(1)).toBe('What git said, without its line of progress.')
    expect(hiddenNote(14)).toBe('What git said, without its 14 lines of progress.')
  })
})
