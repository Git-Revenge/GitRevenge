import { SvelteMap } from 'svelte/reactivity'
import { Keybindings, SetKeybindings } from '../../wailsjs/go/app/App.js'
import { isTyping, matchesShortcut, parseShortcut } from './shortcuts'

export interface Command {
  /** Stable id, unique across providers; also the list key. */
  id: string
  title: string
  /** The palette section it's listed under. */
  group: string
  /** Extra words to match on that the title doesn't show. */
  keywords?: string
  /** Shortcut in Mod+Shift+K form ("Mod" is Ctrl, or ⌘ on macOS), shown beside it. */
  shortcut?: string
  /** Other key combinations that run it, not shown. */
  aliases?: string[]
  /** Bound to its keys but not listed in the palette. */
  hidden?: boolean
  /** Its keys work while typing in a field (most shortcuts leave text editing alone). */
  whenTyping?: boolean
  /** Its keys work even while a dialog or menu is open. */
  global?: boolean
  /** The shortcut the command comes with, when the user has changed it (set by all()). */
  defaultShortcut?: string
  run: () => unknown
}

type Provider = () => Command[]

export interface ProviderOptions {
  /** Has no shortcuts of its own, so a key press needn't build its commands. For providers that
   * can list thousands, such as one per ref. */
  paletteOnly?: boolean
}

export const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform)

/** "Mod+Shift+Z" → "Ctrl+Shift+Z", or "⌘⇧Z" on macOS. */
export function formatShortcut(s: string): string {
  if (!isMac) return s.replace(/\bMod\b/g, 'Ctrl')
  const glyph: Record<string, string> = { Mod: '⌘', Shift: '⇧', Alt: '⌥', Ctrl: '⌃', Enter: '↩' }
  return s
    .split('+')
    .map((k) => glyph[k] ?? k)
    .join('')
}

/** Commands contributed by whatever is on screen, and the palette's open state. */
class Commands {
  private providers = new SvelteMap<symbol, { provider: Provider; paletteOnly: boolean }>()
  paletteOpen = $state(false)
  shortcutsOpen = $state(false)
  /** The clone dialog lives at the top level so the palette can open it from anywhere. */
  cloneOpen = $state(false)
  settingsOpen = $state(false)
  aboutOpen = $state(false)
  keybindingsOpen = $state(false)
  /** The user's shortcut changes, by command id; "" unbinds the command. */
  overrides = $state<Record<string, string>>({})

  /** Loads the user's shortcut changes. Until then, and if they can't be read, the defaults stand. */
  async loadBindings() {
    try {
      this.overrides = (await Keybindings()) ?? {}
    } catch {
      // Keep the defaults.
    }
  }

  /** Changes a command's shortcut ("" unbinds it; null restores the default) and saves. */
  async rebind(id: string, keys: string | null) {
    const next = { ...this.overrides }
    if (keys === null) delete next[id]
    else next[id] = keys
    this.overrides = next
    await SetKeybindings(next)
  }

  /** The commands, other than except, whose keys include shortcut. */
  conflicts(shortcut: string, except: string): Command[] {
    const want = parseShortcut(shortcut)
    const same = (s: string) => {
      const k = parseShortcut(s)
      return k.mod === want.mod && k.shift === want.shift && k.alt === want.alt && k.key === want.key
    }
    return this.all().filter((c) => c.id !== except && [c.shortcut, ...(c.aliases ?? [])].some((s) => s && same(s)))
  }

  /** Adds a provider; returns the function that removes it. */
  add(provider: Provider, options: ProviderOptions = {}): () => void {
    const key = Symbol()
    this.providers.set(key, { provider, paletteOnly: !!options.paletteOnly })
    return () => this.providers.delete(key)
  }

  /** Every command available right now. Providers are read on demand, so titles reflect
   * current state (the undo label, the checked-out branch). */
  all(): Command[] {
    return this.collect(true)
  }

  private collect(withPaletteOnly: boolean): Command[] {
    const seen: Record<string, true> = {}
    const out: Command[] = []
    for (const { provider, paletteOnly } of this.providers.values()) {
      if (paletteOnly && !withPaletteOnly) continue
      for (const c of provider()) {
        if (seen[c.id]) continue
        seen[c.id] = true
        if (c.id in this.overrides) {
          // A changed shortcut replaces the default and its aliases.
          out.push({ ...c, shortcut: this.overrides[c.id] || undefined, aliases: [], defaultShortcut: c.shortcut ?? '' })
        } else out.push(c)
      }
    }
    return out
  }

  /** What a key press has to check: everything with a shortcut. Commands from palette-only
   * providers are left out unless a saved shortcut names one of them. */
  private withShortcuts(): Command[] {
    const base = this.collect(false)
    const ids = new Set(base.map((c) => c.id))
    const boundElsewhere = Object.entries(this.overrides).some(([id, keys]) => keys && !ids.has(id))
    return boundElsewhere ? this.collect(true) : base
  }

  /** Runs the command bound to a key event, if any. Returns whether one ran. */
  dispatch(e: KeyboardEvent): boolean {
    if (e.defaultPrevented) return false
    const overlay = !!document.querySelector('[aria-modal="true"], [role="menu"]')
    const typing = isTyping(e.target)
    for (const c of this.withShortcuts()) {
      if (!c.shortcut && !c.aliases) continue
      if ((overlay && !c.global) || (typing && !c.whenTyping && !c.global)) continue
      const keys = c.shortcut ? [c.shortcut, ...(c.aliases ?? [])] : (c.aliases ?? [])
      if (!keys.some((k) => matchesShortcut(e, k, isMac))) continue
      e.preventDefault()
      void c.run()
      return true
    }
    return false
  }
}

export const commands = new Commands()

/** Contributes commands while the calling component is mounted. Call during component setup. */
export function provideCommands(provider: Provider, options: ProviderOptions = {}) {
  $effect(() => commands.add(provider, options))
}
