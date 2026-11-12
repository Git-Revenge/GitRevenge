#!/usr/bin/env bash
# Builds the demo repository the docs' screenshots are taken from: "Tidewater", a small,
# made-up weather-station dashboard with a few months of history by four fictional people.
#
#   docs-site/scripts/demo-repo.sh <folder>
#
# It makes <folder>/tidewater (the repository) and <folder>/origin.git (its remote), and
# deletes both first if they exist. Dates and authors are fixed, so the graph looks the same
# every time. Nothing is signed, whatever your git config says.
#
# What it sets up, for the screenshots:
# - main with merged feature branches, annotated release tags, trailers and a git note;
# - feature/offline-cache: work in progress with fixup and "wip" commits, for interactive rebase;
# - feature/metric-units: changes the same lines as main, so merging it conflicts;
# - a remote with a branch that's ahead, a stash, and uncommitted changes;
# - for the sidebar: a release folder, a bot branch, a branch whose upstream is gone, and a
#   linked worktree.
set -euo pipefail

out=${1:?usage: demo-repo.sh <folder>}
mkdir -p "$out"
out=$(cd "$out" && pwd)
repo="$out/tidewater"
# A running fsmonitor daemon (core.fsmonitor in your own config) keeps a repository's folder
# open, so stop any before deleting.
for r in "$repo" "$out/harbour-api" "$out/tide-tables"; do
  [ -d "$r/.git" ] && git -C "$r" fsmonitor--daemon stop >/dev/null 2>&1 || true
done
rm -rf "$repo" "$out/origin.git" "$out/tidewater-hotfix"

export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
day=0
base=$(date -d 2026-07-01T09:00:00 +%s 2>/dev/null || date -j -f %Y-%m-%dT%H:%M:%S 2026-07-01T09:00:00 +%s)

who() {
  case $1 in
    maya) export GIT_AUTHOR_NAME="Maya Okafor" GIT_AUTHOR_EMAIL="maya@tidewater.example" ;;
    leo) export GIT_AUTHOR_NAME="Leo Brandt" GIT_AUTHOR_EMAIL="leo@tidewater.example" ;;
    priya) export GIT_AUTHOR_NAME="Priya Raman" GIT_AUTHOR_EMAIL="priya@tidewater.example" ;;
    sam) export GIT_AUTHOR_NAME="Sam Ortiz" GIT_AUTHOR_EMAIL="sam@tidewater.example" ;;
  esac
  export GIT_COMMITTER_NAME="$GIT_AUTHOR_NAME" GIT_COMMITTER_EMAIL="$GIT_AUTHOR_EMAIL"
}

# commit <who> <message> [hours later]: commits everything staged or changed.
commit() {
  who "$1"
  day=$((day + ${3:-20}))
  local at
  at=$(date -u -d "@$((base + day * 3600))" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -r $((base + day * 3600)) +%Y-%m-%dT%H:%M:%SZ)
  export GIT_AUTHOR_DATE="$at" GIT_COMMITTER_DATE="$at"
  git add -A
  git commit -q -m "$2"
}

merge() {
  who "$1"
  day=$((day + 3))
  local at
  at=$(date -u -d "@$((base + day * 3600))" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -r $((base + day * 3600)) +%Y-%m-%dT%H:%M:%SZ)
  export GIT_AUTHOR_DATE="$at" GIT_COMMITTER_DATE="$at"
  git merge -q --no-ff "$2" -m "Merge branch '$2'"
}

tag() {
  git tag -a "$1" -m "$2"
}

git init -q -b main "$repo"
cd "$repo"
git config commit.gpgsign false
git config tag.gpgsign false
git config core.fsmonitor false
# Commits made in the app during the screenshots are Maya's, never whoever runs this.
git config user.name "Maya Okafor"
git config user.email maya@tidewater.example

# ── The first release ──────────────────────────────────────────────────────────────────
cat > README.md <<'EOF'
# Tidewater

A small dashboard for a network of coastal weather stations: live readings, tide
predictions, and alerts when the wind picks up.

## Running it

```sh
npm install
npm start
```

