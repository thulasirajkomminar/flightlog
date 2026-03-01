import { MagnifyingGlass } from "@phosphor-icons/react"

import { FlightCard } from "~/components/flight-card"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
} from "~/components/ui/breadcrumb"
import { Button } from "~/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card"
import { Input } from "~/components/ui/input"
import { ItemGroup } from "~/components/ui/item"
import { Label } from "~/components/ui/label"
import { Separator } from "~/components/ui/separator"
import { SidebarTrigger } from "~/components/ui/sidebar"
import { useFlightSearch } from "~/hooks/use-flight-search"

export default function SearchPage() {
  const {
    results,
    flightNumber,
    setFlightNumber,
    flightDate,
    setFlightDate,
    isSearching,
    search,
    addFlight,
    addingId,
    addedIds,
  } = useFlightSearch()

  return (
    <>
      <header className="flex h-16 shrink-0 items-center gap-2 border-b px-6">
        <SidebarTrigger className="-ml-1" />
        <Separator orientation="vertical" className="mr-2 h-4" />
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbPage className="text-base">
                Search & Add
              </BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      </header>

      <div className="flex flex-1 flex-col gap-6 p-6 md:gap-8 md:p-8">
        <Card>
          <CardHeader>
            <CardTitle className="text-xl">Search Flight</CardTitle>
            <CardDescription className="text-sm">
              Enter the flight number from your boarding pass and date to find
              and save flights
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form
              onSubmit={search}
              className="flex flex-col gap-5 sm:flex-row sm:items-end"
            >
              <div className="flex flex-1 flex-col gap-2">
                <Label htmlFor="flightNumber" className="text-sm">
                  Flight Number
                </Label>
                <Input
                  id="flightNumber"
                  placeholder="e.g. UA123, KL1024, U2789"
                  value={flightNumber}
                  onChange={(e) => setFlightNumber(e.target.value)}
                  required
                  className="h-10 text-sm"
                />
              </div>
              <div className="flex flex-1 flex-col gap-2">
                <Label htmlFor="flightDate" className="text-sm">
                  Departure / Arrival Date
                </Label>
                <Input
                  id="flightDate"
                  type="date"
                  value={flightDate}
                  onChange={(e) => setFlightDate(e.target.value)}
                  required
                  className="h-10 text-sm [&::-webkit-calendar-picker-indicator]:invert"
                />
              </div>
              <Button type="submit" disabled={isSearching} size="lg">
                <MagnifyingGlass className="size-4" />
                {isSearching ? "Searching..." : "Search"}
              </Button>
            </form>
          </CardContent>
        </Card>

        {results.length > 0 && (
          <div className="flex flex-col gap-4">
            <h3 className="text-base font-medium">Search Results</h3>
            <ItemGroup>
              {results.map((flight, index) => (
                <FlightCard
                  key={flight.id || index}
                  flight={flight}
                  onAdd={addFlight}
                  isAdding={addingId === flight.id}
                  isAdded={addedIds.has(flight.id)}
                />
              ))}
            </ItemGroup>
          </div>
        )}
      </div>
    </>
  )
}
