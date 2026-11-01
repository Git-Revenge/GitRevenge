<script lang="ts" module>
  export interface Field {
    key: string
    label: string
    /** A combobox is a text field with suggestions, filtered as you type. */
    type?: 'text' | 'textarea' | 'checkbox' | 'select' | 'combobox'
    value?: string | boolean
    placeholder?: string
    options?: { value: string; label: string }[]
    suggestions?: string[]
  }

  export interface DialogSpec {
    title: string
    message?: string
    fields?: Field[]
    submit: string
    danger?: boolean
    /** Runs on submit; return an error message to keep the dialog open, or nothing to close. */
    run: (values: Record<string, string | boolean>) => Promise<string | void>
  }
</script>

<script lang="ts">
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  let { spec, onClose }: { spec: DialogSpec; onClose: () => void } = $props()
  const id = $props.id()

  // svelte-ignore state_referenced_locally
  let values = $state<Record<string, string | boolean>>(
    Object.fromEntries((spec.fields ?? []).map((f) => [f.key, f.value ?? (f.type === 'checkbox' ? false : '')])),
  )
  let error = $state('')
  let busy = $state(false)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    error = ''
    const res = await spec.run(values)
    busy = false
    if (res) error = res
    else onClose()
  }
</script>

<Modal title={spec.title} {onClose} dismissable={!busy}>
  <form onsubmit={submit}>
    {#if spec.message}<p class="message">{spec.message}</p>{/if}
    {#each spec.fields ?? [] as f (f.key)}
      {#if f.type === 'checkbox'}
        <label class="form-check"><input type="checkbox" bind:checked={values[f.key] as boolean} /> {f.label}</label>
      {:else}
        <label class="form-field">
          <span>{f.label}</span>
          {#if f.type === 'textarea'}
            <textarea bind:value={values[f.key]} placeholder={f.placeholder} rows="4"></textarea>
          {:else if f.type === 'select'}
            <select bind:value={values[f.key]}>
              {#each f.options ?? [] as o (o.value)}<option value={o.value}>{o.label}</option>{/each}
            </select>
          {:else if f.type === 'combobox'}
            <input bind:value={values[f.key]} placeholder={f.placeholder} list="{id}-{f.key}" autocomplete="off" />
            <datalist id="{id}-{f.key}">
              {#each f.suggestions ?? [] as s (s)}<option value={s}></option>{/each}
            </datalist>
          {:else}
            <input bind:value={values[f.key]} placeholder={f.placeholder} />
          {/if}
        </label>
      {/if}
    {/each}
    {#if error}<p class="form-error" role="alert">{error}</p>{/if}
    <div class="form-actions">
      <InkButton variant="secondary" onclick={onClose} disabled={busy}>Cancel</InkButton>
      <InkButton type="submit" variant={spec.danger ? 'danger' : 'primary'} disabled={busy}>{spec.submit}</InkButton>
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
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
</style>
