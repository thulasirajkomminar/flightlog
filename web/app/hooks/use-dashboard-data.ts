import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { ApiError } from "~/lib/api"

import type { FlightBreakdown, FlightRoute, FlightStats } from "~/types"

import { flightService } from "~/services"

export function useDashboardData() {
  const [routes, setRoutes] = useState<FlightRoute[]>([])
  const [stats, setStats] = useState<FlightStats | null>(null)
  const [breakdown, setBreakdown] = useState<FlightBreakdown | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const loadData = useCallback(async () => {
    try {
      const [routeData, statsData, breakdownData] = await Promise.all([
        flightService.routes(),
        flightService.stats(),
        flightService.breakdown(),
      ])
      setRoutes(routeData)
      setBreakdown(breakdownData)
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

  return { routes, stats, breakdown, isLoading }
}
