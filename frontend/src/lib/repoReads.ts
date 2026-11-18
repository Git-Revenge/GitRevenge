import { Refs, WorkingTree } from '../../wailsjs/go/app/App.js'
import type { app, git } from '../../wailsjs/go/models'
import { sharedRead } from './sharedRead'

/**
 * The reads of a repository that more than one of its views makes, each shared (see
 * sharedRead): the views all re-read on the same change, and each used to ask the backend for
 * itself. The working tree is a `git status` and more, and the refs can be tens of thousands.
 *
 * A view still decides when to read and keeps its own copy of the answer. What it must not do
 * is change that answer in place, since another view may hold the same one.
 */
export interface RepoReads {
  /** Status, staged and unstaged counts, and what a commit would be made with. */
  workingTree: () => Promise<app.WorkingTree>
  /** Branches, remote branches, tags and stashes, as the displayed graph has them. */
  refs: () => Promise<git.Ref[]>
}

const byPath = new Map<string, RepoReads>()

/** The shared reads of the repository at path. */
export function repoReads(path: string): RepoReads {
  let reads = byPath.get(path)
  if (!reads) {
    reads = { workingTree: sharedRead(() => WorkingTree(path)), refs: sharedRead(() => Refs(path)) }
    byPath.set(path, reads)
  }
  return reads
}

/** Forgets a repository whose tab has closed. */
export function forgetRepoReads(path: string) {
  byPath.delete(path)
}
