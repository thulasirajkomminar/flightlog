export const API_ENDPOINTS = {
  AUTH: {
    REGISTER: "/api/v1/auth/register",
    LOGIN: "/api/v1/auth/login",
    LOGOUT: "/api/v1/auth/logout",
    ME: "/api/v1/auth/me",
    PASSWORD: "/api/v1/auth/password",
    CONFIG: "/api/v1/auth/config",
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
  PROVIDER: {
    QUOTA: "/api/v1/provider/quota",
  },
  IMPORT: {
    SOURCES: "/api/v1/import/sources",
    PREVIEW: "/api/v1/import/preview",
    IMPORT: "/api/v1/import",
    STATUS: "/api/v1/import/status",
    CANCEL: "/api/v1/import/cancel",
  },
} as const
