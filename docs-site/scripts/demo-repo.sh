#!/usr/bin/env bash
# Builds Tidewater, the demo repository the docs' screenshots and walkthroughs use: a made-up
# dashboard for a network of coastal weather stations, with almost two years of history by a
# small team.
#
#   docs-site/scripts/demo-repo.sh [--git3] <folder>
#
# It makes <folder>/tidewater (the repository), <folder>/origin.git (its remote), a linked
# worktree and two sibling projects, and deletes them first if they exist. Dates and authors
# are fixed, so the graph looks the same every time. Nothing is signed, whatever your git
# config says. --git3 builds the same thing as <folder>/tidewater-git3 and friends, the way Git
# 3.0 makes a new repository: SHA-256 object IDs and reftable refs (needs git 2.45 or later).
#
# What it sets up:
# - January 2025 to September 2026, made in a second or two with git fast-import: six people
#   and a dependency bot opening pull requests, merging (and sometimes squashing) them, cutting
#   releases (0.1.0 to 1.7.0, with release branches and patch releases), reverting the odd
#   merge, and leaving branches behind: stale ones, merged ones nobody deleted, a stack of
#   three, open bot bumps. About 1,500 commits. The generator is at the end of this file.
# - The last few weeks, by hand, for the walkthroughs and screenshots: stations from a CSV
#   file, the station map and wind alerts (v1.8.0, v1.9.0), git notes and trailers, files for
#   every rendered preview, feature/offline-cache (work in progress with fixup and "wip"
#   commits, for interactive rebase) and feature/metric-units (conflicts with main);
# - a remote with a branch that's ahead and branches only it has, a stash, uncommitted
#   changes, a release folder, a branch whose upstream is gone and a linked worktree;
# - two sibling projects, for a group on the start page, and a made-up global config.
set -euo pipefail

script=$(cd "$(dirname "$0")" && pwd)/$(basename "$0")

git3=
if [ "${1:-}" = --git3 ]; then
  git3=1
  shift
fi
out=${1:?usage: demo-repo.sh [--git3] <folder>}
mkdir -p "$out"
out=$(cd "$out" && pwd)
# The Git 3.0 build sits beside the other, every name ending in -git3.
sfx=${git3:+-git3}
repo="$out/tidewater$sfx"
origin="$out/origin$sfx.git"
hotfix="$out/tidewater$sfx-hotfix"
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
initflags=
cloneflags=
if [ -n "$git3" ]; then
  initflags="--object-format=sha256 --ref-format=reftable"
  cloneflags="--ref-format=reftable"
  probe=$(mktemp -d)
  if ! git init -q $initflags "$probe/r" 2>/dev/null; then
    echo "demo-repo.sh: --git3 needs git 2.45 or later, for SHA-256 and reftable repositories" >&2
    rm -rf "$probe"
    exit 1
  fi
  rm -rf "$probe"
  # Git 3.0 won't look for a bare repository it wasn't told about; nothing here relies on it.
  export GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=safe.bareRepository GIT_CONFIG_VALUE_0=explicit
fi
# A running fsmonitor daemon (core.fsmonitor in your own config) keeps a repository's folder
# open, so stop any before deleting.
for r in "$repo" "$out/harbour-api$sfx" "$out/tide-tables$sfx"; do
  [ -d "$r/.git" ] && git -C "$r" fsmonitor--daemon stop >/dev/null 2>&1 || true
done
rm -rf "$repo" "$origin" "$hotfix"

day=0
base=$(date -d 2026-10-05T09:00:00 +%s 2>/dev/null || date -j -f %Y-%m-%dT%H:%M:%S 2026-10-05T09:00:00 +%s)

who() {
  case $1 in
    emily) export GIT_AUTHOR_NAME="Emily Carter" GIT_AUTHOR_EMAIL="emily@tidewater.example" ;;
    marcus) export GIT_AUTHOR_NAME="Marcus Johnson" GIT_AUTHOR_EMAIL="marcus@tidewater.example" ;;
    sarah) export GIT_AUTHOR_NAME="Sarah Mitchell" GIT_AUTHOR_EMAIL="sarah@tidewater.example" ;;
    tyler) export GIT_AUTHOR_NAME="Tyler Brooks" GIT_AUTHOR_EMAIL="tyler@tidewater.example" ;;
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

# version <x.y.z>: sets package.json's version.
version() {
  sed -i.bak "s/\"version\": \"[^\"]*\"/\"version\": \"$1\"/" package.json && rm package.json.bak
}

# generate <days>: the generated history at the end of this file, from the commit HEAD is on.
generate() {
  LC_ALL=C awk -v start=1736121600 -v days="$1" -v root="$(git rev-parse HEAD)" "$(sed -n '/^# ── tidewater-history\.awk/,/^AWK$/{/^AWK$/!p;}' "$script")" |
    git fast-import --quiet
  git update-ref -d refs/gen/scratch
  git reset -q --hard main
}

git init -q -b main $initflags "$repo"
cd "$repo"
git config commit.gpgsign false
git config tag.gpgsign false
git config core.fsmonitor false
# Commits made in the app during the screenshots are Emily's, never whoever runs this.
git config user.name "Emily Carter"
git config user.email emily@tidewater.example

# ── The first commit ───────────────────────────────────────────────────────────────────
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
who emily
export GIT_AUTHOR_DATE=2025-01-06T14:00:00Z GIT_COMMITTER_DATE=2025-01-06T14:00:00Z
git add -A
git commit -q -m "Start Tidewater: a server for station readings"

# ── Twenty months of a team's work ─────────────────────────────────────────────────────
# From 6 January 2025 to the end of September 2026: see the generator at the end.
generate 626

# ── Stations from a CSV file (v1.8.0) ──────────────────────────────────────────────────

cat > data/stations.csv <<'EOF'
id,name,lat,lon,elevation_m
HBR,Harbour Light,50.3712,-4.1423,12
GUL,Gull Point,50.3381,-4.2210,31
SAL,Saltmarsh,50.4020,-4.0875,4
CRG,Craig Head,50.3195,-4.1902,58
EOF
commit sarah "Add the station list as CSV"

cat > src/stations.js <<'EOF'
import { readFileSync } from 'node:fs'

const rows = readFileSync(new URL('../data/stations.csv', import.meta.url), 'utf8').trim().split('\n').slice(1)

