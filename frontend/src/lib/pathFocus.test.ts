import { describe, expect, it } from 'vitest'
import { underPath, workingTreeSaid } from './pathFocus'

describe('a path focus', () => {
  it('knows what is under a folder, and a file', () => {
    expect(underPath('src/alerts/sms.js', 'src/alerts/')).toBe(true)
    expect(underPath('src/alerts-old/x.js', 'src/alerts/')).toBe(false)
    expect(underPath('src/alerts/sms.js', 'src/alerts/sms.js')).toBe(true)
    expect(underPath('src/alerts/sms.js.bak', 'src/alerts/sms.js')).toBe(false)
    expect(underPath('anything', '')).toBe(true)
  })

  it('counts only the changes under it in the working tree row', () => {
    const files = ['src/alerts/sms.js', 'src/alerts/gusts.js', 'README.md']
    expect(workingTreeSaid(files, '')).toEqual({ changes: 3, title: '3 files changed' })
    expect(workingTreeSaid(files, 'src/alerts/')).toEqual({ changes: 2, title: '2 files changed in src/alerts/' })
    expect(workingTreeSaid(files, 'src/alerts/sms.js')).toEqual({ changes: 1, title: '1 file changed in src/alerts/sms.js' })
    expect(workingTreeSaid(files, 'docs/')).toEqual({ changes: 0, title: 'Nothing changed in docs/' })
    expect(workingTreeSaid([], '')).toEqual({ changes: 0, title: 'Working tree' })
    expect(workingTreeSaid(null, 'src/')).toEqual({ changes: null, title: 'Working tree' })
  })
})
