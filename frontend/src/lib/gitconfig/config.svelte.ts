import { CloseGitConfig, EffectiveGitConfig, GitConfigScopes, OpenGitConfig, ReloadGitConfig, SaveGitConfig, SetGitConfigTab } from '../../../wailsjs/go/app/App.js'
import type { app, git } from '../../../wailsjs/go/models'
import { EventsOn } from '../../../wailsjs/runtime/runtime.js'
import { errorText } from '../errors'
import { session } from '../session.svelte'
import { canonicalKey } from './catalog'
import { samePath } from './rows'
import { scopeLabel, type Scope } from './scopes'

/** Payload of the "gitconfig:changed" event (ConfigChange in Go). */
interface ConfigChange {
  path: string
}

/** The views of a config file, in the order the rail lists them. */
export type View = 'settings' | 'aliases' | 'raw' | 'history'

export const VIEWS: { id: View; label: string }[] = [
  { id: 'settings', label: 'Settings' },
  { id: 'aliases', label: 'Aliases' },
  { id: 'raw', label: 'Raw text' },
  { id: 'history', label: 'History' },
]

/** A save that stopped because the file changed on disk, waiting for the user's choice. */
export interface SaveConflict {
  path: string
  /** The file as it is on disk now. */
  disk: string
}

/** The Git Config tab (ADR-0007): which config files it can edit, the one selected, and each
 * open file's working copy as the backend reports it. */
class GitConfigTab {
  /** Whether the tab is open. */
  open = $state(false)
  /** Path of the selected config file. */
  selected = $state('')
  /** How the selected file is shown. */
  view = $state<View>('settings')
  global = $state('')
  /** The XDG global file when git reads it alongside the global one. */
  globalAlt = $state('')
  /** Local config file by repository path, for open repository tabs. */
  locals = $state<Record<string, string>>({})
  /** Included files opened from an inherited row, with the repository they were opened from. */
  extra = $state<Scope[]>([])
  docs = $state<Record<string, app.ConfigDoc>>({})
  /** Every entry git sees from the selected scope's repository (or from no repository). */
  effective = $state<git.ConfigEntry[]>([])
  /** Files changed on disk while they had unsaved edits; the banner offers Reload or Keep mine. */
  outside = $state<Record<string, boolean>>({})
  conflict = $state<SaveConflict | null>(null)
  error = $state('')
  busy = $state(false)
  /** Sends edits a view is still holding back, such as the raw editor's last keystrokes. */
  flush: (() => Promise<void>) | undefined = undefined
  /** Drops edits a view is still holding back, before a reload discards the rest. */
  cancel: (() => void) | undefined = undefined
  /** A key the Settings view should scroll to and highlight once, then clear. */
  focusKey = $state('')
  /** Set while asking whether closing the tab may discard unsaved edits. */
  confirmClose = $state(false)

  scopes = $derived.by((): Scope[] => {
    const list: Scope[] = []
    if (this.global) list.push({ kind: 'global', path: this.global, repo: '' })
    for (const t of session.tabs) {
      const p = this.locals[t.path]
      if (p) list.push({ kind: 'local', path: p, repo: t.path })
    }
    return [...list, ...this.extra.filter((e) => !list.some((s) => s.path === e.path))]
  })
  scope = $derived(this.scopes.find((s) => s.path === this.selected) ?? this.scopes[0])
  doc = $derived(this.scope ? this.docs[this.scope.path] : undefined)
  dirtyPaths = $derived(
    Object.values(this.docs)
      .filter((d) => d.dirty)
      .map((d) => d.path),
  )

  private listening = false

  /** Restores the tab from the last session, selecting the file it had open. */
  async restore(path: string) {
    if (!path) return
    this.open = true
    this.listen()
    await this.loadScopes()
    await this.select(this.scopes.some((s) => s.path === path) ? path : this.global)
  }

  /** Opens the tab and selects a scope: a repository's local config, or global with no repo. */
  async show(repo = '') {
    this.open = true
    this.listen()
    await this.loadScopes()
    if (repo && !this.locals[repo]) await this.addLocal(repo)
    await this.select(repo ? (this.locals[repo] ?? this.global) : this.selected || this.global)
    await session.select(session.configTabId)
  }

  /** Opens the tab at the file that sets key for a repository (or global config with no
   * repository) and points the Settings view at it. Where nothing sets it, the repository's
   * own file opens, ready to add it. */
  async showKey(repo: string, key: string) {
    await this.show(repo)
    const id = canonicalKey(key)
    const source = this.effective.findLast((e) => canonicalKey(e.key) === id && e.scope !== 'system' && e.file)
    if (source && !samePath(source.file, this.selected)) {
      const known = this.scopes.find((s) => samePath(s.path, source.file))
      if (known) await this.select(known.path)
      else await this.showFile(source.file, repo)
    }
    this.view = 'settings'
    this.focusKey = key
  }

  /** Opens an included config file in the scope switcher and selects it. */
  async showFile(path: string, repo: string) {
    if (!this.scopes.some((s) => s.path === path)) this.extra.push({ kind: 'file', path, repo })
    await this.select(path)
  }

