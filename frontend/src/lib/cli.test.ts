import { beforeEach, describe, expect, it, vi } from 'vitest'

const { api, shown, problems } = vi.hoisted(() => ({
  api: { InstallCommandLineTool: vi.fn() },
  shown: [] as string[],
  problems: [] as unknown[],
}))

vi.mock('../../wailsjs/go/app/App.js', () => api)
vi.mock('./toasts.svelte', () => ({ toasts: { show: (message: string) => shown.push(message) } }))
// As the real one does: a failure becomes the session's problem, and the call answers undefined.
vi.mock('./session.svelte', () => ({
  session: {
    run: async (fn: () => Promise<unknown>) => {
      try {
        return await fn()
      } catch (e) {
        problems.push(e)
        return undefined
      }
    },
  },
}))

import { cliCommands } from './cli'

beforeEach(() => {
  api.InstallCommandLineTool.mockReset()
  shown.length = 0
  problems.length = 0
})

describe('the command that installs gitrevenge', () => {
  it("isn't offered off a Mac, where installing the app already did it", () => {
    expect(cliCommands(false)).toEqual([])
  })

  it('is in the palette on a Mac', () => {
    expect(cliCommands(true).map((c) => [c.id, c.group, c.title])).toEqual([['app.cli', 'App', 'Install the gitrevenge command']])
  })

  it('says what the app answered', async () => {
    api.InstallCommandLineTool.mockResolvedValue('The gitrevenge command is installed. A new terminal window has it.')
    await cliCommands(true)[0].run()
    expect(shown).toEqual(['The gitrevenge command is installed. A new terminal window has it.'])
  })

  it('says nothing when the password prompt was turned down', async () => {
    api.InstallCommandLineTool.mockResolvedValue('')
    await cliCommands(true)[0].run()
    expect(shown).toEqual([])
    expect(problems).toEqual([])
  })

  it('shows a failure as an error, not as a toast', async () => {
    api.InstallCommandLineTool.mockRejectedValue("There's already a different gitrevenge command at /usr/local/bin/gitrevenge.")
    await cliCommands(true)[0].run()
    expect(shown).toEqual([])
    expect(problems).toEqual(["There's already a different gitrevenge command at /usr/local/bin/gitrevenge."])
  })
})
