<script lang="ts">
  // Asks before Undo or Redo takes commits off a branch that are already on its remote. Undo
  // only ever changes this copy, so it can't take them back from anyone else; Revert can.
  import { Integrate } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import CommitLines from './CommitLines.svelte'
  import { session } from './session.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  let {
    path,
    verb,
    label,
    shared,
    onConfirm,
    onClose,
  }: {
    path: string
    /** "Undo" or "Redo". */
    verb: string
    /** What it would reverse. */
    label: string
    shared: app.SharedCommits
    onConfirm: () => void
    onClose: () => void
  } = $props()

  let one = $derived(shared.total === 1)
  // Reverting a merge means choosing a side, and an unlisted commit can't be reverted at all.
  let canRevert = $derived(shared.commits.length === shared.total && !shared.commits.some((c) => c.merge))

  // Closing takes this component's props away, so whatever's needed is read first.
  async function revert() {
    const commits = shared.commits
    onClose()
    await session.run(async () => {
      for (const c of commits) await Integrate(path, 'revert', [c.id, ''])
    })
  }

  function confirm() {
    onConfirm()
    onClose()
  }
</script>

<Modal title="{verb} {label}?" eyebrow="already pushed" {onClose}>
  <p class="text">
    This only changes your copy. {shared.upstream} keeps {one ? 'this commit' : `these ${shared.total.toLocaleString()} commits`}, so
    {one ? 'it stays' : 'they stay'} for everyone else, and your next push would be refused.
  </p>
  <CommitLines commits={shared.commits} total={shared.total} />
  {#if canRevert}
    <p class="text">To remove {one ? 'it' : 'them'} for everyone, use Revert instead, which adds {one ? 'a commit' : 'commits'} undoing {one ? 'it' : 'them'}.</p>
  {/if}
  <div class="form-actions">
    <InkButton variant="secondary" onclick={onClose}>Cancel</InkButton>
    {#if canRevert}<InkButton variant="secondary" onclick={revert}>Revert</InkButton>{/if}
    <InkButton onclick={confirm}>{verb} anyway</InkButton>
  </div>
</Modal>

<style>
  .text {
    margin: 0 0 12px;
    font-size: 15px;
    line-height: 1.55;
  }

  .text:not(:first-child) {
    margin-top: 12px;
  }
</style>
