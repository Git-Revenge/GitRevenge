import { describe, expect, it } from 'vitest'
import { answerPinOffer, keyId, pinLabel, pinOffer } from './filters'

describe('keyId', () => {
  it('joins a key however it is spelled, as the trailer index does', () => {
    expect(keyId('Reviewed-By')).toBe('reviewed-by')
    expect(keyId('  Changelog ')).toBe('changelog')
    for (const spelling of ['Closes', 'closed', 'CLOSE']) expect(keyId(spelling)).toBe('closes')
    for (const spelling of ['Fix', 'fixed', 'Fixes']) expect(keyId(spelling)).toBe('fixes')
    for (const spelling of ['Ref', 'Reference', 'references', 'Refs']) expect(keyId(spelling)).toBe('refs')
    expect(keyId('Resolved')).toBe('resolves')
    // A key that only looks like one stays itself.
    expect(keyId('Closing')).toBe('closing')
  })
})

describe('pinning', () => {
  const store = () => {
    const kept: Record<string, string> = {}
    return { getItem: (k: string) => kept[k] ?? null, setItem: (k: string, v: string) => void (kept[k] = v) }
  }
  const k = (id: string, kind: string, held = '') => ({ id, kind, held })
  const keys = [k('signed-off-by', 'person'), k('change-id', 'identifier', 'identifier'), k('changelog', 'label'), k('area', 'label')]

  it('says what the pin does', () => {
    expect(pinLabel('Changelog', false)).toBe('Pin Changelog to the graph’s rows')
    expect(pinLabel('Changelog', true)).toBe('Unpin Changelog from the graph’s rows')
  })

  it('offers the most used label, once for a repository', () => {
    const s = store()
    expect(pinOffer(keys, [], '/r', s)?.id).toBe('changelog')
    answerPinOffer('/r', s)
    expect(pinOffer(keys, [], '/r', s)).toBeUndefined()
    // Another repository hasn't been asked.
    expect(pinOffer(keys, [], '/other', s)?.id).toBe('changelog')
  })

  it('offers nothing once something is pinned, or where there is no label to offer', () => {
    expect(pinOffer(keys, ['closes'], '/r', store())).toBeUndefined()
    expect(pinOffer([k('signed-off-by', 'person'), k('change-id', 'identifier', 'identifier')], [], '/r', store())).toBeUndefined()
    expect(pinOffer([k('changelog', 'label', 'rare')], [], '/r', store())).toBeUndefined()
  })

  it('offers nothing when it cannot remember the answer', () => {
    const blocked = {
      getItem: () => {
        throw new Error('blocked')
      },
      setItem: () => {
        throw new Error('blocked')
      },
    }
    expect(pinOffer(keys, [], '/r', blocked)).toBeUndefined()
    expect(() => answerPinOffer('/r', blocked)).not.toThrow()
  })
})
