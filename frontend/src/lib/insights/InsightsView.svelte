<script lang="ts">
  // The Insights tab (#141): what shipped in a window and which work is stuck, computed from
  // the repositories alone (ADR-0008). The side panel holds every input a result depends on.
  import { session } from '../session.svelte'
  import Icon from '../ui/Icon.svelte'
  import InkButton from '../ui/InkButton.svelte'
  import { insights, VIEWS, type WindowKind } from './insights.svelte'
  import { ago, attention, groupLabel, latest, perDay, tiles, utcDay, utcMoment } from './model'
  import AttentionView from './AttentionView.svelte'
  import HotspotsView from './HotspotsView.svelte'
  import ShippedView from './ShippedView.svelte'
  import SummaryDialog from './SummaryDialog.svelte'

  const windows: { id: WindowKind; label: string }[] = [
    { id: '7d', label: '7 days' },
    { id: '14d', label: '14 days' },
    { id: 'month', label: 'month' },
    { id: 'releases', label: 'releases' },
  ]

  let scope = $derived(insights.scope)
  let w = $derived(insights.resolved)
  let t = $derived(tiles(insights.items, insights.branches))
  let flagged = $derived(attention(insights.branches))
  let ships = $derived(latest(insights.items))
  let days = $derived(w ? perDay(insights.items, w.since, w.until) : [])
  let peak = $derived(Math.max(1, ...days.map((d) => d.count)))
  let failed = $derived(insights.repos.filter((r) => insights.data[r]?.error))
  let multi = $derived(insights.repos.length > 1)
  let pickingDate = $state(false)
  let summaryOpen = $state(false)

  const repoName = (path: string) => session.tabs.find((t) => t.path === path)?.name ?? path.split(/[\\/]/).pop() ?? path
  const scopeValue = $derived(scope ? `${scope.kind}:${scope.id}` : '')
  /** The whole of what the scope menu shows, which a long name can cut off. */
  const scopeTitle = $derived.by(() => {
    if (scope?.kind === 'group') {
      const g = insights.groups.find((x) => x.name === scope.id)
      return g ? groupLabel(g.name, g.paths.length) : scope.id
    }
    return scope ? repoName(scope.id) : ''
  })

  function onScope(e: Event & { currentTarget: HTMLSelectElement }) {
    const v = e.currentTarget.value
    const i = v.indexOf(':')
    void insights.setScope({ kind: v.slice(0, i) as 'repo' | 'group', id: v.slice(i + 1) })
  }

  function onDate(e: Event & { currentTarget: HTMLInputElement }) {
    const v = e.currentTarget.value
    // The end of the chosen UTC day, so the whole day counts.
    if (v) void insights.setAsOf(Date.parse(`${v}T23:59:59Z`) / 1000)
  }

  const eyebrow = $derived(
    insights.window === 'releases' ? 'between two releases' : insights.window === 'month' ? 'this month so far' : insights.window === '7d' ? 'the last week' : 'the last two weeks',
  )
  const flagLabel: Record<string, string> = { stale: 'stale', 'long-lived': 'long-lived', drifting: 'drifting', 'big-batch': 'big batch' }
</script>