export const stations = rows.map((row) => {
  const [id, name, lat, lon, elevation] = row.split(',')
  return { id, name, lat: Number(lat), lon: Number(lon), elevation: Number(elevation) }
})
EOF
commit marcus "Read stations from data/stations.csv"

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
commit marcus "Move the feed client into its own module"

cat > config.yaml <<'EOF'
server:
  port: 8080
feed:
  url: https://feed.tidewater.example
  timeout_s: 5
alerts:
  wind_kmh: 60
EOF
commit emily "Move settings into config.yaml"
version 1.8.0
commit emily "Release 1.8.0" 2
tag v1.8.0 "Tidewater 1.8.0: stations from a CSV file"

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
commit tyler "Add a page for the station map"
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
commit tyler "Plot each station as a pin"
printf 'body { margin: 0; font: 15px system-ui; }\n#map { position: relative; height: 100vh; background: #dfe9ee; }\n.pin { position: absolute; width: 12px; height: 12px; border-radius: 50%%; background: #cb0162; }\n' > public/style.css
commit tyler "Style the map and pins"
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
commit tyler "Export the stations as GeoJSON" 1
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
commit tyler "Add an about page" 1
git checkout -q main
cat >> src/server.js <<'EOF'

// Static files for the dashboard live in public/.
EOF
commit marcus "Note where the dashboard's files live"
merge emily feature/station-map
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
commit sarah "Raise an alert when wind passes the configured limit"
cat > docs/alerts.mmd <<'EOF'
flowchart LR
  feed[Station feed] --> readings[readings.js]
  readings --> alerts[alerts.js]
  alerts -->|wind >= limit| page[Dashboard banner]
  alerts -->|wind >= limit| mail[Email to harbour master]
EOF
commit sarah "Diagram how an alert travels"
git checkout -q main
{
  printf '# Changelog\n\n## Unreleased\n\n- A map of every station.\n- Wind alerts.\n\n'
  tail -n +3 CHANGELOG.md
} > CHANGELOG.new && mv CHANGELOG.new CHANGELOG.md
commit emily "Note the map and alerts in the changelog"
merge emily feature/wind-alerts
git branch -q -d feature/wind-alerts

cat > src/tides.js <<'EOF'
// Tide height from the harmonic constants for each station's nearest port.
export function tideHeight(constants, at) {
  const hours = at.getTime() / 3_600_000
  return constants.reduce((h, c) => h + c.amplitude * Math.cos(((c.speed * hours - c.phase) * Math.PI) / 180), 0)
}
EOF
commit marcus "Predict tide height from harmonic constants

The four largest constituents (M2, S2, N2, K1) get within 10 cm at Harbour Light,
which is good enough for the dashboard.

Co-authored-by: Sarah Mitchell <sarah@tidewater.example>
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
commit marcus "Add tidal constants for Harbour Light"
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
commit marcus "Look at a week of wind in a notebook" 2
version 1.9.0
sed -i.bak 's/^## Unreleased/## 1.9.0/' CHANGELOG.md && rm CHANGELOG.md.bak
commit emily "Release 1.9.0"
tag v1.9.0 "Tidewater 1.9.0: the station map and wind alerts"

# ── Fixes and polish ───────────────────────────────────────────────────────────────────
sed -i.bak 's/timeout_s: 5/timeout_s: 10/' config.yaml && rm config.yaml.bak
commit tyler "Give the feed more time on a slow connection"
printf 'BRK,Breakwater,50.3590,-4.1611,6\n' >> data/stations.csv
commit sarah "Add the Breakwater station"
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
commit marcus "Time out a station that stops answering"

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
commit emily "Show wind in knots"
sed -i.bak 's/wind_kmh: 60/wind_knots: 32/' config.yaml && rm config.yaml.bak
sed -i.bak 's/wind_kmh/wind_knots/' src/alerts.js && rm src/alerts.js.bak
commit emily "Set the alert limit in knots too"

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
commit tyler "Report wind in metres per second"
cat >> README.md <<'EOF'

## Units

Every reading is in SI units: wind in m/s, temperature in °C, pressure in hPa.
EOF
commit tyler "Document the units"
git checkout -q main

# ── Work in progress: offline cache, with a messy history to tidy ──────────────────────
git checkout -q -b feature/offline-cache
cat > src/cache.js <<'EOF'
const cache = new Map()

export function remember(id, reading) {
  cache.set(id, { reading, at: Date.now() })
}
EOF
commit sarah "Cache the last reading from each station"
cat >> src/cache.js <<'EOF'

export function recall(id, maxAgeMs = 15 * 60_000) {
  const hit = cache.get(id)
  return hit && Date.now() - hit.at < maxAgeMs ? hit.reading : undefined
}
EOF
commit sarah "wip" 2
# The fixup changes a line of the commit it names, not of "wip", so folding it in applies cleanly.
sed -i.bak 's/^  cache.set(id/  if (reading) cache.set(id/' src/cache.js && rm src/cache.js.bak
commit sarah "fixup! Cache the last reading from each station" 1
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
commit sarah "Fall back to the cached reading when a station is down"
printf 'console.log(cache)\n' >> src/cache.js
commit sarah "debugging" 1
git checkout -q main

# ── A remote, a stash, and uncommitted work ────────────────────────────────────────────
git clone -q --bare $cloneflags "$repo" "$origin"
git remote add origin "$origin"
git fetch -q origin
git branch -q -u origin/main main
# Someone else pushed a fix to origin/main.
work="$out/.other$sfx"
rm -rf "$work"
git clone -q $cloneflags "$origin" "$work"
(
  cd "$work"
  git config commit.gpgsign false
  printf 'node_modules/\n.env\n*.log\n' > .gitignore
  commit tyler "Ignore log files" 30
  git push -q origin main
)
rm -rf "$work"
git fetch -q origin

# Branches for the sidebar. They're dated before the work above, so the top of the graph
# doesn't change.
git branch release/v1.8 v1.8.0
git branch release/v1.9 v1.9.0
git push -q origin release/v1.9
# Merged and deleted on the remote, as a host does after a pull request: its upstream is gone.
git branch fix/tide-rounding main~3
git push -q -u origin fix/tide-rounding
git push -q origin --delete fix/tide-rounding
# Other people's branches: the oldest fixes are only on the remote, the rest track theirs.
for b in $(git for-each-ref --format='%(refname:short)' 'refs/heads/fix/*' | head -n 3); do
  git branch -q -D "$b"
