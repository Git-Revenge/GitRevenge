/**
 * Trailer care as the app shell knows it: whether it's open, and for which repository. That's
 * all Settings, the sidebar's pill and the command palette need, so it's all that loads with
 * the app. The dialog itself loads when it's asked for (the main chunk has a size budget).
 */
class TrailerCareDialog {
  /** The repository whose trailers are being cared for, or "" while it's closed. */
  repo = $state('')

  show(repo: string) {
    this.repo = repo
  }

  close() {
    this.repo = ''
  }
}

export const trailerCare = new TrailerCareDialog()
