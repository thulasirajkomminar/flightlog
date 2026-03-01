import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react"
import { useNavigate } from "react-router"

import { ROUTES } from "~/constants"

import type { User } from "~/types"

import { authService } from "~/services"

interface AuthContextValue {
  user: User | null
  isLoading: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, name: string) => Promise<void>
  logout: () => void
  updateUser: (user: User) => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const navigate = useNavigate()

  useEffect(() => {
    authService
      .getProfile()
      .then(setUser)
      .catch(() => {})
      .finally(() => setIsLoading(false))
  }, [])

  const login = useCallback(
    async (email: string, password: string) => {
      const response = await authService.login(email, password)
      setUser(response.user)
      navigate(ROUTES.HOME)
    },
    [navigate]
  )

  const register = useCallback(
    async (email: string, password: string, name: string) => {
      const response = await authService.register(email, password, name)
      setUser(response.user)
      navigate(ROUTES.HOME)
    },
    [navigate]
  )

  const logout = useCallback(async () => {
    await authService.logout().catch(() => {})
    document.cookie = "flightlog_sidebar=; path=/; max-age=0"
    setUser(null)
    navigate(ROUTES.LOGIN)
  }, [navigate])

  const updateUser = useCallback((updatedUser: User) => {
    setUser(updatedUser)
  }, [])

  return (
    <AuthContext.Provider
      value={{ user, isLoading, login, register, logout, updateUser }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider")
  }
  return context
}
