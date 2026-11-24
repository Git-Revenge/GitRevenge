/** The Push button's words (#927): where a push goes, by git's rules and the Pushing
 * settings, so it never surprises. */

import type { git } from '../../wailsjs/go/models'

export interface PushWords {
  /** The button's label: "Push", "Push → fork" when a setting sends it elsewhere than its
   * upstream, or "Push…" when it'll ask. */
  label: string
  /** The tooltip: exactly where, and why. */
  title: string
  /** For screen readers. */
  spoken: string
}

export function pushWords(plan: git.PushPlan | null | undefined): PushWords {
  if (!plan) return { label: 'Push', title: 'Push the current branch', spoken: 'Push' }
  if (plan.refusal) return { label: 'Push', title: plan.refusal, spoken: `Push. ${plan.refusal}` }
  if (plan.ask) {
    const title = `Push ${plan.branch}: it hasn't been pushed, so this asks where its first push goes`
    return { label: 'Push…', title, spoken: title }
  }
  const where = `${plan.remote}/${plan.ref}`
  const chosen = plan.why === 'branch.' + plan.branch + '.pushRemote' || plan.why === 'remote.pushDefault'
  const why = chosen ? ` (${plan.why})` : ''
  const first = plan.setUpstream ? `, its first push, and track it there` : ''
  const title = `Push ${plan.branch} to ${where}${why}${first}`
  return { label: chosen ? `Push → ${plan.remote}` : 'Push', title, spoken: title }
}
