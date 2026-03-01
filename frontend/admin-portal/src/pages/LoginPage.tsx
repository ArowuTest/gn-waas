import { useState } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { Droplets, Eye, EyeOff } from 'lucide-react'
import apiClient from '../lib/api-client'

export function LoginPage() {
  const { login } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsLoading(true)

    try {
      // In development mode, the API gateway accepts any credentials
      // and returns a dev token. In production, this calls Keycloak.
      const response = await apiClient.post('/auth/login', { email, password })
      await login(response.data.data.access_token)
    } catch (err: any) {
      setError(err.response?.data?.error || 'Invalid credentials. Please try again.')
    } finally {
      setIsLoading(false)
    }
  }

  // Dev mode quick login
  const devLogin = async (role: string) => {
    setIsLoading(true)
    try {
      const response = await apiClient.post('/auth/dev-login', { role })
      await login(response.data.data.access_token)
    } catch {
      setError('Dev login failed')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-brand-600 to-brand-800 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 bg-white rounded-2xl shadow-lg mb-4">
            <Droplets size={32} className="text-brand-600" />
          </div>
          <h1 className="text-white text-2xl font-bold">GN-WAAS</h1>
          <p className="text-brand-200 text-sm mt-1">Ghana National Water Audit & Assurance System</p>
        </div>

        {/* Login Card */}
        <div className="bg-white rounded-2xl shadow-2xl p-8">
          <h2 className="text-gray-900 text-xl font-bold mb-6">Sign in to your account</h2>

          {error && (
            <div className="mb-4 p-3 bg-danger-light border border-danger rounded-lg text-danger text-sm">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="label">Email address</label>
              <input
                type="email"
                className="input"
                placeholder="officer@gwl.gov.gh"
                value={email}
                onChange={e => setEmail(e.target.value)}
                required
              />
            </div>

            <div>
              <label className="label">Password</label>
              <div className="relative">
                <input
                  type={showPassword ? 'text' : 'password'}
                  className="input pr-10"
                  placeholder="••••••••"
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  required
                />
                <button
                  type="button"
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
                  onClick={() => setShowPassword(!showPassword)}
                >
                  {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </div>

            <button
              type="submit"
              className="btn-primary w-full btn-lg"
              disabled={isLoading}
            >
              {isLoading ? 'Signing in...' : 'Sign in'}
            </button>
          </form>

          {/* Dev mode quick access */}
          {import.meta.env.DEV && (
            <div className="mt-6 pt-6 border-t border-gray-100">
              <p className="text-xs text-gray-400 text-center mb-3">
                Development Mode — Quick Access
              </p>
              <div className="grid grid-cols-2 gap-2">
                {[
                  { role: 'SYSTEM_ADMIN', label: 'System Admin' },
                  { role: 'AUDIT_SUPERVISOR', label: 'Supervisor' },
                  { role: 'FIELD_OFFICER', label: 'Field Officer' },
                  { role: 'FINANCE_ANALYST', label: 'Finance Analyst' },
                ].map(({ role, label }) => (
                  <button
                    key={role}
                    onClick={() => devLogin(role)}
                    className="btn-secondary btn-sm text-xs"
                    disabled={isLoading}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>

        <p className="text-center text-brand-200 text-xs mt-6">
          Secured by Keycloak · NITA Ghana Infrastructure · v1.0.0
        </p>
      </div>
    </div>
  )
}
