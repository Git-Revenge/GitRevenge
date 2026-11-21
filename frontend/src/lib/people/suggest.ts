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
