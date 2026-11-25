/** Text for a repository's Find panel to search for (#980), from Ctrl+Alt+F or the search
 * box's row. The sidebar shows the panel; the panel takes the text and searches. */
class FindAsk {
  ask = $state<{ path: string; text: string; seq: number } | null>(null)

  show(path: string, text: string) {
    this.ask = { path, text, seq: (this.ask?.seq ?? 0) + 1 }
  }
}

export const findAsk = new FindAsk()
