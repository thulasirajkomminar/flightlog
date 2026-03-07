import {
  AirplaneIcon,
  AirplaneLandingIcon,
  AirplaneTakeoffIcon,
  CaretDownIcon,
  CheckIcon,
  PlusIcon,
  TrashIcon,
} from "@phosphor-icons/react"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "~/components/ui/alert-dialog"
import { Badge } from "~/components/ui/badge"
import { Button } from "~/components/ui/button"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "~/components/ui/collapsible"
import { Item } from "~/components/ui/item"
import { Separator } from "~/components/ui/separator"
import { formatTime } from "~/lib/format"

import type { Flight, Movement } from "~/types"

function statusColor(status: string): string {
  switch (status.toLowerCase()) {
    case "landed":
    case "active":
      return "bg-green-500/15 text-green-400 border-green-500/20"
    case "diverted":
    case "incident":
      return "bg-orange-500/15 text-orange-400 border-orange-500/20"
    case "cancelled":
      return "bg-red-500/15 text-red-400 border-red-500/20"
    default:
      return "bg-muted text-muted-foreground"
  }
}

function TimeRow({
  label,
  time,
}: {
  label: string
  time: { utc?: string; local?: string }
}) {
  if (!time.local && !time.utc) return null

  return (
    <div className="flex items-center justify-between">
      <span className="text-muted-foreground">{label}</span>
      <div className="grid w-56 grid-cols-2 gap-3 text-right">
        <span>{time.local ? `${formatTime(time.local)} local` : ""}</span>
        <span className="text-muted-foreground">
          {time.utc ? `${formatTime(time.utc)} UTC` : ""}
        </span>
      </div>
    </div>
  )
}

function MovementDetail({
  movement,
  label,
}: {
  movement: Movement
  label: string
}) {
  return (
    <div className="flex flex-col gap-2">
      <h4 className="text-sm font-medium">{label}</h4>
      <div className="grid gap-1.5 text-sm">
        <div className="flex items-center justify-between">
          <span className="text-muted-foreground">Airport</span>
          <span>
            {movement.airport.name} ({movement.airport.iata})
          </span>
        </div>
        {movement.airport.municipalityName && (
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">City</span>
            <span>
              {movement.airport.municipalityName}
              {movement.airport.countryCode
                ? `, ${movement.airport.countryCode}`
                : ""}
            </span>
          </div>
        )}
        <TimeRow label="Scheduled" time={movement.scheduledTime} />
        <TimeRow label="Revised" time={movement.revisedTime} />
        <TimeRow label="Runway" time={movement.runwayTime} />
        {movement.terminal && (
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Terminal</span>
            <span>{movement.terminal}</span>
          </div>
        )}
        {movement.gate && (
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Gate</span>
            <span>{movement.gate}</span>
          </div>
        )}
        {movement.checkInDesk && (
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Check-in</span>
            <span>{movement.checkInDesk}</span>
          </div>
        )}
        {movement.baggageBelt && (
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Baggage</span>
            <span>{movement.baggageBelt}</span>
          </div>
        )}
      </div>
    </div>
  )
}