done
for b in $(git for-each-ref --format='%(refname:short)' refs/heads/feature refs/heads/fix refs/heads/docs refs/heads/chore refs/heads/refactor); do
  case $b in feature/offline-cache | feature/metric-units | fix/tide-rounding) continue ;; esac
  git branch -q -u "origin/$b" "$b"
done
# A second working folder, for the Repository panel.
git worktree add -q -b hotfix/gauge-offset "$hotfix" main~1

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
for name in harbour-api$sfx tide-tables$sfx; do
  rm -rf "$out/$name" "$out/$name.git"
  git init -q -b main $initflags "$out/$name"
  (
    cd "$out/$name"
    git config commit.gpgsign false
    git config core.fsmonitor false
    printf '# %s\n\nPart of Tidewater.\n' "${name%-git3}" > README.md
    commit marcus "Start ${name%-git3}" 5
    printf 'export const version = 1\n' > index.js
    commit sarah "Add the entry point" 5
    git clone -q --bare $cloneflags . "$out/$name.git"
    git remote add origin "$out/$name.git"
    git fetch -q origin
    git branch -q -u origin/main main
  )
done
printf 'export const version = 2\n' > "$out/tide-tables$sfx/index.js"

# ── A made-up global config, for the Git Config tab ────────────────────────────────────
# Start the app with GIT_CONFIG_GLOBAL pointing here, so your own config never appears in a
# screenshot. The token is fake; it shows the masking.
win() { cygpath -m "$1" 2>/dev/null || echo "$1"; }
cat > "$out/work$sfx.gitconfig" <<EOF
[user]
	email = emily@harbour-works.example
EOF
# Its own global ignore file too, or the Ignore rules tab would show yours, path and all.
printf '.DS_Store\nThumbs.db\n*.swp\n' > "$out/gitignore$sfx"
cat > "$out/gitconfig$sfx" <<EOF
[user]
	name = Emily Carter
	email = emily@tidewater.example
[init]
	defaultBranch = main
[core]
	editor = code --wait
	excludesFile = $(win "$out")/gitignore$sfx
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
[url "https://emily:not-a-real-token@git.tidewater.example/"]
	insteadOf = https://git.tidewater.example/
[includeIf "gitdir:$(win "$out")/harbour-api$sfx/"]
	path = $(win "$out")/work$sfx.gitconfig
EOF

echo "Built $repo ($(git -C "$repo" rev-list --count --all) commits)"
exit 0

# The generator: an awk program that generate() reads out of this file. To bash it's a here-doc
# that never runs.
: <<'AWK'
# ── tidewater-history.awk ─────────────────────────────────────────────────────────────────
# Writes about twenty months of Tidewater's history as a git fast-import stream: six people
# and a dependency bot working the way a small team does, with pull requests, squash merges,
# releases, release branches with patch releases, reverts, stale and abandoned branches and a
# stack. demo-repo.sh pipes it into `git fast-import`, then adds the last few weeks by hand.
#
#   awk -v start=<epoch of the first Monday, 00:00 UTC> -v days=<how many> -v root=<commit> "$program"
#
# Everything comes from one seeded generator, so the output is the same on every run and every
# awk (gawk, mawk, BSD awk). Run it with LC_ALL=C: `data` counts bytes. Nothing here names a
# hash, so the same stream builds a SHA-1 or a SHA-256 repository.

# ── Randomness and output ───────────────────────────────────────────────────────────────

# Park–Miller: the product stays below 2^53, so every awk computes it exactly.
function rnd() { seed = (seed * 16807) % 2147483647; return seed / 2147483647 }
function ri(a, b) { return a + int(rnd() * (b - a + 1)) }
function chance(p) { return rnd() < p }
function pick(list,   parts, n) { n = split(list, parts, "|"); return parts[ri(1, n)] }

function datablock(s) { return sprintf("data %d\n%s\n", length(s), s) }
function mop(path, content) { return sprintf("M 100644 inline %s\n", path) datablock(content) }
function dop(path) { return sprintf("D %s\n", path) }
function ident(k, t) { return sprintf("%s <%s> %d %s", pname[k], pmail[k], t, ptz[k]) }

# commit writes a commit on the scratch ref and returns its mark.
function commit(author, at, committer, ct, msg, from, merge, ops) {
  marks++
  printf "commit refs/gen/scratch\nmark :%d\nauthor %s\ncommitter %s\n", marks, ident(author, at), ident(committer, ct)
  printf "%s", datablock(msg)
  printf "from %s\n", from
  if (merge != "") printf "merge %s\n", merge
  printf "%s\n", ops
  commits++
  return ":" marks
}

# ── Dates ───────────────────────────────────────────────────────────────────────────────

# civil turns days since 1970-01-01 into YYYY-MM-DD (Howard Hinnant's algorithm).
function civil(z,   era, doe, yoe, y, doy, mp, d, m) {
  z += 719468
  era = int(z / 146097)
  doe = z - era * 146097
  yoe = int((doe - int(doe / 1460) + int(doe / 36524) - int(doe / 146096)) / 365)
  y = yoe + era * 400
  doy = doe - (365 * yoe + int(yoe / 4) - int(yoe / 100))
  mp = int((5 * doy + 2) / 153)
  d = doy - int((153 * mp + 2) / 5) + 1
  m = mp < 10 ? mp + 3 : mp - 9
  if (m <= 2) y++
  return sprintf("%04d-%02d-%02d", y, m, d)
}
function dateof(day) { return civil(int(start / 86400) + day) }
function monthname(day,   ymd, parts) {
  ymd = dateof(day)
  split(ymd, parts, "-")
  return mon[parts[2] + 0] " " parts[1]
}
function weekday(day) { return day % 7 }  # start is a Monday: 0..4 are working days

# ── Files ───────────────────────────────────────────────────────────────────────────────


function addpath(p) {
  if (!(p in known)) { known[p] = 1; paths[++npaths] = p }
}

function ident_name(base,   n) { names++; return base names }

