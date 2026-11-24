<script lang="ts">
  // Settings › Pushing (#927): where pushes go. Each row is a git key, shown beside it, read
  // and written in one scope, this repository's config or the global one. A row is written
  // only when it's changed, straight away, as Links are. Untouched, a row says what's used now:
  // git's setting from another scope, or Git Revenge's own default where git says nothing.
  import { PushSettingsFor, SetFetchAfterPush, SetPushSetting } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import { errorText } from './errors'

  let { repo }: { repo: string } = $props()

  let scope = $state<'local' | 'global'>('local')
  let data = $state<app.PushSettings | null>(null)
  let error = $state('')

  async function load() {
    if (!repo) return
    try {
      data = await PushSettingsFor(repo, scope)
      error = ''
    } catch (e) {
      error = errorText(e)
    }
  }
  $effect(() => {
    void scope
    void repo
    void load()
  })

  const row = (key: string) => data?.settings.find((s) => s.key === key)

  async function set(key: string, value: string) {
    try {
      await SetPushSetting(repo, scope, key, value)
    } catch (e) {
      error = errorText(e)
    }
    await load()
  }

  async function setFetch(on: boolean) {
    try {
      await SetFetchAfterPush(on)
    } catch (e) {
      error = errorText(e)
    }
    await load()
  }

  /** What an untouched row means now: another scope's setting, or the app's own default. */
  function untouched(key: string, appDefault: string, label: (v: string) => string): string {
    const r = row(key)
    if (r?.inUse) return `Not set here: ${label(r.inUse)}, from ${scope === 'local' ? 'your global config' : "this repository's config"}`
    return `Not set: ${appDefault}`
  }

  const DEFAULT_NAMES: Record<string, string> = {
    simple: 'the same as here (simple)',
    current: 'the same as here, always (current)',
    upstream: 'whatever it tracks (upstream)',
    matching: 'matching (pushes every branch of the same name)',
    nothing: 'nothing (pushes only when told where)',
  }
  const firstPush = (v: string) => (v === 'true' ? 'track the branch it creates' : 'ask where it should track')
  const tags = (v: string) => (v === 'true' ? 'yes' : 'no')
</script>

<fieldset class="pushing">
  <legend class="meta">Pushing</legend>
  {#if !repo}
    <p class="hint">Open a repository to see where its pushes go.</p>
  {:else}
    <div class="scope segmented" role="group" aria-label="Where these are kept">
      <button type="button" aria-pressed={scope === 'local'} onclick={() => (scope = 'local')}>This repository</button>
      <button type="button" aria-pressed={scope === 'global'} onclick={() => (scope = 'global')}>All my repositories</button>
    </div>
    {#if error}<p class="form-error" role="alert">{error}</p>{/if}
    {#if data}
      {@const pd = row('remote.pushDefault')}
      <label class="row">
        <span class="t"><span>Push goes to</span><span class="d">{pd?.here ? '' : untouched('remote.pushDefault', 'the remote each branch tracks', (v) => v)}</span></span>
        <select value={pd?.here ?? ''} onchange={(e) => set('remote.pushDefault', e.currentTarget.value)}>
          <option value="">The remote each branch tracks</option>
          {#each data.remotes as r (r)}<option value={r}>Always {r}</option>{/each}
          {#if pd?.here && !data.remotes.includes(pd.here)}<option value={pd.here}>Always {pd.here}</option>{/if}
        </select>
        <code>remote.pushDefault</code>
      </label>

      {@const pdef = row('push.default')}
      {@const odd = pdef?.here === 'matching' || pdef?.here === 'nothing'}
      <label class="row">
        <span class="t"
          ><span>On the remote, the branch is named</span><span class="d"
            >{pdef?.here ? (odd ? `${DEFAULT_NAMES[pdef.here]}: shown as it is, change it in Git Config` : '') : untouched('push.default', "the branch it tracks, where it tracks one on that remote, else the same as here (Git Revenge's default)", (v) => DEFAULT_NAMES[v] ?? v)}</span
          ></span
        >
        <select value={pdef?.here ?? ''} disabled={odd} onchange={(e) => set('push.default', e.currentTarget.value)}>
          <option value="">Not set</option>
          <option value="simple">The same as here</option>
          <option value="current">The same as here, always</option>
          <option value="upstream">Whatever it tracks</option>
          {#if odd}<option value={pdef?.here}>{pdef?.here}</option>{/if}
        </select>
        <code>push.default</code>
      </label>

      {@const auto = row('push.autoSetupRemote')}
      <label class="row">
        <span class="t"><span>A branch's first push</span><span class="d">{auto?.here ? '' : untouched('push.autoSetupRemote', "publish it and track the branch it creates (Git Revenge's default)", firstPush)}</span></span>
        <select value={auto?.here ?? ''} onchange={(e) => set('push.autoSetupRemote', e.currentTarget.value)}>
          <option value="">Not set</option>
          <option value="true">Track the branch it creates</option>
          <option value="false">Ask where it should track</option>
        </select>
        <code>push.autoSetupRemote</code>
      </label>

      {@const ft = row('push.followTags')}
      <label class="row">
        <span class="t"><span>Send tags with the commits they mark</span><span class="d">{ft?.here ? '' : untouched('push.followTags', 'no', tags)}</span></span>
        <select value={ft?.here ?? ''} onchange={(e) => set('push.followTags', e.currentTarget.value)}>
          <option value="">Not set</option>
          <option value="true">Yes</option>
          <option value="false">No</option>
        </select>
        <code>push.followTags</code>
      </label>

      <label class="row">
        <span class="t"
          ><span>After a push, fetch the other places it went</span><span class="d"
            >When a remote also pushes to another remote's URL, that remote is fetched afterwards, so its branches match what was sent.</span
          ></span
        >
        <input type="checkbox" checked={data.fetchAfterPush} onchange={(e) => setFetch(e.currentTarget.checked)} />
        <span class="pill">app setting</span>
      </label>
      <p class="hint">Push this branch to ▸, in a branch's menu, sends one branch somewhere else (<code>branch.&lt;name&gt;.pushRemote</code>).</p>
    {/if}
  {/if}
</fieldset>

<style>
  fieldset {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    border: none;
    min-width: 0;
  }

  legend {
    margin-bottom: 6px;
  }

  .scope {
    align-self: flex-start;
    margin-bottom: 6px;
  }

  .row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    gap: 10px;
    align-items: center;
    padding: 8px 0;
    border-bottom: 1px dashed var(--hairline);
  }

  .t {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    font-size: 14px;
  }

  .d {
    font-size: 12px;
    color: var(--ink-mute);
  }

  .d:empty {
    display: none;
  }

  select {
    font-size: 13px;
    max-width: 220px;
  }

  input[type='checkbox'] {
    accent-color: var(--accent);
    justify-self: end;
  }

  code {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    white-space: nowrap;
  }

  .pill {
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    padding: 2px 8px;
    border-radius: 999px;
    background: var(--paper-deep);
    color: var(--ink-soft);
    white-space: nowrap;
  }

  .hint {
    margin: 6px 0 0;
    font-size: 13px;
    color: var(--ink-soft);
  }
</style>