Then open http://localhost:8080.
EOF
mkdir -p src data docs
cat > src/server.js <<'EOF'
import { createServer } from 'node:http'
import { readings } from './readings.js'

const port = process.env.PORT ?? 8080

createServer(async (req, res) => {
  if (req.url === '/api/readings') {
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify(await readings()))
    return
  }
  res.statusCode = 404
  res.end('not found')
}).listen(port)
EOF
cat > src/readings.js <<'EOF'
import { stations } from './stations.js'

export async function readings() {
  return stations.map((s) => ({ id: s.id, name: s.name, wind: 0, temp: 0 }))
}
EOF
cat > src/stations.js <<'EOF'
export const stations = [
  { id: 'HBR', name: 'Harbour Light' },
  { id: 'GUL', name: 'Gull Point' },
]
EOF
printf '{\n  "name": "tidewater",\n  "version": "0.1.0",\n  "type": "module",\n  "scripts": { "start": "node src/server.js" }\n}\n' > package.json
printf 'node_modules/\n.env\n' > .gitignore
commit maya "Start Tidewater: a server for station readings" 0

cat > data/stations.csv <<'EOF'
id,name,lat,lon,elevation_m
HBR,Harbour Light,50.3712,-4.1423,12
GUL,Gull Point,50.3381,-4.2210,31
SAL,Saltmarsh,50.4020,-4.0875,4
CRG,Craig Head,50.3195,-4.1902,58
EOF
commit priya "Add the station list as CSV"

cat > src/stations.js <<'EOF'
import { readFileSync } from 'node:fs'

const rows = readFileSync(new URL('../data/stations.csv', import.meta.url), 'utf8').trim().split('\n').slice(1)

export const stations = rows.map((row) => {
  const [id, name, lat, lon, elevation] = row.split(',')
  return { id, name, lat: Number(lat), lon: Number(lon), elevation: Number(elevation) }
})
EOF
commit leo "Read stations from data/stations.csv"

cat > src/readings.js <<'EOF'
import { stations } from './stations.js'
import { fetchStation } from './feed.js'

export async function readings() {
  return Promise.all(stations.map(async (s) => ({ ...s, ...(await fetchStation(s.id)) })))
}
EOF
cat > src/feed.js <<'EOF'
const FEED = process.env.FEED_URL ?? 'https://feed.tidewater.example'

// Wind in km/h, temperature in °C, pressure in hPa.
export async function fetchStation(id) {
  const res = await fetch(`${FEED}/stations/${id}/latest`)
  if (!res.ok) throw new Error(`feed: ${id}: ${res.status}`)
  const { wind_kmh, temp_c, pressure_hpa } = await res.json()
  return { wind: wind_kmh, temp: temp_c, pressure: pressure_hpa }
}
EOF
commit leo "Fetch live readings from the station feed"

cat > config.yaml <<'EOF'
server:
  port: 8080
feed:
  url: https://feed.tidewater.example
  timeout_s: 5
alerts:
  wind_kmh: 60
EOF
commit maya "Add config.yaml"
tag v0.1.0 "Tidewater 0.1.0: live readings for four stations"

