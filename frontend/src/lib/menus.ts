import { BranchAction, BranchMergeState, HideBranch, Integrate, MarkBranchKind, PullBranch, PushRef, Refs, Restack, SetStackParent, StashAction } from '../../wailsjs/go/app/App.js'
import { CONFIRM_DELETE, KINDS, kindWarning } from './branchKinds'
import type { MenuItem } from './ContextMenu.svelte'
import type { DialogSpec } from './Dialog.svelte'
import type { Ref, Row } from './graph/rowCache'
import { errorText } from './errors'
import { session } from './session.svelte'

/** What the menus need from the view that opens them. */
export interface MenuContext {
  path: string
  /** The checked-out branch, or "" when HEAD is detached. */
  current: () => string
  openDialog: (spec: DialogSpec) => void
  /** Opens the interactive rebase editor for the commits after base. */
  openRebase: (base: string) => void
  /** Opens the reflog browser on a ref ("HEAD" or refs/heads/…). */
  openReflog: (ref: string) => void
}

/** Joins groups of items, drawing a rule between non-empty groups. */
export function groups(...lists: MenuItem[][]): MenuItem[] {
  const out: MenuItem[] = []
  const seen = new Set<string>()
  for (const list of lists) {
    let rule = out.length > 0
    for (const item of list) {
      rule ||= !!item.separator
      // A commit's menu combines its refs' items, and a branch and its remote offer the same
      // Mark and Hide. A dropped item's rule passes to the next one.
      if (seen.has(item.label)) continue
      seen.add(item.label)
      out.push(rule ? { ...item, separator: true } : item)
      rule = false
    }
  }
  return out
}

const integrate = (path: string, action: string, ...args: string[]) => () =>
  session.run(() => Integrate(path, action, args))

/** Merge and rebase items for integrating target (a ref name or commit) with the current branch. */
function integrateItems(ctx: MenuContext, target: string, label: string): MenuItem[] {
  const cur = ctx.current()
  if (!cur) return []
  return [
    { label: `Merge ${label} into ${cur}`, icon: 'merge', run: integrate(ctx.path, 'merge', target) },
    { label: `Merge ${label} into ${cur} (no fast-forward)`, icon: 'merge', run: integrate(ctx.path, 'merge', target, 'no-ff') },
    { label: `Rebase ${cur} onto ${label}`, icon: 'rebase', run: integrate(ctx.path, 'rebase', target) },
  ]
}

/** Runs a backend call for a dialog: an error message keeps the dialog open. */
async function attempt(fn: () => Promise<unknown>): Promise<string | void> {
  try {
    await fn()
  } catch (e) {
    return errorText(e)
  }
}

const branch = (path: string, action: string, ...args: string[]) => BranchAction(path, action, args)

/** Whether a branch's work is on another branch (git.BranchMerge in Go). */
export interface BranchMerge {
  target: string
  how: string
  unmerged: number
}

/** Confirms deleting a local branch. If its work is known to be on another branch there's
 * nothing to force; otherwise it takes a tick, and says what would be lost if it can. */
export function deleteBranchDialog(path: string, ref: Ref, merge: BranchMerge | null): DialogSpec {
  const warning = kindWarning(ref.short, ref.branchKind, CONFIRM_DELETE)
  const text = (...parts: (string | undefined)[]) => parts.filter(Boolean).join(' ')
  const title = `Delete branch ${ref.short}?`
  if (merge?.how) {
    const landed = merge.how === 'squashed' ? `Its changes are already on ${merge.target}, from a squash or a rebase.` : `It's already merged into ${merge.target}.`
    return { title, message: text(warning, landed), submit: 'Delete', danger: !!warning, run: () => attempt(() => branch(path, 'delete', ref.short, 'true')) }
  }
  const losing = merge?.target && merge.unmerged ? `${merge.unmerged === 1 ? '1 commit is' : `${merge.unmerged} commits are`} not on ${merge.target}.` : 'The branch is removed locally. Its commits stay reachable from other branches if merged.'
  return {
    title,
    message: text(warning, losing),
    fields: [{ key: 'force', label: 'Delete even if it has unmerged commits', type: 'checkbox' }],
    submit: 'Delete',
    danger: true,
    run: (v) => attempt(() => branch(path, 'delete', ref.short, v.force ? 'true' : '')),
  }
}

