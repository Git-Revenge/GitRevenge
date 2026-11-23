<script lang="ts">
  // A remote's fetch rules (#925), edited in the Remotes dialog and written on its Save. Each
  // row names the git config key it writes, so a terminal sees exactly what changed. Nothing
  // here fetches pull requests: a refspec someone wrote for them is listed as it is.
  import type { git } from '../../wailsjs/go/models'

  let { remote, rules = $bindable() }: { remote: string; rules: git.FetchRules } = $props()

  const TAGS = [
    { value: '', label: 'With their commits' },
    { value: '--tags', label: 'Every tag' },
    { value: '--no-tags', label: 'None' },
  ]

  // The patterns as typed: "main, release/*".
  let patternText = $state((rules.patterns ?? []).join(', '))
  $effect(() => {
    rules.patterns = patternText
      .split(/[\s,]+/)
      .map((p) => p.trim())
      .filter(Boolean)
  })
</script>

<fieldset class="rules">
  <legend class="meta">Fetching</legend>

  <label class="rule">
    <input type="checkbox" checked={!rules.skipFetchAll} onchange={(e) => (rules.skipFetchAll = !e.currentTarget.checked)} />
    <span class="t"><span>Include in Fetch all</span><span class="d">Leave a slow or rarely used remote out; it can still be fetched on its own.</span></span>
    <code>remote.{remote}.skipFetchAll</code>
  </label>

  <label class="rule">
    <input type="checkbox" bind:checked={rules.prune} />
    <span class="t"><span>Drop branches {remote} has deleted</span><span class="d">Only your copies of deleted branches go; local branches always stay.</span></span>
    <code>remote.{remote}.prune</code>
  </label>

  <div class="rule">
    <span></span>
    <label class="t"
      ><span>Tags</span>
      <select bind:value={rules.tagOpt}>
        {#each TAGS as t (t.value)}<option value={t.value}>{t.label}</option>{/each}
      </select></label
    >
    <code>remote.{remote}.tagOpt</code>
  </div>

  <div class="rule">
    <span></span>
    <div class="t">
      <span id="branches-{remote}">Branches</span>
      <div class="choice" role="radiogroup" aria-labelledby="branches-{remote}">
        <label><input type="radio" name="branches-{remote}" checked={rules.allBranches} onchange={() => (rules.allBranches = true)} /> All of them</label>
        <label><input type="radio" name="branches-{remote}" checked={!rules.allBranches} onchange={() => (rules.allBranches = false)} /> Only these</label>
      </div>
      {#if !rules.allBranches}
        <input class="mono" bind:value={patternText} placeholder="main, release/*" aria-label="Branches to fetch, separated by commas" />
      {/if}
      {#if rules.other?.length}
        <span class="d">Also fetched, as written in git config (Git Revenge keeps these as they are):</span>
        {#each rules.other as spec (spec)}<code class="other">{spec}</code>{/each}
      {/if}
    </div>
    <code>remote.{remote}.fetch</code>
  </div>
</fieldset>

<style>
  .rules {
    display: flex;
    flex-direction: column;
    margin: 12px 0 0;
    padding: 0;
    border: none;
    min-width: 0;
  }

  legend {
    margin-bottom: 4px;
  }

  .rule {
    display: grid;
    grid-template-columns: 20px minmax(0, 1fr) auto;
    gap: 10px;
    align-items: start;
    padding: 8px 0;
    border-bottom: 1px dashed var(--hairline);
  }

  .rule input[type='checkbox'] {
    accent-color: var(--accent);
    margin: 3px 0 0;
  }

  .t {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
    font-size: 14px;
  }

  .d {
    font-size: 12px;
    color: var(--ink-mute);
  }

  code {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    white-space: nowrap;
  }

  .other {
    white-space: normal;
    overflow-wrap: anywhere;
  }

  .choice {
    display: flex;
    gap: 14px;
    font-size: 13px;
  }

  .choice input {
    accent-color: var(--accent);
  }

  select,
  .mono {
    font-size: 13px;
    max-width: 280px;
  }

  .mono {
    font-family: var(--font-mono);
  }
</style>
