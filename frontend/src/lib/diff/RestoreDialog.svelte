<script lang="ts">
  // Restore asks first (#1082): what restoring a version, or a block's lines, writes into the
  // work tree. It's a change to commit or throw away; no commit changes, and Undo brings back
  // what was there.
  import type { app } from '../../../wailsjs/go/models'
  import InkButton from '../ui/InkButton.svelte'
  import Modal from '../ui/Modal.svelte'

  let {
    file,
    commit,
    lines = null,
    plan = null,
    busy = false,
    problem = '',
    onRestore,
    onClose,
  }: {
    /** The file restored, under its name today. */
    file: string
    /** The version restored from. */
    commit: string
    /** For a block, which lines; null for the whole file. */
    lines?: { from: number; to: number } | null
    /** What restoring the whole file changes, once worked out. */
    plan?: app.RestorePlan | null
    busy?: boolean
    problem?: string
    onRestore: () => void
    onClose: () => void
  } = $props()

  const n = (count: number, one: string, many: string) => `${count.toLocaleString()} ${count === 1 ? one : many}`
  let sha = $derived(commit.slice(0, 7))
  let title = $derived(lines ? (lines.from === lines.to ? `Put line ${lines.from} back as it was at ${sha}?` : `Put lines ${lines.from} to ${lines.to} back as they were at ${sha}?`) : `Restore ${file} as it was at ${sha}?`)
  let what = $derived.by(() => {
    if (lines) return 'Only those lines change, where they are in the file now. It’s a change in the working tree, ready to commit or throw away. No commit changes.'
    const counts = plan ? (plan.added || plan.removed ? `: ${n(plan.added, 'line', 'lines')} added and ${plan.removed.toLocaleString()} taken out, against the file as it is` : ': the file is already as it was then') : ''
    return `It becomes a change in the working tree, ready to commit or throw away${counts}. No commit changes.`
  })
</script>

<Modal {title} {onClose} dismissable={!busy}>
  <form
    onsubmit={(e) => {
      e.preventDefault()
      onRestore()
    }}
  >
    <p class="message">{what}</p>
    {#if !lines && plan?.uncommitted}
      <p class="uncommitted" role="note">
        <strong>Not committed:</strong> the file has {n(plan.uncommitted, 'changed line', 'changed lines')} you haven’t committed. {plan.uncommitted === 1 ? 'Restoring replaces it; Undo brings it back.' : 'Restoring replaces them; Undo brings them back.'}
      </p>
    {/if}
    {#if problem}<p class="form-error" role="alert">{problem}</p>{/if}
    <div class="form-actions">
      <InkButton variant="secondary" onclick={onClose} disabled={busy}>Cancel</InkButton>
      <InkButton type="submit" variant="primary" disabled={busy || (!lines && !plan && !problem)}>{lines ? 'Put them back' : 'Restore'}</InkButton>
    </div>
  </form>
</Modal>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .message {
    margin: 0;
    font-size: 15px;
    line-height: 1.55;
    color: var(--ink-soft);
  }

  .uncommitted {
    margin: 0;
    padding: 10px 12px;
    border-radius: var(--r-sm);
    background: color-mix(in srgb, var(--ink-ochre) 18%, transparent);
    font-size: 14px;
    line-height: 1.5;
    color: var(--ink);
  }

  .uncommitted strong {
    font-weight: 500;
    color: var(--warning);
  }
</style>
