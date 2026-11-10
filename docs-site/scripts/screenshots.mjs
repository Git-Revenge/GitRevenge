// Takes the docs' screenshots and animations from the running app.
//
// The app runs as `wails dev`, which serves the real frontend and Go backend to a browser at
// http://localhost:34115. This drives Chrome against it with Puppeteer, resets the demo
// repository (scripts/demo-repo.sh) before each scene, and writes WebP files to docs/img/:
// stills in a light and a dark version, and animations as animated WebP.
//
// To run it (Windows; the paths are what the screenshots show):
//
//   1. Map a drive to an empty folder. Git resolves a mapped drive, so some shots show the
//      real path: keep it short.
//        subst T: A:\demo
//   2. Build the demo once, so its global git config exists:
//        bash scripts/demo-repo.sh /t/
//   3. Point the app's settings at a scratch folder and its global git config at the demo's,
//      so your own repositories and config stay out of the shots, and start it:
//        $env:APPDATA = "C:\some\scratch\appdata"; $env:GIT_CONFIG_GLOBAL = "T:\gitconfig"
//        wails dev -browser=false
//      To keep your everyday wails dev running, start this one from a git worktree, with
//      -devserver localhost:34116, and set APP_URL to match.
//   4. From docs-site/:
//        node scripts/screenshots.mjs [scene ...]
//
// Each scene closes every tab and removes recent repositories, so it would wreck a session you
// use. It refuses to run unless the app's global config is the demo's, which step 3 sets.
//
// It needs Chrome, Git Bash, ffmpeg and ImageMagick (magick). CHROME, BASH_EXE, APP_URL and
// DEMO override where Chrome and bash are, where the app is served, and the demo folder.
import { execFileSync } from 'node:child_process'
import { existsSync, mkdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import puppeteer from 'puppeteer-core'

const here = path.dirname(fileURLToPath(import.meta.url))
const out = path.resolve(here, '../../docs/img')
const work = path.join(tmpdir(), 'gitrevenge-shots')
const APP = process.env.APP_URL ?? 'http://localhost:34115/'
const DEMO = process.env.DEMO ?? 'T:\\'
const CHROME = process.env.CHROME ?? 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'
// Git's bash, not WSL's, which is also on PATH as bash.exe.
const BASH = process.env.BASH_EXE ?? 'C:\\Program Files\\Git\\bin\\bash.exe'
const REPO = path.win32.join(DEMO, 'tidewater')
const WIDTH = 1280
const HEIGHT = 800

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

/** Runs a program, and on failure says what it printed. */
function run(file, args) {
  try {
    return execFileSync(file, args, { stdio: 'pipe', encoding: 'utf8' })
  } catch (e) {
    throw new Error(`${path.basename(file)} ${args.join(' ')}: ${e.stderr || e.message}`)
  }
}

// ── The demo repository and the app's session ─────────────────────────────────────────

/** Closes every tab, rebuilds the demo repositories, and opens the ones named. With clean,
 * tidewater's uncommitted changes are dropped first; with branch, that branch is checked out. */
async function reset(page, open = ['tidewater'], { clean = false, branch } = {}) {
  await page.evaluate(async () => {
    // Tabs that aren't repositories stay open across a reload.
    const pause = (ms) => new Promise((r) => setTimeout(r, ms))
    for (const name of ['Insights', 'Git Config']) {
      const close = document.querySelector(`[aria-label="Close ${name}"]`)
      if (!close) continue
      close.click()
      await pause(300)
      ;[...document.querySelectorAll('[role=dialog] button')].find((b) => b.textContent.includes('Discard'))?.click()
      await pause(300)
    }
    const st = await window.go.app.App.RestoreSession()
    for (const t of st?.tabs ?? []) await window.go.app.App.CloseRepo(t.path)
    await window.go.app.App.SetActiveRepo('')
  })
  // Git processes the app started can hold the folder for a moment after the tab closes.
  for (let tries = 1; ; tries++) {
    try {
      run(BASH, [path.join(here, 'demo-repo.sh'), DEMO.replace(/\\/g, '/')])
      break
    } catch (e) {
      if (tries === 8 || !/busy|denied/i.test(e.message)) throw e
      await sleep(1500)
    }
  }
  if (clean) run('git', ['-C', REPO, 'reset', '-q', '--hard'])
  if (branch) run('git', ['-C', REPO, 'checkout', '-q', branch])
  // Only the demo repositories belong on the start page, and it should look lived in: all
  // three recent, and a group in a known order. tidewater is opened last, to lead the list.
  await page.evaluate(async (demo) => {
    const A = window.go.app.App
    for (const r of (await A.RecentRepos()) ?? []) await A.RemoveRecentRepo(r.path)
    for (const name of ['Harbour team', 'Field notes']) await A.GroupAction('delete', [name]).catch(() => {})
    await A.GroupAction('create', ['Harbour team'])
    for (const name of ['tidewater', 'harbour-api', 'tide-tables']) await A.GroupAction('add', ['Harbour team', `${demo}${name}`])
    for (const name of ['tide-tables', 'harbour-api', 'tidewater']) {
      const tab = await A.OpenRepo(`${demo}${name}`)
      await A.CloseRepo(tab.path)
    }
    // The sidebar remembers its panel, filter and sort for each repository.
    for (const key of Object.keys(localStorage)) if (key.startsWith('gitrevenge.sidebar:')) localStorage.removeItem(key)
  }, DEMO)
  for (const name of open) {
    await page.evaluate((p) => window.go.app.App.OpenRepo(p), path.win32.join(DEMO, name))
  }
  await page.evaluate((p) => window.go.app.App.SetActiveRepo(p), open.length ? path.win32.join(DEMO, open[0]) : '')
  await page.reload({ waitUntil: 'networkidle0' })
  await overlay(page)
  if (open.length) {
    await page.waitForSelector('.scroller')
    await untilSettled(page, () => !!document.querySelector('.row.ref') && !document.body.innerText.includes('Loading history'))
  }
  await sleep(800)
}

// ── What a viewer sees in an animation: a cursor, clicks and keys ─────────────────────

/** Adds a pointer that follows the mouse, a ring on each click, and a caption for keys. */
async function overlay(page) {
  await page.evaluate(() => {
    if (document.getElementById('demo-cursor')) return
    const css = document.createElement('style')
    css.textContent = `
      #demo-cursor { position: fixed; z-index: 99999; pointer-events: none; width: 22px; height: 22px;
        left: -40px; top: -40px; transition: transform 80ms; }
      #demo-cursor.down { transform: scale(0.85); }
      .demo-ring { position: fixed; z-index: 99998; pointer-events: none; width: 36px; height: 36px;
        margin: -18px 0 0 -18px; border-radius: 50%; border: 3px solid #cb0162; opacity: 0.9;
        animation: demo-ring 450ms ease-out forwards; }
      @keyframes demo-ring { to { transform: scale(1.8); opacity: 0; } }
      #demo-keys { position: fixed; z-index: 99999; left: 50%; bottom: 48px; transform: translateX(-50%);
        display: none; gap: 6px; padding: 10px 14px; border-radius: 10px; background: rgba(26, 22, 20, 0.88);
        box-shadow: 0 8px 30px rgba(0, 0, 0, 0.3); font: 600 18px 'JetBrains Mono', ui-monospace, monospace; }
      #demo-keys kbd { color: #f4ecdd; background: #3a322e; border: 1px solid #5a4f48; border-bottom-width: 3px;
        border-radius: 6px; padding: 3px 10px; font: inherit; }`
    document.head.append(css)
    const cursor = document.createElement('div')
    cursor.id = 'demo-cursor'
    cursor.innerHTML =
      '<svg viewBox="0 0 22 22" width="22" height="22"><path d="M3 2 L3 18 L7.5 13.8 L10.6 20.6 L13.4 19.4 L10.3 12.7 L16.5 12.4 Z" fill="#1a1614" stroke="#fff" stroke-width="1.5" stroke-linejoin="round"/></svg>'
    const keys = document.createElement('div')
    keys.id = 'demo-keys'
    document.body.append(cursor, keys)
    addEventListener('mousemove', (e) => {
      cursor.style.left = `${e.clientX - 3}px`
      cursor.style.top = `${e.clientY - 2}px`
    }, true)
    addEventListener('mousedown', (e) => {
      cursor.classList.add('down')
      const ring = document.createElement('div')
      ring.className = 'demo-ring'
      ring.style.left = `${e.clientX}px`
      ring.style.top = `${e.clientY}px`
      document.body.append(ring)
      setTimeout(() => ring.remove(), 500)
    }, true)
    addEventListener('mouseup', () => cursor.classList.remove('down'), true)
  })
}

/** Hides the cursor, for stills. */
const hideCursor = (page) => page.evaluate(() => document.getElementById('demo-cursor')?.style.setProperty('left', '-40px'))

/** Shows keys being pressed, such as "Ctrl+K", while pressing them. */
async function keys(page, combo, { hold = 900 } = {}) {
  const parts = combo.split('+')
  await page.evaluate((parts) => {
    const k = document.getElementById('demo-keys')
    k.innerHTML = parts.map((p) => `<kbd>${p}</kbd>`).join('')
    k.style.display = 'flex'
  }, parts)
  const names = parts.map((p) => ({ Ctrl: 'Control', Esc: 'Escape' })[p] ?? p)
  for (const n of names.slice(0, -1)) await page.keyboard.down(n)
  await page.keyboard.press(names.at(-1))
  for (const n of names.slice(0, -1).reverse()) await page.keyboard.up(n)
  await sleep(hold)
  await page.evaluate(() => (document.getElementById('demo-keys').style.display = 'none'))
}

let mouse = { x: WIDTH / 2, y: HEIGHT / 2 }

/** Moves the pointer to x, y in a smooth, human-looking path. */
async function glide(page, x, y, ms = 450) {
  const steps = Math.max(8, Math.round(ms / 16))
  const from = { ...mouse }
  for (let i = 1; i <= steps; i++) {
    const t = i / steps
    const e = t < 0.5 ? 2 * t * t : 1 - (-2 * t + 2) ** 2 / 2
    await page.mouse.move(from.x + (x - from.x) * e, from.y + (y - from.y) * e)
    await sleep(ms / steps)
  }
  mouse = { x, y }
}

/** The middle of the first element matching selector. */
async function centre(page, selector) {
  const el = await page.waitForSelector(selector, { visible: true })
  const box = await el.boundingBox()
  return { x: box.x + box.width / 2, y: box.y + box.height / 2 }
}

/** Glides to an element and clicks it. */
async function click(page, selector, { button = 'left', pause = 350, offset } = {}) {
  const c = await centre(page, selector)
  const x = c.x + (offset?.x ?? 0)
  const y = c.y + (offset?.y ?? 0)
  await glide(page, x, y)
  await page.mouse.click(x, y, { button })
  await sleep(pause)
}

/** Glides to a point and clicks it. */
async function clickAt(page, x, y, { button = 'left', pause = 350, count = 1 } = {}) {
  await glide(page, x, y)
  await page.mouse.click(x, y, { button, count })
  await sleep(pause)
}

/** Types like a person: one key at a time. */
async function type(page, text, delay = 45) {
  await page.keyboard.type(text, { delay })
}

/** A button, link or menu item whose text contains text. */
const byText = (text, tag = '*') =>
  `::-p-xpath(//${tag}[(self::button or self::a or @role='menuitem' or self::summary) and contains(normalize-space(.), ${JSON.stringify(text)})])`

// ── The commit graph ──────────────────────────────────────────────────────────────────

/** Where row i of the graph is on screen (its middle), counting from the top row shown. */
async function row(page, i) {
  return page.evaluate((i) => {
    const s = document.querySelector('.scroller')
    const r = s.getBoundingClientRect()
    const h = document.documentElement.dataset.density === 'compact' ? 30 : 52
    return { x: r.left + Math.min(r.width / 2, 360), y: r.top + (i + 0.5) * h, left: r.left }
  }, i)
}

/** Which row of the graph holds the commit with this subject, counting from the top. */
async function rowOf(page, subject) {
  const i = await page.evaluate(async (subject) => {
    const A = window.go.app.App
    const tab = (await A.RestoreSession()).tabs[0]
    const { rows } = await A.GraphRows(tab.path, 0, 40)
    return rows.findIndex((r) => r.subject === subject)
  }, subject)
  if (i < 0) throw new Error(`no commit "${subject}" in the first 40 rows`)
  return i
}

/** A branch's row in the sidebar, by the name it shows there: the part after its folder. */
const sidebarRef = (name) =>
  `::-p-xpath(//div[contains(@class,'row') and contains(@class,'ref')][.//span[contains(@class,'name') and normalize-space(.)='${name}']])`

/** Selects the commit whose subject is given, by searching for it. */
async function selectCommit(page, subject) {
  const box = await page.waitForSelector('::-p-aria(Search commits)')
  await box.click({ count: 3 })
  await page.keyboard.type(subject)
  await page.keyboard.press('Enter')
  await sleep(700)
  await page.keyboard.press('Escape')
  await sleep(300)
}

// ── Output ────────────────────────────────────────────────────────────────────────────

async function setTheme(page, theme) {
  await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: theme }])
  await sleep(250)
}

