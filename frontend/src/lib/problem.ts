// An error as the banner shows it: what failed and why, where that leaves you, what you can
// do next, and what git said behind Details. The backend builds these for failures it
// understands (Problem in Go); any other error is text, read into the same shape here.

export interface ProblemAction {
  /** "push-new-branch", "open-url", "pull-then-push", "force-push" or "retry". */
  kind: string
  label: string
  url?: string
}

export interface Problem {
  title: string
  detail: string
  /** Git's own last word, shown as git wrote it. */
  quote: string
  actions: ProblemAction[]
  /** What git said without its progress lines, and how many were left out. */
  output: string
  hidden: number
  /** Everything git said, for a bug report. */
  full: string
  /** The repository it happened in, or '' when it wasn't about one. */
  path: string
  /** The error as text, as code that only wants a message reads it. */
  text: string
  /** Runs what failed again, for "Try again". */
  retry?: () => Promise<unknown>
}

/** Reads an error that is only text. Its first line is the headline; when there's more, all
 * of it waits behind Details so a long message from git can't fill the window. */
export function fromText(text: string): Problem {
  const lines = text.split('\n').filter((l) => l.trim() !== '')
  const more = lines.length > 1
  return { title: lines[0] ?? '', detail: '', quote: '', actions: [], output: more ? text.trim() : '', hidden: 0, full: more ? text.trim() : '', path: '', text }
}

/** A line of git's output, with the part that marks it as the failure split off so it can be
 * drawn in the error colour: "error:", "fatal:", "remote: error:" or " ! [remote rejected]". */
export function outputLine(line: string): { mark: string; rest: string } {
  const m = /^(\s*(?:remote: )?(?:error|fatal):|\s*! \[[^\]]+\])/.exec(line)
  return m ? { mark: m[1], rest: line.slice(m[1].length) } : { mark: '', rest: line }
}

/** The note under git's output: what was left out of it. */
export function hiddenNote(hidden: number): string {
  if (hidden <= 0) return 'What git said.'
  return hidden === 1 ? 'What git said, without its line of progress.' : `What git said, without its ${hidden} lines of progress.`
}