function jsblock(   fn, v) {
  fn = ident_name(pick("parse|scale|format|merge|clamp|smooth|round|pick|group|average|limit|score"))
  v = pick("reading|gust|level|station|value|sample|window|series")
  return sprintf("\nexport function %s(%s) {\n  const %sValue = %s?.%s ?? %d\n  return Math.round(%sValue * %d) / %d\n}", fn, v, v, v, pick("wind|temp|pressure|height|speed|count|offset"), ri(0, 60), v, ri(2, 100), ri(2, 20))
}
function testblock(   n) {
  n = ri(1, 90)
  return sprintf("\ntest('%s %s', () => {\n  assert.equal(subject(%d), %d)\n})", pick("handles|rounds|rejects|keeps|skips|reports"), pick("an empty reading|a gust over the limit|a missing station|midnight|a negative offset|a late sample|duplicate rows"), n, n * ri(1, 4))
}
function cssblock() {
  return sprintf("\n.%s-%d {\n  margin: %dpx %dpx;\n  color: #%02x%02x%02x;\n}", pick("panel|legend|chip|row|badge|axis|tooltip|card"), ri(1, 99), ri(0, 24), ri(0, 24), ri(16, 200), ri(16, 200), ri(16, 200))
}
function sentence() {
  return pick("The feed|Each station|The dashboard|An alert|The tide model|The API|A reading|The archive") " " pick("now keeps|no longer drops|checks|reports|caches|rounds|retries|groups") " " pick("readings older than an hour|gusts above the limit|the last good value|times in UTC|one sample per minute|stations that stop answering|pressure in hPa|every partner station") "."
}
function mdblock() {
  return sprintf("\n## %s\n\n%s %s\n", pick("Setup|Limits|Errors|Examples|Configuration|Troubleshooting|Notes|Changes"), sentence(), sentence())
}
function csvrows(day, n,   s, i) {
  s = ""
  for (i = 0; i < n; i++)
    s = s sprintf("\n%sT%02d:%02d:00Z,%s,%.1f,%.1f,%d", dateof(day - ri(0, 27)), ri(0, 23), ri(0, 5) * 10, pick("HBR|GUL|SAL|CRG|BRK"), ri(0, 900) / 10, ri(-40, 280) / 10, ri(985, 1035))
  return s
}
function ymlblock() {
  return sprintf("\n      - name: %s\n        run: npm run %s", pick("Lint|Test|Build|Check types|Check links"), pick("lint|test|build|check|docs"))
}
function shblock() {
  return sprintf("\n# %s\n%s", pick("Keep a week of backups|Retry once on a timeout|Skip stations that are offline"), pick("sleep " ri(1, 30) "|retries=" ri(1, 5) "|keep_days=" ri(3, 30)))
}

# first returns the content a new file starts with.
function first(p, day) {
  if (p ~ /\.test\.js$/) return sprintf("import { test } from 'vitest'\nimport assert from 'node:assert/strict'\nimport { subject } from '../../%s'\n%s\n", modulefor[p], testblock())
  if (p ~ /\.js$/) return sprintf("// %s\n%s\n", pick("Part of the readings pipeline.|Used by the dashboard.|Shared by the API and the jobs.|Kept small on purpose."), jsblock())
  if (p ~ /\.css$/) return sprintf("/* %s */\n%s\n", p, cssblock())
  if (p ~ /\.md$/) return sprintf("# %s\n\n%s\n%s", title_of[p], sentence(), mdblock())
  if (p ~ /\.csv$/) return "at,station,wind_kmh,temp_c,pressure_hpa" csvrows(day, ri(20, 40)) "\n"
  if (p ~ /\.yml$/) return sprintf("name: %s\non: [push, pull_request]\njobs:\n  build:\n    runs-on: ubuntu-24.04\n    steps:\n      - uses: actions/checkout@v4\n      - uses: actions/setup-node@v4\n        with:\n          node-version: %d%s\n", pick("CI|Deploy|Nightly"), pick("20|22"), ymlblock())
  if (p == "Dockerfile") return "FROM node:22-slim\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci --omit=dev\nCOPY . .\nEXPOSE 8080\nCMD [\"node\", \"src/server.js\"]\n"
  return sprintf("#!/usr/bin/env bash\nset -euo pipefail\n%s\n", shblock())
}

