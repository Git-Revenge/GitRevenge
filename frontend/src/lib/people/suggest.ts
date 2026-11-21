import type { app, git } from '../../../wailsjs/go/models'

/** What a suggestion is about, in a few words. */
export function suggestionLabel(s: Pick<app.Suggestion, 'kind'>): string {
  return s.kind === 'name' ? 'Same name, another address' : 'Same address, another name'
}

/** Every name and address the history has for these people, each once: what the lines that
 * join them have to cover. */
export function identitiesOf(people: Pick<app.SuggestedPerson, 'identities'>[]): git.Identity[] {
  const seen = new Set<string>()
  const out: git.Identity[] = []
  for (const p of people) {
    for (const id of p.identities ?? []) {
      const key = `${id.name}\n${id.email}`
      if (seen.has(key)) continue
      seen.add(key)
      out.push({ name: id.name, email: id.email } as git.Identity)
    }
  }
  return out
}

/** A person from the list, in the shape the Same person dialog takes. */
export function asSuggested(p: app.Person): app.SuggestedPerson {
  return {
    name: p.name,
    email: p.email,
    commits: p.commits,
    coAuthored: p.coAuthored,
    bot: p.bot,
    identities: p.identities.map((id) => ({ name: id.name, email: id.email })),
  } as app.SuggestedPerson
}

/** The dialog's title: how many are being joined. */
export function joinTitle(n: number): string {
  return n === 2 ? 'Two identities, one person' : `${n} identities, one person`
}

/** The heading over a plan's lines. */
export function planHeading(plan: Pick<git.JoinPlan, 'lines'>): string {
  const n = plan.lines.length
  return n === 0 ? 'Nothing to add' : n === 1 ? 'This adds one line' : `This adds ${n} lines`
}

/** What the button that takes the plan says. */
export function planAction(plan: Pick<git.JoinPlan, 'lines'> | null): string {
  return (plan?.lines.length ?? 0) > 1 ? 'Add the lines' : 'Add the line'
}

/** "Name <address>", or whichever part there is. */
export function identityText(id: Pick<git.Identity, 'name' | 'email'>): string {
  if (id.name && id.email) return `${id.name} <${id.email}>`
  return id.name || `<${id.email}>`
}

/**
 * What taking someone apart did to the others on the same line: a line for one address joins
 * every name under it, so the others get lines of their own. Said in words because those
 * lines have to spell the names the first line left out.
 */
export function apartNote(plan: Pick<git.ApartPlan, 'kept' | 'lost' | 'added'>): string {
  const out: string[] = []
  const all = (ids: git.Identity[]) => ids.map(identityText).join(', ')
  if (plan.kept.length) {
    const one = plan.kept.length === 1
    out.push(`That line also joined ${all(plan.kept)}. ${one ? "It's" : "They're"} still joined, by ${plan.added.length === 1 ? `a line that names ${one ? 'it' : 'them'}` : 'lines that name them'}.`)
  }
  if (plan.lost.length) out.push(`${all(plan.lost)} came apart as well: no line could keep ${plan.lost.length === 1 ? 'it' : 'them'} joined.`)
  return out.join(' ')
}
