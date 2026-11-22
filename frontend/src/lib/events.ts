import type { app } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import type { GraphProgress } from './graph/rowCache'
import type { ConflictInfo } from './insights/model'
import type { Prompt } from './promptOrigin'
import type { TrailerStatus } from './trailerProgress'

/** What a repository's watcher saw change (watch.Kind in Go). */
export type ChangeKind = 'head' | 'refs' | 'index' | 'state' | 'config' | 'worktree'

/**
 * The events the backend sends, by name, with what each carries. Events aren't bindings, so
 * Wails generates no types for them: this is the one place the frontend says what it expects,
 * and each is named after the Go type that's sent (see its a.emit call).
 */
export interface BackendEvents {
  /** RepoChange: a repository on screen changed on disk, or came on screen and wants reading again. */
  'repo:changed': { path: string; kinds: ChangeKind[] }
  /** UndoChange: a repository's undo or redo history changed. */
  'undo:changed': { path: string }
  /** ConfigChange: a config file outside a repository (the global one, or one open in the Git Config tab) changed. */
  'gitconfig:changed': { path: string }
  /** GraphProgress: how far a repository's commit graph has loaded. */
  'graph:progress': GraphProgress
  /** TrailerProgress: how far reading a repository's trailers has got. */
  'trailers:progress': TrailerStatus & { path: string }
  /** The chips of pinned trailer keys on a repository's graph rows may have changed: its path. */
  'trailers:rows': string
  /** NetProgress: a fetch, pull, push or clone's progress. */
  'net:progress': { path: string; op: string; phase: string; percent: number; line: string }
  /** Prompt: git, ssh or a server is asking for a credential. */
  'askpass:prompt': Prompt
  /** The id of a prompt nobody answered in time. */
  'askpass:expired': string
  /** One repository's result in a group's batch fetch or pull. */
  'group:progress': app.BatchResult & { group: string }
  /** Something worth knowing that isn't an error in what was asked for. */
  notice: { path: string; message: string }
  /** BranchesCleaned: cleanup on pull deleted these. */
  'branches:cleaned': { path: string; branches: string[]; undoLabel: string; merged: boolean }
  /** ConflictResult: one branch's dry-run merge, for Insights. */
  'insights:conflict': ConflictInfo
}

/** Listens for a backend event, and returns the function that stops listening. */
export function onBackend<K extends keyof BackendEvents>(name: K, listen: (payload: BackendEvents[K]) => void): () => void {
  return EventsOn(name, listen)
}
