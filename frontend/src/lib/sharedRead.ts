/**
 * Wraps a read of something that changes under the app (a repository's status, its refs) for
 * several views that each want it fresh, so they stop paying for it once each.
 *
 * - Calls made in the same turn share one read: nothing can have changed between them. Two
 *   views that both re-read when the watcher reports a change are the usual case.
 * - A call that finds a read from an earlier turn still running can't use that one, since what
 *   it's asking about may have changed after that read began. It gets the next read, which
 *   starts when the running one ends, and so does every other call that arrives meanwhile.
 *
 * Answers therefore arrive in the order they were asked for, so a slow old answer can't land
 * on top of a newer one, and a burst of requests costs two reads however many ask.
 */
export function sharedRead<T>(read: () => Promise<T>): () => Promise<T> {
  let running: Promise<T> | null = null
  let sameTurn = false
  let queued: Promise<T> | null = null

  const begin = (): Promise<T> => {
    const mine = read()
    running = mine
    sameTurn = true
    queueMicrotask(() => {
      if (running === mine) sameTurn = false
    })
    const end = () => {
      if (running === mine) running = null
    }
    mine.then(end, end)
    return mine
  }

  return () => {
    if (queued) return queued
    if (!running) return begin()
    if (sameTurn) return running
    const settled = () => {}
    queued = running.then(settled, settled).then(() => {
      queued = null
      return begin()
    })
    return queued
  }
}
