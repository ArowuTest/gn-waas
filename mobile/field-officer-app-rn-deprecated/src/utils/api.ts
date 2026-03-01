import axios from 'axios'
import * as SecureStore from 'expo-secure-store'

const API_BASE = process.env.EXPO_PUBLIC_API_URL || 'https://api.gnwaas.nita.gov.gh'

export const apiClient = axios.create({
  baseURL: API_BASE,
  timeout: 30_000,
  headers: { 'Content-Type': 'application/json' },
})

// Attach JWT token to every request
apiClient.interceptors.request.use(async (config) => {
  try {
    const token = await SecureStore.getItemAsync('gnwaas_token')
    if (token) config.headers.Authorization = `Bearer ${token}`
  } catch {
    // SecureStore unavailable in simulator
  }
  return config
})

// Handle 401 — token expired
apiClient.interceptors.response.use(
  (res) => res,
  async (error) => {
    if (error.response?.status === 401) {
      await SecureStore.deleteItemAsync('gnwaas_token')
      await SecureStore.deleteItemAsync('gnwaas_user')
      // Navigation to login handled by root layout
    }
    return Promise.reject(error)
  }
)
