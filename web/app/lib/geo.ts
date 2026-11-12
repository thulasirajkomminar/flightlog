type LatLng = [number, number]

interface Point {
  lat: number
  lon: number
}

const DEG = Math.PI / 180
// One path vertex per this many degrees of arc keeps curves smooth without
// drawing hundreds of points for short hops.
const DEGREES_PER_STEP = 2

/**
 * Returns the great-circle path between two points as one or more polylines.
 * A straight line on a Mercator map is not the route an aircraft flies, and a
 * route crossing the antimeridian (e.g. Sydney to Los Angeles) would otherwise
 * be drawn the long way round the map, so the path is split there.
 */
export function greatCirclePath(from: Point, to: Point): LatLng[][] {
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

  // Identical or antipodal points have no single great circle; draw directly.
  if (arc < 1e-9 || Math.abs(arc - Math.PI) < 1e-9) {
    return [
      [
        [from.lat, from.lon],
        [to.lat, to.lon],
      ],
    ]
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
  points[0] = [from.lat, from.lon]
  points[steps] = [to.lat, to.lon]

  return splitAtAntimeridian(points)
}

function splitAtAntimeridian(points: LatLng[]): LatLng[][] {
  const lines: LatLng[][] = [[points[0]]]

  for (let i = 1; i < points.length; i++) {
    const [prevLat, prevLon] = points[i - 1]
    const [lat, lon] = points[i]

    if (Math.abs(lon - prevLon) > 180) {
      // Interpolate the latitude where the path meets the +/-180 meridian.
      const edge = prevLon > 0 ? 180 : -180
      const unwrapped = lon + (prevLon > 0 ? 360 : -360)
      const t = (edge - prevLon) / (unwrapped - prevLon)
      const crossLat = prevLat + t * (lat - prevLat)

      lines[lines.length - 1].push([crossLat, edge])
      lines.push([[crossLat, -edge]])
    }

    lines[lines.length - 1].push([lat, lon])
  }

  return lines
}