/** Saves the page (or clip, a box in CSS pixels) as docs/img/<name>-<theme>.webp. */
async function still(page, name, theme, clip) {
  // Out of the way, so nothing is left looking hovered.
  await page.mouse.move(WIDTH - 2, HEIGHT - 2)
  await hideCursor(page)
  await sleep(150)
  // Headless Chrome paints on demand, so a screenshot can reuse a frame from mid-transition
  // (a row still fading from its hover). Two animation frames make it paint the page as it is.
  await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))))
  const png = path.join(work, `${name}-${theme}.png`)
  await page.screenshot({ path: png, clip })
  execFileSync('magick', [png, '-quality', '90', '-define', 'webp:method=6', path.join(out, `${name}-${theme}.webp`)])
  console.log(`  ${name}-${theme}.webp`)
}

/** Takes a still in both themes; prepare puts the app in the state to show. */
async function stills(page, name, prepare, clip) {
  for (const theme of ['light', 'dark']) {
    await setTheme(page, theme)
    await prepare(page)
    // A rebuild can catch git mid-read, and an error banner has no place in the docs.
    const alert = await page.$('[role=alert]')
    if (alert) throw new Error(`error on screen: ${await alert.evaluate((el) => el.innerText)}`)
    await still(page, name, theme, typeof clip === 'function' ? await clip(page) : clip)
  }
}

