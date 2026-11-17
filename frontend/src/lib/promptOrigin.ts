/** A credential or host-key prompt relayed from git or ssh (app.Prompt in Go). */
export interface Prompt {
  id: string
  /** What git, ssh or the server wrote. A server can write anything here. */
  text: string
  kind: 'text' | 'secret' | 'hostkey'
  /** The operation that's running: "fetch", "pull", "push" or "clone". */
  op?: string
  /** The repository's folder name. */
  repo?: string
  /** The server the prompt names, if it names one. */
  host?: string
  /** The named server isn't one the operation was set up to talk to. */
  unexpected?: boolean
}

const OPS: Record<string, string> = { fetch: 'a fetch', pull: 'a pull', push: 'a push' }

/**
 * Where a prompt came from, in the app's own words: the dialog's text is the server's, so this
 * line is what says which operation is really asking. Empty when the app doesn't know.
 */
export function askedDuring(p: Prompt): string {
  if (p.op === 'clone') return p.repo ? `Asked while cloning into ${p.repo}.` : 'Asked while cloning.'
  const op = p.op ? OPS[p.op] : undefined
  if (!op) return ''
  return p.repo ? `Asked during ${op} in ${p.repo}.` : `Asked during ${op}.`
}

/** What to say when a prompt names a server the operation wasn't set up to talk to, or "". */
export function unexpectedHost(p: Prompt): string {
  if (!p.unexpected || !p.host) return ''
  const where = p.op === 'clone' ? "isn't the server being cloned from" : "isn't one of this repository's remotes"
  return `This names ${p.host}, which ${where}. That's expected with an SSH alias, a URL rewrite or a redirect. If you use none of those, cancel: the server may be asking for a password that isn't its own.`
}