<div class="insights">
  <nav class="rail" aria-label="Insights inputs">
    <span class="hand brand">insights</span>

    <label class="group">
      <span class="meta">Scope</span>
      <select class="field" value={scopeValue} onchange={onScope} title={scopeTitle}>
        {#if session.tabs.length}
          <optgroup label="Repositories">
            {#each session.tabs as tab (tab.path)}<option value="repo:{tab.path}">{tab.name}</option>{/each}
          </optgroup>
        {/if}
        {#if insights.groups.length}
          <optgroup label="Project groups">
            {#each insights.groups as g (g.name)}<option value="group:{g.name}">{groupLabel(g.name, g.paths.length)}</option>{/each}
          </optgroup>
        {/if}
      </select>
    </label>

    <div class="group">
      <span class="meta">Window</span>
      <span class="segmented" role="group" aria-label="Window">
        {#each windows as win (win.id)}
          {#if win.id !== 'releases' || (scope?.kind === 'repo' && insights.tags.length >= 2)}
            <button aria-pressed={insights.window === win.id} onclick={() => insights.setWindow(win.id)}>{win.label}</button>
          {/if}
        {/each}
      </span>
      {#if insights.window === 'releases'}
        <label class="sub-field"
          ><span>From</span>
          <select class="field" bind:value={insights.from} onchange={() => insights.load()}>
            {#each insights.tags as tag (tag.name)}<option value={tag.name}>{tag.name} · {tag.branch ? 'forked ' : ''}{utcDay(tag.time)}</option>{/each}
          </select></label
        >
        <label class="sub-field"
          ><span>To</span>
          <select class="field" bind:value={insights.to} onchange={() => insights.load()}>
            {#each insights.tags as tag (tag.name)}<option value={tag.name}>{tag.name} · {tag.branch ? 'forked ' : ''}{utcDay(tag.time)}</option>{/each}
          </select></label
        >
      {/if}
      {#if w}
        <span class="resolved" title="Committer dates, inclusive">{w.range || `${utcDay(w.since, w.until)} – ${utcDay(w.until)}, ${new Date(w.until * 1000).getUTCFullYear()} UTC`}</span>
      {/if}
    </div>

    <div class="group">
      <span class="meta">As of</span>
      {#if w}
        <span class="as-of">
          {#if insights.asOf}{utcMoment(w.asOf)} UTC{:else}{w.target} at <code>{w.targetSha.slice(0, 7)}</code>, {utcMoment(w.asOf)} UTC{/if}
        </span>
      {/if}
      {#if pickingDate}
        <input class="field" type="date" aria-label="As of date" onchange={onDate} />
      {/if}
      {#if insights.asOf}
        <button class="btn text start" onclick={() => ((pickingDate = false), insights.setAsOf(0))}>use the branch tip</button>
      {:else}
        <button class="btn text start" onclick={() => (pickingDate = !pickingDate)}>pick a date instead</button>
      {/if}
    </div>

    <div class="group views">
      <h2 class="meta">View</h2>
      {#each VIEWS as v (v.id)}
        <button class="item" class:on={insights.view === v.id} aria-current={insights.view === v.id ? 'page' : undefined} onclick={() => insights.setView(v.id)}
          >{v.label}{#if v.id === 'shipped' && w}<span class="count">{insights.items.length}</span>{:else if v.id === 'attention' && w}<span class="count"
              >{t.attention}</span
            >{/if}</button
        >
      {/each}
    </div>

    <div class="fresh">
      <span class="meta">Remote branches</span>
      {#each insights.repos as repo (repo)}
        {@const at = insights.data[repo]?.fetchedAt ?? 0}
        {@const old = !at || Date.now() / 1000 - at > 86400}
        <span class="fresh-line" class:old>{multi ? `${repoName(repo)}: ` : ''}{at ? `fetched ${ago(at)}` : 'never fetched'}</span>
      {/each}
      <button class="btn secondary fetch" disabled={insights.fetching} onclick={() => insights.fetch()}>{insights.fetching ? 'Fetching…' : multi ? 'Fetch all' : 'Fetch'}</button>
    </div>
  </nav>

  <section class="main" aria-label="Insights" aria-busy={insights.loading}>
    <header class="head">
      <div class="titles">
        <span class="hand eyebrow">{eyebrow}</span>
        <h1>{insights.label()}</h1>
        {#if multi}<span class="repos">{insights.repos.map(repoName).join(' · ')}</span>{/if}
      </div>
      {#if insights.loading}<span class="busy">Computing…</span>{/if}
      {#if w}
        <div class="exports">
          {#if insights.view !== 'overview'}
            <button class="btn secondary" onclick={() => insights.exportCSV(insights.view === 'shipped' ? 'shipped' : insights.view === 'attention' ? 'attention' : 'hotspots')}>Export CSV</button>
          {/if}
          <InkButton onclick={() => (summaryOpen = true)}>Copy summary</InkButton>
        </div>
      {/if}
    </header>

    {#if insights.error}<p class="banner" role="alert"><Icon name="warning" size={16} />{insights.error}</p>{/if}
    {#each failed as repo (repo)}
      <p class="banner" role="alert"><Icon name="warning" size={16} />{repoName(repo)}: {insights.data[repo].error}</p>
    {/each}
    {#if !scope}
      <p class="empty"><span class="hand">nothing to measure</span> Open a repository or make a project group to see its insights.</p>
    {/if}

    {#if w && insights.view === 'shipped'}
      <ShippedView />
    {:else if w && insights.view === 'attention'}
      <AttentionView />
    {:else if w && insights.view === 'hotspots'}
      <HotspotsView />
    {:else if w}
      <div class="tiles">
        <div class="tile teal">
          <span class="meta">Shipped</span><span class="num">{t.shipped}</span>
          <span class="note" title="First-parent commits on {w.target} committed in the window">items on {w.target.replace(/^origin\//, '')}</span>
        </div>
        <div class="tile plum">
          <span class="meta">People</span><span class="num">{t.people}</span>
          <span class="note" title="Distinct author emails of shipped items, merged by the .mailmap committed on the target">authored a ship</span>
        </div>
        <div class="tile rose">
          <span class="meta">Needs attention</span><span class="num accent">{t.attention}</span>
          <span class="note" title="Open branches past a threshold, or that would conflict with the target"
            >branches{#if t.conflicts}, <span class="danger">{t.conflicts} would conflict</span>{/if}{#if insights.pending}, {insights.pending} checking{/if}</span
          >
        </div>
        <div class="tile sepia">
          <span class="meta">Rework</span><span class="num">{t.rework}</span>
          <span class="note" title="Reverts, and fixup!, squash! or amend! commits, among the shipped items">reverts and fixups</span>
        </div>
      </div>

      <section class="strip-section" aria-labelledby="per-day">
        <h2 id="per-day">Shipped per day <span class="meta">UTC</span></h2>
        <div class="strip" role="img" aria-label="Items shipped per day: {days.map((d) => `${utcDay(d.day)} ${d.count}`).join(', ')}">
          {#each days as d (d.day)}
            <span class="bar" class:none={d.count === 0} style:height={d.count ? `${(d.count / peak) * 100}%` : undefined} title="{utcDay(d.day)}: {d.count}"></span>
          {/each}
        </div>
        {#if days.length}
          <div class="axis"><span>{utcDay(days[0].day)}</span><span>{utcDay(days[days.length - 1].day)}</span></div>
        {/if}
      </section>

      <div class="columns">
        <section class="list cascade" aria-labelledby="attn">
          <h2 id="attn">Needs attention <button class="btn text all" onclick={() => insights.setView('attention')}>all {t.attention} →</button></h2>
          {#each flagged.slice(0, 5) as b (b.repo + b.value.name)}
            <div class="row">
              <div class="who">
                <code class="branch">{b.value.name}</code>
                <span class="sub">{multi ? `${repoName(b.repo)} · ` : ''}{b.value.author}{b.value.local ? ' · local only' : ''}</span>
              </div>
              <div class="tags">
                {#if b.conflict?.conflicts}<span class="tag rose" title={b.conflict.files.join(', ')}>conflicts</span>{/if}
                {#each b.value.flags as f (f)}<span class="tag {f === 'drifting' ? 'sepia' : 'plum'}">{flagLabel[f] ?? f}</span>{/each}
              </div>
            </div>
          {:else}
            <p class="sub quiet">{insights.pending ? 'Nothing flagged yet; conflict checks are still running.' : 'Nothing is stuck. Every open branch is within its thresholds.'}</p>
          {/each}
        </section>

        <section class="list cascade" aria-labelledby="latest">
          <h2 id="latest">Latest ships <button class="btn text all" onclick={() => insights.setView('shipped')}>all {t.shipped} →</button></h2>
          {#each ships.slice(0, 5) as s (s.repo + s.value.sha)}
            <div class="row">
              <div class="who">
                <span>{s.value.title}</span>
                <span class="sub">
                  {#if s.value.refs.length}<code>{s.value.refs.join(' ')}</code> ·
                  {/if}{multi ? `${repoName(s.repo)} · ` : ''}{s.value.author}{#if s.value.revert || s.value.fixup}
                    · <span class="rework">rework</span>{/if}
                </span>
              </div>
              <span class="when">{utcDay(s.value.committed)}</span>
            </div>
          {:else}
            <p class="sub quiet">Nothing landed in this window.</p>
          {/each}
        </section>
      </div>

      <p class="same"><span class="hand">same for everyone</span>Computed from the repositories alone. Anyone with them at these commits, with this window and as-of point, gets the same numbers.</p>
    {/if}
  </section>
</div>

{#if summaryOpen}<SummaryDialog onClose={() => (summaryOpen = false)} />{/if}

<style>
  .insights {
    display: flex;
    height: 100%;
  }

  .rail {
    flex: 0 0 272px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 22px;
    padding: 26px 16px 20px;
    background: var(--paper-warm);
    border-right: 1px solid var(--paper-edge);
  }

  .brand {
    margin-left: 12px;
    font-size: 26px;
    line-height: 1;
    transform-origin: left;
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 0 12px;
  }

  .group h2 {
    margin: 0;
    font-weight: 400;
  }

  /* The window options wrap as a row, never inside a label. */
  .group .segmented {
    flex-wrap: wrap;
    align-self: flex-start;
  }

  .group .segmented button {
    white-space: nowrap;
  }

  .views {
    gap: 2px;
    padding: 0;
  }

  .views h2 {
    padding: 0 12px 6px;
  }

  .field {
    width: 100%;
    font-family: var(--font-serif);
    font-size: 14px;
  }

  .sub-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .resolved,
  .as-of {
    font-size: 13px;
    color: var(--ink-soft);
  }

  .resolved {
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .as-of code,
  .who code {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-b);
  }

  .start {
    align-self: flex-start;
  }

  .item {
    position: relative;
    display: flex;
    align-items: center;
    min-height: 40px;
    padding: 8px 12px;
    background: none;
    border: none;
    border-radius: var(--r-md);
    color: var(--ink-soft);
    font-size: 15px;
    text-align: left;
  }

  .item:hover:not(:disabled) {
    background: var(--wash-hover);
    color: var(--ink);
  }

  .item.on {
    color: var(--ink);
  }

  .item.on::before {
    content: '';
    position: absolute;
    left: 0;
    top: 8px;
    bottom: 8px;
    width: 4px;
    background: var(--link);
    filter: url(#wc-edge);
    border-radius: 45% 55% 50% 50% / 55% 45%;
    opacity: 0.9;
  }

  .count {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-faint);
  }

  .all {
    margin-left: auto;
    font-weight: 400;
  }

  .fresh {
    margin-top: auto;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px;
    border-top: 1px dotted var(--paper-edge);
    font-size: 13px;
    color: var(--ink-mute);
  }

  .fresh-line.old {
    color: var(--warning);
  }

  .fetch {
    align-self: flex-start;
    margin-top: 6px;
  }

  .main {
    flex: 1;
    min-width: 0;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 26px;
    padding: 30px 40px 48px;
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 16px 24px;
  }

  .titles {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .eyebrow {
    align-self: flex-start;
    margin-bottom: 6px;
    color: var(--link);
    transform: rotate(-1.5deg);
    transform-origin: left;
  }

  h1 {
    margin: 0;
    font-size: 34px;
    font-weight: 400;
    letter-spacing: -0.02em;
    line-height: 1.1;
    font-variation-settings:
      'SOFT' 50,
      'WONK' 1;
  }

  .repos {
    margin-top: 8px;
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--ink-mute);
  }

  .exports {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .busy {
    font-style: italic;
    color: var(--ink-mute);
  }

  .banner {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 0;
    padding: 10px 16px;
    border-radius: var(--r-md);
    background: var(--danger-wash);
  }

  .tiles {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 18px;
  }

  .tile {
    position: relative;
    isolation: isolate;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 18px 20px;
  }

  /* Each tile sits on a torn wash of its own colour. */
  .tile::before {
    content: '';
    position: absolute;
    inset: 0;
    z-index: -1;
    filter: url(#wc-edge);
    border-radius: 46% 54% 52% 48% / 14% 18% 16% 12%;
  }

  .tile.teal::before {
    background: color-mix(in srgb, var(--accent-a) 13%, transparent);
  }

  .tile.plum::before {
    background: color-mix(in srgb, var(--accent-b) 13%, transparent);
    border-radius: 52% 48% 46% 54% / 16% 12% 18% 14%;
  }

  .tile.rose::before {
    background: color-mix(in srgb, var(--link) 12%, transparent);
    border-radius: 48% 52% 54% 46% / 12% 16% 14% 18%;
  }

  .tile.sepia::before {
    background: color-mix(in srgb, var(--accent-c) 15%, transparent);
    border-radius: 54% 46% 48% 52% / 18% 14% 12% 16%;
  }

  .num {
    font-size: 44px;
    line-height: 1;
    letter-spacing: -0.02em;
  }

  .num.accent {
    color: var(--link);
  }

  .note {
    font-size: 13px;
    color: var(--ink-mute);
  }

  .danger {
    color: var(--danger);
  }

  .main h2 {
    display: flex;
    align-items: baseline;
    gap: 12px;
    margin: 0;
    font-size: 20px;
    font-weight: 500;
    letter-spacing: -0.01em;
  }

  .strip-section {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .strip {
    display: flex;
    align-items: flex-end;
    gap: 6px;
    height: 120px;
    border-bottom: 1px solid var(--paper-edge);
  }

  .bar {
    flex: 1;
    min-width: 2px;
    border-radius: var(--r-sm) var(--r-sm) 0 0;
    background: var(--accent-a);
    opacity: 0.85;
  }

  .bar.none {
    height: 2px;
    border-radius: 0;
    background: var(--paper-edge);
    opacity: 1;
  }

  .axis {
    display: flex;
    justify-content: space-between;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-faint);
  }

  .columns {
    display: flex;
    flex-wrap: wrap;
    gap: 28px 40px;
  }

  .list {
    flex: 1 1 360px;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .list h2 {
    margin-bottom: 6px;
  }

  .row {
    display: flex;
    align-items: flex-start;
    gap: 16px;
    padding: 12px 4px;
    border-top: 1px dotted var(--paper-edge);
  }

  .who {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .branch {
    font-size: 14px !important;
    overflow-wrap: anywhere;
  }

  .sub {
    font-size: 13px;
    color: var(--ink-mute);
  }

  .quiet {
    margin: 0;
    padding: 12px 4px;
    border-top: 1px dotted var(--paper-edge);
  }

  .tags {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 6px;
  }

  .rework {
    color: var(--accent-c);
  }

  .when {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-faint);
    white-space: nowrap;
  }

  .same,
  .empty {
    display: flex;
    align-items: baseline;
    gap: 12px;
    margin: 0;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .same .hand,
  .empty .hand {
    font-size: 20px;
    color: var(--accent-c);
    white-space: nowrap;
  }
</style>
