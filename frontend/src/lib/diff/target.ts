import type { DiffTarget } from './DiffView.svelte'

/** Whether a diff shows commits, which nothing stages or changes: a commit's, or one commit
 * against another (#1019). */
export const ofCommits = (mode: DiffTarget['mode'] | undefined) => mode === 'commit' || mode === 'compare'

/** The commit a diff's new side is from, for blame and the like: a comparison's second. */
export const newSideCommit = (t: Pick<DiffTarget, 'mode' | 'commit'>) => (t.mode === 'compare' ? t.commit?.split('..')[1] : t.mode === 'commit' ? t.commit : undefined)
