<script lang="ts">
  // Trailer care: where a repository says how its trailers should be read. Real histories are
  // inconsistent, and some of that isn't the app's to settle. This asks what it won't decide,
  // lists what it joined without asking, and shows every key with where it's offered and why.
  //
  // Nothing here changes a commit. An answer is a rule in the repository's git config, which
  // only says how to read what's already written, and each is written the moment it's given.
  import { AnswerTrailerQuestion, AnswerTrailerSlips, SetTrailerKind, SetTrailerPlace, TrailerCare, UndoTrailerJoin } from '../../../wailsjs/go/app/App.js'
  import type { app } from '../../../wailsjs/go/models'
  import { errorText } from '../errors'
  import { onBackend } from '../events'
  import { gitConfig } from '../gitconfig/config.svelte'
  import { peopleTab } from '../people/tab.svelte'
  import { session } from '../session.svelte'
  import Icon from '../ui/Icon.svelte'
  import Loading from '../ui/Loading.svelte'
  import Modal from '../ui/Modal.svelte'
  import { alsoWritten, filterKeys, groupQuestions, keysText, kindRule, KINDS, openText, peopleText, PLACES, sameText, sortKeys, whyText, type CareKey, type Question, type SortBy } from './care'
  import { commitsText, kindWord, soFar } from './filters'

  let { repo, onClose }: { repo: string; onClose: () => void } = $props()

  let repoName = $derived(session.tabs.find((t) => t.path === repo)?.name ?? '')
  let care = $state<app.TrailerCare | null>(null)
  let error = $state('')
  /** What's being written, so its buttons wait: a question's keys, or a table row's key. */
  let busy = $state('')

  let asks = 0
  async function load() {
    const n = ++asks
    try {
      const got = await TrailerCare(repo)
      if (n === asks) care = got
    } catch (e) {
      if (n === asks) error = errorText(e)
    }
  }
  $effect(() => {
    void repo
    void load()
  })
  // More of the history read, or a rule written by hand, changes what there is to ask.
  $effect(() =>
    onBackend('trailers:progress', (p) => {
      if (p.path === repo && !p.reading) void load()
    }),
  )
  $effect(() => {
    void session.configRevision[repo]
    void load()
  })

  /** Writes a rule, and shows the repository as it's read with it. */
  async function write(what: string, fn: () => Promise<app.TrailerCare>) {
    busy = what
    asks++
    try {
      const got = await fn()
      if (got) care = got
      error = ''
    } catch (e) {
      error = errorText(e)
      void load()
    } finally {
      busy = ''
    }
  }

  const answer = (q: Question, join: boolean) => write(`${q.key}\n${q.other}`, () => AnswerTrailerQuestion(repo, q.key, q.other, join))
  /** Every slip of one key read as that key, written together: each is its own rule line, and
   * the history is read through them once. */
  const answerAll = (slips: Question[]) =>
    write(
      slips[0].key,
      () =>
        AnswerTrailerSlips(
          repo,
          slips[0].key,
          slips.map((q) => q.other),
        ),
    )

  let groups = $derived(groupQuestions(care?.questions ?? []))
  let listed = $derived(care?.questions.length ?? 0)

  // The table of keys: sorted by a column, narrowed by a filter, and shown a hundred at a
  // time. A long history has a couple of thousand keys.
  const PAGE = 100
  let filter = $state('')
  let sortBy = $state<SortBy>('commits')
  let reverse = $state(false)
  let shown = $state(PAGE)
  let rows = $derived(sortKeys(filterKeys(care?.keys ?? [], filter), sortBy, reverse))
  $effect(() => {
    void filter
    void sortBy
    void reverse
    shown = PAGE
  })
  function sort(by: SortBy) {
    if (sortBy === by) reverse = !reverse
    else {
      sortBy = by
      reverse = false
    }
  }
  /** How a column says it's sorted. Numbers start from the most, so that's "descending". */
  function sorted(by: SortBy): 'ascending' | 'descending' | 'none' {
    if (sortBy !== by) return 'none'
    const fromMost = by === 'values' || by === 'commits'
    return reverse !== fromMost ? 'descending' : 'ascending'
  }

  const setKind = (k: CareKey, kind: string) => write(k.key.id, () => SetTrailerKind(repo, k.key.id, kind))
  const setPlace = (k: CareKey, place: string) => write(k.key.id, () => SetTrailerPlace(repo, k.key.id, place))
  const undo = (k: CareKey, other: string) => write(k.key.id, () => UndoTrailerJoin(repo, k.key.id, other))

  let joinedOpen = $state(false)
  let note = $derived(care ? soFar(care.status) : '')
