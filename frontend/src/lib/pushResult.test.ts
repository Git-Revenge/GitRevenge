import { describe, expect, it } from 'vitest'
import { pushSummary } from './pushResult'

const d = (name: string, state: string, detail = '') => ({ url: `u:${name}`, name, state, detail })

describe('pushSummary', () => {
  it('names every destination', () => {
    expect(pushSummary([d('github.com', 'pushed'), d('gitlab.com', 'pushed'), d('nas.local', 'up to date')])).toEqual({
      message: 'Pushed to github.com and gitlab.com. nas.local was already up to date.',
      retry: [],
    })
  })

  it('says where it stopped, and offers the rest', () => {
    const s = pushSummary([d('github.com', 'pushed'), d('nas.local', 'failed', "'/srv/x.git' does not appear to be a git repository"), d('codeberg.org', 'not tried')])
    expect(s.message).toBe(
      "Pushed to github.com. nas.local FAILED: '/srv/x.git' does not appear to be a git repository. Not tried: codeberg.org, since git stops at the first place it can't reach.",
    )
    expect(s.retry).toEqual(['u:nas.local', 'u:codeberg.org'])
  })

  it("doesn't offer to retry a refusal", () => {
    const s = pushSummary([d('github.com', 'pushed'), d('gitlab.com', 'rejected', 'fetch first')])
    expect(s.message).toContain('gitlab.com refused it')
    expect(s.retry).toEqual([])
  })
})
