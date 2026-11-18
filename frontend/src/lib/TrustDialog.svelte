<script lang="ts">
  // Asked before a repository opens for the first time, when its own git config or hooks
  // would run programs (app.TrustQuestion in Go). A clone never brings such settings with
  // it, so this is only seen for a repository that arrived with its .git folder already made,
  // or one the user set up that way themselves. Nothing in it has been run yet.
  import { session } from './session.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  let question = $derived(session.trustQuestion)
  let settings = $derived(question?.findings.filter((f) => f.kind === 'config') ?? [])
  let hooks = $derived(question?.findings.filter((f) => f.kind === 'hook') ?? [])
  let busy = $state(false)

  async function trust() {
    busy = true
    try {
      await session.trustAndOpen()
    } finally {
      busy = false
    }
  }
</script>

{#if question}
  {#key question.path}
    <Modal title="Let {question.name} run its own programs?" eyebrow="before it opens" width={600} layer={300} scrimCloses={false} onClose={() => session.declineTrust()}>
      <div class="body">
        <p class="form-note lead">
          This repository has git settings of its own that run programs on your computer. Nothing has been run yet. A repository you cloned has none of these unless you added them; one that came as a folder or an archive
          has whatever its maker put there.
        </p>
        <p class="where" title={question.path}>{question.path}</p>

        {#if settings.length}
          <h3>Settings in its git config</h3>
          <ul aria-label="Settings that run programs">
            {#each settings as f (f.name + f.value)}
              <li>
                <code class="what"><span class="name">{f.name}</span> = {f.value}</code>
                <span class="why">This {f.why}.</span>
              </li>
            {/each}
          </ul>
        {/if}

        {#if hooks.length}
          <h3>Hook scripts in its git folder</h3>
          <ul aria-label="Hook scripts">
            {#each hooks as f (f.name)}
              <li>
                <code class="what"><span class="name">{f.name}</span></code>
                <span class="why">This {f.why}.</span>
              </li>
            {/each}
          </ul>
        {/if}

        <p class="form-warning">Only go on if you know where this repository came from and what these do. Opening it lets git run them, now and from then on.</p>
        <div class="form-actions">
          <InkButton onclick={() => session.declineTrust()}>Don’t open</InkButton>
          <InkButton variant="danger" disabled={busy} onclick={trust}>Trust and open</InkButton>
        </div>
      </div>
    </Modal>
  {/key}
{/if}

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .lead {
    color: var(--ink-soft);
  }

  .where {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  h3 {
    margin: 4px 0 0;
    font-size: 13px;
    font-weight: 600;
    color: var(--ink);
  }

  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 240px;
    overflow-y: auto;
  }

  li {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .what {
    padding: 6px 10px;
    background: var(--code-bg);
    color: var(--code-text);
    border-radius: var(--r-sm);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.5;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .name {
    font-weight: 600;
  }

  .why {
    font-size: 13px;
    color: var(--ink-mute);
  }
</style>
