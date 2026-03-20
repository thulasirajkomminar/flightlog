import { lazy, Suspense } from "react"
import {
  Airplane,
  AirplaneTakeoff,
  Buildings,
  Clock,
  Path,
} from "@phosphor-icons/react"

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
} from "~/components/ui/breadcrumb"
import { Card, CardContent } from "~/components/ui/card"
import { Separator } from "~/components/ui/separator"
import { SidebarTrigger } from "~/components/ui/sidebar"
import { useDashboardData } from "~/hooks/use-dashboard-data"

const FlightMap = lazy(() => import("~/components/flight-map"))

function formatDuration(hours: number): string {
  const totalMinutes = Math.round(hours * 60)
  const m = totalMinutes % 60
  const totalHours = Math.floor(totalMinutes / 60)
  const h = totalHours % 24
  const totalDays = Math.floor(totalHours / 24)
  const d = totalDays % 30
  const totalMonths = Math.floor(totalDays / 30)
  const mo = totalMonths % 12
  const y = Math.floor(totalMonths / 12)

  const parts: string[] = []
  if (y > 0) parts.push(`${y}y`)
  if (mo > 0) parts.push(`${mo}mo`)
  if (d > 0) parts.push(`${d}d`)
  if (h > 0) parts.push(`${h}h`)
  if (m > 0) parts.push(`${m}m`)

  return parts.join(" ") || "0m"
}

export default function DashboardPage() {
  const { flights, stats, isLoading } = useDashboardData()

  const metrics = stats
    ? [
        {
          label: "Flights",
          value: stats.flights,
          icon: Airplane,
        },
        {
          label: "Distance",
          value: `${Math.round(stats.distance).toLocaleString()} km`,
          icon: Path,
        },
        {
          label: "Flight Time",
          value: formatDuration(stats.flightTime),
          icon: Clock,
        },
        {
          label: "Airports",
          value: stats.airports,
          icon: Buildings,
        },
        {
          label: "Airlines",
          value: stats.airlines,
          icon: AirplaneTakeoff,
        },
      ]
    : []

  return (
    <>
      <header className="flex h-16 shrink-0 items-center gap-2 border-b px-6">
        <SidebarTrigger className="-ml-1" />
        <Separator orientation="vertical" className="mr-2 h-4" />
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbPage className="text-base">Dashboard</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      </header>

      <div className="flex flex-1 flex-col gap-6 p-6 md:gap-8 md:p-8">
        <Card className="gap-0 overflow-hidden py-0">
          <CardContent className="p-0">
            <div className="h-[500px] w-full">
              {isLoading ? (
                <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                  Loading map...
                </div>
              ) : (
                <Suspense
                  fallback={
                    <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                      Loading map...
                    </div>
                  }
                >
                  <FlightMap flights={flights} />
                </Suspense>
              )}
            </div>
          </CardContent>
        </Card>

        <div>
          <h2 className="mb-4 text-lg font-semibold">Flightlog Passport</h2>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5">
            {isLoading
              ? Array.from({ length: 5 }).map((_, i) => (
                  <Card key={i}>
                    <CardContent className="flex flex-col items-center gap-2 py-6">
                      <div className="size-8 animate-pulse rounded bg-muted" />
                      <div className="h-4 w-16 animate-pulse rounded bg-muted" />
                      <div className="h-6 w-12 animate-pulse rounded bg-muted" />
                    </CardContent>
                  </Card>
                ))
              : metrics.map((metric) => (
                  <Card key={metric.label}>
                    <CardContent className="flex flex-col items-center gap-2 py-6">
                      <metric.icon
                        className="size-8 text-primary"
                        weight="duotone"
                      />
                      <span className="text-sm text-muted-foreground">
                        {metric.label}
                      </span>
                      <span className="text-2xl font-bold">{metric.value}</span>
                    </CardContent>
                  </Card>
                ))}
          </div>
        </div>
      </div>
    </>
  )
}
