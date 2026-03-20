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
  provider: string
  createdAt: string
  updatedAt: string
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

export interface TimeInfo {
  utc?: string
  local?: string
}

export interface FlightListResponse {
  flights: Flight[]
  count: number
  total: number
  years: number[]
}

export interface FlightStats {
  flights: number
  distance: number
  flightTime: number
  airports: number
  airlines: number
}

export interface ImportPreview {
  total: number
  enrichable: number
}

export interface ImportResult {
  total: number
  imported: number
  skipped: number
  failed: number
  errors?: { flightNumber: string; date: string; reason: string }[]
}
