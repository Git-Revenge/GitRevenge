import { session } from './session.svelte'

/** What the toolbar says while each kind of operation runs. */
export const NETWORK_LABELS: Record<string, string> = {
  fetch: 'Fetching',
  pull: 'Pulling',
  push: 'Pushing',
  history: 'Fetching history',
  branches: 'Fetching every branch',
  backfill: 'Downloading every file',
  folders: 'Checking out folders',
  maintenance: 'Running maintenance',
}

/**
 * The network operation running in each repository, whoever started it: the toolbar or the
 * Repository panel's This clone section (#1005). The toolbar shows its progress and Stop, and
 * nothing else starts one meanwhile, as the backend allows one at a time.
 */
class Network {
  running = $state<Record<string, string>>({})

  /** Runs fn as op in the repository at path, unless one is running there already. */
  async run(path: string, op: string, fn: () => Promise<unknown>): Promise<void> {
    if (this.running[path]) return
    this.running[path] = op
    try {
      await session.run(fn)
    } finally {
      delete this.running[path]
    }
  }
}

export const network = new Network()
