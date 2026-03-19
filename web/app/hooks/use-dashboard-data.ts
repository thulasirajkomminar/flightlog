import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { ApiError } from "~/lib/api"

import type { Flight, FlightStats } from "~/types"

import { flightService } from "~/services"

export function useDashboardData() {
  const [flights, setFlights] = useState<Flight[]>([])
  const [stats, setStats] = useState<FlightStats | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const loadData = useCallback(async () => {
    try {
      const [flightData, statsData] = await Promise.all([
        flightService.list(1000, 0, undefined, "landed"),
        flightService.stats(),
      ])
      setFlights(flightData.flights || [])
      setStats(statsData)
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message : "Failed to load dashboard data"
      toast.error(message)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    loadData()
  }, [loadData])

  return { flights, stats, isLoading }
}
