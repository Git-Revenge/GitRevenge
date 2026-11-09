<script lang="ts">
  // A CSV or TSV file as a sortable table. Large files are capped; Raw shows the whole text.
  import type { PreviewProps } from './registry'
  import { compareCells, parseDelimited } from './csv'

  const MAX_ROWS = 5000

  let { path, text }: PreviewProps = $props()

  let delimiter = $derived(/\.(tsv|tab)$/i.test(path) ? '\t' : ',')
  let parsed = $derived(parseDelimited(text, delimiter, MAX_ROWS + 1))
  let header = $state(true)
  let sort = $state<{ col: number; dir: 1 | -1 } | null>(null)

  let width = $derived(Math.max(0, ...parsed.rows.map((r) => r.length)))
  let head = $derived(header && parsed.rows.length ? parsed.rows[0] : null)
  let body = $derived.by(() => {
    const rows = (header ? parsed.rows.slice(1) : parsed.rows).slice(0, MAX_ROWS)
    if (!sort) return rows
    const { col, dir } = sort
    return [...rows].sort((a, b) => dir * compareCells(a[col] ?? '', b[col] ?? ''))
  })
  let truncated = $derived(parsed.truncated || parsed.rows.length > MAX_ROWS)

  /** Click order: ascending, descending, then back to file order. */
  function toggleSort(col: number) {
    if (sort?.col !== col) sort = { col, dir: 1 }
    else if (sort.dir === 1) sort = { col, dir: -1 }
    else sort = null
  }

  const ariaSort = (col: number) => (sort?.col === col ? (sort.dir === 1 ? 'ascending' : 'descending') : 'none')
  const columnName = (col: number) => head?.[col] || `Column ${col + 1}`
</script>

<div class="csv">
  <div class="bar">
    <label><input type="checkbox" bind:checked={header} /> First row is a header</label>
    <span class="meta">{body.length.toLocaleString()} rows · {width} columns{truncated ? ` · first ${MAX_ROWS.toLocaleString()} shown, Raw has the rest` : ''}</span>
  </div>
  <div class="scroll">
    <table>
      <thead>
        <tr>
          {#each Array.from({ length: width }, (_, i) => i) as col (col)}
            <th aria-sort={ariaSort(col)}>
              <button onclick={() => toggleSort(col)} title="Sort by {columnName(col)}"
                >{columnName(col)}<span class="arrow" aria-hidden="true">{sort?.col === col ? (sort.dir === 1 ? '↑' : '↓') : ''}</span></button
              >
            </th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each body as row, r (r)}
          <tr>
            {#each Array.from({ length: width }, (_, i) => i) as col (col)}<td>{row[col] ?? ''}</td>{/each}
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

<style>
  .csv {
    padding: 12px 16px 24px;
  }

  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 18px;
    margin-bottom: 10px;
    font-size: 13px;
  }

  .scroll {
    overflow: auto;
  }

  table {
    border-collapse: collapse;
    font-family: var(--font-mono);
    font-size: 13px;
  }

  th,
  td {
    padding: 4px 10px;
    border: 1px solid var(--paper-edge);
    text-align: left;
    vertical-align: top;
    white-space: pre-wrap;
    max-width: 40ch;
    overflow-wrap: anywhere;
  }

  th {
    position: sticky;
    top: 0;
    padding: 0;
    background: var(--paper-warm);
  }

  th button {
    display: flex;
    gap: 6px;
    width: 100%;
    padding: 5px 10px;
    background: none;
    border: none;
    border-radius: 0;
    font: inherit;
    font-weight: 600;
    text-align: left;
  }

  th button:hover:not(:disabled) {
    background: var(--wash-hover);
  }

  .arrow {
    color: var(--link);
  }

  tbody tr:nth-child(even) {
    background: color-mix(in srgb, var(--paper-warm) 50%, transparent);
  }
</style>
