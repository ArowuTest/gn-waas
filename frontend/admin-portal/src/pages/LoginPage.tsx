import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Droplets, Eye, EyeOff, Shield, BarChart3, Map } from 'lucide-react'
import { useAuth } from '../contexts/AuthContext'

const API_BASE_URL = import.meta.env.VITE_API_URL || ''

// DEV_MODE: Controlled exclusively via VITE_DEV_MODE environment variable.
// Set VITE_DEV_MODE=true in .env.local for local development.
// .env.production sets VITE_DEV_MODE=false — demo panel is hidden in production.
// SECURITY NOTE (P3-03): Passwords below are demo-only staging credentials.
const DEV_MODE = import.meta.env.VITE_DEV_MODE === 'true'

const DEV_ACCOUNTS = [
  { label: 'Super Admin',   email: 'superadmin@gnwaas.gov.gh',      role: 'SUPER_ADMIN',  password: 'Admin@GN2026!' },
  { label: 'System Admin',  email: 'sysadmin@gnwaas.gov.gh',        role: 'SYSTEM_ADMIN', password: 'Admin@GN2026!' },
  { label: 'MOF Auditor',   email: 'auditor1@mof.gov.gh',           role: 'MOF_AUDITOR',  password: 'MoF@Audit2026!' },
  { label: 'GWL Manager',   email: 'manager.accrawest@gwl.com.gh',  role: 'GWL_MANAGER',  password: 'GWL@Manager2026!' },
]

export function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPw, setShowPw] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!email || !password) { setError('Please enter your email and password.'); return }
    setLoading(true)
    setError('')
    try {
      const res = await fetch(`${API_BASE_URL}/api/v1/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.message || 'Login failed')
      await login(data.data.access_token)
      navigate('/dashboard')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Invalid credentials. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  const handleDevLogin = async (devEmail: string, _role: string, devPassword: string) => {
    setLoading(true)
    setError('')
    setEmail(devEmail)
    setPassword(devPassword)
    try {
      const res = await fetch(`${API_BASE_URL}/api/v1/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: devEmail, password: devPassword }),
      })
      const data = await res.json()
      if (!res.ok || !data.success) throw new Error(data.error?.message || 'Login failed')
      await login(data.data.access_token)
      navigate('/dashboard')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Login failed. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex">
      {/* Left panel — branding */}
      <div className="hidden lg:flex lg:w-1/2 bg-gray-900 flex-col justify-between p-12 relative overflow-hidden">
        {/* Background pattern */}
        <div className="absolute inset-0 opacity-5">
          <div className="absolute top-0 left-0 w-96 h-96 bg-brand-500 rounded-full -translate-x-1/2 -translate-y-1/2" />
          <div className="absolute bottom-0 right-0 w-96 h-96 bg-gold-500 rounded-full translate-x-1/2 translate-y-1/2" />
        </div>

        <div className="relative">
          <div className="flex items-center gap-3 mb-12">
            <div className="w-10 h-10 bg-brand-600 rounded-xl flex items-center justify-center shadow-lg">
              <Droplets size={20} className="text-white" />
            </div>
            <div>
              <p className="text-white font-bold text-lg leading-tight">GN-WAAS</p>
              <p className="text-gray-400 text-xs">Ghana National Water Audit & Assurance System</p>
            </div>
          </div>

          <h1 className="text-4xl font-black text-white leading-tight mb-4">
            Water Audit<br />
            <span className="text-brand-400">Administration</span>
          </h1>
          <p className="text-gray-400 text-base leading-relaxed max-w-sm">
            Sovereign oversight of Ghana's water distribution network.
            Real-time anomaly detection, GRA compliance, and revenue recovery.
          </p>
        </div>

        {/* Feature highlights */}
        <div className="relative space-y-4">
          {[
            { icon: <BarChart3 size={18} />, title: 'NRW Analysis', desc: 'IWA/AWWA water balance framework' },
            { icon: <Shield size={18} />, title: 'GRA Compliance', desc: 'Automated VAT audit trail & QR signing' },
            { icon: <Map size={18} />, title: 'DMA Mapping', desc: 'District metered area visualisation' },
          ].map(f => (
            <div key={f.title} className="flex items-start gap-3">
              <div className="w-8 h-8 bg-gray-800 rounded-lg flex items-center justify-center text-brand-400 flex-shrink-0">
                {f.icon}
              </div>
              <div>
                <p className="text-white text-sm font-semibold">{f.title}</p>
                <p className="text-gray-500 text-xs">{f.desc}</p>
              </div>
            </div>
          ))}
        </div>

        <div className="relative">
          <p className="text-gray-600 text-xs">
            © 2026 Ghana Water Limited · Ministry of Finance · GRA
          </p>
        </div>
      </div>

      {/* Right panel — login form */}
      <div className="flex-1 flex items-center justify-center p-8 bg-slate-50">
        <div className="w-full max-w-md">
          {/* Mobile logo */}
          <div className="lg:hidden flex items-center gap-3 mb-8">
            <div className="w-10 h-10 bg-brand-600 rounded-xl flex items-center justify-center">
              <Droplets size={20} className="text-white" />
            </div>
            <div>
              <p className="font-bold text-gray-900">GN-WAAS Admin</p>
              <p className="text-xs text-gray-500">Water Audit System</p>
            </div>
          </div>

          <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-8">
            <div className="mb-8">
              <h2 className="text-2xl font-bold text-gray-900">Sign in</h2>
              <p className="text-sm text-gray-500 mt-1">
                Access restricted to authorised personnel only
              </p>
            </div>

            <form onSubmit={handleSubmit} className="space-y-5">
              <div>
                <label className="label">Email address</label>
                <input
                  type="email"
                  className="input"
                  placeholder="officer@gnwaas.gov.gh"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  autoComplete="email"
                  required
                />
              </div>

              <div>
                <label className="label">Password</label>
                <div className="relative">
                  <input
                    type={showPw ? 'text' : 'password'}
                    className="input pr-10"
                    placeholder="••••••••"
                    value={password}
                    onChange={e => setPassword(e.target.value)}
                    autoComplete="current-password"
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowPw(!showPw)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
                  >
                    {showPw ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>

              {error && (
                <div className="bg-red-50 border border-red-200 rounded-xl px-4 py-3 text-sm text-red-700">
                  {error}
                </div>
              )}

              <button
                type="submit"
                disabled={loading}
                className="btn-primary w-full py-2.5"
              >
                {loading ? (
                  <span className="flex items-center gap-2">
                    <span className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                    Signing in…
                  </span>
                ) : 'Sign in'}
              </button>
            </form>

            {DEV_MODE && (
              <div className="mt-6 pt-6 border-t border-gray-100">
                <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">
                  Development Quick Login
                </p>
                <div className="grid grid-cols-2 gap-2">
                  {DEV_ACCOUNTS.map(acc => (
                    <button
                      key={acc.role}
                      onClick={() => handleDevLogin(acc.email, acc.role, acc.password)}
                      disabled={loading}
                      className="text-left px-3 py-2 rounded-xl border border-gray-200 hover:border-brand-300 hover:bg-brand-50 transition-colors group"
                    >
                      <p className="text-xs font-semibold text-gray-700 group-hover:text-brand-700">{acc.label}</p>
                      <p className="text-[10px] text-gray-400 truncate">{acc.email}</p>
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>

          <p className="text-center text-xs text-gray-400 mt-6">
            GN-WAAS v8 · Secured by Keycloak OIDC
          </p>
        </div>
      </div>
    </div>
  )
}
