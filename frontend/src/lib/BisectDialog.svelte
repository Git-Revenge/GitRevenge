<script lang="ts">
  import { PlanBisect, StartBisect } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import { bisect } from './bisect.svelte'
  import { errorText } from './errors'
  import Modal from './ui/Modal.svelte'

  /** Starts a bisect: one commit where it's broken, one where it worked (issue #592). */
  let { path, bad: initialBad, onClose }: { path: string; bad: string; onClose: () => void } = $props()

  // svelte-ignore state_referenced_locally
  let bad = $state(initialBad)
  let good = $state('')
  let plan = $state<app.BisectPlan | null>(null)
  let error = $state('')
  let busy = $state(false)
  let asked = 0
  let timer: ReturnType<typeof setTimeout> | undefined

  /** Asks what bisecting these two would involve. An empty good asks for a suggestion. */
  async function look() {
    const n = ++asked
    try {
      const p = await PlanBisect(path, bad.trim(), good.trim())
      if (n !== asked) return
      plan = p
      // The suggestion fills the field, so it can be seen and changed.
      if (!good.trim() && p?.suggested) good = p.suggested
    } catch (e) {
      if (n === asked) error = errorText(e)
    }
  }

  function lookSoon() {
    clearTimeout(timer)
    timer = setTimeout(() => void look(), 250)
  }

  $effect(() => {
    void look()
    return () => clearTimeout(timer)
  })

  let ready = $derived(!!plan && !!plan.good && !plan.problem && plan.commits > 0 && !busy)

  async function start() {
    if (!plan || !ready) return
    busy = true
    error = ''
    try {
      await StartBisect(path, plan.bad, plan.good)
      await bisect.refresh(path)
      onClose()
    } catch (e) {
      error = errorText(e)
    } finally {
      busy = false
    }
  }

  const short = (id: string) => id.slice(0, 7)
</script>

<Modal title="Find the commit that broke it" eyebrow="where did it go wrong?" width={620} {onClose} dismissable={!busy}>
  <form
    class="bisect"
    onsubmit={(e) => {
      e.preventDefault()
      void start()
    }}
  >
    <p class="form-note lede">
      Name one commit where it's broken and one where it worked. Git Revenge checks out commits in between, you say whether each is good or bad, and it halves the
      range each time.
    </p>

    <div class="ends">
      <label class="end">
        <span class="tag rose">Bad</span>
        <input class="field" bind:value={bad} oninput={lookSoon} placeholder="HEAD" spellcheck="false" autocomplete="off" aria-label="The commit where it's broken" />
        <span class="what">{plan?.badSubject ?? ''}</span>
      </label>
      <label class="end">
        <span class="tag green">Good</span>
        <input class="field" bind:value={good} oninput={lookSoon} placeholder="A commit, tag or branch where it worked" spellcheck="false" autocomplete="off" aria-label="A commit where it worked" />
        <span class="what">{plan?.goodSubject ?? ''}{#if plan?.suggested && good.trim() === plan.suggested}<span class="why"> · the last tag</span>{/if}</span>
      </label>
    </div>

    {#if plan?.problem}
      <p class="form-error" role="alert">{plan.problem}</p>
    {:else if plan && plan.good && plan.commits > 0}
      <p class="count" role="status">
        <strong>{plan.commits.toLocaleString()}</strong>
        <span
          >{plan.commits === 1 ? 'commit lies' : 'commits lie'} between them.
          {#if plan.steps > 0}That's <b>about {plan.steps} {plan.steps === 1 ? 'step' : 'steps'}</b>, each one a checkout and a test.{:else}That one is the answer: there's nothing to test.{/if}</span
        >
      </p>
    {:else if plan && !plan.good}
      <p class="form-note">No tag comes before the bad commit, so there's nothing to suggest. Type a commit, tag or branch where it still worked.</p>
    {/if}

    <p class="form-note">At each step you build and try it yourself, then press Good, Bad, or Can't tell for a commit that won't build. Stopping brings you back to where you are now.</p>

    {#if error}<p class="form-error" role="alert">{error}</p>{/if}

    <div class="form-actions">
      <button type="button" class="btn quiet" onclick={onClose} disabled={busy}>Cancel</button>
      <button type="submit" class="btn lg primary" disabled={!ready} aria-busy={busy || undefined}>
        {#if plan?.first && ready}Start with {short(plan.first)}{:else}Start{/if}
        <span class="arrow" aria-hidden="true">→</span>
      </button>
    </div>
  </form>
</Modal>

<style>
  .bisect {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .lede {
    font-size: 14px;
    color: var(--ink-soft);
  }

  .ends {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 14px 16px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
  }

  .end {
    display: grid;
    grid-template-columns: 64px minmax(0, 1fr);
    align-items: center;
    gap: 4px 12px;
  }

  .end .tag {
    justify-self: start;
  }

  .tag.green {
    background: var(--success-wash);
    color: var(--success);
  }

  .end .field {
    width: 100%;
  }

  .what {
    grid-column: 2;
    min-height: 18px;
    font-size: 13px;
    color: var(--ink-mute);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .why {
    color: var(--ink-faint);
  }

  .count {
    display: flex;
    align-items: center;
    gap: 16px;
    margin: 0;
    padding: 12px 16px;
    border-radius: var(--r-md);
    background: var(--paper-warm);
    color: var(--ink-soft);
  }

  .count strong {
    font-size: 30px;
    font-weight: 500;
    line-height: 1;
    color: var(--ink);
  }

  .count b {
    font-weight: 600;
    color: var(--ink);
  }
</style>
