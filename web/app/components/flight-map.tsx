import { useEffect, useMemo, useState } from "react"
import {
  CircleMarker,
  MapContainer,
  Polyline,
  TileLayer,
  Tooltip,
  useMap,
  useMapEvents,
} from "react-leaflet"
import { Minus, Plus } from "@phosphor-icons/react"

import { Button } from "~/components/ui/button"
import { ButtonGroup } from "~/components/ui/button-group"
import { routePath } from "~/lib/geo"

import type { FlightRoute, RouteAirport } from "~/types"

import "leaflet/dist/leaflet.css"

function ZoomControl() {
  const map = useMap()
  const [zoom, setZoom] = useState(map.getZoom())

  useMapEvents({
    zoomend: () => setZoom(map.getZoom()),
  })

  return (
    <div className="absolute top-2 left-2 z-[1000]">
      <ButtonGroup orientation="vertical">
        <Button
          type="button"
          size="icon"
          variant="secondary"
          className="border border-zinc-600 bg-zinc-800 text-zinc-100 hover:bg-zinc-700"
          disabled={zoom >= map.getMaxZoom()}
          onClick={() => map.zoomIn()}
        >
          <Plus className="size-4" />
        </Button>
        <Button
          type="button"
          size="icon"
          variant="secondary"
          className="border border-zinc-600 bg-zinc-800 text-zinc-100 hover:bg-zinc-700"
          disabled={zoom <= map.getMinZoom()}
          onClick={() => map.zoomOut()}
        >
          <Minus className="size-4" />
        </Button>
      </ButtonGroup>
    </div>
  )
}

// Zooms the map to fit all airports once they are known.
function FitBounds({ airports }: { airports: RouteAirport[] }) {
  const map = useMap()

  useEffect(() => {
    if (airports.length > 0) {
      map.fitBounds(
        airports.map((a) => [a.location.lat, a.location.lon]),
        { padding: [50, 50], maxZoom: 10 }
      )
    }
  }, [map, airports])

  return null
}

interface FlightMapProps {
  routes: FlightRoute[]
}

export default function FlightMap({ routes }: FlightMapProps) {
  // Keyless dark basemap (Esri Dark Gray Canvas). The tile host must stay
  // allowed in the CSP img-src directive (internal/server/middleware.go) or
  // tiles are blocked in production builds.
  const mapTileUrl =
    "https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Base/MapServer/tile/{z}/{y}/{x}"

  const airports = useMemo(() => {
    const byIata = new Map<string, RouteAirport>()
    for (const route of routes) {
      byIata.set(route.departure.iata, route.departure)
      byIata.set(route.arrival.iata, route.arrival)
    }
    return Array.from(byIata.values())
  }, [routes])

  const paths = useMemo(
    () =>
      routes.map((route) => ({
        key: `${route.departure.iata}-${route.arrival.iata}`,
        path: routePath(route.departure.location, route.arrival.location),
      })),
    [routes]
  )

  return (
    <MapContainer
      center={[30, 0]}
      zoom={2}
      minZoom={2}
      maxBounds={[
        [-90, -180],
        [90, 180],
      ]}
      maxBoundsViscosity={1.0}
      className="h-full w-full"
      scrollWheelZoom={true}
      zoomControl={false}
    >
      <TileLayer
        url={mapTileUrl}
        maxNativeZoom={16}
        attribution="Tiles &copy; Esri &mdash; Esri, HERE, Garmin, &copy; OpenStreetMap contributors"
      />
      <ZoomControl />
      <FitBounds airports={airports} />

      {paths.map(({ key, path }) => (
        <Polyline
          key={key}
          positions={path.positions}
          pathOptions={{
            color: "#3b82f6",
            weight: 2,
            opacity: 0.6,
            // Dashed marks a plain connection that is not the path flown.
            dashArray: path.direct ? "6 6" : undefined,
          }}
        />
      ))}

      {airports.map((airport) => (
        <CircleMarker
          key={airport.iata}
          center={[airport.location.lat, airport.location.lon]}
          radius={5}
          pathOptions={{
            fillColor: "#3b82f6",
            fillOpacity: 1,
            color: "#1d4ed8",
            weight: 1,
          }}
        >
          <Tooltip direction="top" offset={[0, -8]}>
            <span className="font-medium">{airport.iata}</span>
            <br />
            <span className="text-xs">{airport.name}</span>
          </Tooltip>
        </CircleMarker>
      ))}
    </MapContainer>
  )
}
