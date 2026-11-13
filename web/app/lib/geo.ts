// A [longitude, latitude] pair, the coordinate order GeoJSON and MapLibre use.
export type LngLat = [number, number]

interface Point {
  lat: number
  lon: number
}

const DEG = Math.PI / 180
// One path vertex per this many degrees of arc keeps curves smooth without
// drawing hundreds of points for short hops.
const DEGREES_PER_STEP = 2

/**
 * Returns the great-circle path between two points: the route an aircraft
 * flies, which is not a straight line on a flat map.
 *
 * Longitudes are continuous rather than wrapped to [-180, 180]: a path from
 * Sydney (151) to Los Angeles (-118) ends at 242. That keeps the line in one
 * piece on a globe; on a flat map it runs off one edge and re-enters through
 * the opposite one.
 */
export function greatCirclePath(from: Point, to: Point): LngLat[] {
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
    return [
      [from.lon, from.lat],
      [to.lon, to.lat],
    ]
  }

  const steps = Math.max(2, Math.ceil(arc / DEG / DEGREES_PER_STEP))
  const path: LngLat[] = [[from.lon, from.lat]]

  for (let i = 1; i <= steps; i++) {
    const f = i / steps
    const a = Math.sin((1 - f) * arc) / Math.sin(arc)
    const b = Math.sin(f * arc) / Math.sin(arc)
    const x =
      a * Math.cos(lat1) * Math.cos(lon1) + b * Math.cos(lat2) * Math.cos(lon2)
    const y =
      a * Math.cos(lat1) * Math.sin(lon1) + b * Math.cos(lat2) * Math.sin(lon2)
    const z = a * Math.sin(lat1) + b * Math.sin(lat2)

    const lat = Math.atan2(z, Math.sqrt(x * x + y * y)) / DEG
    let lon = Math.atan2(y, x) / DEG

    // Keep longitude continuous with the previous vertex across +/-180.
    const prevLon = path[i - 1][0]
    while (lon - prevLon > 180) lon -= 360
    while (lon - prevLon < -180) lon += 360

    path.push([lon, lat])
  }

  return path
}

/**
 * Returns the point on the sphere "in the middle" of a set of points, used to
 * turn a globe towards where most of them are.
 */
export function sphericalCenter(points: Point[]): LngLat {
  let x = 0
  let y = 0
  let z = 0

  for (const p of points) {
    x += Math.cos(p.lat * DEG) * Math.cos(p.lon * DEG)
    y += Math.cos(p.lat * DEG) * Math.sin(p.lon * DEG)
    z += Math.sin(p.lat * DEG)
  }

  // Points spread evenly around the globe cancel out; any centre will do.
  if (Math.hypot(x, y, z) < 1e-9) {
    return [0, 0]
  }

  return [Math.atan2(y, x) / DEG, Math.atan2(z, Math.hypot(x, y)) / DEG]
}
