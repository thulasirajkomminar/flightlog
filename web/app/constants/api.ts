export const API_ENDPOINTS = {
  AUTH: {
    REGISTER: "/api/v1/auth/register",
    LOGIN: "/api/v1/auth/login",
    LOGOUT: "/api/v1/auth/logout",
    ME: "/api/v1/auth/me",
  },
  FLIGHTS: {
    LIST: "/api/v1/flights",
    SEARCH: "/api/v1/flights/search",
    STATS: "/api/v1/flights/stats",
    ROUTES: "/api/v1/flights/routes",
    EXPORT: "/api/v1/flights/export",
    BY_ID: (id: string) => `/api/v1/flights/${id}`,
    ADD: (id: string) => `/api/v1/flights/${id}/add`,
  },
  IMPORT: {
    SOURCES: "/api/v1/import/sources",
    PREVIEW: "/api/v1/import/preview",
    IMPORT: "/api/v1/import",
  },
} as const
