import { describe, expect, it, vi } from 'vitest'
import { CHUNK, RowCache, type Page, type Row } from './rowCache'

function rows(start: number, count: number): Row[] {
  return Array.from({ length: count }, (_, i) => ({
    index: start + i,
    col: 0,
    color: 0,
    up: [],
    down: [],
    width: 1,
    id: String(start + i),
    parents: 1,
    author: 'a',
    authorTime: 0,
    subject: 's',
  }))
}

/** A fake backend with `total` rows, `available` of which are loaded so far. */
function backend(total: number, available = total) {
  const state = { total, available }
  const fetch = vi.fn(async (start: number, count: number): Promise<Page> => {
    const n = Math.max(0, Math.min(count, state.available - start))
    return { rows: rows(start, n), refs: {}, total: state.available, loading: state.available < state.total, paused: false, estimate: state.total }
  })
  return { state, fetch }
}

const flush = () => new Promise((r) => setTimeout(r, 0))

describe('a failed fetch', () => {
  it('reports the error without repainting, and clears it once a fetch works', async () => {
    let fail = true
    const fetch = vi.fn(async (start: number, count: number): Promise<Page> => {
      if (fail) throw new Error('no such repository')
      return { rows: rows(start, count), refs: {}, total: 1000, loading: false, paused: false, estimate: 0 }
    })
    const onUpdate = vi.fn()
    const onError = vi.fn()
    const cache = new RowCache(fetch, onUpdate, onError)
    cache.ensure(0, 10)
    await vi.waitFor(() => expect(onError).toHaveBeenCalledTimes(1))
    expect(String(onError.mock.calls[0][0])).toContain('no such repository')
    // A repaint would call ensure() again, and retry at the frame rate.
    expect(onUpdate).not.toHaveBeenCalled()
    fail = false
    cache.ensure(0, 10)
    await vi.waitFor(() => expect(onUpdate).toHaveBeenCalled())
    expect(onError).toHaveBeenLastCalledWith(null)
  })
})

describe('RowCache', () => {
  it('fetches whole chunks once', async () => {
    const be = backend(1000)
    const cache = new RowCache(be.fetch, () => {})
    cache.ensure(10, 50)
    cache.ensure(20, 60)
    await flush()
    expect(be.fetch).toHaveBeenCalledTimes(1)
    expect(be.fetch).toHaveBeenCalledWith(0, CHUNK)
    expect(cache.get(42)?.index).toBe(42)
    expect(cache.total).toBe(1000)
  })

  it('spans chunk boundaries', async () => {
    const be = backend(1000)
    const cache = new RowCache(be.fetch, () => {})
    cache.ensure(CHUNK - 5, CHUNK + 5)
    await flush()
    expect(be.fetch.mock.calls.map((c) => c[0])).toEqual([0, CHUNK])
  })

  it('refetches a short chunk while history is still loading', async () => {
    const be = backend(1000, 50)
    const cache = new RowCache(be.fetch, () => {})
    cache.ensure(0, 100)
    await flush()
    expect(cache.get(60)).toBeUndefined()
    be.state.available = 1000
    cache.setTotal(1000, false)
    cache.ensure(0, 100)
    await flush()
    expect(cache.get(60)?.index).toBe(60)
  })

  it('does not refetch the final short chunk once loading is done', async () => {
    const be = backend(250)
    const cache = new RowCache(be.fetch, () => {})
    cache.ensure(CHUNK, CHUNK + 10)
    await flush()
    cache.ensure(CHUNK, CHUNK + 10)
    await flush()
    expect(be.fetch).toHaveBeenCalledTimes(1)
  })

  it('ignores responses from before an invalidate', async () => {
    const be = backend(1000)
    const cache = new RowCache(be.fetch, () => {})
    cache.ensure(0, 10)
    cache.invalidate()
    await flush()
    expect(cache.get(0)).toBeUndefined()
  })
})
