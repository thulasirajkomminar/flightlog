import {
  Airplane,
  CaretLeft,
  CaretRight,
  Export,
  MagnifyingGlass,
} from "@phosphor-icons/react"
import { toast } from "sonner"

import { FlightCard } from "~/components/flight-card"
import { ImportDialog } from "~/components/import-dialog"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
} from "~/components/ui/breadcrumb"
import { Button } from "~/components/ui/button"
import { Card, CardContent } from "~/components/ui/card"
import { Input } from "~/components/ui/input"
import { ItemGroup } from "~/components/ui/item"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select"
import { Separator } from "~/components/ui/separator"
import { SidebarTrigger } from "~/components/ui/sidebar"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs"
import { useFlightList } from "~/hooks/use-flight-list"

import type { Flight } from "~/types"

import { flightService } from "~/services"

const ALL_STATUSES = "all"

const STATUS_FILTERS: Record<string, string> = {
  [ALL_STATUSES]: "All statuses",
  landed: "Landed",
  scheduled: "Scheduled",
  active: "In the air",
  cancelled: "Cancelled",
  diverted: "Diverted",
}

function EmptyState({ message }: { message: string }) {
  return (
    <Card>
      <CardContent className="py-16 text-center">
        <Airplane className="mx-auto size-12 text-muted-foreground" />
        <p className="mt-4 text-base text-muted-foreground">{message}</p>
      </CardContent>
    </Card>
  )
}

interface FlightRowsProps {
  flights: Flight[]
  isLoading: boolean
  emptyMessage: string
  page: number
  totalPages: number
  onPage: (page: number) => void
  onDelete: (id: string) => void
  onEdited: () => void
}

// One page of flights with its pager, shared by the year tabs and the search.
function FlightRows({
  flights,
  isLoading,
  emptyMessage,
  page,
  totalPages,
  onPage,
  onDelete,
  onEdited,
}: FlightRowsProps) {
  if (isLoading) {
    return <p className="text-sm text-muted-foreground">Loading flights...</p>
  }

  if (flights.length === 0) {
    return <EmptyState message={emptyMessage} />
  }

  return (
    <>
      <ItemGroup>
        {flights.map((flight) => (
          <FlightCard
            key={flight.id}
            flight={flight}
            onDelete={onDelete}
            onEdited={onEdited}
          />
        ))}
      </ItemGroup>

      {totalPages > 1 && (
        <div className="mt-6 flex items-center justify-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => onPage(page - 1)}
          >
            <CaretLeft className="size-4" />
            <span className="hidden sm:inline">Previous</span>
          </Button>
          <span className="text-sm text-muted-foreground">
            Page {page} of {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => onPage(page + 1)}
          >
            <span className="hidden sm:inline">Next</span>
            <CaretRight className="size-4" />
          </Button>
        </div>
      )}
    </>
  )
}

export default function FlightsPage() {
  const {
    flights,
    years,
    selectedYear,
    isLoading,
    page,
    total,
    totalPages,
    query,
    status,
    isSearching,
    setPage,
    setQuery,
    changeYear,
    changeStatus,
    deleteFlight,
    refresh,
  } = useFlightList()

  const hasFlights = years.length > 0
  const rows = {
    flights,
    isLoading,
    page,
    totalPages,
    onPage: setPage,
    onDelete: deleteFlight,
    onEdited: refresh,
  }

  return (
    <>
      <header className="flex h-16 shrink-0 items-center gap-2 border-b px-6">
        <SidebarTrigger className="-ml-1" />
        <Separator orientation="vertical" className="mr-2 h-4" />
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbPage className="text-base">My Flights</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      </header>

      <div className="flex flex-1 flex-col gap-2 p-4 md:gap-2 md:p-6">
        <div className="flex flex-wrap items-center justify-end gap-2">
          {hasFlights && (
            <div className="mr-auto flex flex-wrap items-center gap-2">
              <div className="relative">
                <MagnifyingGlass className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  type="search"
                  aria-label="Search flights"
                  placeholder="Flight, airline, airport or city"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  className="h-10 w-64 pl-9 text-sm"
                />
              </div>
              <Select
                value={status || ALL_STATUSES}
                onValueChange={(v) => changeStatus(v === ALL_STATUSES ? "" : v)}
              >
                <SelectTrigger className="h-10 w-40" aria-label="Status">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Object.entries(STATUS_FILTERS).map(([value, label]) => (
                    <SelectItem key={value} value={value}>
                      {label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          <ImportDialog onImported={refresh} />
          <Button
            size="lg"
            disabled={!hasFlights}
            onClick={async () => {
              try {
                await flightService.exportFlights()
                toast.success("Flights exported")
              } catch {
                toast.error("Failed to export flights")
              }
            }}
          >
            <Export className="size-4" />
            Export
          </Button>
        </div>

        {!hasFlights && !isLoading && (
          <EmptyState message="No flights yet. Search for a flight to get started." />
        )}

        {hasFlights && isSearching && (
          <>
            {!isLoading && (
              <p className="text-sm text-muted-foreground">
                {total} {total === 1 ? "flight matches" : "flights match"} in
                all years
              </p>
            )}
            <FlightRows
              {...rows}
              emptyMessage="No flights match your search."
            />
          </>
        )}

        {hasFlights && !isSearching && (
          <Tabs
            value={selectedYear}
            onValueChange={changeYear}
            className="w-full"
          >
            <TabsList className="mb-2 w-full flex-wrap">
              {years.map((year) => (
                <TabsTrigger
                  key={year}
                  value={String(year)}
                  className="data-active:bg-primary! data-active:text-primary-foreground!"
                >
                  {year}
                </TabsTrigger>
              ))}
            </TabsList>

            {years.map((year) => (
              <TabsContent key={year} value={String(year)}>
                <FlightRows
                  {...rows}
                  emptyMessage={
                    status
                      ? `No ${STATUS_FILTERS[status].toLowerCase()} flights in ${year}.`
                      : `No flights in ${year}.`
                  }
                />
              </TabsContent>
            ))}
          </Tabs>
        )}
      </div>
    </>
  )
}
