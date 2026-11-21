<script lang="ts">
  import { untrack } from 'svelte'
  import type { app, git } from '../../../wailsjs/go/models'
  import { errorText } from '../errors'
  import InkButton from '../ui/InkButton.svelte'
  import Modal from '../ui/Modal.svelte'
  import { wrote } from './mailmap'
  import { people as t } from './people.svelte'
  import { identitiesOf, identityText, joinTitle, planAction, planHeading } from './suggest'

  /**
   * Says that several identities are one person. It shows the exact mailmap lines before
   * anything changes, and on a yes puts them into the .mailmap being edited. Nothing is
   * written to disk here: that's the view's Save, which asks first when there's no .mailmap.
   */
  let { who, onClose }: { who: app.SuggestedPerson[]; onClose: () => void } = $props()

  // Shown as the first of them (the one with the most commits) until another is picked or typed.
  let chosen = $state(0)
  let name = $state(untrack(() => who[0]?.name ?? ''))
  let email = $state(untrack(() => who[0]?.email ?? ''))

  function pick(i: number) {
    chosen = i
    name = who[i].name
    email = who[i].email
  }

  let plan = $state<git.JoinPlan | null>(null)
  let error = $state('')
  let asked = 0
  $effect(() => {
    const [n, e, members] = [name, email, identitiesOf(who)]
    void t.doc?.text
    const k = ++asked
    const timer = setTimeout(async () => {
      try {
        const next = await t.plan(members, n, e)
        if (k === asked) [plan, error] = [next, '']
      } catch (err) {
        if (k === asked) [plan, error] = [null, errorText(err)]
      }
    }, 150)
    return () => clearTimeout(timer)
  })

  function add() {
    if (!plan?.lines.length) return
    t.edit(plan.text)
    onClose()
  }
</script>

<Modal title={joinTitle(who.length)} eyebrow="same person" width={600} {onClose}>
  <fieldset class="as">
    <legend>Show them as</legend>
    {#each who as p, i (p.name + '\n' + p.email)}
      <label class="one" class:on={chosen === i}>
        <input type="radio" name="shown-as" checked={chosen === i} onchange={() => pick(i)} />
        <span class="id"><span class="pname">{p.name || '(no name)'}</span> <span class="mail">{p.email}</span></span>
        <span class="n">{wrote(p.commits, p.coAuthored)}</span>
      </label>
    {/each}
  </fieldset>

  <div class="fields">
    <label class="form-field">
      <span>Name</span>
      <input bind:value={name} spellcheck="false" />
    </label>
    <label class="form-field">
      <span>Address</span>
      <input class="mono" bind:value={email} spellcheck="false" />
    </label>
  </div>

  <section class="plan" aria-live="polite">
    {#if error}
      <p class="problem" role="alert">{error}</p>
    {:else if !plan}
      <p class="mute">Working out the line…</p>
    {:else}
      <h3>{planHeading(plan)}</h3>
      {#if plan.lines.length}
        <pre class="lines">{plan.lines.join('\n')}</pre>
      {:else if !plan.blocked.length}
        <p>They already read as that person. There's nothing to add.</p>
      {/if}
      {#if plan.blocked.length}
        <p class="problem" role="alert">
          {plan.blocked.map(identityText).join(', ')} can't be joined from this file: another mailmap in your git config already says who that is.
        </p>
      {/if}
      {#if plan.lines.length}
        <ul class="notes">
          <li>Every name and address here is already in the history, so the {plan.lines.length > 1 ? 'lines tell' : 'line tells'} nobody anything new.</li>
          {#if plan.named.length}
            <li>For {plan.named.map(identityText).join(', ')} the line gives the commit's name as well: the address alone wouldn't do, because someone else uses it or another mailmap has a line for it.</li>
          {:else}
            <li>No other spelling of a name is written out. The address is enough to match.</li>
          {/if}
          <li>No commit changes. Take {plan.lines.length > 1 ? 'the lines' : 'the line'} out and the graph reads as before.</li>
          {#if !t.exists}
            <li>This repository has no <code>.mailmap</code> yet. Adding the line doesn't create one: saving asks you first.</li>
          {/if}
        </ul>
      {/if}
    {/if}
  </section>

  <div class="form-actions">
    <InkButton variant="secondary" onclick={onClose}>Cancel</InkButton>
    <InkButton variant="primary" disabled={!plan?.lines.length || !!error} onclick={add}>{planAction(plan)}</InkButton>
  </div>
</Modal>

<style>
  .as {
    margin: 0 0 12px;
    padding: 0;
    border: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  legend,
  h3 {
    margin: 0;
    padding: 0 0 6px;
    font-family: var(--font-mono);
    font-size: 12px;
    font-weight: 400;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .one {
    display: flex;
    align-items: baseline;
    gap: 10px;
    padding: 8px 10px;
    border-radius: var(--r-md);
  }

  .one.on {
    background: var(--selected);
  }

  .one input {
    align-self: center;
    accent-color: var(--accent);
  }

  .id {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .pname {
    font-weight: 500;
  }

  .mail {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-soft);
  }

  .n {
    font-size: 13px;
    color: var(--ink-soft);
    white-space: nowrap;
  }

  .fields {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
    margin-bottom: 14px;
  }

  .fields .mono {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .plan {
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-height: 120px;
  }

  .plan p {
    margin: 0;
  }

  .lines {
    margin: 0;
    padding: 12px 14px;
    border-radius: var(--r-sm);
    background: var(--code-bg);
    color: var(--code-text);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    max-height: 180px;
    overflow-y: auto;
  }

  .notes {
    margin: 0;
    padding-left: 18px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 14px;
    color: var(--ink-soft);
  }

  .mute {
    color: var(--ink-mute);
  }

  .problem {
    padding: 10px 12px;
    border-radius: var(--r-md);
    background: var(--danger-wash);
    font-size: 14px;
  }

  code {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-b);
  }
</style>
