import { describe, expect, it } from 'vitest'
import { boundsOf, degreeLabel, gridStep, project, propertyText, readGeoJson, shapeSummary, shapesOf, unproject } from './geojson'

const fc = (features: unknown[]) => JSON.stringify({ type: 'FeatureCollection', features })

describe('readGeoJson', () => {
  it('reads a collection’s features, named from their properties', () => {
    const features = readGeoJson(
      fc([
        { type: 'Feature', properties: { name: 'Park', fill: '#00aa00' }, geometry: { type: 'Polygon', coordinates: [[[0, 0], [1, 0], [1, 1], [0, 0]]] } },
        { type: 'Feature', id: 7, properties: {}, geometry: { type: 'Point', coordinates: [2, 3] } },
        { type: 'Feature', properties: null, geometry: { type: 'LineString', coordinates: [[0, 0], [5, 5]] } },
      ]),
    )
    expect(features.map((f) => f.name)).toEqual(['Park', '7', 'Feature 3'])
    expect(features[0].fill).toBe('#00aa00')
    expect(features[0].shapes).toEqual([{ kind: 'area', parts: [[[0, 0], [1, 0], [1, 1], [0, 0]]] }])
    expect(features[1].shapes).toEqual([{ kind: 'point', parts: [[[2, 3]]] }])
  })

  it('reads a single feature, or a bare geometry', () => {
    expect(readGeoJson(JSON.stringify({ type: 'Feature', properties: { title: 'Here' }, geometry: { type: 'Point', coordinates: [1, 2] } }))[0].name).toBe('Here')
    expect(readGeoJson(JSON.stringify({ type: 'MultiPoint', coordinates: [[1, 2], [3, 4]] }))[0].shapes).toEqual([{ kind: 'point', parts: [[[1, 2]], [[3, 4]]] }])
  })

  it('only takes a colour that’s a plain hex colour', () => {
    const [f] = readGeoJson(fc([{ type: 'Feature', properties: { fill: 'url(https://x.io/a)', stroke: 'javascript:x', 'marker-color': '#f00' }, geometry: null }]))
    expect(f.fill).toBe('#f00')
    expect(f.stroke).toBe('#f00')
    expect(f.shapes).toEqual([])
  })

  it('says so when the text isn’t GeoJSON', () => {
    expect(() => readGeoJson('{')).toThrow("This isn't valid JSON")
    expect(() => readGeoJson('{"features": []}')).toThrow('it has no type')
  })
})

describe('shapesOf', () => {
  it('reads every geometry type, and skips bad coordinates', () => {
    expect(shapesOf({ type: 'MultiPolygon', coordinates: [[[[0, 0], [1, 1], [0, 0]]], [[[5, 5], [6, 6], [5, 5]]]] })).toHaveLength(2)
    expect(shapesOf({ type: 'MultiLineString', coordinates: [[[0, 0], [1, 1]], [[2, 2], ['x', 3]]] })).toEqual([{ kind: 'line', parts: [[[0, 0], [1, 1]], [[2, 2]]] }])
    expect(shapesOf({ type: 'GeometryCollection', geometries: [{ type: 'Point', coordinates: [1, 1] }, { type: 'Nope' }] })).toEqual([{ kind: 'point', parts: [[[1, 1]]] }])
    expect(shapesOf({ type: 'Point', coordinates: [NaN, 1] })).toEqual([])
  })
})

describe('the projection', () => {
  it('puts the equator and the prime meridian in the middle, north at the top', () => {
    expect(project([0, 0])).toEqual([0.5, 0.5])
    const [, north] = project([0, 60])
    expect(north).toBeLessThan(0.5)
    expect(project([-180, 0])[0]).toBe(0)
  })

  it('goes back where it came from', () => {
    const [lon, lat] = unproject(project([-71.06, 42.36]))
    expect(lon).toBeCloseTo(-71.06, 6)
    expect(lat).toBeCloseTo(42.36, 6)
  })

  it('stops short of the poles', () => {
    expect(project([0, 90])[1]).toBeCloseTo(0, 6)
    expect(Number.isFinite(project([0, -90])[1])).toBe(true)
  })
})

describe('bounds and the graticule', () => {
  it('spans every feature, with a little room around it', () => {
    const b = boundsOf(readGeoJson(fc([{ type: 'Feature', geometry: { type: 'LineString', coordinates: [[0, 0], [10, 20]] } }])))!
    expect(b.west).toBeLessThan(0)
    expect(b.east).toBeGreaterThan(10)
    expect(b.north).toBeGreaterThan(20)
  })

  it('gives a single point room to be seen', () => {
    const b = boundsOf(readGeoJson(JSON.stringify({ type: 'Point', coordinates: [2.35, 48.85] })))!
    expect(b.east - b.west).toBeGreaterThan(0)
    expect(b.north - b.south).toBeGreaterThan(0)
  })

  it('is null with nothing to draw', () => {
    expect(boundsOf([])).toBeNull()
  })

  it('picks a step that draws a handful of lines', () => {
    expect(gridStep(360)).toBe(90)
    expect(gridStep(40)).toBe(10)
    expect(gridStep(1.2)).toBe(0.25)
    expect(gridStep(0.001)).toBe(0.001)
  })

  it('labels degrees with their hemisphere', () => {
    expect(degreeLabel(30, 'lat', 10)).toBe('30°N')
    expect(degreeLabel(-45, 'lon', 15)).toBe('45°W')
    expect(degreeLabel(0.5, 'lon', 0.5)).toBe('0.5°E')
    expect(degreeLabel(0, 'lat', 10)).toBe('0°')
  })
})

describe('the words around the map', () => {
  it('counts the shapes', () => {
    const features = readGeoJson(
      fc([
        { type: 'Feature', geometry: { type: 'MultiPoint', coordinates: [[0, 0], [1, 1]] } },
        { type: 'Feature', geometry: { type: 'Polygon', coordinates: [[[0, 0], [1, 1], [0, 0]]] } },
      ]),
    )
    expect(shapeSummary(features)).toBe('1 area · 2 points')
  })

  it('writes property values for a table', () => {
    expect(propertyText(null)).toBe('—')
    expect(propertyText({ a: 1 })).toBe('{"a":1}')
    expect(propertyText(3.5)).toBe('3.5')
  })
})