/** The remote a local branch pushes to: its upstream's remote, else "origin". */
function remoteOf(ref: Ref): { remote: string; branch: string } {
  const [remote, ...rest] = ref.upstream.split('/')
  return ref.upstream ? { remote, branch: rest.join('/') } : { remote: 'origin', branch: ref.short }
}

const push = (path: string, remote: string, kind: string, name: string) => () => session.run(() => PushRef(path, remote, kind, name))

/** Hide or unhide a branch by its name without the remote, so origin/wip and wip go
 * together. */
function hideItem(ctx: MenuContext, ref: Ref, name: string): MenuItem {
  return {
    label: ref.hidden ? `Unhide ${name}` : `Hide ${name}`,
    icon: 'eye',
    run: () => session.run(() => HideBranch(ctx.path, name, !ref.hidden)),
  }
}

/** Mark a branch (named without its remote) as one of the kinds, or ordinary. */
function markItem(ctx: MenuContext, ref: Ref, name: string): MenuItem {
  return {
    label: `Mark ${name} as…`,
    icon: 'tag',
    run: () =>
      ctx.openDialog({
        title: `Mark ${name}`,
        message: 'Kinds change how a branch looks and is treated, in this repository on this computer. Settings, This repository, has the patterns.',
        fields: [
          {
            key: 'kind',
            label: 'Kind',
            type: 'select',
            value: ref.branchKind ?? '',
            options: [{ value: '', label: 'Ordinary branch' }, ...Object.entries(KINDS).map(([value, k]) => ({ value, label: k.label }))],
          },
        ],
        submit: 'Mark',
        run: (v) => attempt(() => MarkBranchKind(ctx.path, name, String(v.kind))),
      }),
  }
}

/** Set which branch a local branch is stacked on, or let Git Revenge work it out. */
function stackItem(ctx: MenuContext, ref: Ref): MenuItem {
  return {
    label: `Stack ${ref.short} on…`,
    icon: 'stack',
    run: async () => {
      const refs = ((await session.run(() => Refs(ctx.path))) as Ref[] | undefined) ?? []
      const others = refs.filter((r) => r.kind === 'local' && r.short !== ref.short).map((r) => r.short)
      ctx.openDialog({
        title: `${ref.short} is stacked on`,
        message: 'Git Revenge works this out from history. Set it here when the guess is wrong; it stays on this computer.',
        fields: [
          {
            key: 'parent',
            label: 'Parent branch',
            type: 'select',
            value: '\u0000auto',
            options: [
              { value: '\u0000auto', label: `Detect automatically${ref.stackParent ? ` (now ${ref.stackParent})` : ''}` },
              { value: '', label: 'Not stacked' },
              ...others.map((n) => ({ value: n, label: n })),
            ],
          },
        ],
        submit: 'Save',
        run: (v) => {
          const parent = String(v.parent)
          const auto = parent === '\u0000auto'
          return attempt(() => SetStackParent(ctx.path, ref.short, auto ? '' : parent, auto))
        },
      })
    },
  }
}

/** The dialog for setting a branch's upstream: any name git accepts, with the remote branches
 * it knows about as suggestions. Empty stops tracking. */
export function upstreamDialog(path: string, ref: Ref, refs: Ref[]): DialogSpec {
  return {
    title: `Upstream for ${ref.short}`,
    message: `${ref.upstream ? `Currently tracking ${ref.upstream}.` : 'Not tracking a remote branch.'} Leave this empty to stop tracking.`,
    fields: [
      {
        key: 'upstream',
        label: 'Track',
        type: 'combobox',
        value: ref.upstream,
        placeholder: 'origin/main',
        suggestions: refs.filter((r) => r.kind === 'remote').map((r) => r.short),
      },
    ],
    submit: 'Save',
    run: (v) => attempt(() => branch(path, 'upstream', ref.short, String(v.upstream).trim())),
  }
}

