import { useEffect, useRef, useState } from "react"
import {
  GlobeHemisphereWest,
  MapTrifold,
  Minus,
  Plus,
} from "@phosphor-icons/react"
import {
  type GeoJSONSource,
  LngLatBounds,
  Map as MapLibreMap,
  Popup,
  setWorkerUrl,
} from "maplibre-gl"
// Bundle MapLibre's worker with the app so it is served from our own origin,
// which the Content-Security-Policy allows without extra rules.
import workerUrl from "maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url"

import { Button } from "~/components/ui/button"
import { ButtonGroup } from "~/components/ui/button-group"
import { greatCirclePath, type LngLat, sphericalCenter } from "~/lib/geo"

import type { FlightRoute, RouteAirport } from "~/types"

import "maplibre-gl/dist/maplibre-gl.css"

setWorkerUrl(workerUrl)

// Keyless dark basemap (Esri Dark Gray Canvas). The tile host must stay
// allowed in the CSP connect-src directive (internal/server/middleware.go) or
// tiles are blocked in production builds.
const TILE_URL =
  "https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Base/MapServer/tile/{z}/{y}/{x}"

const ROUTES_SOURCE = "routes"
const AIRPORTS_SOURCE = "airports"
const AIRPORTS_LAYER = "airports"
const ROUTE_COLOR = "#3b82f6"

// Zoom at which the whole globe fits a map of GLOBE_REFERENCE_SIZE pixels
// on its shorter side when centred on the equator. Halving the size takes
// one zoom level off.
const WHOLE_GLOBE_ZOOM = 1.9
const GLOBE_REFERENCE_SIZE = 500
// Where the globe faces before the flights have loaded.
const INITIAL_LATITUDE = 30
// Keeps the latitude correction finite when the centre is near a pole.
const MIN_LATITUDE_SCALE = 0.05

// Zoom at which the whole globe fits the map as it is currently sized, so it
// is not cut off on a narrow phone screen. MapLibre draws the globe larger
// the further the centre is from the equator (by 1 / cos(latitude)), so the
// zoom is lowered by the same factor to keep the globe the same size
// wherever the flights are.
function wholeGlobeZoom(container: HTMLElement, latitude: number): number {
  const { clientWidth, clientHeight } = container
  const shorter = Math.min(clientWidth, clientHeight) || GLOBE_REFERENCE_SIZE
  const latitudeScale = Math.max(
    MIN_LATITUDE_SCALE,
    Math.cos((latitude * Math.PI) / 180)
  )
  return (
    WHOLE_GLOBE_ZOOM +
    Math.log2(shorter / GLOBE_REFERENCE_SIZE) +
    Math.log2(latitudeScale)
  )
}

type Projection = "globe" | "mercator"

const PROJECTION_KEY = "flightlog.mapProjection"

// The globe is the default; the choice is remembered in this browser.
function savedProjection(): Projection {
  try {
    return localStorage.getItem(PROJECTION_KEY) === "mercator"
      ? "mercator"
      : "globe"
  } catch {
    return "globe"
  }
}

function saveProjection(projection: Projection) {
  try {
    localStorage.setItem(PROJECTION_KEY, projection)
  } catch {
    // Storage can be unavailable (private mode); the choice just isn't kept.
  }
}
// GeoJSON as accepted by a MapLibre source.
type Features = Exclude<Parameters<GeoJSONSource["setData"]>[0], string>

function airportsOf(routes: FlightRoute[]): RouteAirport[] {
  const byIata = new Map<string, RouteAirport>()
  for (const route of routes) {
    byIata.set(route.departure.iata, route.departure)
    byIata.set(route.arrival.iata, route.arrival)
  }
  return Array.from(byIata.values())
}

// Every route follows its great circle. The path keeps continuous longitudes,
// so a route across the date line stays in one piece on the globe; on the flat
// map it leaves through one edge and comes back in through the opposite one.
function routeFeatures(routes: FlightRoute[]): Features {
  return {
    type: "FeatureCollection",
    features: routes.map((route) => ({
      type: "Feature",
      properties: {},
      geometry: {
        type: "LineString",
        coordinates: greatCirclePath(
          route.departure.location,
          route.arrival.location
        ),
      },
    })),
  }
}

function airportFeatures(airports: RouteAirport[]): Features {
  return {
    type: "FeatureCollection",
    features: airports.map((airport) => ({
      type: "Feature",
      properties: { iata: airport.iata, name: airport.name },
      geometry: {
        type: "Point",
        coordinates: [airport.location.lon, airport.location.lat],
      },
    })),
  }
}

function addLayers(map: MapLibreMap) {
  const empty: Features = { type: "FeatureCollection", features: [] }
  map.addSource(ROUTES_SOURCE, { type: "geojson", data: empty })
  map.addSource(AIRPORTS_SOURCE, { type: "geojson", data: empty })

  map.addLayer({
    id: "routes",
    type: "line",
    source: ROUTES_SOURCE,
    layout: { "line-cap": "round", "line-join": "round" },
    paint: { "line-color": ROUTE_COLOR, "line-width": 2, "line-opacity": 0.6 },
  })
  map.addLayer({
    id: AIRPORTS_LAYER,
    type: "circle",
    source: AIRPORTS_SOURCE,
    paint: {
      "circle-radius": 5,
      "circle-color": ROUTE_COLOR,
      "circle-stroke-color": "#1d4ed8",
      "circle-stroke-width": 1,
    },
  })
}

