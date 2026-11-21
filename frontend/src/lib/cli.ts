import { InstallCommandLineTool } from '../../wailsjs/go/app/App.js'
import type { Command } from './commands.svelte'
import { session } from './session.svelte'
import { toasts } from './toasts.svelte'

/**
 * The palette's command that installs `gitrevenge`, which opens the app from a terminal. Only
 * a Mac is offered it: the Windows installer and the Linux packages put the command on the
 * PATH themselves, but an app dragged out of a disk image has had no install step to do it.
 */
export function cliCommands(mac: boolean): Command[] {
  if (!mac) return []
  return [{ id: 'app.cli', group: 'App', title: 'Install the gitrevenge command', keywords: 'terminal command line cli shell path launch', run: install }]
}

async function install() {
  const said = await session.run(() => InstallCommandLineTool())
  // The answer is empty when the password prompt was turned down, which needs no comment.
  if (said) toasts.show(said)
}
