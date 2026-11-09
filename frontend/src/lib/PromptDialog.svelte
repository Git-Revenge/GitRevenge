<script lang="ts">
  import { onMount } from 'svelte'
  import { AnswerPrompt } from '../../wailsjs/go/app/App.js'
  import { EventsOn } from '../../wailsjs/runtime/runtime.js'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  /** A credential or host-key prompt relayed from git/ssh (app.Prompt in Go). */
  interface Prompt {
    id: string
    text: string
    kind: 'text' | 'secret' | 'hostkey'
  }

  // Prompts can arrive back to back (username, then password); show them in order.
  let queue = $state<Prompt[]>([])
  let current = $derived(queue[0])
  let value = $state('')

  function answer(ok: boolean) {
    if (!current) return
    const p = current
    AnswerPrompt(p.id, p.kind === 'hostkey' ? 'yes' : value, ok)
    value = ''
    queue = queue.slice(1)
  }

  onMount(() => {
    const offPrompt = EventsOn('askpass:prompt', (p: Prompt) => (queue = [...queue, p]))
    const offExpired = EventsOn('askpass:expired', (id: string) => (queue = queue.filter((p) => p.id !== id)))
    return () => {
      offPrompt()
      offExpired()
    }
  })
</script>

{#if current}
  {#key current.id}
    <Modal
      title={current.kind === 'hostkey' ? 'Trust this SSH host?' : 'Git needs your credentials'}
      eyebrow={current.kind === 'hostkey' ? 'a new harbour' : undefined}
      width={500}
      layer={300}
      scrimCloses={false}
      onClose={() => answer(false)}
    >
      <form
        onsubmit={(e) => {
          e.preventDefault()
          answer(true)
        }}
      >
        {#if current.kind === 'hostkey'}
          <!-- ssh's own wording, including the key fingerprint to compare. -->
          <pre>{current.text}</pre>
          <p class="form-note">Only continue if the fingerprint matches the one your Git host publishes. The key is then saved to known_hosts.</p>
          <div class="form-actions">
            <InkButton variant="secondary" onclick={() => answer(false)}>Cancel</InkButton>
            <InkButton type="submit">Trust and connect</InkButton>
          </div>
        {:else}
          <label class="form-field">
            <span>{current.text}</span>
            {#if current.kind === 'secret'}
              <input type="password" bind:value autocomplete="off" />
            {:else}
              <input bind:value autocomplete="off" />
            {/if}
          </label>
          <p class="form-note">Git Revenge passes this to git and doesn't store it.</p>
          <div class="form-actions">
            <InkButton variant="secondary" onclick={() => answer(false)}>Cancel</InkButton>
            <InkButton type="submit">Continue</InkButton>
          </div>
        {/if}
      </form>
    </Modal>
  {/key}
{/if}

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  pre {
    margin: 0;
    padding: 12px 14px;
    background: var(--code-bg);
    color: var(--code-text);
    border-radius: var(--r-sm);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.6;
  }
</style>
