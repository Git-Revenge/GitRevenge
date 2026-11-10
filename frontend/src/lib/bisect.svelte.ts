import { Bisect, MarkBisect, RunOperationAction, UndoBisect } from '../../wailsjs/go/app/App.js'
import type { app } from '../../wailsjs/go/models'
import { session } from './session.svelte'

export type BisectTerm = 'good' | 'bad' | 'skip'

/** What the graph needs to show a bisect: each marked commit's answer, the commits still in
 * question (null when there were too many to list), the commit under test and the answer. */
export interface BisectMarks {
  terms: Record<string, BisectTerm>
  inQuestion: string[] | null
  current: string
  found: string
  /** What to write beside the current commit, when it isn't one under test: a rebase marks
   * its commits the same way (see operation.ts). */
  currentNote?: string
  /** What its mark holds in place of the question mark. */
  currentGlyph?: string
}

/** The words for an answer, wherever one is shown. */
export const TERM_LABEL: Record<string, string> = { good: 'Good', bad: 'Bad', skip: "Can't tell" }

/** How many answers a bisect is expected to take in all: those given, plus those git says
 * are left. */
export function expectedSteps(info: Pick<app.BisectInfo, 'steps' | 'stepsLeft' | 'firstBad' | 'candidates'>): number {
  if (info.firstBad || info.candidates.length) return info.steps
  return info.steps + Math.max(info.stepsLeft, 1)
}

/** The graph's view of a bisect, or null when none is running. */
export function marksOf(info: app.BisectInfo | null | undefined): BisectMarks | null {
  if (!info?.running) return null
  const terms: Record<string, BisectTerm> = {}
  // A later answer for the same commit replaces an earlier one, as it does for git.
  for (const m of info.marks) terms[m.commit] = m.term as BisectTerm
  return { terms, inQuestion: info.rangeCapped ? null : info.inQuestion, current: info.current, found: info.firstBad }
}

/** Each open repository's bisect, as git reports it, and the request to start one. */
class BisectStore {
  info = $state<Record<string, app.BisectInfo | null>>({})
  /** The start dialog, when open: for which repository, and from which bad commit. */
  starting = $state<{ path: string; bad: string } | null>(null)
  busy = $state(false)

  of(path: string): app.BisectInfo | null {
    const i = this.info[path]
    return i?.running ? i : null
  }

  async refresh(path: string) {
    // The bisect is decoration over the repository: a read that fails shows none.
    this.info[path] = await Bisect(path).catch(() => null)
  }

  private async act(path: string, fn: () => Promise<unknown>) {
    this.busy = true
    await session.run(fn)
    this.busy = false
    await this.refresh(path)
  }

  mark(path: string, term: BisectTerm) {
    return this.act(path, () => MarkBisect(path, term))
  }

  undo(path: string) {
    return this.act(path, () => UndoBisect(path))
  }

  stop(path: string) {
    return this.act(path, () => RunOperationAction(path, 'abort'))
  }

  /** The bisect so far as text, for pasting into an issue. */
  log(path: string): string {
    const i = this.of(path)
    if (!i) return ''
    const lines = i.marks.map((m) => `${TERM_LABEL[m.term] ?? m.term}: ${m.commit.slice(0, 10)} ${m.subject}${m.step ? ` (step ${m.step})` : ' (start)'}`)
    if (i.firstBad) lines.push(`First bad commit: ${i.firstBad}`)
    else if (i.candidates.length) lines.push(`The first bad commit is one of: ${i.candidates.join(', ')}`)
    return lines.join('\n')
  }
}

export const bisect = new BisectStore()
