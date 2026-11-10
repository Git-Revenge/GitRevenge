<script lang="ts">
  // The error banner under the tabs: one headline, one line on where that leaves you, at most
  // two next steps, and git's own words behind Details (#614).
  import { BranchAction, Pull, PushBranch, PushCurrent } from '../../wailsjs/go/app/App.js'
  import { BrowserOpenURL, ClipboardSetText } from '../../wailsjs/runtime/runtime.js'
  import { commands } from './commands.svelte'
  import Dialog, { type DialogSpec } from './Dialog.svelte'
  import { errorText } from './errors'
  import { hiddenNote, outputLine, type Problem, type ProblemAction } from './problem'
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'
  import { reveal } from './ui/motion'

  let { problem }: { problem: Problem } = $props()

  const id = $props.id()
  let open = $state(false)
  let copied = $state(false)
  let dialog = $state<DialogSpec | null>(null)

  let lines = $derived(problem.output.split('\n').map(outputLine))

  /** Where the problem happened: its own repository, else the one on screen. */
  const repo = () => problem.path || session.activeTab?.path || ''

  /** Runs one of the toolbar's commands, so its progress and confirmations show as usual.
   * False when it isn't on offer, such as while something else is running. */
  async function command(id: string): Promise<boolean> {
    const c = commands.all().find((x) => x.id === id)
    if (!c) return false
    await c.run()
    return true
  }

  async function act(a: ProblemAction) {
    const path = repo()
    switch (a.kind) {
      case 'open-url':
        if (a.url) BrowserOpenURL(a.url)
        return
      case 'retry': {
        const again = problem.retry
        session.error = ''
        if (again) await session.run(again)
        return
      }
      case 'pull-then-push':
        session.error = ''
        if (!(await command('repo.pull'))) await session.run(() => Pull(path, ''))
        // A pull that failed or stopped on a conflict has said so; pushing would only add to it.
        if (session.problem) return
        if (!(await command('repo.push'))) await session.run(() => PushCurrent(path, false))
        return
      case 'force-push':
        session.error = ''
        if (!(await command('repo.force-push'))) await session.run(() => PushCurrent(path, true))
        return
      case 'push-new-branch':
        dialog = pushNewBranch(path)
        return
    }
  }

  /** Starts a branch at the current commit, switches to it and pushes it: the way round a
   * branch that only takes pull requests. The branch that was refused stays as it is. */
  function pushNewBranch(path: string): DialogSpec {
    return {
      title: 'Push to a new branch',
      message: 'Starts a branch at your current commit, switches to it and pushes it. The branch you were on stays as it is.',
      fields: [{ key: 'name', label: 'Branch name', placeholder: 'feature/my-change' }],
      submit: 'Create and push',
      run: async (v) => {
        const name = String(v.name).trim()
        if (!name) return 'Give the branch a name.'
        try {
          await BranchAction(path, 'create', [name, '', 'true'])
        } catch (e) {
          return errorText(e)
        }
        // The branch exists now, so the dialog's job is done: a push that fails says so in
        // the banner, where its own next steps fit.
        session.error = ''
        void session.run(() => PushBranch(path, name))
      },
    }
  }

  async function copy() {
    copied = (await ClipboardSetText(problem.full || problem.output).catch(() => false)) !== false
  }
</script>

<div class="banner" role="alert" in:reveal out:reveal={{ out: true }}>
  <div class="row">
    <Icon name="warning" size={18} />
    <div class="words">
      <span class="title">{problem.title}</span>
      {#if problem.detail}<span class="detail">{problem.detail}</span>{/if}
      {#if problem.quote}<code class="quote">{problem.quote}</code>{/if}
    </div>
    <div class="actions">
      {#each problem.actions as a, i (a.kind)}
        <button class={['btn sm', i === 0 ? 'secondary' : 'quiet']} onclick={() => act(a)}>
          {#if a.kind === 'open-url'}<Icon name="external" />{/if}{a.label}
        </button>
      {/each}
      {#if problem.output}
        <button class="btn sm quiet" class:on={open} aria-expanded={open} aria-controls="{id}-details" onclick={() => (open = !open)}>
          Details<span class="chevron" class:up={open}><Icon name="chevron" size={14} /></span>
        </button>
      {/if}
      <button class="btn sm quiet icon" aria-label="Dismiss" title="Dismiss" onclick={() => (session.error = '')}><Icon name="close" /></button>
    </div>
  </div>
  {#if open}
    <div class="details code-ground" id="{id}-details" in:reveal out:reveal={{ out: true }}>
      <pre>{#each lines as l, i (i)}<span class="line">{#if l.mark}<span class="mark">{l.mark}</span>{/if}{l.rest}</span>{/each}</pre>
      <div class="foot">
        <span class="note">{hiddenNote(problem.hidden)}</span>
        <button class="btn sm secondary" onclick={copy}>{copied ? 'Copied' : 'Copy everything'}</button>
      </div>
    </div>
  {/if}
</div>

{#if dialog}
  <Dialog spec={dialog} onClose={() => (dialog = null)} />
{/if}

<style>
  .banner {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 10px 8px 10px 14px;
    background: var(--danger-wash);
    border-bottom: 1px solid var(--paper-edge);
    color: var(--ink);
  }

  .row {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .row > :global(svg) {
    color: var(--danger);
  }

  .words {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .title {
    font-weight: 500;
    line-height: 1.35;
    overflow-wrap: anywhere;
  }

  .detail {
    font-size: 13px;
    line-height: 1.45;
    color: var(--ink-soft);
  }

  .quote {
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.5;
    color: var(--ink-soft);
    overflow-wrap: anywhere;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    align-items: center;
    gap: 4px;
  }

  .actions .secondary {
    margin-right: 4px;
  }

  .actions .on {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .chevron {
    display: inline-flex;
    margin-left: -4px;
    transition: transform var(--dur-base) var(--ease-ink);
  }

  .chevron.up {
    transform: rotate(180deg);
  }

  /* Git's own words, on the same dark ground as code. Long output scrolls here, so the banner
     never takes the window. */
  .details {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin: 0 6px 2px 30px;
    padding: 12px 14px;
    background: var(--code-bg);
    border-radius: var(--r-sm);
    color: var(--code-text);
  }

  pre {
    margin: 0;
    max-height: min(40vh, 320px);
    overflow: auto;
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    user-select: text;
  }

  .line {
    display: block;
    min-height: 1.6em;
  }

  .mark {
    color: var(--code-keyword);
  }

  .foot {
    display: flex;
    align-items: center;
    gap: 12px;
    padding-top: 8px;
    border-top: 1px dashed var(--hairline);
  }

  .note {
    flex: 1;
    font-family: var(--font-mono);
    font-size: 12px;
    font-style: italic;
    color: var(--code-comment);
  }

  @media (max-width: 760px) {
    .row {
      flex-wrap: wrap;
    }

    .words {
      flex-basis: calc(100% - 30px);
    }

    .actions {
      width: 100%;
    }
  }
</style>
