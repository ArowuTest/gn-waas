import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'
import type { User } from '../types'
import apiClient from '../lib/api-client'

interface AuthContextType {
  user: User | null
  token: string | null
  isLoading: boolean
  login: (token: string) => Promise<void>
  logout: () => void
  hasRole: (...roles: string[]) => boolean
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const storedToken = localStorage.getItem('gnwaas_token')
    if (storedToken) {
      setToken(storedToken)
      fetchCurrentUser()
    } else {
      setIsLoading(false)
    }
  }, [])

  const fetchCurrentUser = async () => {
    try {
      const response = await apiClient.get('/users/me')
      setUser(response.data.data)
    } catch {
      localStorage.removeItem('gnwaas_token')
      setToken(null)
    } finally {
      setIsLoading(false)
    }
  }

  const login = async (newToken: string) => {
    localStorage.setItem('gnwaas_token', newToken)
    setToken(newToken)
    await fetchCurrentUser()
  }

  const logout = () => {
    localStorage.removeItem('gnwaas_token')
    setToken(null)
    setUser(null)
    window.location.href = '/login'
  }

  const hasRole = (...roles: string[]) => {
    if (!user) return false
    return roles.includes(user.role)
  }

  return (
    <AuthContext.Provider value={{ user, token, isLoading, login, logout, hasRole }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
