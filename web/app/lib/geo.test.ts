import { describe, expect, it } from "vitest"

import { greatCirclePath, type LngLat, sphericalCenter } from "./geo"

const AMS = { lat: 52.3086, lon: 4.7639 }
const JFK = { lat: 40.6413, lon: -73.7781 }
const SYD = { lat: -33.9399, lon: 151.1753 }
const LAX = { lat: 33.9416, lon: -118.4085 }

function last(path: LngLat[]): LngLat {
  return path[path.length - 1]
}

describe("greatCirclePath", () => {
  it("starts and ends exactly at the two airports", () => {
    const path = greatCirclePath(AMS, JFK)

    expect(path[0]).toEqual([AMS.lon, AMS.lat])
    expect(last(path)[0]).toBeCloseTo(JFK.lon, 6)
    expect(last(path)[1]).toBeCloseTo(JFK.lat, 6)
  })

  it("bulges towards the pole instead of following a straight line", () => {
    const path = greatCirclePath(AMS, JFK)
    const highest = Math.max(...path.map(([, lat]) => lat))

    // The North Atlantic track runs north of both airports.
    expect(highest).toBeGreaterThan(AMS.lat)
  })

  it("keeps longitude continuous across the date line", () => {
    const path = greatCirclePath(SYD, LAX)

    for (let i = 1; i < path.length; i++) {
      expect(Math.abs(path[i][0] - path[i - 1][0])).toBeLessThan(10)
    }

    // Eastwards from Sydney (151) the path ends past 180, at LAX + 360.
    expect(last(path)[0]).toBeCloseTo(LAX.lon + 360, 6)
  })

  it("takes the short way round in both directions", () => {
    const path = greatCirclePath(LAX, SYD)

    expect(path[0]).toEqual([LAX.lon, LAX.lat])
    expect(last(path)[0]).toBeCloseTo(SYD.lon - 360, 6)
  })

  it("uses more points for longer routes", () => {
    const short = greatCirclePath(AMS, { lat: 51.47, lon: -0.45 })
    const long = greatCirclePath(AMS, JFK)

    expect(short.length).toBeGreaterThanOrEqual(3)
    expect(long.length).toBeGreaterThan(short.length)
  })

  it("returns a plain two-point line for identical points", () => {
    expect(greatCirclePath(AMS, AMS)).toEqual([
      [AMS.lon, AMS.lat],
      [AMS.lon, AMS.lat],
    ])
  })
})

describe("sphericalCenter", () => {
  it("is the point itself for a single point", () => {
    const [lon, lat] = sphericalCenter([AMS])

    expect(lon).toBeCloseTo(AMS.lon, 6)
    expect(lat).toBeCloseTo(AMS.lat, 6)
  })

  it("finds the middle across the date line, not on the far side", () => {
    const [lon] = sphericalCenter([
      { lat: 0, lon: 170 },
      { lat: 0, lon: -170 },
    ])

    // A plain average of the longitudes would give 0, the opposite side.
    expect(Math.abs(lon)).toBeCloseTo(180, 6)
  })

  it("falls back to a fixed centre when points cancel out", () => {
    expect(
      sphericalCenter([
        { lat: 0, lon: 0 },
        { lat: 0, lon: 180 },
      ])
    ).toEqual([0, 0])
  })
})
