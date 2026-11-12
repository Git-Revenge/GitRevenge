/**
 * GeoJSON, read and drawn without a map underneath. The app is local-first, so no tiles are
 * fetched from anyone: the shapes are drawn in Web Mercator over lines of latitude and
 * longitude, which is enough to see where they are and how they sit against each other.
 */

export type Position = [number, number]

export interface Shape {
  kind: 'area' | 'line' | 'point'
  /** Rings (areas), lines, or single points, each as positions. */
  parts: Position[][]
}

export interface Feature {
  /** Its name, from its properties or id, for the list beside the map. */
  name: string
  properties: Record<string, unknown>
  shapes: Shape[]
  /** Colours the file gives it (the simplestyle properties), when they're plain hex colours. */
  fill?: string
  stroke?: string
}

type Json = Record<string, unknown>
const isObj = (v: unknown): v is Json => !!v && typeof v === 'object' && !Array.isArray(v)
const isPos = (v: unknown): v is Position => Array.isArray(v) && v.length >= 2 && typeof v[0] === 'number' && typeof v[1] === 'number' && Number.isFinite(v[0]) && Number.isFinite(v[1])

function positions(v: unknown): Position[] {
  return Array.isArray(v) ? v.filter(isPos).map((p) => [p[0], p[1]] as Position) : []
}

/** A geometry's shapes. Unknown types and bad coordinates are skipped, not fatal. */
export function shapesOf(g: unknown): Shape[] {
  if (!isObj(g)) return []
  const c = g.coordinates
  switch (g.type) {
    case 'Point':
      return isPos(c) ? [{ kind: 'point', parts: [[[c[0], c[1]]]] }] : []
    case 'MultiPoint':
      return [{ kind: 'point', parts: positions(c).map((p) => [p]) }]
    case 'LineString':
      return [{ kind: 'line', parts: [positions(c)] }]
    case 'MultiLineString':
      return [{ kind: 'line', parts: Array.isArray(c) ? c.map(positions) : [] }]
    case 'Polygon':
      return [{ kind: 'area', parts: Array.isArray(c) ? c.map(positions) : [] }]
    case 'MultiPolygon':
      return Array.isArray(c) ? c.map((poly) => ({ kind: 'area' as const, parts: Array.isArray(poly) ? poly.map(positions) : [] })) : []
    case 'GeometryCollection':
      return Array.isArray(g.geometries) ? g.geometries.flatMap(shapesOf) : []
  }
  return []
}

const HEX = /^#(?:[0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})$/i
const colour = (v: unknown) => (typeof v === 'string' && HEX.test(v.trim()) ? v.trim() : undefined)

/** Reads a GeoJSON document's features. It throws, with a message to show, when it isn't one. */
export function readGeoJson(text: string): Feature[] {
  let doc: unknown
  try {
    doc = JSON.parse(text)
  } catch {
    throw new Error("This isn't valid JSON, so it can't be read as GeoJSON.")
  }
  if (!isObj(doc) || typeof doc.type !== 'string') throw new Error("This doesn't look like GeoJSON: it has no type.")
  const raw: unknown[] = doc.type === 'FeatureCollection' ? (Array.isArray(doc.features) ? doc.features : []) : doc.type === 'Feature' ? [doc] : [{ type: 'Feature', geometry: doc, properties: {} }]
  return raw.filter(isObj).map((f, i) => {
    const properties = isObj(f.properties) ? f.properties : {}
    const label = properties.name ?? properties.title ?? properties.NAME ?? properties.Name ?? f.id
    return {
      name: typeof label === 'string' || typeof label === 'number' ? String(label) : `Feature ${i + 1}`,
      properties,
      shapes: shapesOf(f.geometry),
      fill: colour(properties.fill) ?? colour(properties['marker-color']),
      stroke: colour(properties.stroke) ?? colour(properties['marker-color']),
    }
  })
}

const MAX_LAT = 85.05112878

/** Web Mercator, as unit coordinates: x from 0 to 1 west to east, y from 0 to 1 north to south. */
export function project([lon, lat]: Position): [number, number] {
  const phi = (Math.max(-MAX_LAT, Math.min(MAX_LAT, lat)) * Math.PI) / 180
  return [(lon + 180) / 360, (1 - Math.log(Math.tan(phi) + 1 / Math.cos(phi)) / Math.PI) / 2]
}

/** And back: a unit point's longitude and latitude. */
export function unproject([x, y]: [number, number]): Position {
  const lat = (Math.atan(Math.sinh(Math.PI * (1 - 2 * y))) * 180) / Math.PI
  return [x * 360 - 180, lat]
}

export interface Box {
  west: number
  south: number
  east: number
  north: number
}

/** What every feature spans, in degrees, or null when there's nothing to draw. */
export function boundsOf(features: Feature[]): Box | null {
  let west = Infinity
  let south = Infinity
  let east = -Infinity
  let north = -Infinity
  for (const f of features)
    for (const s of f.shapes)
      for (const part of s.parts)
        for (const [lon, lat] of part) {
          west = Math.min(west, lon)
          east = Math.max(east, lon)
          south = Math.min(south, lat)
          north = Math.max(north, lat)
        }
  if (!Number.isFinite(west)) return null
  // A single point, or a line along one meridian, still needs some room around it.
  const pad = (span: number) => Math.max(span * 0.08, 0.005)
  const dx = pad(east - west)
  const dy = pad(north - south)
  return { west: Math.max(-180, west - dx), east: Math.min(180, east + dx), south: Math.max(-MAX_LAT, south - dy), north: Math.min(MAX_LAT, north + dy) }
}

/** The step between graticule lines for a span of degrees: about four to eight lines. */
export function gridStep(span: number): number {
  const steps = [90, 45, 30, 20, 10, 5, 2, 1, 0.5, 0.25, 0.1, 0.05, 0.02, 0.01, 0.005, 0.002, 0.001]
  return steps.find((s) => span / s >= 4) ?? steps[steps.length - 1]
}

/** A degree as the graticule labels it: 30°N, 0.5°W. */
export function degreeLabel(value: number, axis: 'lat' | 'lon', step: number): string {
  const places = Math.max(0, Math.ceil(-Math.log10(step)))
  const abs = Math.abs(value).toFixed(places)
  if (Number(abs) === 0) return '0°'
  return `${abs}°${axis === 'lat' ? (value > 0 ? 'N' : 'S') : value > 0 ? 'E' : 'W'}`
}

/** How many of each kind of shape there are, for the line above the map: "3 areas · 2 points". */
export function shapeSummary(features: Feature[]): string {
  const n = { area: 0, line: 0, point: 0 }
  for (const f of features) for (const s of f.shapes) n[s.kind] += s.kind === 'point' ? s.parts.length : 1
  const say = (k: number, one: string, many: string) => (k ? `${k.toLocaleString()} ${k === 1 ? one : many}` : '')
  return [say(n.area, 'area', 'areas'), say(n.line, 'line', 'lines'), say(n.point, 'point', 'points')].filter(Boolean).join(' · ')
}

/** A property's value, written for the table of a feature's properties. */
export function propertyText(v: unknown): string {
  if (v === null || v === undefined) return '—'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}