/** Menu items for one ref (branch, remote branch, tag or stash). */
export function refItems(ref: Ref, ctx: MenuContext): MenuItem[] {
  const { path, openDialog } = ctx
  switch (ref.kind) {
    case 'local':
      return groups(
        [
          // The checked-out branch keeps these items, disabled, so the menu's shape doesn't shift.
          { label: `Checkout ${ref.short}`, icon: 'branch', disabled: ref.head, run: () => session.run(() => branch(path, 'checkout', ref.short)) },
          ...(ref.head ? [] : integrateItems(ctx, ref.short, ref.short)),
          // Only moves the branch, so it's safe beside uncommitted changes on the current one.
          ...(ref.head || !ref.upstream || ref.gone
            ? []
            : [{ label: `Pull ${ref.short}`, icon: 'pull' as const, run: () => session.run(() => PullBranch(path, ref.short)) }]),
          ...(ref.head
            ? []
            : [
                {
                  label: `Push ${ref.short} to ${remoteOf(ref).remote}`,
                  icon: 'push' as const,
                  run: push(path, remoteOf(ref).remote, 'branch', ref.short),
                },
              ]),
        ],
        [
          {
            label: `Rename ${ref.short}…`,
            icon: 'edit',
            run: () =>
              openDialog({
                title: `Rename branch ${ref.short}`,
                fields: [{ key: 'name', label: 'New name', value: ref.short }],
                submit: 'Rename',
                run: (v) => attempt(() => branch(path, 'rename', ref.short, String(v.name).trim())),
              }),
          },
          {
            label: `Reflog of ${ref.short}…`,
            icon: 'history',
            run: () => ctx.openReflog(`refs/heads/${ref.short}`),
          },
          {
            label: `Set upstream of ${ref.short}…`,
            icon: 'remote',
            run: async () => {
              const refs = ((await session.run(() => Refs(path))) ?? []) as Ref[]
              openDialog(upstreamDialog(path, ref, refs))
            },
          },
        ],
        // Restacking is for work still to land: a branch already merged only needs deleting.
        ref.stackParentMerged && !session.merged[path]?.[ref.short]
          ? [{ label: `Restack ${ref.short} (${ref.stackParent} was merged)`, icon: 'rebase', run: () => session.run(() => Restack(ctx.path, ref.short)) }]
          : [],
        [markItem(ctx, ref, ref.short), stackItem(ctx, ref), hideItem(ctx, ref, ref.short)],
        [
          {
            label: `Delete ${ref.short}…`,
            icon: 'trash',
            disabled: ref.head,
            run: async () => {
              // Not knowing just means asking, as if the branch weren't merged.
              const merge = await BranchMergeState(path, ref.short).catch(() => null)
              openDialog(deleteBranchDialog(path, ref, merge))
            },
          },
        ],
      )
    case 'remote':
      {
        const [remote, ...rest] = ref.short.split('/')
        return groups(
          [{ label: `Checkout ${ref.short}`, icon: 'branch', run: () => session.run(() => branch(path, 'checkout', ref.short)) }, ...integrateItems(ctx, ref.short, ref.short)],
          [markItem(ctx, ref, rest.join('/')), hideItem(ctx, ref, rest.join('/'))],
          [
            {
              label: `Delete ${ref.short} from ${remote}`,
              icon: 'trash',
              danger: true,
              run: (() => {
                const del = push(path, remote, 'delete-branch', rest.join('/'))
                // Trunk and Environment branches ask first, naming the kind; others go at once.
                const warning = kindWarning(rest.join('/'), ref.branchKind, CONFIRM_DELETE)
                if (!warning) return del
                return () =>
                  openDialog({
                    title: `Delete ${rest.join('/')} from ${remote}?`,
                    message: `${warning} Deleting it removes it from ${remote} for everyone.`,
                    submit: 'Delete from ' + remote,
                    danger: true,
                    run: () => attempt(async () => del()),
                  })
              })(),
            },
          ],
        )
      }
    case 'tag':
      return groups(
        [...integrateItems(ctx, ref.name, ref.short), { label: `Push tag ${ref.short} to origin`, icon: 'push', run: push(path, 'origin', 'tag', ref.short) }],
        [
          { label: `Delete tag ${ref.short}`, icon: 'trash', danger: true, run: () => session.run(() => branch(path, 'untag', ref.short)) },
          { label: `Delete tag ${ref.short} from origin`, icon: 'trash', danger: true, run: push(path, 'origin', 'delete-tag', ref.short) },
        ],
      )
    case 'stash': {
      const act = (action: string) => () => session.run(() => StashAction(path, ref.commit, action))
      return groups(
        [
          { label: 'Apply stash', icon: 'stash', run: act('apply') },
          { label: 'Pop stash', icon: 'pull', run: act('pop') },
        ],
        [{ label: 'Drop stash', icon: 'trash', danger: true, run: act('drop') }],
      )
    }
  }
}

