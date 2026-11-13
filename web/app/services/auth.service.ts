import { api } from "~/lib/api"
import { API_ENDPOINTS } from "~/constants"

import type { AuthResponse, User } from "~/types"

export const authService = {
  async register(
    email: string,
    password: string,
    name: string
  ): Promise<AuthResponse> {
    return api.post<AuthResponse>(API_ENDPOINTS.AUTH.REGISTER, {
      email,
      password,
      name,
    })
  },

  async login(email: string, password: string): Promise<AuthResponse> {
    return api.post<AuthResponse>(API_ENDPOINTS.AUTH.LOGIN, {
      email,
      password,
    })
  },

  async logout(): Promise<void> {
    return api.post(API_ENDPOINTS.AUTH.LOGOUT)
  },

  async getProfile(): Promise<User> {
    return api.get<User>(API_ENDPOINTS.AUTH.ME)
  },

  async updateProfile(email: string, name: string): Promise<User> {
    return api.put<User>(API_ENDPOINTS.AUTH.ME, { email, name })
  },

  async changePassword(
    currentPassword: string,
    newPassword: string
  ): Promise<void> {
    return api.put(API_ENDPOINTS.AUTH.PASSWORD, {
      currentPassword,
      newPassword,
    })
  },

  // Permanently deletes the signed-in account and its flight collection.
  async deleteAccount(password: string): Promise<void> {
    return api.delete(API_ENDPOINTS.AUTH.ME, { password })
  },

  // Public sign-in settings, such as whether new accounts can be created.
  async config(): Promise<{ registrationEnabled: boolean }> {
    return api.get(API_ENDPOINTS.AUTH.CONFIG)
  },
}
