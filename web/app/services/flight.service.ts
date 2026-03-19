import { api } from "~/lib/api"
import { API_ENDPOINTS } from "~/constants"

import type { Flight, FlightListResponse, FlightStats } from "~/types"

export const flightService = {
  async list(
    limit = 20,
    offset = 0,
    year?: number
  ): Promise<FlightListResponse> {
    const params = new URLSearchParams({
      limit: String(limit),
      offset: String(offset),
    })
    if (year) {
      params.set("year", String(year))
    }
    return api.get<FlightListResponse>(
      `${API_ENDPOINTS.FLIGHTS.LIST}?${params}`
    )
  },

  async stats(): Promise<FlightStats> {
    return api.get<FlightStats>(API_ENDPOINTS.FLIGHTS.STATS)
  },

  async search(
    flightNumber: string,
    date?: string
  ): Promise<FlightListResponse> {
    const params = new URLSearchParams({ flight_number: flightNumber })
    if (date) {
      params.set("date", date)
    }
    return api.get<FlightListResponse>(
      `${API_ENDPOINTS.FLIGHTS.SEARCH}?${params}`
    )
  },

  async getById(id: string): Promise<Flight> {
    return api.get<Flight>(API_ENDPOINTS.FLIGHTS.BY_ID(id))
  },

  async addFlight(id: string): Promise<void> {
    return api.post(API_ENDPOINTS.FLIGHTS.ADD(id))
  },

  async remove(id: string): Promise<void> {
    return api.delete(API_ENDPOINTS.FLIGHTS.BY_ID(id))
  },

  async exportFlights(): Promise<void> {
    const response = await fetch(API_ENDPOINTS.FLIGHTS.EXPORT)
    if (!response.ok) {
      throw new Error("Failed to export flights")
    }
    const blob = await response.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = "flightlog-export.csv"
    a.click()
    URL.revokeObjectURL(url)
  },
}
