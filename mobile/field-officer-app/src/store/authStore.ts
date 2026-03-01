import { create } from 'zustand'
import * as SecureStore from 'expo-secure-store'
import type { User, AuthState } from '../types'

interface AuthStore extends AuthState {
  login: (token: string, user: User) => Promise<void>
  logout: () => Promise<void>
  loadStoredAuth: () => Promise<void>
}

const TOKEN_KEY = 'gnwaas_token'
const USER_KEY = 'gnwaas_user'

export const useAuthStore = create<AuthStore>((set) => ({
  user: null,
  token: null,
  isAuthenticated: false,

  login: async (token: string, user: User) => {
    await SecureStore.setItemAsync(TOKEN_KEY, token)
    await SecureStore.setItemAsync(USER_KEY, JSON.stringify(user))
    set({ token, user, isAuthenticated: true })
  },

  logout: async () => {
    await SecureStore.deleteItemAsync(TOKEN_KEY)
    await SecureStore.deleteItemAsync(USER_KEY)
    set({ token: null, user: null, isAuthenticated: false })
  },

  loadStoredAuth: async () => {
    try {
      const token = await SecureStore.getItemAsync(TOKEN_KEY)
      const userStr = await SecureStore.getItemAsync(USER_KEY)
      if (token && userStr) {
        const user = JSON.parse(userStr) as User
        set({ token, user, isAuthenticated: true })
      }
    } catch {
      // Secure store unavailable (simulator) — skip
    }
  },
}))
