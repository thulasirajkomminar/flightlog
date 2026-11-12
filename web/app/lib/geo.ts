type LatLng = [number, number]

interface Point {
  lat: number
  lon: number
}

const DEG = Math.PI / 180
// One path vertex per this many degrees of arc keeps curves smooth without
// drawing hundreds of points for short hops.
const DEGREES_PER_STEP = 2

export interface RoutePath {
  positions: LatLng[]
  // True when the line is a plain connection rather than the path flown.
  direct: boolean
}

/**
 * Returns the line to draw for a route. Normally that is the great-circle
 * path, since a straight line on a Mercator map is not what an aircraft flies.
 *
 * A route whose shortest path crosses the antimeridian (e.g. Sydney to Los
 * Angeles) cannot be drawn in one piece on a flat map, and cutting it at the
 * map edge leaves two halves that each end without an airport. Those routes
 * are returned as a direct line between the two airports instead.
 */
export function routePath(from: Point, to: Point): RoutePath {
  const direct: LatLng[] = [
    [from.lat, from.lon],
    [to.lat, to.lon],
  ]

  if (Math.abs(to.lon - from.lon) > 180) {
    return { positions: direct, direct: true }
  }

  const lat1 = from.lat * DEG
  const lon1 = from.lon * DEG
  const lat2 = to.lat * DEG
  const lon2 = to.lon * DEG

  const sinLat = Math.sin((lat2 - lat1) / 2)
  const sinLon = Math.sin((lon2 - lon1) / 2)
  const arc =
    2 *
    Math.asin(
      Math.sqrt(
        sinLat * sinLat + Math.cos(lat1) * Math.cos(lat2) * sinLon * sinLon
      )
    )

  // Identical or antipodal points have no single great circle.
  if (arc < 1e-9 || Math.abs(arc - Math.PI) < 1e-9) {
    return { positions: direct, direct: true }
  }

  const steps = Math.max(2, Math.ceil(arc / DEG / DEGREES_PER_STEP))
  const points: LatLng[] = []

  for (let i = 0; i <= steps; i++) {
    const f = i / steps
    const a = Math.sin((1 - f) * arc) / Math.sin(arc)
    const b = Math.sin(f * arc) / Math.sin(arc)
    const x =
      a * Math.cos(lat1) * Math.cos(lon1) + b * Math.cos(lat2) * Math.cos(lon2)
    const y =
      a * Math.cos(lat1) * Math.sin(lon1) + b * Math.cos(lat2) * Math.sin(lon2)
    const z = a * Math.sin(lat1) + b * Math.sin(lat2)

    points.push([
      Math.atan2(z, Math.sqrt(x * x + y * y)) / DEG,
      Math.atan2(y, x) / DEG,
    ])
  }

  // Pin the ends to the exact airport coordinates.
  points[0] = direct[0]
  points[steps] = direct[1]

  return { positions: points, direct: false }
}
