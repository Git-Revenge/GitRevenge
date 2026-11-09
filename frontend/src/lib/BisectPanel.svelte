<script lang="ts">
  import { CommitDetail } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import { bisect, TERM_LABEL } from './bisect.svelte'
  import { formatTime } from './graph/draw'

  /** The right-hand panel during a bisect: the commit under test and what to say about it, or
   * the answer and what to do with it (issue #592). */
  let {
    path,
    info,
    onOpenCommit,
    onFixBranch,
    onRevert,
  }: {
    path: string
    info: app.BisectInfo
    /** Shows a commit's own details in place of this panel. */
    onOpenCommit: (id: string) => void
    /** Stops the bisect and starts a branch for the fix. */
    onFixBranch: () => void
    /** Stops the bisect and reverts the first bad commit. */
    onRevert: (id: string) => void
  } = $props()

  let found = $derived(info.firstBad)
  let subject = $derived(found || info.current)
  let detail = $state<app.CommitInfo | null>(null)

  // The commit's own details are decoration here: without them the panel still asks its question.
  $effect(() => {
    const id = subject
    detail = null
    if (!id) return
    void CommitDetail(path, id)
      .then((d) => {
        if (id === subject) detail = d
      })
      .catch(() => {})
  })

  let steps = $derived(info.marks.filter((m) => m.step > 0))
  let starts = $derived(info.marks.filter((m) => m.step === 0))
  const short = (id: string) => id.slice(0, 7)
</script>

<aside class="panel" aria-label={found ? 'The first bad commit' : 'The commit under test'}>
  {#if info.candidates.length && !found}
    <header>
      <span class="hand rose">one of these</span>
      <h2>It broke in one of {info.candidates.length} commits</h2>
    </header>
    <p class="say">The commits between them couldn't be tested, so git can't say which one it was.</p>
    <ul class="marks">
      {#each info.candidates as id (id)}
        <li><button class="btn sm text" onclick={() => onOpenCommit(id)}>{short(id)}</button></li>
      {/each}
    </ul>
  {:else}
    <header>
      <span class={['hand', found && 'rose']}>{found ? 'the first bad commit' : 'under test'}</span>
      <h2>{detail?.commit.subject ?? short(subject)}</h2>
      <p class="who">
        {#if detail}<span>{detail.commit.author.name} · {formatTime(detail.commit.author.time)} ·</span>{/if}
        <span class="mono">{short(subject)}</span>
      </p>
    </header>

    {#if found}
      <p class="say">Everything before it worked and everything from it on is broken, so the cause is in what this commit changed.</p>
    {:else}
      <div class="ask">
        <p>Your files are now as they were at this commit. Build it, try the thing that's broken, then say what you saw.</p>
        <div class="answers">
          <button class="btn success" disabled={bisect.busy} onclick={() => bisect.mark(path, 'good')}>It works</button>
          <button class="btn danger" disabled={bisect.busy} onclick={() => bisect.mark(path, 'bad')}>It's broken</button>
        </div>
        <button class="btn sm text" disabled={bisect.busy} onclick={() => bisect.mark(path, 'skip')}>It won't build, skip it</button>
      </div>
    {/if}

    {#if detail?.files?.length}
      <section>
        <h3 class="meta">{found ? 'What it changed' : 'Changed in this commit'} · {detail.files.length}</h3>
        <ul class="files">
          {#each detail.files.slice(0, 8) as f (f.path)}
            <li>
              <span class="mono path" title={f.path}>{f.path}</span>
              {#if !f.binary}<span class="mono added">+{f.added}</span><span class="mono deleted">−{f.deleted}</span>{/if}
            </li>
          {/each}
          {#if detail.files.length > 8}<li class="more">and {detail.files.length - 8} more</li>{/if}
        </ul>
      </section>
    {/if}

    {#if found}
      <section class="next">
        <h3 class="meta">What now</h3>
        <button class="btn secondary" onclick={() => onOpenCommit(found)}>Open its changes</button>
        <button class="btn secondary" onclick={onFixBranch}>Start a fix branch from {info.origin || 'where you were'}…</button>
        <button class="btn secondary" onclick={() => onRevert(found)}>Revert it on {info.origin || 'your branch'}…</button>
      </section>
    {/if}
  {/if}

  <section>
    <h3 class="meta">{found ? 'How it was found' : 'So far'}</h3>
    <ul class="marks">
      {#each found ? steps : [...starts, ...steps] as m (m.commit + m.step)}
        <li>
          <span class={['tag', m.term === 'good' ? 'green' : m.term === 'bad' ? 'rose' : 'plain']}>{TERM_LABEL[m.term] ?? m.term}</span>
          <span class="mono">{short(m.commit)}</span>
          <span class="when">{m.step ? `step ${m.step}` : 'start'}</span>
        </li>
      {/each}
    </ul>
  </section>

  <p class="foot">
    {#if found}The bisect stays as it is until you press Done, so you can look around first.{:else}Stopping returns you to {info.origin || 'where you were'}.{/if}
  </p>
</aside>

<style>
  .panel {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 22px 24px;
    overflow-y: auto;
  }

  header {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .hand.rose {
    color: var(--link);
  }

  h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 500;
    line-height: 1.25;
    letter-spacing: -0.01em;
    overflow-wrap: anywhere;
  }

  h3 {
    margin: 0 0 6px;
    font-weight: 400;
  }

  .who,
  .foot {
    margin: 0;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .foot {
    margin-top: auto;
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .say {
    margin: 0;
    color: var(--ink-soft);
  }

  .ask {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
    padding: 14px 16px;
    border-radius: var(--r-md);
    background: var(--paper-warm);
  }

  .ask p {
    margin: 0;
    color: var(--ink-soft);
  }

  .answers {
    display: flex;
    gap: 8px;
    align-self: stretch;
  }

  .answers .btn {
    flex: 1;
  }

  .files,
  .marks {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .files li,
  .marks li {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 32px;
    border-top: 1px dashed var(--hairline);
  }

  .files li:first-child,
  .marks li:first-child {
    border-top: none;
  }

  .path {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .added {
    color: var(--success);
  }

  .deleted {
    color: var(--danger);
  }

  .more,
  .when {
    font-size: 13px;
    color: var(--ink-mute);
  }

  .next {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 14px 16px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
  }

  .next .btn {
    justify-content: flex-start;
  }

  .tag.green {
    background: var(--success-wash);
    color: var(--success);
  }

  .tag.plain {
    background: var(--wash-hover);
    color: var(--ink-mute);
  }
</style>