/** Menu items for a commit row, before the items for refs pointing at it. */
export function commitItems(row: Row, refs: Ref[], ctx: MenuContext): MenuItem[] {
  const { path, openDialog } = ctx
  if (refs.some((r) => r.kind === 'stash')) return []
  const short = row.id.slice(0, 7)
  return [
    {
      label: 'Create branch here…',
      icon: 'branch',
      run: () => openDialog(branchAtDialog(path, row.id)),
    },
    {
      label: 'Create tag here…',
      icon: 'tag',
      run: () =>
        openDialog({
          title: `New tag at ${short}`,
          fields: [
            { key: 'name', label: 'Tag name', placeholder: 'v1.0.0' },
            { key: 'message', label: 'Message (makes an annotated tag)', type: 'textarea' },
          ],
          submit: 'Create tag',
          run: (v) => attempt(() => branch(path, 'tag', String(v.name).trim(), row.id, String(v.message))),
        }),
    },
    {
      label: `Checkout ${short} (detached)`,
      icon: 'commit',
      separator: true,
      run: () => session.run(() => branch(path, 'checkout', row.id)),
    },
    ...(ctx.current() && !refs.some((r) => r.head)
      ? [{ label: `Cherry-pick ${short} onto ${ctx.current()}`, icon: 'cherry-pick' as const, run: integrate(path, 'cherry-pick', row.id) }]
      : []),
    ...(ctx.current() && !refs.some((r) => r.head)
      ? [{ label: `Interactive rebase ${ctx.current()} from here…`, icon: 'rebase' as const, run: () => ctx.openRebase(row.id) }]
      : []),
    {
      label: `Reset ${ctx.current() || 'HEAD'} to ${short}…`,
      icon: 'reset',
      separator: true,
      run: () =>
        openDialog({
          title: `Reset ${ctx.current() || 'HEAD'} to ${short}`,
          message:
            'Soft keeps your changes staged. Mixed keeps them as unstaged changes. Hard discards uncommitted changes to tracked files and cannot be undone from the working tree.',
          fields: [
            {
              key: 'mode',
              label: 'Mode',
              type: 'select',
              value: 'mixed',
              options: [
                { value: 'soft', label: 'Soft: keep changes staged' },
                { value: 'mixed', label: 'Mixed: keep changes unstaged' },
                { value: 'hard', label: 'Hard: discard all changes' },
              ],
            },
          ],
          submit: 'Reset',
          danger: true,
          run: (v) => attempt(() => Integrate(path, 'reset', [row.id, String(v.mode)])),
        }),
    },
    row.parents > 1
      ? {
          label: `Revert merge ${short}…`,
          icon: 'revert',
          run: () =>
            openDialog({
              title: `Revert merge ${short}`,
              message: 'Choose the parent whose side to keep. Parent 1 is usually the branch that was merged into.',
              fields: [
                {
                  key: 'mainline',
                  label: 'Keep',
                  type: 'select',
                  value: '1',
                  options: Array.from({ length: row.parents }, (_, i) => ({ value: String(i + 1), label: `Parent ${i + 1}` })),
                },
              ],
              submit: 'Revert',
              run: (v) => attempt(() => Integrate(path, 'revert', [row.id, String(v.mainline)])),
            }),
        }
      : { label: `Revert ${short}`, icon: 'revert', run: integrate(path, 'revert', row.id, '') },
  ]
}