</script>

{#snippet header(by: SortBy, label: string, numeric = false)}
  <th scope="col" aria-sort={sorted(by)} class:numeric>
    <button class="sort" onclick={() => sort(by)} title="Sort by {label.toLowerCase()}">
      {label}{#if sortBy === by}<span class="arrow" class:up={sorted(by) === 'ascending'}><Icon name="chevron" size={11} /></span>{/if}
    </button>
  </th>
{/snippet}

<Modal title="Trailer care" eyebrow="how this history is read" width={940} {onClose}>
  {#snippet subtitle()}
    {#if repoName}<span class="meta">{repoName}</span>{/if}
  {/snippet}
  {#if error}<p class="failed" role="alert">{error}</p>{/if}
  {#if !care}
    {#if !error}<Loading />{/if}
  {:else}
    <p class="lede">
      A rule only says how to read what’s already written. Nothing here changes a commit, and every answer can be undone.
      {#if note}<span class="meta">{note}</span>{/if}
    </p>

    <section aria-labelledby="care-needs">
      <h3 id="care-needs">Needs care</h3>
      <p class="count" role="status">{openText(care.open, listed)}</p>

      {#each groups.slips as g (g.key)}
        <div class="question" role="group" aria-label="Spellings one letter off {g.keyLabel}">
          <p>
            {g.slips.length === 1 ? 'One key is' : `${g.slips.length} keys are`} one letter off <strong>{g.keyLabel}</strong>, which is on {commitsText(g.commits)}. A slip, or {g.slips.length === 1 ? 'a key' : 'keys'} of
            {g.slips.length === 1 ? 'its' : 'their'} own?
          </p>
          <ul class="slips">
            {#each g.slips as q (q.other)}
              <li>
                <span class="slip"><strong>{q.otherLabel}</strong> <span class="meta">{commitsText(q.otherCommits)}</span></span>
                <button class="btn sm" disabled={!!busy} onclick={() => answer(q, true)}>Read as {q.keyLabel}</button>
                <button class="btn sm quiet" disabled={!!busy} onclick={() => answer(q, false)} aria-label="Keep {q.otherLabel} apart from {q.keyLabel}">Keep apart</button>
              </li>
            {/each}
          </ul>
          {#if g.slips.length > 1}
            <button class="btn sm text" disabled={!!busy} onclick={() => answerAll(g.slips)}>Read all {g.slips.length} as {g.keyLabel}</button>
          {/if}
        </div>
      {/each}

      {#each groups.same as q (q.key + '\n' + q.other)}
        <div class="question" role="group" aria-label="{q.otherLabel} and {q.keyLabel}">
          <p>
            <strong>{q.otherLabel}</strong> <span class="meta">{commitsText(q.otherCommits)}</span> and <strong>{q.keyLabel}</strong>
            <span class="meta">{commitsText(q.commits)}</span>
            {sameText(q)} Joined, the row reads “{q.keyLabel}”.
          </p>
          <div class="answers">
            <button class="btn sm" disabled={!!busy} onclick={() => answer(q, true)}>Read {q.otherLabel} as {q.keyLabel}</button>
            <button class="btn sm quiet" disabled={!!busy} onclick={() => answer(q, false)} aria-label="Keep {q.otherLabel} apart from {q.keyLabel}">Keep apart</button>
          </div>
        </div>
      {/each}

      {#if care.peopleOpen > 0}
        <div class="question people" role="group" aria-label="People written two ways">
          <p>{peopleText(care.peopleOpen)} They join once the mailmap says so, which is done in People, not here.</p>
          <ul class="written">
            {#each care.people as p, i (i)}
              <li>{p.examples.join('  ·  ')}</li>
            {/each}
          </ul>
          <button
            class="btn sm quiet"
            onclick={() => {
              onClose()
              void peopleTab.show(repo)
            }}><Icon name="people" size={13} />Open People</button
          >
        </div>
      {/if}
    </section>

    <section aria-labelledby="care-joined">
      <h3 id="care-joined">
        <button class="fold" aria-expanded={joinedOpen} onclick={() => (joinedOpen = !joinedOpen)}>
          <span class="caret" class:open={joinedOpen}><Icon name="right" size={11} /></span>Joined without asking
        </button>
      </h3>
      {#if joinedOpen}
        <ul class="joined">
          <li>
            <strong>Letter case and word endings.</strong>
            {#if care.joined.spellings.length}
              {care.joined.spellings.length.toLocaleString()}
              {care.joined.spellings.length === 1 ? 'key is' : 'keys are'} written more than one way and read as one:
              {#each care.joined.spellings.slice(0, 12) as k (k.id)}<span class="spelled">{k.spellings.map((s) => s.text).join(', ')}</span>{/each}
              {#if care.joined.spellings.length > 12}<span class="meta">and {(care.joined.spellings.length - 12).toLocaleString()} more</span>{/if}
            {:else}
              No key here is written two ways.
            {/if}
          </li>
          <li><strong>Several on a line.</strong> {care.joined.several.toLocaleString()} {care.joined.several === 1 ? 'line names' : 'lines name'} more than one value, as “#12, #14” does: each is its own value.</li>
          <li><strong>Repeats.</strong> A value said twice on one commit counts once.</li>
          <li><strong>A number and its address.</strong> “#14” and the address of issue 14 are one ticket, shown by the shorter.</li>
          <li><strong>People.</strong> Someone the mailmap joins is one person under every key.</li>
          <li><strong>Bots.</strong> {care.joined.bots.toLocaleString()} of the people named {care.joined.bots === 1 ? 'is a bot' : 'are bots'}, listed after everyone else.</li>
        </ul>
      {/if}
    </section>

    <section aria-labelledby="care-keys">
      <h3 id="care-keys">Keys</h3>
      <div class="tools">
        <label class="filter">
          <Icon name="search" size={13} />
          <input type="search" bind:value={filter} placeholder="Filter keys" aria-label="Filter keys" />
        </label>
        <span class="meta" role="status">{keysText(rows.length, care.keys.length)}</span>
      </div>
      <div class="table">
        <table>
          <thead>
            <tr>
              {@render header('key', 'Key')}
              {@render header('kind', 'Kind')}
              {@render header('values', 'Values', true)}
              {@render header('commits', 'Commits', true)}
              {@render header('where', 'Shows')}
              <th scope="col">Why</th>
            </tr>
          </thead>
          <tbody>
            {#each rows.slice(0, shown) as k (k.key.id)}
              {@const also = alsoWritten(k)}
              <tr>
                <th scope="row">
                  <span class="key-name">{k.key.label}</span>
                  {#if also.length}<span class="meta also" title="Also written this way, and read as one">{also.join(', ')}</span>{/if}
                  {#each k.also as other (other)}
                    <span class="join">
                      <span class="meta">reads {other} too</span>
                      <button class="btn sm text" disabled={!!busy} onclick={() => undo(k, other)} aria-label="Stop reading {other} as {k.key.label}">Undo</button>
                    </span>
                  {/each}
                </th>
                <td>
                  <select aria-label="What the values of {k.key.label} are" disabled={!!busy} value={k.kindSaid ? kindRule(k.key.kind) : ''} onchange={(e) => setKind(k, e.currentTarget.value)}>
                    <option value="">{kindWord(k.key.kind)} (a guess)</option>
                    {#each KINDS as kind (kind.value)}<option value={kind.value}>{kind.label}</option>{/each}
                  </select>
                </td>
                <td class="numeric">{k.key.values.toLocaleString()}</td>
                <td class="numeric">{k.key.commits.toLocaleString()}</td>
                <td>
                  <select aria-label="Where {k.key.label} shows" disabled={!!busy} value={k.whereSaid ? k.where : ''} onchange={(e) => setPlace(k, e.currentTarget.value)}>
                    <option value="">{PLACES.find((p) => p.value === k.where)?.label ?? 'Panel'} (a guess)</option>
                    {#each PLACES as place (place.value)}<option value={place.value}>{place.label}</option>{/each}
                  </select>
                </td>
                <td class="why">{whyText(k)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
        {#if rows.length === 0}
          <p class="meta none">{filter.trim() ? 'No key matches that.' : 'No trailers in this history yet.'}</p>
        {/if}
      </div>
      {#if rows.length > shown}
        <button class="btn sm quiet" onclick={() => (shown += PAGE)}>Show {Math.min(PAGE, rows.length - shown).toLocaleString()} more of {(rows.length - shown).toLocaleString()}</button>
      {/if}
    </section>

    <p class="kept">
      Rules are kept in this repository’s git config, under <code>gitrevenge-trailer</code>, beside its smart links. A rule there replaces one for the same key in your global config.
      <button
        class="btn sm text"
        onclick={() => {
          onClose()
          void gitConfig.show(repo)
        }}>Edit git config…</button
      >
    </p>
  {/if}
</Modal>

<style>
  .lede {
    margin: 0 0 14px;
    color: var(--ink-soft);
  }

  .lede .meta {
    display: block;
    margin-top: 4px;
  }

  section {
    margin-bottom: 18px;
    padding-bottom: 14px;
    border-bottom: 1px dashed var(--hairline);
  }

  h3 {
    margin: 0 0 6px;
    font-size: 16px;
    font-weight: 500;
  }

  .count {
    margin: 0 0 10px;
    color: var(--ink-soft);
  }

  .failed {
    color: var(--danger);
  }

  .question {
    margin-bottom: 10px;
    padding: 10px 12px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    background: var(--paper-warm);
  }

  .question p {
    margin: 0 0 8px;
  }

  .slips {
    margin: 0 0 6px;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 4px;
  }

  .slips li,
  .answers {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .slip {
    flex: 1 1 auto;
    min-width: 0;
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .written {
    margin: 0 0 8px;
    padding-left: 18px;
    color: var(--ink-soft);
    font-size: 13px;
  }

  .fold {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 0;
    border: 0;
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }

  .caret {
    display: inline-flex;
    transition: transform var(--dur-quick);
  }

  .caret.open {
    transform: rotate(90deg);
  }

  .joined {
    margin: 6px 0 0;
    padding-left: 18px;
    display: grid;
    gap: 6px;
    color: var(--ink-soft);
  }

  .spelled {
    display: inline-block;
    margin: 2px 6px 2px 0;
    padding: 0 6px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-sm);
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .tools {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 8px;
  }

  .filter {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    flex: 0 1 280px;
    color: var(--ink-mute);
  }

  .filter input {
    flex: 1;
    min-width: 0;
    height: 28px;
    padding: 0 8px;
    font-size: 13px;
  }

  .table {
    max-height: 46vh;
    overflow: auto;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }

  thead th {
    position: sticky;
    top: 0;
    z-index: 1;
    padding: 0;
    background: var(--paper-deep);
    text-align: left;
    font-weight: 500;
    white-space: nowrap;
  }

  thead th:last-child {
    padding: 6px 10px;
  }

  .sort {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    width: 100%;
    padding: 6px 10px;
    border: 0;
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }

  .numeric .sort {
    justify-content: flex-end;
  }

  .arrow {
    display: inline-flex;
  }

  .arrow.up {
    transform: rotate(180deg);
  }

  tbody th,
  tbody td {
    padding: 6px 10px;
    border-top: 1px solid var(--hairline);
    vertical-align: top;
    text-align: left;
    font-weight: 400;
  }

  .numeric {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }

  .key-name {
    font-family: var(--font-mono);
    font-weight: 600;
  }

  .also {
    display: block;
    font-family: var(--font-mono);
    letter-spacing: 0;
    text-transform: none;
  }

  .join {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .join .meta {
    font-family: var(--font-mono);
    letter-spacing: 0;
    text-transform: none;
  }

  select {
    max-width: 150px;
    height: 26px;
    font-size: 13px;
  }

  .why {
    color: var(--ink-soft);
    min-width: 180px;
  }

  .none {
    margin: 0;
    padding: 14px 10px;
  }

  .kept {
    margin: 0;
    color: var(--ink-soft);
    font-size: 13px;
  }
</style>
