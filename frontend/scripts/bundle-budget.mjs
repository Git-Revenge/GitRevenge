// Keeps what loads when the app starts in check. Two budgets, for the chunk the page loads
// first (issue #850):
//
//   - Libraries in it: 100kB. This is the one that matters. KaTeX, CodeMirror and two parsers
//     once sat in the startup chunk and made it 1,146kB (#370); a heavy library belongs behind
//     ui/Lazy.svelte, loaded when something needs it. Today the only library there is marked.
//   - The whole chunk: 600kB, as a backstop for the app's own code. Measured in Edge, which is
//     what WebView2 runs, a 400kB chunk takes 13 ms to run, and 73 ms on a machine four times
//     slower: about 18 ms for each 100kB there. 600kB is around 110 ms, where a wait starts to
//     be felt. Reaching it is a prompt to look at what loads at startup, not a wall.
//
// The Svelte runtime has a chunk of its own (see vite.config.ts) and isn't counted.
//
// It builds the app again with source maps, into a folder of its own, to see what the chunk
// holds: the app as shipped has no source maps. Run it with `npm run bundle-budget`.
import { Buffer } from 'node:buffer'
import { execFileSync } from 'node:child_process'
import { readFileSync, readdirSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import process from 'node:process'

const LIBRARIES_KB = 100
const TOTAL_KB = 600

const out = 'node_modules/.cache/bundle-budget'
execFileSync(process.execPath, ['node_modules/vite/bin/vite.js', 'build', '--sourcemap', '--outDir', out, '--emptyOutDir', '--logLevel', 'warn'], { stdio: 'inherit' })

const dir = join(out, 'assets')
const files = readdirSync(dir)
const entry = files.find((f) => /^index-.*\.js$/.test(f))
if (!entry) {
  console.error(`No main chunk in ${dir}.`)
  process.exit(1)
}

/** Decodes one source map segment: its fields are base64 VLQ numbers. */
const BASE64 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
function vlq(segment) {
  const fields = []
  let value = 0
  let shift = 0
  for (const ch of segment) {
    const digit = BASE64.indexOf(ch)
    value |= (digit & 31) << shift
    if (digit & 32) {
      shift += 5
    } else {
      fields.push(value & 1 ? -(value >>> 1) : value >>> 1)
      value = 0
      shift = 0
    }
  }
  return fields
}

/**
 * Charges every character of a chunk to the source file its map says it came from. What the
 * map doesn't place (code the bundler and the compiler wrote, such as a component's markup)
 * is counted apart.
 */
function contents(file) {
  // The map's own comment at the end isn't in the app as shipped.
  const code = readFileSync(join(dir, file), 'utf8').replace(/\n?\/\/# sourceMappingURL=.*\s*$/, '')
  const map = JSON.parse(readFileSync(join(dir, `${file}.map`), 'utf8'))
  const lines = code.split('\n')
  const bySource = new Map()
  let unplaced = 0
  const charge = (source, n) => {
    if (source < 0) unplaced += n
    else bySource.set(source, (bySource.get(source) ?? 0) + n)
  }
  let source = 0
  map.mappings.split(';').forEach((row, i) => {
    const length = lines[i]?.length ?? 0
    if (!row) {
      unplaced += length
      return
    }
    let column = 0
    let from = 0
    let current = -1
    for (const segment of row.split(',')) {
      const fields = vlq(segment)
      column += fields[0]
      charge(current, column - from)
      from = column
      // A segment with one field marks code that came from no source.
      if (fields.length >= 4) {
        source += fields[1]
        current = source
      } else {
        current = -1
      }
    }
    charge(current, length - from)
  })
  // A map names its sources from where the chunk is, which here is inside node_modules, so a
  // library's path wouldn't say "node_modules". From the project's folder, it does.
  const place = (source) => relative(process.cwd(), resolve(dir, source)).replaceAll('\\', '/')
  const sources = [...bySource].map(([index, bytes]) => ({ path: place(map.sources[index]), bytes }))
  sources.sort((a, b) => b.bytes - a.bytes)
  return { bytes: Buffer.byteLength(code), unplaced, sources }
}

const kb = (bytes) => `${Math.round(bytes / 1024)}kB`
const sum = (list) => list.reduce((total, item) => total + item.bytes, 0)

const chunk = contents(entry)
const libraries = new Map()
for (const s of chunk.sources) {
  const name = /(?:^|\/)node_modules\/((?:@[^/]+\/)?[^/]+)/.exec(s.path)?.[1]
  if (name) libraries.set(name, (libraries.get(name) ?? 0) + s.bytes)
}
const libraryBytes = [...libraries.values()].reduce((a, b) => a + b, 0)
const own = chunk.sources.filter((s) => !s.path.includes('node_modules/'))
const byPackage = [...libraries].sort((a, b) => b[1] - a[1]).map(([name, bytes]) => `${name} ${kb(bytes)}`)

const runtime = files.find((f) => /^svelte-.*\.js$/.test(f))
console.log(`Startup chunk: ${kb(chunk.bytes)} of ${TOTAL_KB}kB`)
console.log(`  libraries           ${kb(libraryBytes).padStart(6)} of ${LIBRARIES_KB}kB${byPackage.length ? `: ${byPackage.join(', ')}` : ''}`)
console.log(`  the app's own code  ${kb(sum(own)).padStart(6)} in ${own.length} files`)
console.log(`  written by the build ${kb(chunk.unplaced).padStart(5)} (markup and glue the source map doesn't place)`)
if (runtime) console.log(`  beside it, not counted: the Svelte runtime, ${kb(Buffer.byteLength(readFileSync(join(dir, runtime))))}`)
console.log("  the app's biggest files:")
for (const s of own.slice(0, 8)) console.log(`    ${kb(s.bytes).padStart(6)}  ${s.path}`)

let failed = false
if (libraryBytes > LIBRARIES_KB * 1024) {
  failed = true
  console.error(`\nLibraries in the startup chunk come to ${kb(libraryBytes)}, over their ${LIBRARIES_KB}kB budget.`)
  console.error(`Load the heavy one on demand, where it's first needed: see ui/Lazy.svelte. They are: ${byPackage.join(', ')}.`)
}
if (chunk.bytes > TOTAL_KB * 1024) {
  failed = true
  console.error(`\nThe startup chunk is ${kb(chunk.bytes)}, over its ${TOTAL_KB}kB budget.`)
  console.error("If no library is new, this is the app's own code. Move what isn't needed at startup behind ui/Lazy.svelte:")
  console.error("a tab's state (see people/tab.svelte.ts for how a tab keeps only a small handle here), a dialog, a rarely used panel.")
  console.error('If it all has to load at startup, raise TOTAL_KB in this file and say why in the commit.')
}
process.exit(failed ? 1 : 0)