const REF_ICONS = { local: 'branch', remote: 'remote', tag: 'tag', stash: 'stash' } as const

/**
 * A commit's whole menu: its own items, then one row per ref that opens a submenu with that ref's
 * items, since a commit can carry several refs and a flat list runs to 30 items. A stash has no
 * commit items, so its own go straight in the menu.
 */
export function commitMenuItems(row: Row, refs: Ref[], ctx: MenuContext): MenuItem[] {
  const own = commitItems(row, refs, ctx)
  if (own.length === 0) return groups(...refs.map((r) => refItems(r, ctx)))
  return groups(
    own,
    refs.map((r) => ({ label: r.short, icon: REF_ICONS[r.kind], items: refItems(r, ctx) })),
  )
}

/**
 * Items for dropping one graph row onto another. One side must be the checked-out branch:
 * dropping something onto it brings that thing in; dragging it onto something moves it.
 */
export function dropItems(source: Row, sourceRefs: Ref[], target: Row, targetRefs: Ref[], ctx: MenuContext): MenuItem[] {
  const cur = ctx.current()
  if (!cur || source.id === target.id) return []
  const named = (refs: Ref[]) => refs.filter((r) => r.kind !== 'stash' && !r.head)
  const describe = (row: Row, refs: Ref[]) => {
    const r = named(refs)[0]
    return r ? { target: r.kind === 'tag' ? r.name : r.short, label: r.short } : { target: row.id, label: row.id.slice(0, 7) }
  }
  if (targetRefs.some((r) => r.head)) {
    const s = describe(source, sourceRefs)
    return [
      ...integrateItems(ctx, s.target, s.label),
      ...(named(sourceRefs).length === 0
        ? [{ label: `Cherry-pick ${s.label} onto ${cur}`, icon: 'cherry-pick' as const, run: integrate(ctx.path, 'cherry-pick', source.id) }]
        : []),
    ]
  }
  if (sourceRefs.some((r) => r.head)) {
    const t = describe(target, targetRefs)
    return integrateItems(ctx, t.target, t.label)
  }
  return []
}

/** The dialog for a new branch at a commit (from the graph, or a reflog entry). */
export function branchAtDialog(path: string, commit: string): DialogSpec {
  return {
    title: `New branch at ${commit.slice(0, 7)}`,
    fields: [
      { key: 'name', label: 'Branch name', placeholder: 'feature/my-change' },
      { key: 'checkout', label: 'Check it out', type: 'checkbox', value: true },
    ],
    submit: 'Create branch',
    run: (v) => attempt(() => branch(path, 'create', String(v.name).trim(), commit, v.checkout ? 'true' : '')),
  }
}

/** The dialog for "new branch from HEAD". */
export function newBranchDialog(ctx: MenuContext): DialogSpec {
  return {
    title: 'New branch',
    message: 'Starts at the current commit (HEAD).',
    fields: [
      { key: 'name', label: 'Branch name', placeholder: 'feature/my-change' },
      { key: 'checkout', label: 'Check it out', type: 'checkbox', value: true },
    ],
    submit: 'Create branch',
    run: (v) => attempt(() => branch(ctx.path, 'create', String(v.name).trim(), '', v.checkout ? 'true' : '')),
  }
}

/** Double-click on a ref in the sidebar checks it out. */
export function checkoutRef(path: string, ref: Ref) {
  if (ref.kind === 'stash' || ref.head) return
  void session.run(() => branch(path, 'checkout', ref.kind === 'tag' ? ref.name : ref.short))
}
