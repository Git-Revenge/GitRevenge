// Fails if the app's main chunk outgrows its budget, so a heavy import can't slip back in. Run
// after `vite build`. If this fails, load the new thing on demand rather than raising the limit.
import { readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import process from 'node:process'

const BUDGET_KB = 400
const dir = 'dist/assets'
const entry = readdirSync(dir).find((f) => /^index-.*\.js$/.test(f))
if (!entry) {
  console.error(`No main chunk in ${dir}. Run vite build first.`)
  process.exit(1)
}
const kb = Math.round(statSync(join(dir, entry)).size / 1024)
console.log(`Main chunk: ${kb}kB, budget ${BUDGET_KB}kB`)
if (kb > BUDGET_KB) {
  console.error(`${entry} is over budget. Load something heavy on demand: see ui/Lazy.svelte.`)
  process.exit(1)
}