export function FlightCard({
  flight,
  onAdd,
  onDelete,
  isAdding,
  isAdded,
}: {
  flight: Flight
  onAdd?: (flight: Flight) => void
  onDelete?: (id: string) => void
  isAdding?: boolean
  isAdded?: boolean
}) {
  const depTime =
    flight.departure.scheduledTime.local ||
    flight.departure.scheduledTime.utc ||
    ""
  const arrTime =
    flight.arrival.scheduledTime.local || flight.arrival.scheduledTime.utc || ""

  return (
    <Collapsible>
      <Item variant="outline" className="flex-nowrap gap-6 px-7 py-6">
        <CollapsibleTrigger asChild>
          <button className="flex flex-1 cursor-pointer items-center justify-between text-left">
            <div className="flex min-w-36 flex-col items-start gap-0.5">
              <span className="text-sm text-muted-foreground">
                {flight.airline.name || flight.airline.iata}
              </span>
              <span className="text-lg font-semibold">{flight.number}</span>
            </div>

            <div className="flex flex-col items-center gap-1">
              <div className="flex items-center gap-2">
                <AirplaneTakeoffIcon className="size-5 text-muted-foreground" />
                <span className="text-lg font-medium">
                  {flight.departure.airport.iata}
                </span>
              </div>
              <span className="text-xs text-muted-foreground">
                {flight.flightDate}
                {depTime ? ` · ${formatTime(depTime)}` : ""}
              </span>
            </div>

            <div className="flex flex-col items-center gap-1">
              <div className="flex items-center gap-2">
                <div className="h-px w-12 bg-border" />
                <AirplaneIcon className="size-5 text-primary" weight="fill" />
                <div className="h-px w-12 bg-border" />
              </div>
              {flight.greatCircleDistance.km > 0 && (
                <span className="text-xs text-muted-foreground">
                  {Math.round(flight.greatCircleDistance.km)} KM
                </span>
              )}
            </div>

            <div className="flex flex-col items-center gap-1">
              <div className="flex items-center gap-2">
                <span className="text-lg font-medium">
                  {flight.arrival.airport.iata}
                </span>
                <AirplaneLandingIcon className="size-5 text-muted-foreground" />
              </div>
              <span className="text-xs text-muted-foreground">
                {flight.flightDate}
                {arrTime ? ` · ${formatTime(arrTime)}` : ""}
              </span>
            </div>

            <div className="flex items-center gap-4">
              <Badge
                variant="outline"
                className={`capitalize ${statusColor(flight.status)}`}
              >
                {flight.status}
              </Badge>
              <CaretDownIcon className="size-5 text-muted-foreground transition-transform duration-200 [[data-state=open]_&]:rotate-180" />
            </div>
          </button>
        </CollapsibleTrigger>

        {onAdd && !isAdded && (
          <Button
            size="lg"
            className="shrink-0"
            disabled={isAdding}
            onClick={() => onAdd(flight)}
          >
            <PlusIcon className="size-5" />
            {isAdding ? "Adding..." : "Add"}
          </Button>
        )}
        {isAdded && (
          <Button
            size="lg"
            variant="default"
            className="shrink-0 bg-green-600 text-white disabled:opacity-100"
            disabled
          >
            <CheckIcon className="size-5" />
            Added
          </Button>
        )}
        {onDelete && (
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="ghost" size="icon" className="shrink-0">
                <TrashIcon className="size-4" />
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Delete Flight</AlertDialogTitle>
                <AlertDialogDescription>
                  Are you sure you want to delete flight {flight.number} on{" "}
                  {flight.flightDate}? This action cannot be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction
                  onClick={() => onDelete(flight.id)}
                  className="bg-destructive text-white hover:bg-destructive/90"
                >
                  Delete
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        )}
      </Item>

      <CollapsibleContent>
        <div className="rounded-b-md border border-t-0 border-border bg-muted/30 px-5 py-5">
          <div className="grid gap-6 md:grid-cols-2">
            <MovementDetail movement={flight.departure} label="Departure" />
            <MovementDetail movement={flight.arrival} label="Arrival" />
          </div>

          <Separator className="my-4" />

          <div className="grid grid-cols-3 gap-6 text-sm">
            <div className="flex flex-col gap-1">
              <span className="text-muted-foreground">Airline</span>
              <span>
                {flight.airline.name || "—"}
                {flight.airline.iata ? ` (${flight.airline.iata})` : ""}
              </span>
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-muted-foreground">Aircraft</span>
              <span>{flight.aircraft.model || "—"}</span>
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-muted-foreground">Distance</span>
              <span>
                {flight.greatCircleDistance.km > 0
                  ? `${Math.round(flight.greatCircleDistance.km)} KM / ${Math.round(flight.greatCircleDistance.mile)} MI`
                  : "—"}
              </span>
            </div>
          </div>
        </div>
      </CollapsibleContent>
    </Collapsible>
  )
}
