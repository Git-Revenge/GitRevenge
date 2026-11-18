import { describe, expect, it } from 'vitest'
import { sharedRead } from './sharedRead'

/** A read that answers when the test says so, and counts how often it was started. */
function controlled() {
  const pending: { resolve: (v: number) => void; reject: (e: Error) => void }[] = []
  let started = 0
  const read = sharedRead(
    () =>
      new Promise<number>((resolve, reject) => {
        started++
        pending.push({ resolve, reject })
      }),
  )
  return { read, pending, started: () => started }
}

/** Lets the turn end: what was asked before it and what's asked after aren't the same moment. */
const nextTurn = () => new Promise((resolve) => setTimeout(resolve, 0))

describe('sharedRead', () => {
  it('reads once for callers in the same turn, and gives them all the answer', async () => {
    const { read, pending, started } = controlled()
    const answers = [read(), read(), read()]
    expect(started()).toBe(1)
    pending[0].resolve(7)
    expect(await Promise.all(answers)).toEqual([7, 7, 7])
  })

  it('reads again for a caller in a later turn once the first read is over', async () => {
    const { read, pending, started } = controlled()
    const first = read()
    pending[0].resolve(1)
    await first
    const second = read()
    expect(started()).toBe(2)
    pending[1].resolve(2)
    expect(await second).toBe(2)
  })

  it('doesn’t hand a later caller a read that began before it asked', async () => {
    const { read, pending, started } = controlled()
    const first = read()
    await nextTurn()
    // Whatever this caller is asking about may have changed since the first read began.
    const second = read()
    expect(started()).toBe(1)
    pending[0].resolve(1)
    expect(await first).toBe(1)
    await nextTurn()
    expect(started()).toBe(2)
    pending[1].resolve(2)
    expect(await second).toBe(2)
  })

  it('gives everyone who asks during a read the same next one', async () => {
    const { read, pending, started } = controlled()
    void read()
    await nextTurn()
    const waiting = [read(), read()]
    await nextTurn()
    waiting.push(read())
    pending[0].resolve(1)
    await nextTurn()
    // One more read for all three, not three.
    expect(started()).toBe(2)
    pending[1].resolve(2)
    expect(await Promise.all(waiting)).toEqual([2, 2, 2])
    await nextTurn()
    expect(started()).toBe(2)
  })

  it('answers in the order asked, so an old answer can’t arrive after a newer one', async () => {
    const { read, pending } = controlled()
    const seen: number[] = []
    void read().then((v) => seen.push(v))
    await nextTurn()
    void read().then((v) => seen.push(v))
    pending[0].resolve(1)
    await nextTurn()
    pending[1].resolve(2)
    await nextTurn()
    expect(seen).toEqual([1, 2])
  })

  it('passes a failure to those waiting on that read, and reads again for the next caller', async () => {
    const { read, pending, started } = controlled()
    const failing = [read(), read()]
    pending[0].reject(new Error('gone'))
    for (const p of failing) await expect(p).rejects.toThrow('gone')
    const after = read()
    expect(started()).toBe(2)
    pending[1].resolve(3)
    expect(await after).toBe(3)
  })

  it('still runs the next read when the one before it failed', async () => {
    const { read, pending, started } = controlled()
    const first = read()
    await nextTurn()
    const second = read()
    pending[0].reject(new Error('gone'))
    await expect(first).rejects.toThrow('gone')
    await nextTurn()
    expect(started()).toBe(2)
    pending[1].resolve(5)
    expect(await second).toBe(5)
  })
})
