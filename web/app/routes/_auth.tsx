import { useEffect } from "react"
import { Outlet, useNavigate } from "react-router"

import { ROUTES } from "~/constants"

import { useAuth } from "~/contexts"

export default function AuthLayout() {
  const { user, isLoading } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    if (!isLoading && user) {
      navigate(ROUTES.HOME)
    }
  }, [isLoading, user, navigate])

  return isLoading ? null : <Outlet />
}
