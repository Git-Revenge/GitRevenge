// The keys the Git Config table edits with a typed control (#225). Anything else gets a text
// field: the catalog helps, it never limits what can be set.

export type KeyType = 'bool' | 'enum' | 'int' | 'path' | 'color' | 'string'

export interface KeyInfo {
  key: string
  type: KeyType
  /** The accepted values of an enum, in the order offered. */
  values?: string[]
  description: string
  /** Git appends rather than replaces values of this key. */
  multi?: boolean
}

const k = (key: string, type: KeyType, description: string, extra: Partial<KeyInfo> = {}): KeyInfo => ({ key, type, description, ...extra })

export const CATALOG: KeyInfo[] = [
  k('user.name', 'string', 'Your name, recorded in commits.'),
  k('user.email', 'string', 'Your email address, recorded in commits.'),
  k('user.signingKey', 'string', 'The key that signs commits and tags: a GPG key ID, or an SSH public key or its path.'),
  k('init.defaultBranch', 'string', 'The first branch of a new repository.'),
  k('core.editor', 'string', 'The editor git opens for commit messages and interactive rebases.'),
  k('core.autocrlf', 'enum', 'Converts line endings on checkout and commit. Usually true on Windows, input elsewhere.', { values: ['true', 'false', 'input'] }),
  k('core.eol', 'enum', 'The line ending for text files in the working tree.', { values: ['native', 'lf', 'crlf'] }),
  k('core.excludesFile', 'path', 'A gitignore file that applies to every repository.'),
  k('core.hooksPath', 'path', 'A folder of hooks to use instead of .git/hooks.'),
  k('core.ignoreCase', 'bool', 'Treats file names that differ only in case as the same file.'),
  k('core.fileMode', 'bool', 'Tracks the executable bit of files.'),
  k('core.longpaths', 'bool', 'Allows paths longer than 260 characters (Git for Windows).'),
  k('core.fsmonitor', 'bool', 'Uses a file system monitor to speed up status in large repositories.'),
  k('core.untrackedCache', 'bool', 'Caches untracked files to speed up status.'),
  k('core.pager', 'string', 'The pager for long output, such as less.'),
  k('pull.rebase', 'enum', 'Rebases instead of merging when pulling.', { values: ['false', 'true', 'merges', 'interactive'] }),
  k('pull.ff', 'enum', 'Whether a pull may fast-forward: only refuses anything else.', { values: ['true', 'false', 'only'] }),
  k('push.default', 'enum', 'What git push pushes when no branch is named.', { values: ['simple', 'current', 'upstream', 'matching', 'nothing'] }),
  k('push.autoSetupRemote', 'bool', 'Sets the upstream on the first push of a new branch.'),
  k('push.followTags', 'bool', 'Also pushes annotated tags that point at pushed commits.'),
  k('push.gpgSign', 'enum', 'Signs pushes, where the server supports it.', { values: ['false', 'true', 'if-asked'] }),
  k('fetch.prune', 'bool', 'Removes remote-tracking branches that no longer exist on the remote.'),
  k('fetch.pruneTags', 'bool', 'Removes local tags that no longer exist on the remote.'),
  k('fetch.parallel', 'int', 'How many fetches run at once (0 picks a default).'),
  k('merge.ff', 'enum', 'Whether a merge may fast-forward: only refuses to create a merge commit.', { values: ['true', 'false', 'only'] }),
  k('merge.conflictStyle', 'enum', 'How conflicts are written into files.', { values: ['merge', 'diff3', 'zdiff3'] }),
  k('merge.tool', 'string', 'The merge tool git mergetool runs.'),
  k('rebase.autoStash', 'bool', 'Stashes local changes before a rebase and reapplies them after.'),
  k('rebase.autoSquash', 'bool', 'Moves fixup! and squash! commits into place in interactive rebases.'),
  k('rebase.updateRefs', 'bool', 'Moves branches that point at rebased commits along with them.'),
  k('rerere.enabled', 'bool', 'Records conflict resolutions and replays them when the same conflict comes back.'),
  k('commit.gpgSign', 'bool', 'Signs every commit.'),
  k('commit.template', 'path', 'A file whose text starts every commit message.'),
  k('commit.verbose', 'bool', 'Shows the diff below the commit message in the editor.'),
  k('tag.gpgSign', 'bool', 'Signs every annotated tag.'),
  k('gpg.format', 'enum', 'The kind of key that signs commits and tags.', { values: ['openpgp', 'x509', 'ssh'] }),
  k('gpg.program', 'path', 'The gpg program to sign with.'),
  k('gpg.ssh.allowedSignersFile', 'path', 'The file listing trusted SSH signing keys, for verifying signatures.'),
  k('credential.helper', 'string', 'The program that stores and supplies passwords and tokens.', { multi: true }),
  k('diff.colorMoved', 'enum', 'Colours moved lines differently from added and removed ones.', { values: ['no', 'default', 'plain', 'blocks', 'zebra', 'dimmed-zebra'] }),
  k('diff.algorithm', 'enum', 'The diff algorithm.', { values: ['default', 'myers', 'minimal', 'patience', 'histogram'] }),
  k('diff.tool', 'string', 'The diff tool git difftool runs.'),
  k('color.ui', 'enum', 'Whether git colours its output.', { values: ['auto', 'always', 'never', 'true', 'false'] }),
  k('help.autoCorrect', 'string', 'Runs a mistyped command’s likely meaning: a delay in tenths of a second, immediate, prompt or never.'),
  k('branch.sort', 'string', 'How git branch sorts its list, such as -committerdate.'),
  k('log.date', 'string', 'How dates are shown in logs, such as iso or relative.'),
  k('status.showUntrackedFiles', 'enum', 'How untracked files are listed.', { values: ['normal', 'no', 'all'] }),
  k('safe.directory', 'path', 'A repository owned by someone else that git may still use.', { multi: true }),
  k('include.path', 'path', 'Another config file to read here.', { multi: true }),
  k('remote.<name>.fetch', 'string', 'Which refs a fetch from this remote gets.', { multi: true }),
  k('remote.<name>.url', 'string', 'The remote’s address.'),
  k('remote.<name>.pushurl', 'string', 'Where pushes to this remote go, when different from its url.', { multi: true }),
  // Git Revenge's own settings for a repository (#526). The repository's config wins over the
  // global one, and its list replaces the global list.
  k('gitrevenge.cleanupOnPull', 'bool', 'Git Revenge: after each pull, deletes local branches whose upstream is gone and that have nothing unpushed.'),
  k('gitrevenge.syncNotes', 'bool', 'Git Revenge: fetches git notes on fetch and pull, and pushes changed notes on push.'),
  k('gitrevenge.hide', 'string', 'Git Revenge: a pattern of branches to leave out of the sidebar and graph, such as wip/*. One empty value means none here.', { multi: true }),
  k('gitrevenge.hideKind', 'enum', 'Git Revenge: a branch kind whose branches are hidden.', { multi: true, values: ['trunk', 'release', 'environment', 'docs', 'bot', 'feature'] }),
  k('gitrevenge.webLinks', 'bool', 'Git Revenge: turns web addresses written out in full into links. On unless set to false.'),
  k('gitrevenge-kind.<name>.pattern', 'string', 'Git Revenge: a pattern of branches of this kind, such as release/*. One empty value means the kind matches nothing.', { multi: true }),
  k('gitrevenge-insights.stale', 'int', 'Git Revenge: days without a commit before Insights calls a branch stale.'),
  k('gitrevenge-insights.longLived', 'int', 'Git Revenge: days since a branch left its target before Insights calls it long-lived.'),
  k('gitrevenge-insights.behind', 'int', 'Git Revenge: commits behind its target before Insights flags a branch.'),
  k('gitrevenge-insights.size', 'int', 'Git Revenge: lines changed before Insights calls a branch large.'),
  k('branch.<name>.gitrevengeStackParent', 'string', 'Git Revenge: the branch this one is stacked on, set by hand. Empty means not stacked.'),
]

