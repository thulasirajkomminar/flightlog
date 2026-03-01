import { useCallback, useState } from "react"
import { toast } from "sonner"

import { ApiError } from "~/lib/api"

import type { Flight } from "~/types"

import { flightService } from "~/services"

export function useFlightSearch() {
  const [results, setResults] = useState<Flight[]>([])
  const [flightNumber, setFlightNumber] = useState("")
  const [flightDate, setFlightDate] = useState("")
  const [isSearching, setIsSearching] = useState(false)
  const [addingId, setAddingId] = useState<string | null>(null)
  const [addedIds, setAddedIds] = useState<Set<string>>(new Set())

  const search = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault()
      if (!flightNumber.trim() || !flightDate) return

      setIsSearching(true)
      setResults([])
      setAddedIds(new Set())
      try {
        const data = await flightService.search(flightNumber.trim(), flightDate)
        setResults(data.flights || [])
        if (!data.flights?.length) {
          toast.info("No flights found for this search.")
        } else {
          toast.success(`Found ${data.count} flight(s)`)
        }
      } catch (err) {
        if (err instanceof ApiError) {
          toast.error(err.message)
        } else {
          toast.error("Search failed. Please try again.")
        }
      } finally {
        setIsSearching(false)
      }
    },
    [flightNumber, flightDate]
  )

  const addFlight = useCallback(async (flight: Flight) => {
    try {
      setAddingId(flight.id)
      await flightService.addFlight(flight.id)
      toast.success(`Flight ${flight.number} added`)
      setAddedIds((prev) => new Set(prev).add(flight.id))
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.statusCode === 409) {
          toast.info(`Flight ${flight.number} is already in your collection`)
        } else {
          toast.error(err.message)
        }
      } else {
        toast.error("Failed to add flight")
      }
    } finally {
      setAddingId(null)
    }
  }, [])

  return {
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
  }
}
