import { useEffect } from "react"
import { Outlet, useNavigate } from "react-router"

import { AppSidebar } from "~/components/app-sidebar"
import { SidebarInset, SidebarProvider } from "~/components/ui/sidebar"
import { ROUTES } from "~/constants"

import { useAuth } from "~/contexts"

export default function DashboardLayout() {
  const { user, isLoading } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    if (!isLoading && !user) {
      navigate(ROUTES.LOGIN)
    }
  }, [isLoading, user, navigate])

  if (isLoading || !user) return null

  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <Outlet />
      </SidebarInset>
    </SidebarProvider>
  )
}