// Shows the airport code and name while the pointer is over an airport dot.
function addAirportTooltip(map: MapLibreMap) {
  const popup = new Popup({
    closeButton: false,
    closeOnClick: false,
    offset: 10,
    className: "airport-tooltip",
  })

  map.on("mousemove", AIRPORTS_LAYER, (e) => {
    const feature = e.features?.[0]
    if (!feature || feature.geometry.type !== "Point") return

    // Nearby airports (two in one city) overlap when zoomed out, so list
    // every airport under the pointer. Build the content from text nodes:
    // names come from imported files.
    const content = document.createElement("div")
    const seen = new Set<string>()
    for (const { properties } of e.features ?? []) {
      const iata = String(properties.iata)
      if (seen.has(iata)) continue
      seen.add(iata)

      const row = document.createElement("div")
      const code = document.createElement("span")
      code.className = "font-medium"
      code.textContent = iata
      const name = document.createElement("span")
      name.className = "block text-xs"
      name.textContent = String(properties.name)
      row.append(code, name)
      content.append(row)
    }

    map.getCanvas().style.cursor = "pointer"
    popup
      .setLngLat(feature.geometry.coordinates as LngLat)
      .setDOMContent(content)
      .addTo(map)
  })

  map.on("mouseleave", AIRPORTS_LAYER, () => {
    map.getCanvas().style.cursor = ""
    popup.remove()
  })
}

// Points the map at the airports: the flat map zooms to fit them all, the
// globe turns to face where most of them are.
function showAirports(
  map: MapLibreMap,
  airports: RouteAirport[],
  projection: Projection
) {
  if (airports.length === 0) return

  const bounds = new LngLatBounds()
  for (const airport of airports) {
    bounds.extend([airport.location.lon, airport.location.lat])
  }

  const spansGlobe = bounds.getEast() - bounds.getWest() > 120
  if (projection === "globe" && spansGlobe) {
    const center = sphericalCenter(airports.map((a) => a.location))
    map.jumpTo({ center, zoom: wholeGlobeZoom(map.getContainer(), center[1]) })
    return
  }

  map.fitBounds(bounds, { padding: 50, maxZoom: 8, animate: false })
}

interface FlightMapProps {
  routes: FlightRoute[]
}

export default function FlightMap({ routes }: FlightMapProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const mapRef = useRef<MapLibreMap | null>(null)
  const [isReady, setIsReady] = useState(false)
  const [projection, setProjection] = useState<Projection>(savedProjection)

  function chooseProjection(next: Projection) {
    saveProjection(next)
    setProjection(next)
  }

  useEffect(() => {
    if (!containerRef.current) return

    const map = new MapLibreMap({
      container: containerRef.current,
      style: {
        version: 8,
        sources: {
          basemap: {
            type: "raster",
            tiles: [TILE_URL],
            tileSize: 256,
            maxzoom: 16,
            attribution:
              "Tiles &copy; Esri &mdash; Esri, HERE, Garmin, &copy; OpenStreetMap contributors",
          },
        },
        layers: [{ id: "basemap", type: "raster", source: "basemap" }],
      },
      center: [0, INITIAL_LATITUDE],
      zoom: wholeGlobeZoom(containerRef.current, INITIAL_LATITUDE),
      dragRotate: false,
      pitchWithRotate: false,
      touchPitch: false,
      attributionControl: { compact: true },
    })
    mapRef.current = map

    map.on("load", () => {
      addLayers(map)
      addAirportTooltip(map)
      setIsReady(true)
    })

    return () => {
      mapRef.current = null
      map.remove()
    }
  }, [])

  useEffect(() => {
    const map = mapRef.current
    if (!map || !isReady) return

    const airports = airportsOf(routes)
    map.setProjection({ type: projection })
    map.getSource<GeoJSONSource>(ROUTES_SOURCE)?.setData(routeFeatures(routes))
    map
      .getSource<GeoJSONSource>(AIRPORTS_SOURCE)
      ?.setData(airportFeatures(airports))
    showAirports(map, airports, projection)
  }, [isReady, routes, projection])

  const controlClass =
    "border border-zinc-600 bg-zinc-800 text-zinc-100 hover:bg-zinc-700"
  const isGlobe = projection === "globe"

  return (
    <div className="relative h-full w-full">
      <div ref={containerRef} className="h-full w-full" />
      <div className="absolute top-2 left-2 z-10 flex flex-col gap-2">
        <ButtonGroup orientation="vertical">
          <Button
            type="button"
            size="icon"
            variant="secondary"
            className={controlClass}
            aria-label="Zoom in"
            onClick={() => mapRef.current?.zoomIn()}
          >
            <Plus className="size-4" />
          </Button>
          <Button
            type="button"
            size="icon"
            variant="secondary"
            className={controlClass}
            aria-label="Zoom out"
            onClick={() => mapRef.current?.zoomOut()}
          >
            <Minus className="size-4" />
          </Button>
        </ButtonGroup>
        <Button
          type="button"
          size="icon"
          variant="secondary"
          className={controlClass}
          aria-label={isGlobe ? "Show flat map" : "Show globe"}
          title={isGlobe ? "Show flat map" : "Show globe"}
          onClick={() => chooseProjection(isGlobe ? "mercator" : "globe")}
        >
          {isGlobe ? (
            <MapTrifold className="size-4" />
          ) : (
            <GlobeHemisphereWest className="size-4" />
          )}
        </Button>
      </div>
    </div>
  )
}
