export interface User {
  id: string
  email: string
  name: string
  createdAt: string
  updatedAt: string
}

export interface AuthResponse {
  token: string
  user: User
}

export interface Flight {
  id: string
  userId: string
  number: string
  flightDate: string
  status: string
  callSign?: string
  codeshareStatus?: string
  isCargo: boolean
  airline: {
    name: string
    iata?: string
    icao?: string
  }
  aircraft: {
    reg?: string
    modeS?: string
    model?: string
  }
  departure: Movement
  arrival: Movement
  greatCircleDistance: {
    meter: number
    km: number
    mile: number
    nm: number
    feet: number
  }
  lastUpdatedUtc?: string
  // True for flights the signed-in user imported or added by hand.
  editable?: boolean
  provider: string
  createdAt: string
  updatedAt: string
}

// What a user enters to add or change a flight by hand.
export interface ManualFlightInput {
  number: string
  date: string
  departureIata: string
  arrivalIata: string
  airlineName: string
  aircraftModel: string
  status: string
}

export interface Movement {
  airport: {
    icao?: string
    iata: string
    name: string
    shortName?: string
    municipalityName?: string
    location: { lat: number; lon: number }
    countryCode?: string
    timeZone?: string
  }
  scheduledTime: TimeInfo
  revisedTime: TimeInfo
  runwayTime: TimeInfo
  terminal?: string
  gate?: string
  checkInDesk?: string
  baggageBelt?: string
}

interface TimeInfo {
  utc?: string
  local?: string
}

export interface FlightListResponse {
  flights: Flight[]
  count: number
  total: number
  years: number[]
}

export interface RouteAirport {
  iata: string
  name: string
  location: { lat: number; lon: number }
}

export interface FlightRoute {
  departure: RouteAirport
  arrival: RouteAirport
  flights: number
}

export interface FlightStats {
  flights: number
  distance: number
  flightTime: number
  airports: number
  airlines: number
}

export interface FlightBreakdown {
  years: { year: number; flights: number; distance: number }[] | null
  months: { year: number; month: number; flights: number }[] | null
  longest: { from: string; to: string; distance: number } | null
  routes: { from: string; to: string; flights: number }[] | null
  airlines: { name: string; flights: number }[] | null
  countries: number
}

export interface ImportPreview {
  total: number
  enrichable: number
}

// The flight data provider's API quota, as it last reported it.
export interface ProviderQuota {
  known: boolean
  limit: number
  remaining: number
  pausedUntil?: string
}

export interface ImportJob {
  state: "running" | "completed"
  total: number
  processed: number
  result?: ImportResult
}

export interface ImportResult {
  total: number
  imported: number
  skipped: number
  failed: number
  // True when the user stopped the import before it finished.
  cancelled?: boolean
  errors?: { flightNumber: string; date: string; reason: string }[]
}
