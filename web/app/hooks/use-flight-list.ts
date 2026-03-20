import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { ApiError } from "~/lib/api"

import type { Flight } from "~/types"

import { flightService } from "~/services"

const PAGE_SIZE = 20

export function useFlightList() {
  const [flights, setFlights] = useState<Flight[]>([])
  const [years, setYears] = useState<number[]>([])
  const [selectedYear, setSelectedYear] = useState<string>("")
  const [isLoading, setIsLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)

  const totalPages = Math.ceil(total / PAGE_SIZE)

  const loadFlights = useCallback(async () => {
    setIsLoading(true)
    try {
      const offset = (page - 1) * PAGE_SIZE
      const year = selectedYear ? Number(selectedYear) : undefined
      const data = await flightService.list(PAGE_SIZE, offset, year)
      setFlights(data.flights || [])
      setTotal(data.total)
      if (data.years?.length > 0) {
        setYears(data.years)
        if (!selectedYear) {
          setSelectedYear(String(data.years[0]))
        }
      }
    } catch (err) {
      const message =
        err instanceof ApiError
          ? err.message
          : "Failed to load flights. Please try again."
      toast.error(message)
    } finally {
      setIsLoading(false)
    }
  }, [selectedYear, page])

  useEffect(() => {
    loadFlights()
  }, [loadFlights])

  const changeYear = useCallback((year: string) => {
    setSelectedYear(year)
    setPage(1)
  }, [])

  const deleteFlight = useCallback(async (id: string) => {
    try {
      await flightService.remove(id)
      toast.success("Flight deleted")
      setFlights((prev) => prev.filter((f) => f.id !== id))
      setTotal((prev) => prev - 1)
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message : "Failed to delete flight"
      toast.error(message)
    }
  }, [])

  return {
    flights,
    years,
    selectedYear,
    isLoading,
    page,
    total,
    totalPages,
    setPage,
    changeYear,
    deleteFlight,
    refresh: loadFlights,
  }
}