/** The catalog entry for a full key, matching <name> placeholders and ignoring the case of
 * section and variable names. */
export function keyInfo(key: string): KeyInfo | undefined {
  const [section, sub, name] = splitKey(key)
  return CATALOG.find((info) => {
    const [s, isub, n] = splitKey(info.key)
    return s === section && n === name && (isub === sub || (isub === '<name>' && sub !== ''))
  })
}

/** Splits a key into its lower-cased section, its subsection (case kept) and its lower-cased
 * variable name. A subsection can itself contain dots. */
export function splitKey(key: string): [string, string, string] {
  const i = key.indexOf('.')
  const j = key.lastIndexOf('.')
  if (i < 0) return [key.toLowerCase(), '', '']
  if (i === j) return [key.slice(0, i).toLowerCase(), '', key.slice(j + 1).toLowerCase()]
  return [key.slice(0, i).toLowerCase(), key.slice(i + 1, j), key.slice(j + 1).toLowerCase()]
}

/** A key in comparable form: section and name lower-cased, subsection kept. */
export function canonicalKey(key: string): string {
  const [s, sub, n] = splitKey(key)
  return sub ? `${s}.${sub}.${n}` : `${s}.${n}`
}

/** The section header a key lives under, as written in the file: core, or remote "origin". */
export function sectionOf(key: string): string {
  const [s, sub] = splitKey(key)
  return sub ? `${s} "${sub}"` : s
}

/** Git's reading of a boolean, or undefined if the value isn't one. */
export function parseBool(v: string, implicit = false): boolean | undefined {
  if (implicit) return true
  switch (v.toLowerCase()) {
    case 'true':
    case 'yes':
    case 'on':
    case '1':
      return true
    case 'false':
    case 'no':
    case 'off':
    case '0':
    case '':
      return false
  }
  return undefined
}
