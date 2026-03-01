import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Droplets, Eye, EyeOff } from 'lucide-react'
import { useAuth } from '../contexts/AuthContext'
import apiClient from '../lib/api-client'

export default function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPw, setShowPw] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      const res = await apiClient.post('/auth/login', { email, password })
      await login(res.data.data.token)
      navigate('/district')
    } catch {
      setError('Invalid credentials. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  const quickLogin = async (role: string) => {
    const accounts: Record<string, [string, string]> = {
      'DM':  ['dm.accra@gwl.gov.gh', 'Demo@1234'],
      'FO':  ['officer.kofi@gwl.gov.gh', 'Demo@1234'],
      'FA':  ['finance.ama@gwl.gov.gh', 'Demo@1234'],
    }
    const [e, p] = accounts[role] || ['', '']
    setEmail(e)
    setPassword(p)
  }

  return (
    <div className="min-h-screen bg-green-900 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <div className="w-16 h-16 bg-yellow-400 rounded-2xl flex items-center justify-center mx-auto mb-4">
            <Droplets className="w-9 h-9 text-green-900" />
          </div>
          <h1 className="text-2xl font-black text-white">GN-WAAS</h1>
          <p className="text-green-300 text-sm mt-1">Ghana Water Authority Portal</p>
        </div>

        <div className="bg-white rounded-2xl p-8 shadow-2xl">
          <h2 className="text-xl font-bold text-gray-900 mb-6">Sign In</h2>

          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg px-4 py-3 mb-4">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-semibold text-gray-700 mb-1.5">Email Address</label>
              <input type="email" required value={email} onChange={e => setEmail(e.target.value)}
                className="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-green-600"
                placeholder="you@gwl.gov.gh" />
            </div>
            <div>
              <label className="block text-sm font-semibold text-gray-700 mb-1.5">Password</label>
              <div className="relative">
                <input type={showPw ? 'text' : 'password'} required value={password} onChange={e => setPassword(e.target.value)}
                  className="w-full border border-gray-200 rounded-lg px-4 py-2.5 pr-10 text-sm focus:outline-none focus:ring-2 focus:ring-green-600"
                  placeholder="••••••••" />
                <button type="button" onClick={() => setShowPw(!showPw)} className="absolute right-3 top-2.5 text-gray-400">
                  {showPw ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>
            <button type="submit" disabled={loading}
              className="w-full bg-green-800 text-white font-bold py-3 rounded-xl hover:bg-green-900 transition-colors disabled:opacity-50">
              {loading ? 'Signing in...' : 'Sign In'}
            </button>
          </form>

          <div className="mt-6 pt-6 border-t border-gray-100">
            <p className="text-xs text-gray-400 text-center mb-3">Dev Mode — Quick Access</p>
            <div className="grid grid-cols-3 gap-2">
              {[['DM', 'District Mgr'], ['FO', 'Field Officer'], ['FA', 'Finance']].map(([key, label]) => (
                <button key={key} onClick={() => quickLogin(key)}
                  className="text-xs bg-gray-100 hover:bg-gray-200 text-gray-600 py-1.5 px-2 rounded-lg transition-colors">
                  {label}
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