# grow returns content with one more piece, or with a number changed once a file is long.
function grow(p, c, day,   L, n, i, x, out) {
  n = split(c, L, "\n")
  if (n > 140 || chance(0.25)) {
    for (i = 0; i < 8; i++) {
      x = ri(1, n)
      if (L[x] ~ /[0-9]/ && L[x] !~ /^(import|FROM|#!)/) {
        sub(/[0-9]+/, ri(2, 99), L[x])
        out = L[1]
        for (i = 2; i <= n; i++) out = out "\n" L[i]
        return out
      }
    }
  }
  sub(/\n$/, "", c)
  if (p ~ /\.test\.js$/) return c "\n" testblock() "\n"
  if (p ~ /\.js$/) return c "\n" jsblock() "\n"
  if (p ~ /\.css$/) return c "\n" cssblock() "\n"
  if (p ~ /\.md$/) return c "\n" mdblock()
  if (p ~ /\.csv$/) return c csvrows(day, ri(5, 15)) "\n"
  if (p ~ /\.yml$/) return c ymlblock() "\n"
  return c "\n" shblock() "\n"
}

# ── package.json and its lock ───────────────────────────────────────────────────────────

function pkgjson(version, bumped, to,   s, j, v) {
  s = sprintf("{\n  \"name\": \"tidewater\",\n  \"version\": \"%s\",\n  \"private\": true,\n  \"type\": \"module\",\n  \"scripts\": {\n    \"start\": \"node src/server.js\",\n    \"test\": \"vitest run\",\n    \"lint\": \"eslint .\"\n  },\n  \"dependencies\": {", version)
  for (j = 1; j <= ndeps; j++) {
    v = j == bumped ? to : dver[j]
    if (j == ndev + 1) s = s "\n  },\n  \"devDependencies\": {"
    s = s sprintf("\n    \"%s\": \"^%s\"%s", dname[j], v, (j == ndev || j == ndeps) ? "" : ",")
  }
  return s "\n  }\n}\n"
}
function lockjson(version, bumped, to,   s, j) {
  s = sprintf("{\n  \"name\": \"tidewater\",\n  \"version\": \"%s\",\n  \"lockfileVersion\": 3,\n  \"packages\": {", version)
  for (j = 1; j <= ndeps; j++)
    s = s sprintf("\n    \"node_modules/%s\": { \"version\": \"%s\" }%s", dname[j], j == bumped ? to : dver[j], j == ndeps ? "" : ",")
  return s "\n  }\n}\n"
}

# ── People ──────────────────────────────────────────────────────────────────────────────

function person(k, name, mail, tz, from, to, areas) {
  pname[k] = name; pmail[k] = mail; ptz[k] = tz; pfrom[k] = from; pto[k] = to; pareas[k] = areas
}
function active(k, day) { return day >= pfrom[k] && day <= pto[k] }
function other(k, day,   j, tries) {
  if (k != 1 && chance(0.45)) return 1
  for (tries = 0; tries < 20; tries++) {
    j = ri(1, people)
    if (j != k && active(j, day)) return j
  }
  return 1
}

# ── Pull requests ───────────────────────────────────────────────────────────────────────

function slug(s) {
  s = tolower(s)
  gsub(/[^a-z0-9]+/, "-", s)
  sub(/^-/, "", s); sub(/-$/, "", s)
  # Long titles are cut at a word, as hosts do.
  while (length(s) > 40 && index(s, "-")) sub(/-[^-]*$/, "", s)
  return s
}
function unique(b) {
  if (!(b in usedbranch)) { usedbranch[b] = 1; return b }
  usedbranch[b]++
  return b "-" usedbranch[b]
}

function lockable(p) { return !(p in lock) }

# addfile gives feature f path p to work on, creating it on the branch if it's new.
function addfile(f, p) {
  if (!lockable(p)) return 0
  lock[p] = f
  fn[f]++
  ffile[f, fn[f]] = p
  if (p in mc) fc[f, p] = mc[p]
  return 1
}

# existing finds an unlocked file under dir (or "" if there's none).
function existing(dir,   i, tries, p) {
  for (tries = 0; tries < 30; tries++) {
    p = paths[ri(1, npaths)]
    if (index(p, dir) == 1 && (p in mc) && lockable(p) && p !~ /CHANGELOG|package/) return p
  }
  return ""
}

function newmodule(area,   tries, m, p) {
  for (tries = 0; tries < 10; tries++) {
    m = pick(modules[area])
    p = dir[area] m ext[area]
    if (!(p in mc) && lockable(p)) {
      modname = m
      return p
    }
  }
  return ""
}

# openpr opens a pull request for person k, or returns 0 when nothing's free to work on.
function openpr(k, day, forced,   f, area, kind, p, q, t, noun, prefix) {
  f = ++features
  area = pick(pareas[k])
  kind = forced != "" ? "feature" : pick("feature|feature|feature|feature|fix|fix|fix|refactor|chore")
  if (area == "docs") kind = "docs"
  if (area == "infra" && kind == "feature") kind = "chore"
  fn[f] = 0
  noun = pick(nouns[area])
  if (kind == "feature" || kind == "docs" || area == "data") {
    if (area == "data") {
      p = "data/archive/" substr(dateof(day), 1, 7) ".csv"
      if ((p in mc) || !lockable(p)) p = ""
      t = "Archive readings for " monthname(day)
      kind = "chore"
    } else if (area == "docs") {
      p = "docs/" pick(docpages) ".md"
      if (!lockable(p)) p = ""
      t = (p in mc ? "Expand " : "Write ") pick("the deploy guide|the API reference|the station handbook|the alert guide|the runbook|the FAQ|the architecture notes|the contributor guide")
      title_of[p] = t
    } else {
      p = newmodule(area)
      if (p == "") { p = existing(dir[area]); kind = "fix" }
      else t = (forced != "" ? forced : pick("Add|Support|Show|Let people choose") " " noun)
    }
    if (p == "" || !addfile(f, p)) return 0
    if (area != "docs" && area != "data" && area != "infra" && area != "ui" && !(p in mc)) {
      q = "test/" area "/" modname ".test.js"
      modulefor[q] = p
      addfile(f, q)
    }
  }
  if (kind == "fix" || kind == "refactor" || kind == "chore") {
    if (fn[f] == 0) {
      p = existing(area == "infra" ? "" : dir[area])
      if (area == "infra") p = pick(".github/workflows/ci.yml|.github/workflows/deploy.yml|Dockerfile|scripts/backup.sh|scripts/seed.sh")
      if (p == "" || !addfile(f, p)) return 0
      if (kind == "refactor" && (q = existing(dir[area])) != "") addfile(f, q)
    }
    if (t == "") {
      if (kind == "fix") t = pick("Fix|Handle") " " pick(bugs[area])
      else if (kind == "refactor") t = pick("Split up|Simplify|Tidy") " " noun
      else t = pick("Update|Speed up|Tidy") " " pick(chores)
    }
  }
  # A title that's been used reads better with a place or a reason than with a number.
  prefix = (kind == "docs" ? "docs" : kind == "refactor" ? "refactor" : kind == "chore" ? "chore" : kind) "/"
  noun = t
  for (q = 0; q < 4 && forced == "" && ((prefix slug(t)) in usedbranch); q++)
    t = noun " " pick("on phones|for partner stations|at Gull Point|at Saltmarsh|at Craig Head|in the export|on the map|for the archive|in Safari|after an outage|for new stations|in the nightly job|with slow feeds|in the tide tables")
  ftitle[f] = t
  fkind[f] = kind
  fowner[f] = k
  fbranch[f] = forced != "" ? forced_branch : unique(prefix slug(t))
  fbase[f] = maintip
  ftip[f] = maintip
  fplanned[f] = forced != "" ? 3 : 1 + (chance(0.55) ? ri(0, 2) : ri(2, 7))
  fdone[f] = 0
  fstate[f] = "open"
  fsubjects[f] = ""
  fstart[f] = day
  pr++
  fpr[f] = pr
  current[k] = f
  return f
}

function work(f, day, t,   p, i, msg, ops, subject) {
  i = fdone[f] < fn[f] ? fdone[f] + 1 : ri(1, fn[f])
  p = ffile[f, i]
  if ((f, p) in fc) fc[f, p] = grow(p, fc[f, p], day)
  else fc[f, p] = first(p, day)
  ops = mop(p, fc[f, p])
  # A test usually changes with its module.
  if (fdone[f] > 0 && i == 1 && fn[f] > 1 && ffile[f, 2] ~ /\.test\.js$/ && chance(0.6)) {
    fc[f, ffile[f, 2]] = (f, ffile[f, 2]) in fc ? grow(ffile[f, 2], fc[f, ffile[f, 2]], day) : first(ffile[f, 2], day)
    ops = ops mop(ffile[f, 2], fc[f, ffile[f, 2]])
  }
  if (fdone[f] == 0) subject = ftitle[f]
  else subject = pick("Address review comments|Cover the edge cases|Rename for clarity|Fix lint|Handle an empty response|Add tests|Tidy up|Update the docs comment|Keep the old behaviour behind a flag|Simplify the loop|Check the input first|wip")
  if (chance(0.06)) subject = subject "\n\nCo-authored-by: " pname[other(fowner[f], day)] " <" pmail[other(fowner[f], day)] ">"
  fsubjects[f] = fsubjects[f] "\n* " (index(subject, "\n") ? substr(subject, 1, index(subject, "\n") - 1) : subject)
  ftip[f] = commit(fowner[f], t, fowner[f], t, subject "\n", ftip[f], "", ops)
  fdone[f]++
  flast[f] = day
  if (fdone[f] >= fplanned[f]) {
    fstate[f] = fstacked[f] ? "stacked" : "ready"
    fready[f] = day + ri(1, 3)
    if (weekday(fready[f]) >= 5) fready[f] += 7 - weekday(fready[f])
    current[fowner[f]] = 0
  }
}

function release_files(f,   i) { for (i = 1; i <= fn[f]; i++) delete lock[ffile[f, i]] }

function land(f, day, t,   k, i, p, ops, msg, squash, undo, revert, body) {
  k = other(fowner[f], day)
  ops = ""
  undo = ""
  for (i = 1; i <= fn[f]; i++) {
    p = ffile[f, i]
    if (!((f, p) in fc)) continue
    undo = undo ((p in mc) ? mop(p, mc[p]) : dop(p))
    mc[p] = fc[f, p]
    addpath(p)
    ops = ops mop(p, mc[p])
  }
  release_files(f)
  squash = chance(fdone[f] > 1 ? 0.25 : 0.5)
  body = ftitle[f]
  if (fkind[f] == "fix" && pr > 20 && chance(0.5)) body = body "\n\nCloses #" ri(1, pr - 1)
  if (squash) {
    msg = sprintf("%s (#%d)\n%s\n", ftitle[f], fpr[f], fdone[f] > 1 ? "\n" substr(fsubjects[f], 2) "\n" : "")
    maintip = commit(fowner[f], t, k, t, msg, maintip, "", ops)
  } else {
    msg = sprintf("Merge pull request #%d from tidewater/%s\n\n%s\n", fpr[f], fbranch[f], body)
    maintip = commit(k, t, k, t, msg, maintip, ftip[f], ops)
  }
  fstate[f] = squash ? "squashed" : "merged"
  fland[f] = day
  if (squash) lastsquash[++nsquash] = f
  else lastmerge[++nmerge] = f
  if (fkind[f] == "fix") lastfix = fpr[f]
  notes = notes sprintf("\n- %s (#%d)", ftitle[f], fpr[f])
  # Now and then a merge breaks something and goes straight back out.
  if (!squash && (fkind[f] == "feature" || fkind[f] == "fix") && chance(0.04)) {
    t += ri(20, 90) * 60
    maintip = commit(k, t, k, t, sprintf("Revert \"%s\"\n\nIt broke %s. Reverts #%d.\n", ftitle[f], pick("the readings page for stations without a feed|the nightly export|alerts for Gull Point|the map on Safari"), fpr[f]), maintip, "", undo)
    reverts++
    for (i = 1; i <= fn[f]; i++) {
      p = ffile[f, i]
      if (index(undo, "D " p "\n")) delete mc[p]
    }
    # Put mc back from the undo ops by replaying what land saved.
    restore(f)
  }
  return t
}

# restore puts back what the files of f held before it landed (saved in prev[]).
function restore(f,   i, p) {
  for (i = 1; i <= fn[f]; i++) {
    p = ffile[f, i]
    if ((f, p) in prev) mc[p] = prev[f, p]
  }
}

function release(day, t, version,   ops, ch, msg, r, i, p) {
  curversion = version
  ch = sprintf("## %s - %s\n%s\n", version, dateof(day), notes == "" ? "\n- Small fixes." : notes)
  mc["CHANGELOG.md"] = "# Changelog\n\n" ch (("CHANGELOG.md" in body_of) ? "\n" body_of["CHANGELOG.md"] : "")
  body_of["CHANGELOG.md"] = ch (("CHANGELOG.md" in body_of) ? "\n" body_of["CHANGELOG.md"] : "")
  addpath("CHANGELOG.md")
  notes = ""
  ops = mop("CHANGELOG.md", mc["CHANGELOG.md"]) mop("package.json", pkgjson(version, 0, "")) mop("package-lock.json", lockjson(version, 0, ""))
  maintip = commit(1, t, 1, t, "Release " version "\n", maintip, "", ops)
  printf "tag v%s\nfrom %s\ntagger %s\n%s", version, maintip, ident(1, t + 60), datablock("Tidewater " version)
  if (version ~ /^1\./) {
    r = ++releases
    rver[r] = version
    rtip[r] = maintip
    rlog[r] = body_of["CHANGELOG.md"]
    for (i = 1; i <= npaths; i++) {
      p = paths[i]
      if ((p in mc) && p ~ /^src\//) { rsnap[r, ++rn[r]] = p; rcontent[r, p] = mc[p] }
    }
    if (chance(0.65)) rpatch[r] = day + ri(6, 13)
  }
}

function patch(r, day, t,   k, p, ops, parts, v, ch) {
  k = ri(2, people)
  if (!active(k, day)) k = 1
  p = rsnap[r, ri(1, rn[r])]
  rcontent[r, p] = grow(p, rcontent[r, p], day)
  rtip[r] = commit(k, t, k, t, sprintf("%s %s (backport of #%d)\n", pick("Fix|Handle"), pick(bugs["api"] "|" bugs["alerts"]), lastfix > 0 ? lastfix : ri(1, pr)), rtip[r], "", mop(p, rcontent[r, p]))
  split(rver[r], parts, ".")
  rpatchn[r]++
  v = parts[1] "." parts[2] "." rpatchn[r]
  ch = sprintf("## %s - %s\n\n- A backported fix.\n", v, dateof(day))
  rlog[r] = ch "\n" rlog[r]
  t += 1800
  ops = mop("CHANGELOG.md", "# Changelog\n\n" rlog[r]) mop("package.json", pkgjson(v, 0, "")) mop("package-lock.json", lockjson(v, 0, ""))
  rtip[r] = commit(1, t, 1, t, "Release " v "\n", rtip[r], "", ops)
  printf "tag v%s\nfrom %s\ntagger %s\n%s", v, rtip[r], ident(1, t + 60), datablock("Tidewater " v)
  delete rpatch[r]
  if (rpatchn[r] == 1 && chance(0.3)) rpatch[r] = day + ri(8, 15)
}

function bump(day, t,   j, parts, to, b, tip, ops) {
  j = ri(1, ndeps)
  split(dver[j], parts, ".")
  to = chance(0.75) ? parts[1] "." parts[2] "." (parts[3] + ri(1, 3)) : parts[1] "." (parts[2] + 1) ".0"
  b = "dependabot/npm_and_yarn/" dname[j] "-" to
  pr++
  ops = mop("package.json", pkgjson(curversion, j, to)) mop("package-lock.json", lockjson(curversion, j, to))
  tip = commit(bot, t, bot, t, sprintf("Bump %s from %s to %s\n\nBumps [%s](https://www.npmjs.example/package/%s) from %s to %s.\n", dname[j], dver[j], to, dname[j], dname[j], dver[j], to), maintip, "", ops)
  if (chance(0.8)) {
    t += ri(30, 180) * 60
    dver[j] = to
    maintip = commit(1, t, 1, t, sprintf("Merge pull request #%d from tidewater/%s\n\nBump %s from %s to %s\n", pr, b, dname[j], parts[1] "." parts[2] "." parts[3], to), maintip, tip, ops)
  } else {
    nbots++
    botbranch[nbots] = b; bottip[nbots] = tip; botday[nbots] = day
  }
  return t
}

# ── The simulation ──────────────────────────────────────────────────────────────────────

BEGIN {
  seed = 20250106
  split("Jan Feb Mar Apr May Jun Jul Aug Sep Oct Nov Dec", mon, " ")

  people = 7
  person(1, "Emily Carter", "emily@tidewater.example", "-0500", 0, 9999, "api|alerts|ui|docs|infra|tides")
  person(2, "Marcus Johnson", "marcus@tidewater.example", "-0600", 0, 9999, "api|api|tides|infra")
  person(3, "Sarah Mitchell", "sarah@tidewater.example", "-0800", 0, 9999, "docs|alerts|alerts|ui")
  person(4, "Tyler Brooks", "tyler@tidewater.example", "-0500", 90, 9999, "ui|ui|ui|alerts")
  person(5, "Jessica Nguyen", "jessica@tidewater.example", "-0800", 240, 9999, "data|tides|api|tides")
  person(6, "David Ramirez", "david@tidewater.example", "-0700", 0, 9999, "infra|api|alerts")
  person(7, "Kevin Walsh", "kevin@contractors.example", "-0500", 7, 170, "ui|api")
  bot = 8
  pname[bot] = "dependabot[bot]"; pmail[bot] = "bot@tidewater.example"; ptz[bot] = "+0000"

  dir["api"] = "src/api/"; dir["alerts"] = "src/alerts/"; dir["tides"] = "src/tides/"; dir["ui"] = "public/js/"
  dir["docs"] = "docs/"; dir["data"] = "data/archive/"; dir["infra"] = ""
  ext["api"] = ".js"; ext["alerts"] = ".js"; ext["tides"] = ".js"; ext["ui"] = ".js"
  modules["api"] = "routes|history|forecasts|health|export|limits|search|keys|paging|errors|partners|metrics|cache-headers|webhooks"
  modules["alerts"] = "rules|gusts|storms|notify|email|sms|quiet-hours|thresholds|digest|escalation|snooze|channels"
  modules["tides"] = "harmonics|ports|predict|tables|surge|moon|datum|springs|extremes|offsets"
  modules["ui"] = "chart|legend|layers|sidebar|filters|theme|shortcuts|timeline|wind-rose|tooltip|search-box|settings|print"
  nouns["api"] = "gzip on large responses|an endpoint for station metadata|conditional requests|a status page feed|bulk reads for partners|rate limits for the readings API|a health check|paging for the history API|CSV export|forecast data in the API|API keys for partner stations|search across stations|webhooks for partners|cache headers on readings|request metrics"
  nouns["alerts"] = "push notifications|alert history|a test alert button|lightning alerts|fog alerts|gust alerts|quiet hours for alerts|SMS alerts|storm warnings|per-station thresholds|a daily alert digest|alert escalation|snoozing an alert|alert channels"
  nouns["tides"] = "tide curves on the chart|a seven-day outlook|tidal stream arrows|spring and neap labels|a tide table page|moon phase in predictions|surge corrections|chart datum offsets|high and low water times|secondary ports"
  nouns["ui"] = "a compact layout|unit switching|a station comparison view|saved views|a high-contrast theme|a dark theme|a wind rose|station filters|a map legend|keyboard shortcuts|a reading timeline|a settings page|printable tide tables|tooltips on the chart"
  nouns["docs"] = "the docs"; nouns["data"] = "the archive"; nouns["infra"] = "the build"
  bugs["api"] = "stale readings after a feed outage|a timeout on the forecast endpoint|an off-by-one in paging|CORS on the readings API|a memory leak in the poller|empty bodies on 204 responses|paging past the last record|time zones in the history export|a crash when a station has no readings|a 500 on an unknown station|duplicate rows in the export|a slow query on the history page"
  bugs["alerts"] = "alerts for retired stations|the snooze button resetting|alerts with no station name|a race between two alert rules|digest emails sent twice|gust alerts firing twice|alerts during quiet hours|an alert that never clears|emails sent in UTC|a threshold of zero"
  bugs["tides"] = "the springs label a day early|surge offsets for Craig Head|daylight saving in the tables|a negative tide height|tide times around midnight|a wrong datum at Saltmarsh|predictions across a year boundary|rounding in the tide table"
  bugs["ui"] = "the wind rose on narrow screens|dark mode in the tooltips|the date picker in Firefox|the filter menu staying open|pins drawn twice after a reload|the chart's axis labels|the map's zoom on phones|a flicker when the theme changes|the legend covering pins|focus after closing a dialog"
  bugs["docs"] = bugs["api"]; bugs["data"] = bugs["api"]; bugs["infra"] = "a flaky CI step|the Docker build cache|the nightly backup on Sundays"
  chores = "the CI workflow|the Docker image|the backup script|the seed script|the deploy workflow"
  docpages = "deploy|api|station-handbook|alert-guide|runbook|faq|architecture|contributing"

  split("chart.js 4.4.1|date-fns 3.6.0|leaflet 1.9.4|pino 9.1.0|undici 6.18.2|zod 3.23.8|vite 5.2.11|vitest 1.6.0|eslint 9.3.0|prettier 3.2.5", tmp, "|")
  ndeps = 10; ndev = 6
  for (j = 1; j <= ndeps; j++) { split(tmp[j], nv, " "); dname[j] = nv[1]; dver[j] = nv[2] }
  curversion = "0.0.1"

  # The releases: monthly while it's young, every eight weeks after 1.0.
  rel[36] = "0.1.0"; rel[64] = "0.2.0"; rel[92] = "0.3.0"; rel[127] = "0.4.0"; rel[155] = "0.5.0"
  rel[183] = "1.0.0"; rel[239] = "1.1.0"; rel[295] = "1.2.0"; rel[351] = "1.3.0"; rel[407] = "1.4.0"
  rel[463] = "1.5.0"; rel[519] = "1.6.0"; rel[575] = "1.7.0"

  maintip = root
  for (day = 1; day <= days; day++) {
    wd = weekday(day)
    base = start + day * 86400
    # Mornings (from 15:00 UTC, 10:00 on the east coast): the bot, merges, releases.
    t = base + 15 * 3600 + ri(0, 30) * 60
    if (wd == 0 && day >= 14) t = bump(day, base + 7 * 3600) + ri(10, 40) * 60
    if (wd < 5) {
      for (f = 1; f <= features; f++) {
        if (fstate[f] == "ready" && fready[f] <= day) {
          if (chance(0.04) || !active(fowner[f], day)) { fstate[f] = "abandoned"; fland[f] = day; release_files(f); continue }
          for (i = 1; i <= fn[f]; i++) if (ffile[f, i] in mc) prev[f, ffile[f, i]] = mc[ffile[f, i]]
          t = land(f, day, t) + ri(15, 50) * 60
        }
      }
      for (r = 1; r <= releases; r++) if ((r in rpatch) && rpatch[r] == day) { patch(r, day, t); t += 3600 }
      if (day in rel) { release(day, t, rel[day]); t += 1800 }
      # Now and then a small change straight on main.
      if (chance(0.06) && (p = existing(pick("docs/|src/api/|src/alerts/|public/js/"))) != "") {
        mc[p] = grow(p, mc[p], day)
        maintip = commit(1, t, 1, t, pick("Fix a typo|Tidy a comment|Correct a link|Update a default") " in " p "\n", maintip, "", mop(p, mc[p]))
      }
    }
    # Afternoons (from 18:30 UTC): everyone works on their branch.
    for (k = 1; k <= people; k++) {
      if (!active(k, day)) {
        if (current[k] && day > pto[k]) { fstate[current[k]] = "abandoned"; fland[current[k]] = day; release_files(current[k]); current[k] = 0 }
        continue
      }
      if (wd >= 5 && !chance(0.03)) continue
      if (wd < 5 && !chance(0.3)) continue
      if (!current[k]) {
        # Marcus builds a stack near the end: three branches, each on the one before.
        if (k == 2 && day >= 560 && stack < 3) {
          stack++
          forced_branch = "feature/export-" (stack == 1 ? "api" : stack == 2 ? "csv" : "ui")
          f = openpr(k, day, stack == 1 ? "Add an export API" : stack == 2 ? "Export readings as CSV" : "Add an export button")
          if (!f) { stack--; continue }
          fstacked[f] = 1
          if (stack > 1) { fbase[f] = stacktip; ftip[f] = stacktip }
          stackf[stack] = f
        } else if (!openpr(k, day, "")) continue
      }
      f = current[k]
      n = ri(1, 3)
      t = base + 18 * 3600 + 1800 + ri(0, 90) * 60
      for (c = 0; c < n && current[k] == f; c++) {
        work(f, day, t)
        t += ri(25, 140) * 60
      }
      if (fstacked[f] && fstate[f] == "stacked") stacktip = ftip[f]
    }
  }

  # ── What's left as branches ─────────────────────────────────────────────────────────
  printf "reset refs/heads/main\nfrom %s\n\n", maintip
  for (f = 1; f <= features; f++) {
    keep = fstate[f] == "open" || fstate[f] == "ready" || fstate[f] == "stacked"
    if (fstate[f] == "abandoned" && fdone[f] > 0 && days - fland[f] < 270) keep = 1
    if (keep) printf "reset refs/heads/%s\nfrom %s\n\n", fbranch[f], ftip[f]
  }
  # Merged but never deleted: a host's "delete branch" button not pressed.
  for (i = nmerge - 2; i <= nmerge; i++) if (i > 0) printf "reset refs/heads/%s\nfrom %s\n\n", fbranch[lastmerge[i]], ftip[lastmerge[i]]
  for (i = nsquash - 1; i <= nsquash; i++) if (i > 0) printf "reset refs/heads/%s\nfrom %s\n\n", fbranch[lastsquash[i]], ftip[lastsquash[i]]
  for (r = 1; r <= releases; r++) printf "reset refs/heads/release/v%s\nfrom %s\n\n", substr(rver[r], 1, length(rver[r]) - 2), rtip[r]
  for (i = 1; i <= nbots; i++) if (days - botday[i] < 35) printf "reset refs/heads/%s\nfrom %s\n\n", botbranch[i], bottip[i]
}
AWK
