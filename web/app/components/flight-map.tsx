import { useEffect, useMemo, useRef, useState } from "react"
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

import type { Flight } from "~/types"

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

interface FlightMapProps {
  flights: Flight[]
}

export default function FlightMap({ flights }: FlightMapProps) {
  const mapRef = useRef<L.Map | null>(null)

  const routes = flights
    .filter(
      (f) =>
        f.departure.airport.location.lat &&
        f.departure.airport.location.lon &&
        f.arrival.airport.location.lat &&
        f.arrival.airport.location.lon
    )
    .map((f) => ({
      id: f.id,
      dep: {
        lat: f.departure.airport.location.lat,
        lng: f.departure.airport.location.lon,
        iata: f.departure.airport.iata,
        name: f.departure.airport.name,
      },
      arr: {
        lat: f.arrival.airport.location.lat,
        lng: f.arrival.airport.location.lon,
        iata: f.arrival.airport.iata,
        name: f.arrival.airport.name,
      },
    }))

  const airports = useMemo(() => {
    const map = new Map<
      string,
      { lat: number; lng: number; iata: string; name: string }
    >()
    for (const route of routes) {
      map.set(route.dep.iata, route.dep)
      map.set(route.arr.iata, route.arr)
    }
    return map
  }, [routes])

  useEffect(() => {
    if (mapRef.current && airports.size > 0) {
      const points = Array.from(airports.values()).map(
        (a) => [a.lat, a.lng] as [number, number]
      )
      mapRef.current.fitBounds(points, { padding: [50, 50], maxZoom: 10 })
    }
  }, [airports])

  return (
    <MapContainer
      ref={mapRef}
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
      attributionControl={false}
    >
      <TileLayer url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png" />
      <ZoomControl />

      {routes.map((route) => (
        <Polyline
          key={route.id}
          positions={[
            [route.dep.lat, route.dep.lng],
            [route.arr.lat, route.arr.lng],
          ]}
          pathOptions={{
            color: "#3b82f6",
            weight: 2,
            opacity: 0.6,
          }}
        />
      ))}

      {Array.from(airports.values()).map((airport) => (
        <CircleMarker
          key={airport.iata}
          center={[airport.lat, airport.lng]}
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
