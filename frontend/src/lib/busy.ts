/** Shows the progress cursor everywhere while a read that a click waits on runs, such as a menu
 * reading what it needs before it opens, so the click doesn't look ignored. Several at once
 * keep it until the last ends. */

let running = 0

export async function whileBusy<T>(work: Promise<T>): Promise<T> {
  if (running++ === 0) document.documentElement.classList.add('busy')
  try {
    return await work
  } finally {
    if (--running === 0) document.documentElement.classList.remove('busy')
  }
}
