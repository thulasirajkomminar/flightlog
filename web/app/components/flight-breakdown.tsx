import { AirplaneIcon } from "@phosphor-icons/react"

import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "~/components/ui/tooltip"

import type { FlightBreakdown as Breakdown } from "~/types"

const EARTH_CIRCUMFERENCE_KM = 40_075
const MOON_DISTANCE_KM = 384_400

const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
]

// One hue, brighter for more flights: on the dark surface the darkest step
// reads as "few". The first entry is the empty cell.
const HEAT_STEPS = [
  "bg-muted/50",
  "bg-chart-5",
  "bg-chart-3",
  "bg-chart-2",
  "bg-chart-1",
]

function km(value: number): string {
  return `${Math.round(value).toLocaleString()} km`
}

function flightCount(n: number): string {
  return `${n} ${n === 1 ? "flight" : "flights"}`
}

// A headline number with a line saying what it is.
function Fact({
  value,
  label,
  detail,
  children,
}: {
  value: string
  label: string
  detail?: string
  children?: React.ReactNode
}) {
  return (
    <Card>
      <CardContent className="flex h-full flex-col gap-1 py-5">
        <span className="text-3xl font-bold sm:text-4xl">{value}</span>
        <span className="text-sm text-muted-foreground">{label}</span>
        {children}
        {detail && (
          <span className="mt-auto pt-2 text-xs text-muted-foreground">
            {detail}
          </span>
        )}
      </CardContent>
    </Card>
  )
}

// The distance flown, put next to two distances everyone can picture.
function Facts({
  breakdown,
  distance,
}: {
  breakdown: Breakdown
  distance: number
}) {
  const earth = distance / EARTH_CIRCUMFERENCE_KM
  const moon = distance / MOON_DISTANCE_KM
  const { longest } = breakdown

  return (
    <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
      <Fact
        value={`${earth.toFixed(1)}×`}
        label="around the Earth"
        detail={`${km(distance)} flown`}
      />
      <Fact
        value={moon >= 1 ? `${moon.toFixed(1)}×` : `${Math.round(moon * 100)}%`}
        label={
          moon >= 1 ? "the distance to the Moon" : "of the way to the Moon"
        }
        detail={`The Moon is ${km(MOON_DISTANCE_KM)} away`}
      >
        <div
          className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted"
          role="meter"
          aria-label="Share of the distance to the Moon"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.min(100, Math.round(moon * 100))}
        >
          <div
            className="h-full rounded-full bg-chart-2"
            style={{ width: `${Math.min(100, moon * 100)}%` }}
          />
        </div>
      </Fact>
      <Fact
        value={String(breakdown.countries)}
        label={breakdown.countries === 1 ? "country" : "countries"}
        detail="flown to or from"
      />
      <Fact
        value={longest ? km(longest.distance) : "–"}
        label="longest flight"
        detail={longest ? `${longest.from} – ${longest.to}` : undefined}
      />
    </div>
  )
}

// Maps a month's flights onto the colour steps. While the busiest month fits
// the scale, each extra flight is one step, so 1 and 2 flights look different;
// beyond that the steps are relative to the busiest month.
function heatStep(flights: number, busiest: number): number {
  const top = HEAT_STEPS.length - 1
  if (flights === 0) return 0
  if (busiest <= top) return flights
  return Math.max(1, Math.ceil((flights / busiest) * top))
}