  async select(path: string) {
    if (!path) return
    this.selected = path
    this.error = ''
    // Opening an open file returns its current working copy, so this also refreshes it.
    const doc = await this.call(() => OpenGitConfig(path))
    if (doc) this.docs[path] = doc
    await this.refreshEffective()
    // Only remembers the tab for next time.
    void SetGitConfigTab(path).catch(() => {})
  }

  /** Closes the tab, first asking if that would discard unsaved edits. */
  requestClose() {
    if (this.dirtyPaths.length > 0) this.confirmClose = true
    else void this.close()
  }

  /** Closes the tab, discarding unsaved edits. */
  async close() {
    this.confirmClose = false
    if (session.active === session.configTabId) await session.select(session.tabs.at(-1)?.path ?? '')
    for (const p of Object.keys(this.docs)) await CloseGitConfig(p).catch(() => {})
    this.docs = {}
    this.extra = []
    this.outside = {}
    this.open = false
    this.selected = ''
    void SetGitConfigTab('').catch(() => {})
  }

  /** Runs an edit against the selected file's working copy; false if it failed. */
  async apply(fn: (path: string) => Promise<app.ConfigDoc>): Promise<boolean> {
    const path = this.scope?.path
    if (!path) return false
    this.error = ''
    const doc = await this.call(() => fn(path))
    if (doc) this.docs[path] = doc
    return doc !== undefined
  }

  async save(force = false) {
    const path = this.scope?.path
    if (!path) return
    await this.flush?.()
    const res = await this.call(() => SaveGitConfig(path, force))
    if (!res) return
    this.docs[path] = res.doc
    if (res.saved) {
      this.conflict = null
      delete this.outside[path]
      await this.refreshEffective()
    } else if (res.doc.changedOnDisk) {
      this.conflict = { path, disk: res.diskText }
    } else if (res.doc.problem) {
      this.error = `Not saved: ${problemText(res.doc.problem)}`
    }
  }

  /** Discards edits to the selected file and reads it again. */
  async reload() {
    const path = this.scope?.path
    if (!path) return
    this.cancel?.()
    const doc = await this.call(() => ReloadGitConfig(path))
    if (doc) this.docs[path] = doc
    delete this.outside[path]
    this.conflict = null
    await this.refreshEffective()
  }

  /** Keeps unsaved edits after an outside change; saving will ask before overwriting. */
  keepMine() {
    if (this.scope) delete this.outside[this.scope.path]
  }

  async refreshEffective() {
    const repo = this.scope?.repo ?? ''
    this.effective = (await this.call(() => EffectiveGitConfig(repo))) ?? []
  }

  /** Runs a backend call, showing a failure in the tab. */
  async call<T>(fn: () => Promise<T>): Promise<T | undefined> {
    this.busy = true
    try {
      return await fn()
    } catch (e) {
      this.error = errorText(e)
      return undefined
    } finally {
      this.busy = false
    }
  }

  private async loadScopes() {
    const s = await this.call(() => GitConfigScopes(''))
    if (!s) return
    this.global = s.global
    this.globalAlt = s.globalAlt
    await Promise.all(session.tabs.filter((t) => !this.locals[t.path]).map((t) => this.addLocal(t.path)))
  }

  private async addLocal(repo: string) {
    try {
      const s = await GitConfigScopes(repo)
      this.locals[repo] = s.local
    } catch {
      // A tab whose repository can't be read just has no local scope.
    }
  }

  private listen() {
    if (this.listening) return
    this.listening = true
    EventsOn('gitconfig:changed', (c: ConfigChange) => void this.onDisk(c.path))
    EventsOn('repo:changed', (c: { path: string; kinds: string[] }) => {
      if (c.kinds.includes('config')) this.onRepoConfig(c.path)
    })
  }

  /** A watched config file changed on disk: reload it if it has no edits, else flag it. */
  async onDisk(path: string) {
    if (this.docs[path]) {
      // Ask the backend rather than trusting this copy, so edits are never reloaded away.
      let fresh: Awaited<ReturnType<typeof OpenGitConfig>> | undefined
      try {
        fresh = await OpenGitConfig(path)
        if (!fresh.dirty && fresh.changedOnDisk) fresh = await ReloadGitConfig(path)
      } catch (e) {
        // Otherwise the old values stay on screen as if they were current.
        this.error = `Couldn't re-read ${path}: ${errorText(e)}`
      }
      if (fresh) this.docs[path] = fresh
      // Our own save echoes back as a change too; only a real outside edit is flagged.
      if (fresh?.dirty && fresh.changedOnDisk) this.outside[path] = true
    }
    if (this.open) await this.refreshEffective()
  }

  /** A repository's config changed; its local file is watched with the repository. */
  onRepoConfig(repo: string) {
    const p = this.locals[repo]
    if (p) void this.onDisk(p)
  }

  label(s: Scope) {
    return scopeLabel(s, session.tabs)
  }
}

export function problemText(p: app.ConfigProblem): string {
  return p.line ? `line ${p.line}: ${p.message}` : p.message
}

export const gitConfig = new GitConfigTab()
