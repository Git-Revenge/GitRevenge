import { DUR, easeInk, reducedMotion } from '../ui/motion'

/** One value on its way from `from` to `to`, having set off at `start` (ms). */
export interface Tween {
  from: number
  to: number
  start: number
}

/** Where a tween is at `now`, eased with --ease-ink over `duration` ms. */
export function at(t: Tween, now: number, duration: number): number {
  if (duration <= 0 || t.from === t.to) return t.to
  const p = Math.min(1, Math.max(0, (now - t.start) / duration))
  return t.from + (t.to - t.from) * easeInk(p)
}

/** The wash over a row that was just jumped to, at `t` from 0 to 1 of the landing: solid for
 * the first 40%, then fading, so the eye finds the row and is then left alone. */
export function landWash(t: number): number {
  if (t <= 0.4) return 1
  if (t >= 1) return 0
  return 1 - easeInk((t - 0.4) / 0.6)
}

/** Whether a tween has arrived. */
export function settled(t: Tween, now: number, duration: number): boolean {
  return t.from === t.to || now - t.start >= duration
}

/**
 * The canvas's eased values, one per row and channel (selection, hover, search dimming). The
 * canvas repaints from scratch each frame, so these remember where each wash was and move it
 * toward its target over --dur-base; a row seen for the first time starts at its target, so
 * rows scrolling into view don't fade in. Under reduced motion every value is its target.
 */
export class Washes {
  private readonly map = new Map<string, Tween>()

  constructor(private readonly duration: () => number = () => (reducedMotion() ? 0 : DUR.base)) {}

  /** Points `key` at `target` and returns its value at `now`. */
  value(key: string, target: number, now: number): number {
    const d = this.duration()
    let t = this.map.get(key)
    if (!t) {
      t = { from: target, to: target, start: now }
      this.map.set(key, t)
      return target
    }
    if (t.to !== target) {
      t = { from: at(t, now, d), to: target, start: now }
      this.map.set(key, t)
    }
    return at(t, now, d)
  }

  /** Whether any value is still on its way, so the caller should paint another frame. */
  busy(now: number): boolean {
    const d = this.duration()
    for (const t of this.map.values()) if (!settled(t, now, d)) return true
    return false
  }

  /** Forgets the keys `keep` rejects, such as rows that scrolled far out of view. */
  prune(keep: (key: string) => boolean): void {
    for (const key of [...this.map.keys()]) if (!keep(key)) this.map.delete(key)
  }

  get size(): number {
    return this.map.size
  }
}