# ── Station map ────────────────────────────────────────────────────────────────────────
git checkout -q -b feature/station-map
mkdir -p public
cat > public/index.html <<'EOF'
<!doctype html>
<title>Tidewater</title>
<link rel="stylesheet" href="style.css">
<main id="map"></main>
<script type="module" src="map.js"></script>
EOF
commit sam "Add a page for the station map"
cat > public/map.js <<'EOF'
const res = await fetch('/api/readings')
const stations = await res.json()
for (const s of stations) {
  const pin = document.createElement('div')
  pin.className = 'pin'
  pin.title = `${s.name}: ${s.wind} km/h`
  pin.style.left = `${(s.lon + 4.3) * 400}px`
  pin.style.top = `${(50.45 - s.lat) * 600}px`
  document.querySelector('#map').append(pin)
}
EOF
commit sam "Plot each station as a pin"
printf 'body { margin: 0; font: 15px system-ui; }\n#map { position: relative; height: 100vh; background: #dfe9ee; }\n.pin { position: absolute; width: 12px; height: 12px; border-radius: 50%%; background: #cb0162; }\n' > public/style.css
commit sam "Style the map and pins"
cat > data/stations.geojson <<'EOF'
{
  "type": "FeatureCollection",
  "features": [
    { "type": "Feature", "properties": { "id": "HBR", "name": "Harbour Light", "elevation_m": 12 }, "geometry": { "type": "Point", "coordinates": [-4.1423, 50.3712] } },
    { "type": "Feature", "properties": { "id": "GUL", "name": "Gull Point", "elevation_m": 31 }, "geometry": { "type": "Point", "coordinates": [-4.2210, 50.3381] } },
    { "type": "Feature", "properties": { "id": "SAL", "name": "Saltmarsh", "elevation_m": 4 }, "geometry": { "type": "Point", "coordinates": [-4.0875, 50.4020] } },
    { "type": "Feature", "properties": { "id": "CRG", "name": "Craig Head", "elevation_m": 58 }, "geometry": { "type": "Point", "coordinates": [-4.1902, 50.3195] } },
    { "type": "Feature", "properties": { "name": "Breakwater", "built": 1841 }, "geometry": { "type": "LineString", "coordinates": [[-4.1780, 50.3360], [-4.1650, 50.3330], [-4.1520, 50.3335]] } },
    { "type": "Feature", "properties": { "name": "Sheltered anchorage", "depth_m": 9 }, "geometry": { "type": "Polygon", "coordinates": [[[-4.1700, 50.3480], [-4.1350, 50.3470], [-4.1300, 50.3620], [-4.1600, 50.3660], [-4.1700, 50.3480]]] } }
  ]
}
EOF
commit sam "Export the stations as GeoJSON" 1
cat > public/about.html <<'EOF'
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>About Tidewater</title>
<style>
  body { margin: 0; font: 16px/1.55 Georgia, serif; color: #1d2a33; background: #f6f1e7; }
  header { padding: 40px 48px 24px; background: #12395a; color: #f6f1e7; }
  header h1 { margin: 0; font-size: 34px; letter-spacing: 0.02em; }
  header p { margin: 6px 0 0; opacity: 0.8; }
  main { max-width: 720px; padding: 24px 48px 48px; }
  table { border-collapse: collapse; width: 100%; margin: 12px 0 20px; font: 14px system-ui, sans-serif; }
  th, td { text-align: left; padding: 6px 10px; border-bottom: 1px solid #d8cfbf; }
  th { color: #12395a; }
  .note { padding: 12px 16px; border-left: 4px solid #cb0162; background: #fff; font: 14px system-ui, sans-serif; }
</style>
</head>
<body>
<header>
  <h1>Tidewater</h1>
  <p>Live readings from the stations around the harbour.</p>
</header>
<main>
  <h2>The stations</h2>
  <p>Each station reports wind, temperature and pressure every ten minutes. The map pins are coloured by wind speed.</p>
  <table>
    <tr><th>Station</th><th>Since</th><th>Height</th></tr>
    <tr><td>Harbour Light</td><td>1998</td><td>12 m</td></tr>
    <tr><td>Gull Point</td><td>2004</td><td>31 m</td></tr>
    <tr><td>Saltmarsh</td><td>2011</td><td>4 m</td></tr>
    <tr><td>Craig Head</td><td>2016</td><td>58 m</td></tr>
  </table>
  <p class="note">Readings are provisional until the harbour authority checks them, usually within a day.</p>
  <p><a href="index.html">Back to the map</a></p>
</main>
</body>
</html>
EOF
commit sam "Add an about page" 1
git checkout -q main
cat >> src/server.js <<'EOF'

// Static files for the dashboard live in public/.
EOF
commit leo "Note where the dashboard's files live"
merge maya feature/station-map
git branch -q -d feature/station-map

# ── Alerts ─────────────────────────────────────────────────────────────────────────────
git checkout -q -b feature/wind-alerts
cat > src/alerts.js <<'EOF'
import { readFileSync } from 'node:fs'

const limit = Number(/wind_kmh:\s*(\d+)/.exec(readFileSync('config.yaml', 'utf8'))[1])

export function alerts(readings) {
  return readings.filter((r) => r.wind >= limit).map((r) => ({ id: r.id, message: `Gale warning at ${r.name}` }))
}
EOF
commit priya "Raise an alert when wind passes the configured limit"
cat > docs/alerts.mmd <<'EOF'
flowchart LR
  feed[Station feed] --> readings[readings.js]
  readings --> alerts[alerts.js]
  alerts -->|wind >= limit| page[Dashboard banner]
  alerts -->|wind >= limit| mail[Email to harbour master]
EOF
commit priya "Diagram how an alert travels"
git checkout -q main
cat > CHANGELOG.md <<'EOF'
# Changelog

## 0.2.0

- A map of every station.
- Wind alerts.
EOF
commit maya "Start a changelog"
merge maya feature/wind-alerts
git branch -q -d feature/wind-alerts

cat > src/tides.js <<'EOF'
// Tide height from the harmonic constants for each station's nearest port.
export function tideHeight(constants, at) {
  const hours = at.getTime() / 3_600_000
  return constants.reduce((h, c) => h + c.amplitude * Math.cos(((c.speed * hours - c.phase) * Math.PI) / 180), 0)
}
EOF
commit leo "Predict tide height from harmonic constants

The four largest constituents (M2, S2, N2, K1) get within 10 cm at Harbour Light,
which is good enough for the dashboard.

Co-authored-by: Priya Raman <priya@tidewater.example>
Closes: #14"
git notes add -m "Checked against the harbour authority's printed tables for August: worst error 9 cm."

cat > data/constants.json <<'EOF'
{
  "HBR": [
    { "name": "M2", "amplitude": 1.72, "speed": 28.984, "phase": 151.3 },
    { "name": "S2", "amplitude": 0.58, "speed": 30.0, "phase": 196.0 },
    { "name": "N2", "amplitude": 0.33, "speed": 28.44, "phase": 132.9 },
    { "name": "K1", "amplitude": 0.07, "speed": 15.041, "phase": 108.6 }
  ]
}
EOF
commit leo "Add tidal constants for Harbour Light"
mkdir -p notebooks
cat > notebooks/wind.ipynb <<'EOF'
{
 "nbformat": 4,
 "nbformat_minor": 5,
 "metadata": { "kernelspec": { "name": "python3", "display_name": "Python 3" }, "language_info": { "name": "python" } },
 "cells": [
  {
   "cell_type": "markdown",
   "id": "intro",
   "metadata": {},
   "source": ["# A week of wind\n", "\n", "Which station sets off the most alerts? Readings for **12 to 18 August**, from the feed's archive. The alert limit is $v > 60$ km/h."]
  },
  {
   "cell_type": "code",
   "id": "load",
   "execution_count": 1,
   "metadata": {},
   "source": ["import pandas as pd\n", "\n", "week = pd.read_csv('../data/archive/2026-08-12.csv', parse_dates=['at'])\n", "print(f'{len(week):,} readings from {week.station.nunique()} stations')"],
   "outputs": [{ "output_type": "stream", "name": "stdout", "text": ["4,032 readings from 4 stations\n"] }]
  },
  {
   "cell_type": "code",
   "id": "summary",
   "execution_count": 2,
   "metadata": {},
   "source": ["week.groupby('station').wind.agg(['mean', 'max', lambda w: (w > 60).sum()])"],
   "outputs": [
    {
     "output_type": "execute_result",
     "execution_count": 2,
     "metadata": {},
     "data": {
      "text/plain": ["         mean   max  alerts\n", "CRG      38.2  71.0      14\n", "GUL      31.7  64.5       5\n", "HBR      22.4  48.1       0\n", "SAL      18.9  41.3       0"],
      "text/html": ["<table class=\"dataframe\"><thead><tr><th></th><th>mean</th><th>max</th><th>alerts</th></tr><tr><th>station</th><th></th><th></th><th></th></tr></thead><tbody><tr><th>CRG</th><td>38.2</td><td>71.0</td><td>14</td></tr><tr><th>GUL</th><td>31.7</td><td>64.5</td><td>5</td></tr><tr><th>HBR</th><td>22.4</td><td>48.1</td><td>0</td></tr><tr><th>SAL</th><td>18.9</td><td>41.3</td><td>0</td></tr></tbody></table>"]
     }
    }
   ]
  },
  {
   "cell_type": "code",
   "id": "plot",
   "execution_count": 3,
   "metadata": {},
   "source": ["ax = week.pivot_table('wind', 'at', 'station').resample('6h').max().plot(figsize=(6, 2.4))\n", "ax.axhline(60, ls='--')"],
   "outputs": [
    {
     "output_type": "display_data",
     "metadata": {},
     "data": {
      "text/plain": ["<Figure size 600x240 with 1 Axes>"],
      "image/svg+xml": ["<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"600\" height=\"240\" viewBox=\"0 0 600 240\"><rect width=\"600\" height=\"240\" fill=\"#ffffff\"/><g stroke=\"#d0d7de\"><line x1=\"40\" y1=\"20\" x2=\"40\" y2=\"210\"/><line x1=\"40\" y1=\"210\" x2=\"590\" y2=\"210\"/></g><line x1=\"40\" y1=\"80\" x2=\"590\" y2=\"80\" stroke=\"#cb0162\" stroke-dasharray=\"6 4\"/><polyline fill=\"none\" stroke=\"#1f77b4\" stroke-width=\"2\" points=\"40,120 90,100 140,70 190,60 240,95 290,110 340,75 390,64 440,90 490,105 540,86 590,98\"/><polyline fill=\"none\" stroke=\"#ff7f0e\" stroke-width=\"2\" points=\"40,140 90,128 140,96 190,78 240,120 290,132 340,100 390,92 440,118 490,126 540,110 590,121\"/><polyline fill=\"none\" stroke=\"#2ca02c\" stroke-width=\"2\" points=\"40,170 90,160 140,150 190,140 240,158 290,165 340,148 390,142 440,156 490,162 540,151 590,160\"/><polyline fill=\"none\" stroke=\"#9467bd\" stroke-width=\"2\" points=\"40,185 90,180 140,170 190,162 240,178 290,183 340,170 390,166 440,176 490,181 540,172 590,178\"/><g font-family=\"sans-serif\" font-size=\"11\" fill=\"#57606a\"><text x=\"8\" y=\"84\">60</text><text x=\"40\" y=\"228\">12 Aug</text><text x=\"540\" y=\"228\">18 Aug</text></g></svg>"]
     }
    }
   ]
  },
  {
   "cell_type": "markdown",
   "id": "findings",
   "metadata": {},
   "source": ["Craig Head is the windiest by far. It's also the highest, at 58 m, so the limit there may want to be higher."]
  }
 ]
}
EOF
commit leo "Look at a week of wind in a notebook" 2
sed -i.bak 's/"version": "0.1.0"/"version": "0.2.0"/' package.json && rm package.json.bak
commit maya "Release 0.2.0"
tag v0.2.0 "Tidewater 0.2.0: the station map and wind alerts"

# ── Fixes and polish ───────────────────────────────────────────────────────────────────
sed -i.bak 's/timeout_s: 5/timeout_s: 10/' config.yaml && rm config.yaml.bak
commit sam "Give the feed more time on a slow connection"
printf 'BRK,Breakwater,50.3590,-4.1611,6\n' >> data/stations.csv
commit priya "Add the Breakwater station"
cat > src/feed.js <<'EOF'
const FEED = process.env.FEED_URL ?? 'https://feed.tidewater.example'

// Wind in km/h, temperature in °C, pressure in hPa.
export async function fetchStation(id) {
  const res = await fetch(`${FEED}/stations/${id}/latest`, { signal: AbortSignal.timeout(10_000) })
  if (!res.ok) throw new Error(`feed: ${id}: ${res.status}`)
  const { wind_kmh, temp_c, pressure_hpa } = await res.json()
  return { wind: wind_kmh, temp: temp_c, pressure: pressure_hpa }
}
EOF
commit leo "Time out a station that stops answering"

# Branches cut from here change feed.js too: metric-units's edit conflicts with the next one
# on main.
git branch feature/metric-units
cat > src/feed.js <<'EOF'
const FEED = process.env.FEED_URL ?? 'https://feed.tidewater.example'

// Wind in knots, temperature in °C, pressure in hPa. Sailors asked for knots.
export async function fetchStation(id) {
  const res = await fetch(`${FEED}/stations/${id}/latest`, { signal: AbortSignal.timeout(10_000) })
  if (!res.ok) throw new Error(`feed: ${id}: ${res.status}`)
  const { wind_kmh, temp_c, pressure_hpa } = await res.json()
  return { wind: Math.round(wind_kmh / 1.852), temp: temp_c, pressure: pressure_hpa }
}
EOF
commit maya "Show wind in knots"
sed -i.bak 's/wind_kmh: 60/wind_knots: 32/' config.yaml && rm config.yaml.bak
sed -i.bak 's/wind_kmh/wind_knots/' src/alerts.js && rm src/alerts.js.bak
commit maya "Set the alert limit in knots too"

git checkout -q feature/metric-units
cat > src/feed.js <<'EOF'
const FEED = process.env.FEED_URL ?? 'https://feed.tidewater.example'

// Wind in metres per second, temperature in °C, pressure in hPa: SI throughout.
export async function fetchStation(id) {
  const res = await fetch(`${FEED}/stations/${id}/latest`, { signal: AbortSignal.timeout(10_000) })
  if (!res.ok) throw new Error(`feed: ${id}: ${res.status}`)
  const { wind_kmh, temp_c, pressure_hpa } = await res.json()
  return { wind: +(wind_kmh / 3.6).toFixed(1), temp: temp_c, pressure: pressure_hpa }
}
EOF
commit sam "Report wind in metres per second"
cat >> README.md <<'EOF'

## Units

Every reading is in SI units: wind in m/s, temperature in °C, pressure in hPa.
EOF
commit sam "Document the units"
git checkout -q main

# ── Work in progress: offline cache, with a messy history to tidy ──────────────────────
git checkout -q -b feature/offline-cache
cat > src/cache.js <<'EOF'
const cache = new Map()

export function remember(id, reading) {
  cache.set(id, { reading, at: Date.now() })
}
EOF
commit priya "Cache the last reading from each station"
cat >> src/cache.js <<'EOF'

export function recall(id, maxAgeMs = 15 * 60_000) {
  const hit = cache.get(id)
  return hit && Date.now() - hit.at < maxAgeMs ? hit.reading : undefined
}
EOF
commit priya "wip" 2
# The fixup changes a line of the commit it names, not of "wip", so folding it in applies cleanly.
sed -i.bak 's/^  cache.set(id/  if (reading) cache.set(id/' src/cache.js && rm src/cache.js.bak
commit priya "fixup! Cache the last reading from each station" 1
cat > src/readings.js <<'EOF'
import { stations } from './stations.js'
import { fetchStation } from './feed.js'
import { recall, remember } from './cache.js'

export async function readings() {
  return Promise.all(
    stations.map(async (s) => {
      try {
        const r = await fetchStation(s.id)
        remember(s.id, r)
        return { ...s, ...r }
      } catch {
        return { ...s, ...recall(s.id), stale: true }
      }
    }),
  )
}
EOF
commit priya "Fall back to the cached reading when a station is down"
printf 'console.log(cache)\n' >> src/cache.js
commit priya "debugging" 1
git checkout -q main

# ── A remote, a stash, and uncommitted work ────────────────────────────────────────────
git clone -q --bare "$repo" "$out/origin.git"
git remote add origin "$out/origin.git"
git fetch -q origin
git branch -q -u origin/main main
# Someone else pushed a fix to origin/main.
work="$out/.other"
rm -rf "$work"
git clone -q "$out/origin.git" "$work"
(
  cd "$work"
  git config commit.gpgsign false
  printf 'node_modules/\n.env\n*.log\n' > .gitignore
  commit sam "Ignore log files" 30
  git push -q origin main
)
rm -rf "$work"
git fetch -q origin

# Branches for the sidebar. They're dated before the work above, so the top of the graph
# doesn't change.
git branch release/v0.1 v0.1.0
git branch release/v0.2 v0.2.0
git push -q origin release/v0.2
# Merged and deleted on the remote, as a host does after a pull request: its upstream is gone.
git branch fix/tide-rounding main~3
git push -q -u origin fix/tide-rounding
git push -q origin --delete fix/tide-rounding
# A dependency update from a bot, made without touching the working tree.
(
  export GIT_INDEX_FILE="$out/.bot-index"
  export GIT_AUTHOR_NAME='dependabot[bot]' GIT_AUTHOR_EMAIL='bot@tidewater.example' GIT_AUTHOR_DATE=2026-07-11T08:00:00Z
  export GIT_COMMITTER_NAME="$GIT_AUTHOR_NAME" GIT_COMMITTER_EMAIL="$GIT_AUTHOR_EMAIL" GIT_COMMITTER_DATE="$GIT_AUTHOR_DATE"
  from=$(git rev-parse 'v0.1.0^{commit}')
  git read-tree "$from"
  lock=$(printf '{ "name": "tidewater", "lockfileVersion": 3 }\n' | git hash-object -w --stdin)
  git update-index --add --cacheinfo "100644,$lock,package-lock.json"
  bump=$(git commit-tree "$(git write-tree)" -p "$from" -m "Bump vite from 7.0 to 7.1")
  git branch dependabot/npm_and_yarn/vite-7.1 "$bump"
  rm -f "$GIT_INDEX_FILE"
)
# A second working folder, for the Repository panel.
git worktree add -q -b hotfix/gauge-offset "$out/tidewater-hotfix" main~1

# A stash, then changes in the working tree.
printf '\n## Contributing\n\nOpen an issue first for anything bigger than a typo.\n' >> README.md
git stash push -q -m "Contributing section, not ready yet"
sed -i.bak 's/Gale warning at/Strong wind at/' src/alerts.js && rm src/alerts.js.bak
cat > src/units.js <<'EOF'
export const knots = (kmh) => Math.round(kmh / 1.852)
EOF
git add src/units.js

# A few local settings, for the Git Config tab.
git config pull.rebase true
git config core.autocrlf input
git config alias.ship "push --follow-tags"

# ── Two sibling projects, for a group on the start page ────────────────────────────────
for name in harbour-api tide-tables; do
  rm -rf "$out/$name" "$out/$name.git"
  git init -q -b main "$out/$name"
  (
    cd "$out/$name"
    git config commit.gpgsign false
    git config core.fsmonitor false
    printf '# %s\n\nPart of Tidewater.\n' "$name" > README.md
    commit leo "Start $name" 5
    printf 'export const version = 1\n' > index.js
    commit priya "Add the entry point" 5
    git clone -q --bare . "$out/$name.git"
    git remote add origin "$out/$name.git"
    git fetch -q origin
    git branch -q -u origin/main main
  )
done
printf 'export const version = 2\n' > "$out/tide-tables/index.js"

# ── A made-up global config, for the Git Config tab ────────────────────────────────────
# Start the app with GIT_CONFIG_GLOBAL pointing here, so your own config never appears in a
# screenshot. The token is fake; it shows the masking.
win() { cygpath -m "$1" 2>/dev/null || echo "$1"; }
cat > "$out/work.gitconfig" <<EOF
[user]
	email = maya@harbour-works.example
EOF
# Its own global ignore file too, or the Ignore rules tab would show yours, path and all.
printf '.DS_Store\nThumbs.db\n*.swp\n' > "$out/gitignore"
cat > "$out/gitconfig" <<EOF
[user]
	name = Maya Okafor
	email = maya@tidewater.example
[init]
	defaultBranch = main
[core]
	editor = code --wait
	excludesFile = $(win "$out")/gitignore
[push]
	autoSetupRemote = true
[fetch]
	prune = true
[alias]
	co = checkout
	st = status -sb
	lg = log --graph --oneline --decorate
	undo = reset --soft HEAD~1
	wip = "!git add -A && git commit -m \"wip: \${1:-save point}\""
	log = log --oneline -20
[url "https://maya:not-a-real-token@git.tidewater.example/"]
	insteadOf = https://git.tidewater.example/
[includeIf "gitdir:$(win "$out")/harbour-api/"]
	path = $(win "$out")/work.gitconfig
EOF

echo "Built $repo"