let cuts = null
let recording = 0

/** Runs a wait. In an animation, most of its length is cut. */
async function waiting(wait) {
  const start = Date.now()
  await wait()
  const end = Date.now()
  if (cuts && end - start > 1200) cuts.push([(start - recording + 400) / 1000, (end - recording - 400) / 1000])
}

/** Records act as docs/img/<name>-<theme>.webp, an animation, in both themes. */
async function animation(page, name, prepare, act) {
  for (const theme of ['light', 'dark']) {
    await setTheme(page, theme)
    await prepare(page)
    mouse = { x: WIDTH / 2, y: HEIGHT / 2 }
    await page.mouse.move(mouse.x, mouse.y)
    const webm = path.join(work, `${name}-${theme}.webm`)
    // A screencast only gets a frame when something repaints, and loses the last one, which
    // would be the end of what's being shown. So something always repaints: one faint pixel.
    await page.evaluate(() => {
      const dot = document.createElement('div')
      dot.id = 'shots-heartbeat'
      dot.style.cssText = 'position:fixed;left:0;bottom:0;width:1px;height:1px;z-index:99999;pointer-events:none;opacity:.01;background:#000'
      document.body.append(dot)
      window.__heartbeat = setInterval(() => (dot.style.opacity = dot.style.opacity === '0.01' ? '0.02' : '0.01'), 66)
    })
    const rec = await page.screencast({ path: webm, scale: 1 })
    cuts = []
    recording = Date.now()
    await sleep(600)
    await act(page)
    await sleep(1200)
    await rec.stop()
    await page.evaluate(() => {
      clearInterval(window.__heartbeat)
      document.getElementById('shots-heartbeat')?.remove()
    })
    // Waits on git are cut, so the length of the animation doesn't depend on the machine.
    const keep = cuts.map(([a, b]) => `between(t,${a},${b})`).join('+')
    const trim = keep ? `select='not(${keep})',setpts=N/15/TB,` : ''
    cuts = null
    execFileSync('ffmpeg', ['-y', '-loglevel', 'error', '-i', webm, '-vf', `fps=15,${trim}scale=${WIDTH}:-1:flags=lanczos`,
      '-loop', '0', '-c:v', 'libwebp_anim', '-quality', '75', '-compression_level', '6',
      path.join(out, `${name}-${theme}.webp`)])
    console.log(`  ${name}-${theme}.webp (animated)`)
  }
}