// A year-by-month grid: each cell is a month, brighter the more you flew.
function Calendar({ breakdown }: { breakdown: Breakdown }) {
  const years = breakdown.years ?? []
  const months = breakdown.months ?? []
  const byMonth = new Map(
    months.map((m) => [`${m.year}-${m.month}`, m.flights])
  )
  const busiest = Math.max(1, ...months.map((m) => m.flights))

  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
        <CardTitle className="text-base">When you flew</CardTitle>
        <div
          className="flex items-center gap-1.5 text-xs text-muted-foreground"
          aria-hidden="true"
        >
          Fewer
          {HEAT_STEPS.map((step) => (
            <span key={step} className={`size-3 rounded-[3px] ${step}`} />
          ))}
          More
        </div>
      </CardHeader>
      {/* Ten year rows fit; older years scroll under the month names. */}
      <CardContent className="flex max-h-[19.75rem] flex-col gap-0.5 overflow-y-auto pb-0.5">
        <div className="sticky top-0 z-10 flex items-center gap-0.5 bg-card text-[10px] text-muted-foreground">
          <span className="w-10 shrink-0" />
          {MONTHS.map((month) => (
            <span key={month} className="min-w-0 flex-1 text-center">
              <span className="sm:hidden">{month[0]}</span>
              <span className="hidden sm:inline">{month.slice(0, 3)}</span>
            </span>
          ))}
          <span className="w-8 shrink-0 sm:w-44" />
        </div>
        {years.map((year) => (
          <div key={year.year} className="flex items-center gap-0.5">
            <span className="w-10 shrink-0 text-sm font-medium">
              {year.year}
            </span>
            {MONTHS.map((month, index) => {
              const flights = byMonth.get(`${year.year}-${index + 1}`) ?? 0
              const label = `${month} ${year.year}: ${
                flights === 0 ? "no flights" : flightCount(flights)
              }`
              return (
                <Tooltip key={month}>
                  <TooltipTrigger asChild>
                    <div
                      role="img"
                      tabIndex={0}
                      aria-label={label}
                      className={`h-7 min-w-0 flex-1 rounded-[4px] outline-none hover:ring-2 hover:ring-foreground/60 focus-visible:ring-2 focus-visible:ring-ring ${HEAT_STEPS[heatStep(flights, busiest)]}`}
                    />
                  </TooltipTrigger>
                  <TooltipContent>
                    <span className="font-medium">
                      {flights === 0 ? "No flights" : flightCount(flights)}
                    </span>
                    <span className="block text-xs opacity-80">
                      {month} {year.year}
                    </span>
                  </TooltipContent>
                </Tooltip>
              )
            })}
            <span className="w-8 shrink-0 text-right text-xs text-muted-foreground sm:w-44">
              <span className="sm:hidden">{year.flights}</span>
              <span className="hidden sm:inline">
                {flightCount(year.flights)} · {km(year.distance)}
              </span>
            </span>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

// The routes flown most, drawn like the route line on a boarding pass.
function Routes({ breakdown }: { breakdown: Breakdown }) {
  const routes = breakdown.routes ?? []

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Top routes</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {routes.map((route) => (
          <div
            key={`${route.from}-${route.to}`}
            className="flex items-center gap-3"
          >
            <span className="w-12 text-lg font-semibold">{route.from}</span>
            <span className="flex flex-1 items-center gap-2" aria-hidden="true">
              <span className="h-px flex-1 border-t border-dashed border-muted-foreground/50" />
              <AirplaneIcon className="size-4 text-chart-2" weight="fill" />
              <span className="h-px flex-1 border-t border-dashed border-muted-foreground/50" />
            </span>
            <span className="w-12 text-right text-lg font-semibold">
              {route.to}
            </span>
            <span className="w-20 text-right text-sm text-muted-foreground">
              {flightCount(route.flights)}
            </span>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

// The airlines flown most, as a numbered leaderboard.
function Airlines({ breakdown }: { breakdown: Breakdown }) {
  const airlines = breakdown.airlines ?? []

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Top airlines</CardTitle>
      </CardHeader>
      <CardContent>
        <ol className="flex flex-col gap-3">
          {airlines.map((airline, index) => (
            <li key={airline.name} className="flex items-center gap-3">
              <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-semibold">
                {index + 1}
              </span>
              <span className="min-w-0 flex-1 truncate text-sm font-medium">
                {airline.name}
              </span>
              <span className="shrink-0 text-sm text-muted-foreground">
                {flightCount(airline.flights)}
              </span>
            </li>
          ))}
        </ol>
      </CardContent>
    </Card>
  )
}

// A closer look at the flights flown: a few numbers to picture the distance,
// a calendar of when the flying happened, and the favourite routes and airlines.
export function FlightBreakdown({
  breakdown,
  distance,
}: {
  breakdown: Breakdown
  distance: number
}) {
  return (
    <div className="flex flex-col gap-4">
      <h2 className="text-lg font-semibold">A closer look</h2>
      <Facts breakdown={breakdown} distance={distance} />
      <Calendar breakdown={breakdown} />
      <div className="grid gap-4 lg:grid-cols-2">
        <Routes breakdown={breakdown} />
        <Airlines breakdown={breakdown} />
      </div>
    </div>
  )
}
