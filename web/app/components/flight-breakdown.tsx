import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card"

import type { FlightBreakdown as Breakdown } from "~/types"

interface Row {
  label: string
  flights: number
  detail?: string
}

// A ranked list where each row's bar shows its share of the largest count.
function RankedList({ title, rows }: { title: string; rows: Row[] }) {
  const most = Math.max(1, ...rows.map((row) => row.flights))

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{title}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {rows.length === 0 && (
          <p className="text-sm text-muted-foreground">Nothing yet.</p>
        )}
        {rows.map((row) => (
          <div key={row.label} className="flex flex-col gap-1">
            <div className="flex items-baseline justify-between gap-3 text-sm">
              <span className="truncate font-medium">{row.label}</span>
              <span className="shrink-0 text-muted-foreground">
                {row.flights} {row.flights === 1 ? "flight" : "flights"}
                {row.detail ? ` · ${row.detail}` : ""}
              </span>
            </div>
            <div className="h-1.5 overflow-hidden rounded-full bg-muted">
              <div
                className="h-full rounded-full bg-primary"
                style={{ width: `${(row.flights / most) * 100}%` }}
              />
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

// A closer look at the flights flown: per year, top routes and top airlines.
export function FlightBreakdown({ breakdown }: { breakdown: Breakdown }) {
  const years = (breakdown.years ?? []).map((y) => ({
    label: String(y.year),
    flights: y.flights,
    detail: `${Math.round(y.distance).toLocaleString()} km`,
  }))
  const routes = (breakdown.routes ?? []).map((r) => ({
    label: `${r.from} – ${r.to}`,
    flights: r.flights,
  }))
  const airlines = (breakdown.airlines ?? []).map((a) => ({
    label: a.name,
    flights: a.flights,
  }))

  return (
    <div>
      <div className="mb-4 flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="text-lg font-semibold">A closer look</h2>
        <p className="text-sm text-muted-foreground">
          {breakdown.countries}{" "}
          {breakdown.countries === 1 ? "country" : "countries"} visited
        </p>
      </div>
      <div className="grid gap-4 lg:grid-cols-3">
        <RankedList title="By year" rows={years} />
        <RankedList title="Top routes" rows={routes} />
        <RankedList title="Top airlines" rows={airlines} />
      </div>
    </div>
  )
}
