import {
  index,
  layout,
  route,
  type RouteConfig,
} from "@react-router/dev/routes"

export default [
  layout("routes/_dashboard.tsx", [
    index("routes/dashboard.tsx"),
    route("search", "routes/search.tsx"),
    route("flights", "routes/flights.tsx"),
  ]),
  layout("routes/_auth.tsx", [
    route("login", "routes/_auth/login.tsx"),
    route("register", "routes/_auth/register.tsx"),
  ]),
] satisfies RouteConfig
