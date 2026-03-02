import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Droplets, Eye, EyeOff, ExternalLink } from 'lucide-react'
import { useAuth } from '../contexts/AuthContext'
import apiClient from '../lib/api-client'

/**
 * LoginPage — GN-WAAS Authority Portal
 *
 * Authentication flow:
 * 1. PRODUCTION: Redirects to Keycloak OIDC login page (PKCE flow).
 *    The Keycloak URL is configured via VITE_KEYCLOAK_URL and VITE_KEYCLOAK_REALM.
 *    After login, Keycloak redirects back with an authorization code.
 *    The code is exchanged for a JWT token via the Keycloak token endpoint.
 *
 * 2. DEVELOPMENT (DEV_MODE=true): Uses the api-gateway /auth/login endpoint
 *    which accepts email/password and returns a JWT. This is only available
 *    when the api-gateway is running with DEV_MODE=true.
 *
 * NO hardcoded credentials. The dev quick-login buttons only pre-fill the
 * email field — the user must enter their own password.
 */

const KEYCLOAK_URL = import.meta.env.VITE_KEYCLOAK_URL || ''
const KEYCLOAK_REALM = import.meta.env.VITE_KEYCLOAK_REALM || 'gnwaas'
const KEYCLOAK_CLIENT_ID = import.meta.env.VITE_KEYCLOAK_CLIENT_ID || 'authority-portal'
const DEV_MODE = import.meta.env.VITE_DEV_MODE === 'true'

/**
 * Initiates Keycloak OIDC PKCE login flow.
 * Redirects the browser to the Keycloak authorization endpoint.
 */
function initiateKeycloakLogin() {
  const redirectUri = `${window.location.origin}/auth/callback`
  const state = crypto.randomUUID()
  const codeVerifier = generateCodeVerifier()

  // Store PKCE verifier and state for the callback
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

    const authUrl = `${KEYCLOAK_URL}/realms/${KEYCLOAK_REALM}/protocol/openid-connect/auth?${params}`
    window.location.href = authUrl
  })
}

function generateCodeVerifier(): string {
  const array = new Uint8Array(32)
  crypto.getRandomValues(array)
  return btoa(String.fromCharCode(...array))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '')
}

async function generateCodeChallenge(verifier: string): Promise<string> {
  const encoder = new TextEncoder()
  const data = encoder.encode(verifier)
  const digest = await crypto.subtle.digest('SHA-256', data)
  return btoa(String.fromCharCode(...new Uint8Array(digest)))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '')
}

export default function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPw, setShowPw] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // Dev-mode direct login (only available when VITE_DEV_MODE=true)
  const handleDevLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!DEV_MODE) return
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

  return (
    <div className="min-h-screen bg-green-900 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="w-16 h-16 bg-yellow-400 rounded-2xl flex items-center justify-center mx-auto mb-4">
            <Droplets className="w-9 h-9 text-green-900" />
          </div>
          <h1 className="text-2xl font-black text-white">GN-WAAS</h1>
          <p className="text-green-300 text-sm mt-1">Ghana Water Authority Portal</p>
        </div>

        <div className="bg-white rounded-2xl p-8 shadow-2xl">
          <h2 className="text-xl font-bold text-gray-900 mb-2">Sign In</h2>
          <p className="text-sm text-gray-500 mb-6">
            Access is restricted to authorised GWL staff only.
          </p>

          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg px-4 py-3 mb-4">
              {error}
            </div>
          )}

          {/* Production: Keycloak SSO button */}
          {!DEV_MODE && KEYCLOAK_URL && (
            <button
              onClick={initiateKeycloakLogin}
              className="w-full flex items-center justify-center gap-2 bg-green-800 text-white font-bold py-3 rounded-xl hover:bg-green-900 transition-colors"
            >
              <ExternalLink className="w-4 h-4" />
              Sign in with GWL SSO (Keycloak)
            </button>
          )}

          {/* Production: No Keycloak configured */}
          {!DEV_MODE && !KEYCLOAK_URL && (
            <div className="bg-yellow-50 border border-yellow-200 rounded-xl p-4 text-center">
              <p className="text-yellow-800 text-sm font-semibold">SSO not configured</p>
              <p className="text-yellow-600 text-xs mt-1">
                Set VITE_KEYCLOAK_URL in your environment to enable login.
              </p>
            </div>
          )}

          {/* Development: Email/password form */}
          {DEV_MODE && (
            <>
              <div className="bg-amber-50 border border-amber-200 rounded-lg px-4 py-2 mb-4">
                <p className="text-amber-700 text-xs font-semibold">
                  ⚠ Development Mode — Direct login enabled
                </p>
              </div>
              <form onSubmit={handleDevLogin} className="space-y-4">
                <div>
                  <label className="block text-sm font-semibold text-gray-700 mb-1.5">
                    Email Address
                  </label>
                  <input
                    type="email"
                    required
                    value={email}
                    onChange={e => setEmail(e.target.value)}
                    className="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-green-600"
                    placeholder="you@gwl.gov.gh"
                  />
                </div>
                <div>
                  <label className="block text-sm font-semibold text-gray-700 mb-1.5">
                    Password
                  </label>
                  <div className="relative">
                    <input
                      type={showPw ? 'text' : 'password'}
                      required
                      value={password}
                      onChange={e => setPassword(e.target.value)}
                      className="w-full border border-gray-200 rounded-lg px-4 py-2.5 pr-10 text-sm focus:outline-none focus:ring-2 focus:ring-green-600"
                      placeholder="••••••••"
                    />
                    <button
                      type="button"
                      onClick={() => setShowPw(!showPw)}
                      className="absolute right-3 top-2.5 text-gray-400"
                    >
                      {showPw ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                </div>
                <button
                  type="submit"
                  disabled={loading}
                  className="w-full bg-green-800 text-white font-bold py-3 rounded-xl hover:bg-green-900 transition-colors disabled:opacity-50"
                >
                  {loading ? 'Signing in...' : 'Sign In'}
                </button>
              </form>

              {/* Dev quick-fill (email only — no passwords) */}
              <div className="mt-6 pt-6 border-t border-gray-100">
                <p className="text-xs text-gray-400 text-center mb-3">
                  Quick-fill email (enter your own password)
                </p>
                <div className="grid grid-cols-3 gap-2">
                  {[
                    ['dm.accra@gwl.gov.gh', 'District Mgr'],
                    ['officer.kofi@gwl.gov.gh', 'Field Officer'],
                    ['finance.ama@gwl.gov.gh', 'Finance'],
                  ].map(([emailAddr, label]) => (
                    <button
                      key={emailAddr}
                      onClick={() => setEmail(emailAddr)}
                      className="text-xs bg-gray-100 hover:bg-gray-200 text-gray-600 py-1.5 px-2 rounded-lg transition-colors"
                    >
                      {label}
                    </button>
                  ))}
                </div>
              </div>
            </>
          )}
        </div>

        <p className="text-center text-green-400 text-xs mt-6">
          GN-WAAS v1.0 · Ghana National Water Audit & Assurance System
        </p>
      </div>
    </div>
  )
}
