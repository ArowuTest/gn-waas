import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Droplets, Eye, EyeOff, MapPin, Smartphone, ClipboardList } from 'lucide-react'
import { useAuth } from '../contexts/AuthContext'
import apiClient from '../lib/api-client'

const KEYCLOAK_URL = import.meta.env.VITE_KEYCLOAK_URL || ''
const KEYCLOAK_REALM = import.meta.env.VITE_KEYCLOAK_REALM || 'gnwaas'
const KEYCLOAK_CLIENT_ID = import.meta.env.VITE_KEYCLOAK_CLIENT_ID || 'authority-portal'
const DEV_MODE = import.meta.env.VITE_DEV_MODE === 'true'

// Whether to show the Keycloak SSO button (production with Keycloak configured)
const USE_KEYCLOAK = !DEV_MODE && Boolean(KEYCLOAK_URL)

function generateCodeVerifier(): string {
  const array = new Uint8Array(32)
  crypto.getRandomValues(array)
  return btoa(String.fromCharCode(...array)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')
}

async function generateCodeChallenge(verifier: string): Promise<string> {
  const encoder = new TextEncoder()
  const data = encoder.encode(verifier)
  const digest = await crypto.subtle.digest('SHA-256', data)
  return btoa(String.fromCharCode(...new Uint8Array(digest))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')
}

function initiateKeycloakLogin() {
  const redirectUri = `${window.location.origin}/auth/callback`
  const state = crypto.randomUUID()
  const codeVerifier = generateCodeVerifier()
  sessionStorage.setItem('pkce_code_verifier', codeVerifier)
  sessionStorage.setItem('oauth_state', state)
  generateCodeChallenge(codeVerifier).then(codeChallenge => {
    const params = new URLSearchParams({
      client_id: KEYCLOAK_CLIENT_ID,
      redirect_uri: redirectUri,
      response_type: 'code',
      scope: 'openid profile email',
      state,
      code_challenge: codeChallenge,
      code_challenge_method: 'S256',
    })
    window.location.href = `${KEYCLOAK_URL}/realms/${KEYCLOAK_REALM}/protocol/openid-connect/auth?${params}`
  })
}

const DEV_ACCOUNTS = [
  { label: 'GWL Manager',      email: 'manager@gwl.gov.gh',    role: 'GWL_MANAGER' },
  { label: 'Field Officer',    email: 'officer@gwl.gov.gh',    role: 'FIELD_OFFICER' },
  { label: 'Field Supervisor', email: 'supervisor@gwl.gov.gh', role: 'FIELD_SUPERVISOR' },
  { label: 'MOF Auditor',      email: 'auditor@mof.gov.gh',    role: 'MOF_AUDITOR' },
]

export default function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPw, setShowPw] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // Unified login handler — works in both dev mode and production (without Keycloak).
  // v30-02 fix: removed the `if (!DEV_MODE) return` guard that made this handler
  // unreachable when DEV_MODE=false and KEYCLOAK_URL was not set, causing the portal
  // to display a dead-end "Configuration Required" message with no way to log in.
  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      const res = await apiClient.post('/auth/login', { email, password })
      await login(res.data.data.access_token)
      navigate('/district')
    } catch {
      setError('Invalid credentials. Please check your email and password.')
    } finally {
      setLoading(false)
    }
  }

  const handleQuickLogin = async (devEmail: string, role: string) => {
    if (!DEV_MODE) return
    setEmail(devEmail)
    setLoading(true)
    setError('')
    try {
      const res = await apiClient.post('/auth/dev-login', { email: devEmail, role })
      await login(res.data.data.access_token)
      navigate('/district')
    } catch {
      setError('Quick login failed. Please enter credentials manually.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex">
      {/* Left panel */}
      <div className="hidden lg:flex lg:w-1/2 bg-gray-900 flex-col justify-between p-12 relative overflow-hidden">
        <div className="absolute inset-0 opacity-5">
          <div className="absolute top-0 left-0 w-96 h-96 bg-emerald-500 rounded-full -translate-x-1/2 -translate-y-1/2" />
          <div className="absolute bottom-0 right-0 w-96 h-96 bg-yellow-400 rounded-full translate-x-1/2 translate-y-1/2" />
        </div>

        <div className="relative">
          <div className="flex items-center gap-3 mb-12">
            <div className="w-10 h-10 bg-emerald-600 rounded-xl flex items-center justify-center shadow-lg">
              <Droplets size={20} className="text-white" />
            </div>
            <div>
              <p className="text-white font-bold text-lg leading-tight">GN-WAAS</p>
              <p className="text-gray-400 text-xs">Ghana National Water Audit &amp; Assurance System</p>
            </div>
          </div>

          <h1 className="text-4xl font-black text-white leading-tight mb-4">
            Field Authority<br />
            <span className="text-emerald-400">Portal</span>
          </h1>
          <p className="text-gray-400 text-base leading-relaxed max-w-sm">
            For GWL district managers, field officers, and supervisors.
            Manage meter readings, report issues, and track field jobs.
          </p>
        </div>

        <div className="relative space-y-4">
          {[
            { icon: <MapPin size={18} />, title: 'GPS-Locked Evidence', desc: 'Tamper-proof field reports with location data' },
            { icon: <Smartphone size={18} />, title: 'Mobile-First Design', desc: 'Optimised for field use on any device' },
            { icon: <ClipboardList size={18} />, title: 'Job Management', desc: 'Assign and track field officer assignments' },
          ].map(f => (
            <div key={f.title} className="flex items-start gap-3">
              <div className="w-8 h-8 bg-gray-800 rounded-lg flex items-center justify-center text-emerald-400 flex-shrink-0">
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
          <p className="text-gray-600 text-xs">© 2026 Ghana Water Limited · GN-WAAS v8</p>
        </div>
      </div>

      {/* Right panel */}
      <div className="flex-1 flex items-center justify-center p-8 bg-slate-50">
        <div className="w-full max-w-md">
          <div className="lg:hidden flex items-center gap-3 mb-8">
            <div className="w-10 h-10 bg-emerald-600 rounded-xl flex items-center justify-center">
              <Droplets size={20} className="text-white" />
            </div>
            <div>
              <p className="font-bold text-gray-900">GN-WAAS Authority</p>
              <p className="text-xs text-gray-500">Field Portal</p>
            </div>
          </div>

          <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-8">
            <div className="mb-8">
              <h2 className="text-2xl font-bold text-gray-900">Sign in</h2>
              <p className="text-sm text-gray-500 mt-1">
                GWL staff and field officers only
              </p>
            </div>

            {/* ── Auth method selection ──────────────────────────────────────
                Priority:
                  1. Keycloak SSO  — when VITE_KEYCLOAK_URL is set and DEV_MODE=false
                  2. Email/password form — all other cases (dev mode, staging,
                     or production without Keycloak configured).
                     Previously this fell through to a dead-end "Configuration Required"
                     message when DEV_MODE=false and KEYCLOAK_URL was empty.
                     Fixed: always show the email/password form as the fallback.
            ─────────────────────────────────────────────────────────────── */}
            {USE_KEYCLOAK ? (
              <button
                onClick={initiateKeycloakLogin}
                className="w-full py-3 bg-emerald-600 text-white rounded-xl font-semibold hover:bg-emerald-700 transition-colors shadow-sm"
              >
                Sign in with Keycloak
              </button>
            ) : (
              <form onSubmit={handleLogin} className="space-y-5">
                <div>
                  <label className="label">Email address</label>
                  <input
                    type="email"
                    className="input"
                    placeholder="officer@gwl.gov.gh"
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
                  className="w-full py-2.5 bg-emerald-600 text-white rounded-xl font-semibold hover:bg-emerald-700 transition-colors shadow-sm disabled:opacity-50 flex items-center justify-center gap-2"
                >
                  {loading ? (
                    <>
                      <span className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                      Signing in…
                    </>
                  ) : 'Sign in'}
                </button>

                {DEV_MODE && (
                  <div className="pt-4 border-t border-gray-100">
                    <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">
                      Development Quick Login
                    </p>
                    <div className="grid grid-cols-2 gap-2">
                      {DEV_ACCOUNTS.map(acc => (
                        <button
                          key={acc.role}
                          type="button"
                          onClick={() => handleQuickLogin(acc.email, acc.role)}
                          disabled={loading}
                          className="text-left px-3 py-2 rounded-xl border border-gray-200 hover:border-emerald-300 hover:bg-emerald-50 transition-colors group"
                        >
                          <p className="text-xs font-semibold text-gray-700 group-hover:text-emerald-700">{acc.label}</p>
                          <p className="text-[10px] text-gray-400 truncate">{acc.email}</p>
                        </button>
                      ))}
                    </div>
                  </div>
                )}
              </form>
            )}
          </div>

          <p className="text-center text-xs text-gray-400 mt-6">
            GN-WAAS v8 · Authority Portal
          </p>
        </div>
      </div>
    </div>
  )
}
