import { Airplane, CaretLeft, CaretRight } from "@phosphor-icons/react"

import { FlightCard } from "~/components/flight-card"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
} from "~/components/ui/breadcrumb"
import { Button } from "~/components/ui/button"
import { Card, CardContent } from "~/components/ui/card"
import { ItemGroup } from "~/components/ui/item"
import { Separator } from "~/components/ui/separator"
import { SidebarTrigger } from "~/components/ui/sidebar"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs"
import { useFlightList } from "~/hooks/use-flight-list"

export default function FlightsPage() {
  const {
    flights,
    years,
    selectedYear,
    isLoading,
    page,
    totalPages,
    setPage,
    changeYear,
    deleteFlight,
  } = useFlightList()

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

      <div className="flex flex-1 flex-col gap-6 p-6 md:gap-8 md:p-8">
        {years.length === 0 && !isLoading ? (
          <Card>
            <CardContent className="py-16 text-center">
              <Airplane className="mx-auto size-12 text-muted-foreground" />
              <p className="mt-4 text-base text-muted-foreground">
                No flights yet. Search for a flight to get started.
              </p>
            </CardContent>
          </Card>
        ) : (
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
                {isLoading ? (
                  <p className="text-sm text-muted-foreground">
                    Loading flights...
                  </p>
                ) : flights.length === 0 ? (
                  <Card>
                    <CardContent className="py-16 text-center">
                      <Airplane className="mx-auto size-12 text-muted-foreground" />
                      <p className="mt-4 text-base text-muted-foreground">
                        No flights in {year}.
                      </p>
                    </CardContent>
                  </Card>
                ) : (
                  <>
                    <ItemGroup>
                      {flights.map((flight) => (
                        <FlightCard
                          key={flight.id}
                          flight={flight}
                          onDelete={deleteFlight}
                        />
                      ))}
                    </ItemGroup>

                    {totalPages > 1 && (
                      <div className="mt-6 flex items-center justify-center gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={page <= 1}
                          onClick={() => setPage((p) => p - 1)}
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
                          onClick={() => setPage((p) => p + 1)}
                        >
                          <span className="hidden sm:inline">Next</span>
                          <CaretRight className="size-4" />
                        </Button>
                      </div>
                    )}
                  </>
                )}
              </TabsContent>
            ))}
          </Tabs>
        )}
      </div>
    </>
  )
}