/** Waits for git to reach a state, by asking it, then for the app to catch up. */
async function untilGit(test) {
  await waiting(async () => {
    for (const start = Date.now(); !test(); await sleep(100)) {
      if (Date.now() - start > 90000) throw new Error(`git never reached: ${test}`)
    }
  })
}

const headOf = () => execFileSync('git', ['-C', REPO, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim()
const operationRunning = () =>
  ['rebase-merge', 'rebase-apply', 'MERGE_HEAD'].some((name) => existsSync(path.join(REPO, '.git', name)))

/** Waits until the page has passed test for a second without a break. */
async function untilSettled(p, test = () => true) {
  const quiet = () =>
    !document.querySelector('.op') &&
    !document.body.innerText.includes('Loading…') &&
    ![...document.querySelectorAll('button')].some((b) => b.innerText.trim() === 'Cancel' && !b.closest('[role=dialog]') && b.closest('.toolbar'))
  await waiting(async () => {
    const start = Date.now()
    for (let since = Date.now(); Date.now() - since < 1000; await sleep(100)) {
      if (Date.now() - start > 90000) throw new Error(`the app never settled: ${test}`)
      if (!(await p.evaluate(quiet)) || !(await p.evaluate(test))) since = Date.now()
    }
  })
}

// ── Scenes ────────────────────────────────────────────────────────────────────────────

const scenes = {}

scenes.start = (page) =>
  stills(page, 'start-page', async (p) => {
    await reset(p, [])
  })

scenes.repo = async (page) => {
  await stills(page, 'repository', async (p) => {
    await reset(p)
    await p.waitForSelector(byText('src/alerts.js', 'button'))
    await sleep(500)
  })
  // The tour's copy, with numbered markers on each part of the window. Positions are in the
  // still's pixels (2560 x 1600).
  const marks = [
    [400, 45], // tabs
    [250, 1100], // sidebar
    [1650, 124], // toolbar
    [1180, 205], // working tree
    [1510, 780], // graph
    [2490, 130], // right panel
    [2050, 1000], // commit message
    [2030, 40], // command palette
  ]
  for (const theme of ['light', 'dark']) annotate('repository', theme, marks)
}

/** Writes <name>-annotated-<theme>.webp: the still with numbered rose markers at marks. */
function annotate(name, theme, marks) {
  const draw = marks.flatMap(([x, y], i) => [
    '-fill', '#cb0162', '-stroke', '#ffffff', '-strokewidth', '4', '-draw', `circle ${x},${y} ${x + 27},${y}`,
    '-fill', '#ffffff', '-stroke', 'none', '-pointsize', '32', '-annotate', `+${x - (i >= 9 ? 18 : 9)}+${y + 11}`, String(i + 1),
  ])
  run('magick', [path.join(out, `${name}-${theme}.webp`), '-font', 'Arial-Bold', ...draw, '-quality', '90',
    path.join(out, `${name}-annotated-${theme}.webp`)])
  console.log(`  ${name}-annotated-${theme}.webp`)
}

scenes.details = (page) =>
  stills(page, 'commit-details', async (p) => {
    await reset(p, ['tidewater'], { clean: true })
    await selectCommit(p, 'Predict tide height')
  })

scenes.palette = (page) =>
  stills(page, 'command-palette', async (p) => {
    await reset(p, ['tidewater'], { clean: true })
    await p.keyboard.down('Control')
    await p.keyboard.press('k')
    await p.keyboard.up('Control')
    await sleep(300)
    await p.keyboard.type('pull')
    await sleep(400)
  })

scenes.diff = (page) =>
  stills(page, 'diff-unified', async (p) => {
    await reset(p)
    await click(p, byText('src/alerts.js', 'button'))
    await sleep(800)
  })

/** Opens file in the commit with this subject, in the File layout, rendered. */
async function preview(p, subject, file) {
  await reset(p, ['tidewater'], { clean: true })
  await selectCommit(p, subject)
  await click(p, byText(file, 'button'))
  await sleep(700)
  await click(p, byText('File', 'button'))
  const rendered = await p.$(byText('Rendered', 'button'))
  if (rendered) await rendered.click()
  await sleep(1500)
}

scenes.csv = (page) => stills(page, 'preview-csv', (p) => preview(p, 'Add the station list as CSV', 'data/stations.csv'))
scenes.mermaid = (page) => stills(page, 'preview-mermaid', (p) => preview(p, 'Diagram how an alert travels', 'docs/alerts.mmd'))
scenes.markdown = (page) => stills(page, 'preview-markdown', (p) => preview(p, 'Document the units', 'README.md'))
scenes.json = (page) => stills(page, 'preview-json', (p) => preview(p, 'Add tidal constants', 'data/constants.json'))

scenes.blame = (page) =>
  stills(page, 'blame', async (p) => {
    await reset(p, ['tidewater'], { clean: true })
    await selectCommit(p, 'Show wind in knots')
    await click(p, byText('src/feed.js', 'button'))
    await sleep(600)
    await click(p, byText('Blame', 'button'))
    await sleep(1200)
  })

scenes.dashboard = (page) =>
  stills(page, 'group-dashboard', async (p) => {
    await reset(p, [])
    await click(p, byText('Dashboard'))
    await sleep(1500)
  })

scenes.settings = (page) =>
  stills(page, 'settings', async (p) => {
    await reset(p, ['tidewater'], { clean: true })
    await p.keyboard.down('Control')
    await p.keyboard.press(',')
    await p.keyboard.up('Control')
    await sleep(800)
  })

scenes.reflog = (page) =>
  stills(page, 'reflog', async (p) => {
    await reset(p)
    await p.keyboard.down('Control')
    await p.keyboard.press('k')
    await p.keyboard.up('Control')
    await sleep(300)
    await p.keyboard.type('reflog')
    await p.keyboard.press('Enter')
    await sleep(1000)
  })

/** Right-clicks row i of the graph and chooses the menu item containing text. */
async function rowMenu(p, i, text) {
  const r = await row(p, i)
  await clickAt(p, r.x, r.y, { button: 'right', pause: 500 })
  await click(p, byText(text), { pause: 700 })
}

/** Opens the interactive rebase editor for feature/offline-cache, from where it left main. */
async function openRebase(p) {
  await rowMenu(p, await rowOf(p, 'Set the alert limit in knots too'), 'Interactive rebase')
  await p.waitForSelector(byText('Start rebase', 'button'))
  await sleep(500)
}

scenes.rebase = (page) =>
  stills(page, 'rebase-editor', async (p) => {
    await reset(p, ['tidewater'], { clean: true, branch: 'feature/offline-cache' })
    await openRebase(p)
  })

/** Sets the action for the commit with this subject in the rebase editor. */
async function action(p, subject, value) {
  const sel = `select[aria-label="Action for ${subject}"]`
  await click(p, sel, { pause: 200 })
  await p.keyboard.press('Escape')
  await p.select(sel, value)
  await sleep(500)
}

scenes.rebaseAnim = (page) =>
  animation(
    page,
    'rebase',
    (p) => reset(p, ['tidewater'], { clean: true, branch: 'feature/offline-cache' }),
    async (p) => {
      await openRebase(p)
      await action(p, 'debugging', 'drop')
      await action(p, 'fixup! Cache the last reading from each station', 'fixup')
      await action(p, 'wip', 'squash')
      await sleep(600)
      await click(p, byText('Start rebase', 'button'), { pause: 300 })
      await untilSettled(p, () => !document.body.innerText.includes('Interactive rebase') && document.querySelector('.row.ref.head')?.innerText.includes('offline-cache'))
      await glide(p, 700, 400, 400)
      await p.mouse.wheel({ deltaY: -3000 })
      await sleep(1500)
    },
  )

/** The button that runs a merge or rebase from its preview. */
const PREVIEW_GO = '[role=dialog] button[type=submit]'

/** Merges feature/metric-units into main, which stops on a conflict in src/feed.js. */
async function merge(p) {
  await click(p, sidebarRef('metric-units'), { button: 'right', pause: 500 })
  await click(p, byText('Merge feature/metric-units into main'), { pause: 300 })
  // The preview names the conflict; its button goes ahead.
  await p.waitForSelector(`${PREVIEW_GO}:not([disabled])`)
  await click(p, PREVIEW_GO, { pause: 300 })
  // The merge stops on the conflict: wait for its banner, and put away git's own message.
  await p.waitForSelector('.op')
  await sleep(1000)
  if (await p.$(byText('Dismiss', 'button'))) await click(p, byText('Dismiss', 'button'), { pause: 600 })
}

scenes.conflict = (page) =>
  stills(page, 'conflict-tool', async (p) => {
    await reset(p, ['tidewater'], { clean: true })
    await merge(p)
    await click(p, 'button.wip', { pause: 800 })
    await click(p, '[title="Open the conflict tool"]', { pause: 1500 })
  })

scenes.conflictAnim = (page) =>
  animation(
    page,
    'conflict',
    (p) => reset(p, ['tidewater'], { clean: true }),
    async (p) => {
      await merge(p)
      await sleep(800)
      await click(p, 'button.wip', { pause: 900 })
      await click(p, '[title="Open the conflict tool"]', { pause: 1600 })
      await click(p, byText('Use theirs', 'button'), { pause: 300 })
      await untilGit(() => !execFileSync('git', ['-C', REPO, 'diff', '--name-only', '--diff-filter=U'], { encoding: 'utf8' }).trim())
      await sleep(1500)
      await click(p, byText('Commit the merge', 'button'), { pause: 300 })
      await untilGit(() => !operationRunning())
      await untilSettled(p, () => /clean/i.test(document.querySelector('button.wip')?.innerText ?? ''))
    },
  )

scenes.commitAnim = (page) =>
  animation(
    page,
    'stage-and-commit',
    (p) => reset(p),
    async (p) => {
      await click(p, byText('src/alerts.js', 'button'), { pause: 1200 })
      await click(p, byText('Stage hunk', 'button'), { pause: 300 })
      // Esc while it's still staging shows an error (#391).
      await untilGit(() => execFileSync('git', ['-C', REPO, 'diff', '--cached', '--name-only'], { encoding: 'utf8' }).includes('src/alerts.js'))
      await untilSettled(p)
      await keys(p, 'Esc', { hold: 700 })
      await click(p, 'textarea[aria-label="Commit summary"], input[aria-label="Commit summary"]', { pause: 300 })
      await type(p, 'Soften the wind alert and add a knots helper')
      await sleep(500)
      await keys(p, 'Ctrl+Enter', { hold: 800 })
      await untilSettled(p, () => document.querySelector('[aria-label="Commit summary"]')?.value === '')
    },
  )

scenes.undoAnim = (page) =>
  animation(
    page,
    'undo',
    (p) => reset(p, ['tidewater'], { clean: true }),
    async (p) => {
      const before = headOf()
      await click(p, sidebarRef('offline-cache'), { button: 'right', pause: 500 })
      await click(p, byText('Merge feature/offline-cache into main'), { pause: 300 })
      await p.waitForSelector(`${PREVIEW_GO}:not([disabled])`)
      await click(p, PREVIEW_GO, { pause: 300 })
      await untilGit(() => headOf() !== before)
      await untilSettled(p, () => [...document.querySelectorAll('button')].find((b) => b.innerText.trim().startsWith('Undo'))?.disabled === false)
      const merged = headOf()
      await keys(p, 'Ctrl+Z', { hold: 1000 })
      await untilGit(() => headOf() !== merged)
      await untilSettled(p, () => ![...document.querySelectorAll('.row.ref')].some((r) => r.innerText.includes('main') && r.innerText.includes('↑')))
    },
  )

scenes.paletteAnim = (page) =>
  animation(
    page,
    'palette',
    (p) => reset(p, ['tidewater'], { clean: true }),
    async (p) => {
      await keys(p, 'Ctrl+K', { hold: 500 })
      await type(p, 'pull reb', 90)
      await sleep(900)
      await keys(p, 'Esc', { hold: 400 })
      await keys(p, 'Ctrl+K', { hold: 400 })
      await type(p, 'fetch', 90)
      await sleep(500)
      await keys(p, 'Enter', { hold: 800 })
      await untilSettled(p)
    },
  )

scenes.searchAnim = (page) =>
  animation(
    page,
    'search',
    (p) => reset(p, ['tidewater'], { clean: true }),
    async (p) => {
      await keys(p, 'Ctrl+F', { hold: 400 })
      await type(p, 'Closes: #14', 80)
      await sleep(400)
      await keys(p, 'Enter', { hold: 1500 })
      await keys(p, 'Ctrl+F', { hold: 300 })
      await p.keyboard.down('Control')
      await p.keyboard.press('a')
      await p.keyboard.up('Control')
      await type(p, 'Priya', 80)
      await keys(p, 'Enter', { hold: 900 })
      await keys(p, 'Enter', { hold: 900 })
      await keys(p, 'Enter', { hold: 1200 })
    },
  )

// ── The sidebar and what's new around it ──────────────────────────────────────────────

/** The sidebar alone: the rail and its panel. */
const SIDEBAR = { x: 0, y: 40, width: 280, height: 520 }
const rail = (name) => `nav.rail button[aria-label^="${name}"]`
const FILTER = 'input[aria-label="Filter everything in the sidebar"]'

scenes.sidebarBranches = (page) =>
  stills(page, 'sidebar-branches', (p) => reset(p, ['tidewater'], { clean: true }), SIDEBAR)

scenes.sidebarCare = (page) =>
  stills(
    page,
    'sidebar-needs-care',
    async (p) => {
      await reset(p, ['tidewater'], { clean: true })
      await click(p, byText('Needs care', 'button'), { pause: 500 })
    },
    SIDEBAR,
  )

scenes.sidebarFilter = (page) =>
  stills(
    page,
    'sidebar-filter',
    async (p) => {
      await reset(p, ['tidewater'], { clean: true })
      await click(p, FILTER)
      await p.keyboard.type('v0')
      await sleep(600)
    },
    SIDEBAR,
  )

scenes.sidebarRepository = (page) =>
  stills(
    page,
    'sidebar-repository',
    async (p) => {
      await reset(p, ['tidewater'], { clean: true })
      await click(p, rail('Repository'), { pause: 300 })
      await p.waitForSelector('.repo-panel h3 .count', { timeout: 30000 })
      await sleep(500)
    },
    SIDEBAR,
  )

scenes.sidebarAnim = (page) =>
  animation(
    page,
    'sidebar-panels',
    (p) => reset(p, ['tidewater'], { clean: true }),
    async (p) => {
      for (const name of ['Remotes', 'Tags', 'Stashes', 'Repository']) await click(p, rail(name), { pause: 1000 })
      await click(p, rail('Branches'), { pause: 900 })
      await click(p, FILTER, { pause: 300 })
      await type(p, 'v0', 140)
      await sleep(1200)
      await click(p, rail('Tags'), { pause: 1200 })
      await click(p, rail('Branches'), { pause: 900 })
      await click(p, 'button[aria-label="Clear the filter"]', { pause: 700 })
      await click(p, byText('Needs care', 'button'), { pause: 1400 })
    },
  )

/** A commit's menu, with the submenu of one of the branches on it open. It's opened from the
 * keyboard, since parking the pointer for the still would close a hovered submenu. */
scenes.commitMenu = (page) =>
  stills(page, 'commit-menu', async (p) => {
    await reset(p, ['tidewater'], { clean: true })
    const r = await row(p, await rowOf(p, 'Document the units'))
    await clickAt(p, r.x, r.y, { button: 'right', pause: 500 })
    await p.keyboard.press('End')
    await p.keyboard.press('ArrowUp')
    await p.keyboard.press('ArrowRight')
    await sleep(500)
  })

scenes.upstream = (page) =>
  stills(page, 'set-upstream', async (p) => {
    await reset(p, ['tidewater'], { clean: true })
    await click(p, sidebarRef('metric-units'), { button: 'right', pause: 500 })
    await click(p, byText('Set upstream of feature/metric-units'), { pause: 700 })
  })

/** Drags from one point to another, slowly enough to see the drop marker. */
async function drag(p, from, to) {
  await glide(p, from.x, from.y, 500)
  await sleep(300)
  await p.mouse.down()
  await glide(p, to.x, to.y, 900)
  await sleep(350)
  await p.mouse.up()
  await sleep(1000)
}

const middle = (b) => ({ x: b.x + b.width / 2, y: b.y + b.height / 2 })
const card = (name) => `::-p-xpath(//*[@data-card][.//h3[normalize-space(.)="${name}"]])`

scenes.reorderAnim = (page) =>
  animation(
    page,
    'reorder',
    async (p) => {
      await reset(p, [])
      await p.evaluate(async (demo) => {
        const A = window.go.app.App
        await A.GroupAction('create', ['Field notes'])
        await A.GroupAction('add', ['Field notes', `${demo}tide-tables`])
      }, DEMO)
      await p.reload({ waitUntil: 'networkidle0' })
      await overlay(p)
      await sleep(800)
    },
    async (p) => {
      // A repository within its group: the first, down past the last.
      const harbour = await p.waitForSelector(card('Harbour team'))
      const repos = await harbour.$$('[data-repo]')
      await repos[0].hover()
      await sleep(250)
      const grip = await (await repos[0].$('.repo-grip')).boundingBox()
      const last = await repos.at(-1).boundingBox()
      mouse = middle(await repos[0].boundingBox())
      await drag(p, middle(grip), { x: middle(grip).x, y: last.y + last.height - 2 })
      // Then the whole group, past the other one.
      await harbour.hover()
      await sleep(250)
      const cardGrip = await (await harbour.$('.card-grip')).boundingBox()
      const other = await (await p.waitForSelector(card('Field notes'))).boundingBox()
      await drag(p, middle(cardGrip), { x: other.x + other.width - 4, y: other.y + 10 })
      await p.waitForFunction(() => document.querySelector('[data-card]')?.innerText.includes('Field notes'), { timeout: 30000 })
      await sleep(800)
    },
  )

// ── The Git Config tab ────────────────────────────────────────────────────────────────

/** Opens the Git Config tab on tidewater's config (or global, with global set) in one view,
 * after a reset. It refuses to run unless the app's global config is the demo's made-up one,
 * so nobody's own config can end up in a screenshot. */
async function gitConfigTab(p, view, { global = false } = {}) {
  await reset(p, ['tidewater'], { clean: true })
  const demo = path.win32.join(DEMO, 'gitconfig').toLowerCase()
  const actual = await p.evaluate(() => window.go.app.App.GitConfigScopes('').then((s) => s.global))
  if (path.win32.resolve(actual).toLowerCase() !== demo) {
    throw new Error(`the app reads global config from ${actual}; start wails dev with GIT_CONFIG_GLOBAL=${demo}`)
  }
  // A tab left from an earlier scene holds that scene's edits: close it without saving.
  await p.evaluate(async () => {
    const close = document.querySelector('[aria-label="Close Git Config"]')
    if (!close) return
    close.click()
    await new Promise((r) => setTimeout(r, 300))
    ;[...document.querySelectorAll('[role=dialog] button')].find((b) => b.textContent.includes('Discard'))?.click()
  })
  await sleep(400)
  await click(p, byText('tidewater', 'button'), { button: 'right', pause: 400 })
  await click(p, byText('Edit git config…'), { pause: 900 })
  if (global) await click(p, `::-p-xpath(//nav[contains(@class,"rail")]//button[contains(., "Global")])`, { pause: 700 })
  await click(p, `::-p-xpath(//nav[contains(@class,"rail")]//button[normalize-space(text())="${view}"])`, { pause: 900 })
  // Park the pointer in one jump: a glide would cross rows and leave one mid-fade from its
  // hover in the still.
  await p.mouse.move(WIDTH - 2, HEIGHT - 2)
  mouse = { x: WIDTH - 2, y: HEIGHT - 2 }
  await sleep(400)
}

scenes.gitconfig = (page) =>
  stills(page, 'gitconfig-settings', async (p) => {
    await gitConfigTab(p, 'Settings')
    // Two edits, so changed rows show their marks.
    await click(p, `::-p-xpath(//div[@data-key="core.autocrlf"]//button[normalize-space()="true"])`)
    await click(p, `::-p-xpath(//div[@data-key="core.filemode"]//input)`)
    await sleep(600)
  })

scenes.gitconfigAliases = (page) =>
  stills(page, 'gitconfig-aliases', async (p) => {
    await gitConfigTab(p, 'Aliases', { global: true })
  })

scenes.gitconfigRaw = (page) =>
  stills(page, 'gitconfig-raw', async (p) => {
    await gitConfigTab(p, 'Raw text')
    // Break the first header, so the error shows on its line.
    const line = await p.waitForSelector(`::-p-xpath(//div[contains(@class,"cm-line")][starts-with(normalize-space(),"[core")])`)
    await line.click()
    await p.keyboard.press('End')
    if ((await line.evaluate((el) => el.textContent.trim())) === '[core]') await p.keyboard.press('Backspace')
    await sleep(1200)
  })

scenes.gitconfigHistory = (page) =>
  stills(page, 'gitconfig-history', async (p) => {
    await gitConfigTab(p, 'Settings')
    // Two saves, so there are backups to compare. The file is named as the app knows it,
    // which may differ from T:\ (git resolves a mapped drive), since backups go by path.
    await p.evaluate(async (repo) => {
      const A = window.go.app.App
      const file = (await A.GitConfigScopes(repo)).local
      await A.OpenGitConfig(file)
      await A.SetGitConfig(file, 'fetch.prune', 'true')
      await A.SaveGitConfig(file, true)
      await A.SetGitConfig(file, 'pull.rebase', 'merges')
      await A.SaveGitConfig(file, true)
    }, REPO)
    await click(p, `::-p-xpath(//nav[contains(@class,"rail")]//button[normalize-space(text())="History"])`, { pause: 1200 })
  })

// ── The Insights tab ──────────────────────────────────────────────────────────────────

/** Opens Insights on tidewater, or on the "Harbour team" group of all three demo
 * repositories, in one view. Results come from the demo's fixed history, measured from its
 * own branch tip, so the shots come out the same every run. */
async function insightsTab(p, view, { group = false } = {}) {
  await reset(p, ['tidewater'], { clean: true })
  await p.evaluate(async (demo) => {
    const A = window.go.app.App
    await A.GroupAction('create', ['Harbour team']).catch(() => {})
    for (const name of ['tidewater', 'harbour-api', 'tide-tables']) await A.GroupAction('add', ['Harbour team', `${demo}${name}`]).catch(() => {})
    // Default thresholds, whatever an earlier run left saved. Scopes go by the path the app
    // knows the repository by, which may not be T:\ (git resolves a mapped drive).
    const defaults = { stale: 7, longLived: 14, behind: 50, size: 500 }
    const tidewater = (await A.OpenRepo(`${demo}tidewater`))?.path
    if (tidewater) await A.SetInsightsThresholds(`repo:${tidewater}`, defaults)
    await A.SetInsightsThresholds('group:Harbour team', defaults)
  }, DEMO)
  // Only tidewater's tabs belong in these shots: close Insights and Git Config from earlier
  // scenes, discarding any config edits.
  await p.evaluate(async () => {
    document.querySelector('[aria-label="Close Insights"]')?.click()
    const config = document.querySelector('[aria-label="Close Git Config"]')
    if (!config) return
    config.click()
    await new Promise((r) => setTimeout(r, 300))
    ;[...document.querySelectorAll('[role=dialog] button')].find((b) => b.textContent.includes('Discard'))?.click()
  })
  await sleep(400)
  await click(p, byText('tidewater', 'button'), { button: 'right', pause: 400 })
  await click(p, `::-p-xpath(//*[@role="menuitem" and contains(., "Insights")])`, { pause: 2500 })
  if (group) {
    await p.evaluate(() => {
      const sel = document.querySelector('.insights .rail select')
      sel.value = 'group:Harbour team'
      sel.dispatchEvent(new Event('change', { bubbles: true }))
    })
    await sleep(4000)
  }
  if (view !== 'Overview') await click(p, `::-p-xpath(//nav[contains(@class,"rail")]//button[starts-with(normalize-space(.), "${view}")])`, { pause: 1500 })
  // Conflict checks finish in the background; give them time to land.
  await sleep(1500)
  await p.mouse.move(WIDTH - 2, HEIGHT - 2)
  mouse = { x: WIDTH - 2, y: HEIGHT - 2 }
  await sleep(400)
}

scenes.insights = (page) => stills(page, 'insights-overview', (p) => insightsTab(p, 'Overview', { group: true }))

scenes.insightsShipped = (page) => stills(page, 'insights-shipped', (p) => insightsTab(p, 'Shipped'))

scenes.insightsAttention = (page) => stills(page, 'insights-attention', (p) => insightsTab(p, 'Needs attention'))

scenes.insightsSummary = (page) =>
  stills(page, 'insights-summary', async (p) => {
    await insightsTab(p, 'Overview', { group: true })
    await click(p, byText('Copy summary', 'button'), { pause: 1200 })
    await p.mouse.move(WIDTH - 2, HEIGHT - 2)
    mouse = { x: WIDTH - 2, y: HEIGHT - 2 }
    await sleep(400)
  })

// ── Main ──────────────────────────────────────────────────────────────────────────────

mkdirSync(out, { recursive: true })
rmSync(work, { recursive: true, force: true })
mkdirSync(work, { recursive: true })

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: true,
  defaultViewport: { width: WIDTH, height: HEIGHT, deviceScaleFactor: 2 },
  args: ['--hide-scrollbars', '--font-render-hinting=none'],
})
try {
  const page = await browser.newPage()
  await page.goto(APP, { waitUntil: 'networkidle0' })
  // Scenes close tabs and delete recent repositories: never let them near a session in use.
  const global = await page.evaluate(() => window.go.app.App.GitConfigScopes('').then((s) => s.global))
  if (path.win32.resolve(global).toLowerCase() !== path.win32.join(DEMO, 'gitconfig').toLowerCase()) {
    throw new Error(`${APP} reads global config from ${global}, so it looks like a session in use. See the top of this file.`)
  }
  const wanted = process.argv.slice(2)
  for (const [name, scene] of Object.entries(scenes)) {
    if (wanted.length && !wanted.includes(name)) continue
    console.log(name)
    try {
      await scene(page)
    } catch (e) {
      await page.screenshot({ path: path.join(work, `failed-${name}.png`) })
      console.error(`  failed; the page is in ${path.join(work, `failed-${name}.png`)}`)
      throw e
    }
  }
} finally {
  await browser.close()
}
