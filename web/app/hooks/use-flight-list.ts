import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { ApiError } from "~/lib/api"

import type { Flight } from "~/types"

import { flightService } from "~/services"

const PAGE_SIZE = 20
// Wait for a pause in typing before asking the server.
const SEARCH_DELAY_MS = 300

export function useFlightList() {
  const [flights, setFlights] = useState<Flight[]>([])
  const [years, setYears] = useState<number[]>([])
  const [selectedYear, setSelectedYear] = useState<string>("")
  const [isLoading, setIsLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  // `query` follows the input; `search` is what the server was asked for.
  const [query, setQuery] = useState("")
  const [search, setSearch] = useState("")
  const [status, setStatus] = useState("")

  const totalPages = Math.ceil(total / PAGE_SIZE)
  // A text search looks through every year, so the year tabs step aside.
  const isSearching = search !== ""

  useEffect(() => {
    const timer = setTimeout(() => {
      setSearch(query.trim())
      setPage(1)
    }, SEARCH_DELAY_MS)
    return () => clearTimeout(timer)
  }, [query])

  const loadFlights = useCallback(async () => {
    setIsLoading(true)
    try {
      const offset = (page - 1) * PAGE_SIZE
      const year = !search && selectedYear ? Number(selectedYear) : undefined
      const data = await flightService.list(
        PAGE_SIZE,
        offset,
        year,
        status || undefined,
        search || undefined
      )
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
  }, [selectedYear, page, search, status])

  useEffect(() => {
    loadFlights()
  }, [loadFlights])

  const changeYear = useCallback((year: string) => {
    setSelectedYear(year)
    setPage(1)
  }, [])

  const changeStatus = useCallback((next: string) => {
    setStatus(next)
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
    query,
    status,
    isSearching,
    setPage,
    setQuery,
    changeYear,
    changeStatus,
    deleteFlight,
    refresh: loadFlights,
  }
}
