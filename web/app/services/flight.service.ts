import { api } from "~/lib/api"
import { API_ENDPOINTS } from "~/constants"

import type {
  Flight,
  FlightBreakdown,
  FlightListResponse,
  FlightRoute,
  FlightStats,
  ManualFlightInput,
  ProviderQuota,
} from "~/types"

export const flightService = {
  async list(
    limit = 20,
    offset = 0,
    year?: number,
    status?: string,
    query?: string
  ): Promise<FlightListResponse> {
    const params = new URLSearchParams({
      limit: String(limit),
      offset: String(offset),
    })
    if (year) {
      params.set("year", String(year))
    }
    if (status) {
      params.set("status", status)
    }
    if (query) {
      params.set("q", query)
    }
    return api.get<FlightListResponse>(
      `${API_ENDPOINTS.FLIGHTS.LIST}?${params}`
    )
  },

  async stats(): Promise<FlightStats> {
    return api.get<FlightStats>(API_ENDPOINTS.FLIGHTS.STATS)
  },

  async breakdown(): Promise<FlightBreakdown> {
    return api.get<FlightBreakdown>(API_ENDPOINTS.FLIGHTS.BREAKDOWN)
  },

  async quota(): Promise<ProviderQuota> {
    return api.get<ProviderQuota>(API_ENDPOINTS.PROVIDER.QUOTA)
  },

  async routes(): Promise<FlightRoute[]> {
    const data = await api.get<{ routes: FlightRoute[] | null }>(
      API_ENDPOINTS.FLIGHTS.ROUTES
    )
    return data.routes ?? []
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

  // Adds a flight typed in by hand to the user's collection.
  async createManual(input: ManualFlightInput): Promise<Flight> {
    return api.post<Flight>(API_ENDPOINTS.FLIGHTS.LIST, input)
  },

  // Changes a flight the user imported or added by hand.
  async updateManual(id: string, input: ManualFlightInput): Promise<Flight> {
    return api.put<Flight>(API_ENDPOINTS.FLIGHTS.BY_ID(id), input)
  },

  async remove(id: string): Promise<void> {
    return api.delete(API_ENDPOINTS.FLIGHTS.BY_ID(id))
  },

  async exportFlights(): Promise<void> {
    const blob = await api.download(API_ENDPOINTS.FLIGHTS.EXPORT)
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = "flightlog-export.csv"
    a.click()
    URL.revokeObjectURL(url)
  },
}
