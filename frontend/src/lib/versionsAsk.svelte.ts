/** A request to compare two versions of a branch (#1040), from wherever a branch was just
 * rewritten: a toast, the reflog, a menu. The repository view on screen takes it, works out
 * where each version starts, and shows the comparison, or opens Compare two versions when it
 * can't tell. */
export interface VersionsAskFor {
  path: string
  branch: string
  /** The old version's tip. "" takes the branch's latest tip from before a rewrite, from its
   * reflog. */
  oldTip: string
  /** How the old version is named: "Before the rebase", or an upstream's name. */
  oldLabel: string
}

class VersionsAsk {
  ask = $state<(VersionsAskFor & { seq: number }) | null>(null)

  show(a: VersionsAskFor) {
    this.ask = { ...a, seq: (this.ask?.seq ?? 0) + 1 }
  }
}

export const versionsAsk = new VersionsAsk()
