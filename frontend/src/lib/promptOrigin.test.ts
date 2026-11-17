import { describe, expect, it } from 'vitest'
import { askedDuring, unexpectedHost, type Prompt } from './promptOrigin'

const prompt = (p: Partial<Prompt>): Prompt => ({ id: '1', text: 'Password: ', kind: 'secret', ...p })

describe('askedDuring', () => {
  it('names the operation and the repository', () => {
    expect(askedDuring(prompt({ op: 'fetch', repo: 'tidewater' }))).toBe('Asked during a fetch in tidewater.')
    expect(askedDuring(prompt({ op: 'push', repo: 'tidewater' }))).toBe('Asked during a push in tidewater.')
    expect(askedDuring(prompt({ op: 'clone', repo: 'tidewater' }))).toBe('Asked while cloning into tidewater.')
  })

  it('says nothing it does not know', () => {
    expect(askedDuring(prompt({}))).toBe('')
    expect(askedDuring(prompt({ op: 'something-else', repo: 'tidewater' }))).toBe('')
    expect(askedDuring(prompt({ op: 'pull' }))).toBe('Asked during a pull.')
  })
})

describe('unexpectedHost', () => {
  it('warns when the prompt names a server that is not a remote', () => {
    const text = unexpectedHost(prompt({ op: 'fetch', host: 'github.com', unexpected: true }))
    expect(text).toContain('github.com')
    expect(text).toContain("isn't one of this repository's remotes")
    expect(unexpectedHost(prompt({ op: 'clone', host: 'github.com', unexpected: true }))).toContain("isn't the server being cloned from")
  })

  it('is silent for the expected server, or when none is named', () => {
    expect(unexpectedHost(prompt({ host: 'github.com', unexpected: false }))).toBe('')
    expect(unexpectedHost(prompt({ unexpected: true }))).toBe('')
  })
})
