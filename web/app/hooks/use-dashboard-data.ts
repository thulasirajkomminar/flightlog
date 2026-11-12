import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { ApiError } from "~/lib/api"

import type { FlightRoute, FlightStats } from "~/types"

import { flightService } from "~/services"

export function useDashboardData() {
  const [routes, setRoutes] = useState<FlightRoute[]>([])
  const [stats, setStats] = useState<FlightStats | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const loadData = useCallback(async () => {
    try {
      const [routeData, statsData] = await Promise.all([
        flightService.routes(),
        flightService.stats(),
      ])
      setRoutes(routeData)
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

  return { routes, stats, isLoading }
}
