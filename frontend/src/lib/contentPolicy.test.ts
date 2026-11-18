import { describe, expect, it } from 'vitest'
import { contentPolicy } from './contentPolicy'

const directives = (policy: string) => Object.fromEntries(policy.split('; ').map((d) => [d.split(' ')[0], d.split(' ').slice(1)]))

describe('contentPolicy', () => {
  const shipped = directives(contentPolicy())

  it('allows nothing it does not name', () => {
    expect(shipped['default-src']).toEqual(["'none'"])
    expect(shipped['base-uri']).toEqual(["'none'"])
    expect(shipped['form-action']).toEqual(["'none'"])
  })

  it('runs only the app’s own script files', () => {
    const scripts = shipped['script-src']
    expect(scripts).toContain("'self'")
    for (const open of ["'unsafe-inline'", "'unsafe-eval'", 'data:', 'blob:', '*', 'https:', 'http:']) expect(scripts).not.toContain(open)
  })

  it('names no place on the network, in any directive', () => {
    const sources = Object.values(shipped).flat()
    // The app's own origin on Windows is the one http address there is.
    expect(sources.filter((s) => /^(https?|wss?):/.test(s))).toEqual(sources.filter((s) => s === 'http://wails.localhost'))
    expect(sources).not.toContain('*')
  })

  it('opens up for hot reload in development only', () => {
    const dev = directives(contentPolicy(true))
    expect(dev['connect-src']).toContain('ws://localhost:*')
    expect(shipped['connect-src']).not.toContain('ws://localhost:*')
    expect(dev['script-src']).not.toContain("'unsafe-eval'")
  })
})
