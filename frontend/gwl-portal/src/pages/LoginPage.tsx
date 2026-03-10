import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Droplets, Eye, EyeOff, Shield, BarChart3, Users } from 'lucide-react';
import { api as apiClient } from '../utils/api';

const API_BASE_URL = import.meta.env.VITE_API_URL || ''

const DEV_MODE = import.meta.env.VITE_DEV_MODE === 'true';

const DEV_ACCOUNTS = [
  { label: 'GWL Manager',    email: 'manager.accrawest@gwl.com.gh', role: 'GWL_MANAGER' },
  { label: 'GWL Supervisor', email: 'supervisor@gwl.com.gh',        role: 'GWL_SUPERVISOR' },
  { label: 'GWL Analyst',    email: 'analyst1@gwl.com.gh',          role: 'GWL_ANALYST' },
];

export default function LoginPage() {
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPw, setShowPw] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const res = await apiClient.post('/auth/login', { email, password });
      const data = res.data?.data ?? res.data;
      const token = data?.access_token || data?.token;
      if (!token) throw new Error('No token received');
      localStorage.setItem('gwl_token', token);
      // GAP-FIX-06: persist user object so CaseDetailPage can read performed_by/role
      if (data?.user) localStorage.setItem('gwl_user', JSON.stringify(data.user));
      navigate('/');
    } catch {
      setError('Invalid credentials. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  const handleQuickLogin = async (devEmail: string, role: string) => {
    if (!DEV_MODE) return;
    setLoading(true);
    setError('');
    try {
      // Use /auth/dev-login so the backend returns a role-specific token
      const res = await apiClient.post('/auth/dev-login', { email: devEmail, role });
      const data = res.data?.data ?? res.data;
      const token = data?.access_token || data?.token;
      if (!token) throw new Error('No token received');
      localStorage.setItem('gwl_token', token);
      // GAP-FIX-06: persist user object so CaseDetailPage can read performed_by/role
      if (data?.user) localStorage.setItem('gwl_user', JSON.stringify(data.user));
      navigate('/');
    } catch {
      setEmail(devEmail);
      setError('Quick login failed. Enter credentials manually.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex">
      {/* Left panel */}
      <div className="hidden lg:flex lg:w-1/2 bg-gray-900 flex-col justify-between p-12 relative overflow-hidden">
        <div className="absolute inset-0 opacity-5">
          <div className="absolute top-0 left-0 w-96 h-96 bg-blue-500 rounded-full -translate-x-1/2 -translate-y-1/2" />
          <div className="absolute bottom-0 right-0 w-96 h-96 bg-yellow-400 rounded-full translate-x-1/2 translate-y-1/2" />
        </div>

        <div className="relative">
          <div className="flex items-center gap-3 mb-12">
            <div className="w-10 h-10 bg-blue-600 rounded-xl flex items-center justify-center shadow-lg">
              <Droplets size={20} className="text-white" />
            </div>
            <div>
              <p className="text-white font-bold text-lg leading-tight">GN-WAAS</p>
              <p className="text-gray-400 text-xs">Ghana National Water Audit & Assurance System</p>
            </div>
          </div>

          <h1 className="text-4xl font-black text-white leading-tight mb-4">
            GWL Case<br />
            <span className="text-blue-400">Management</span>
          </h1>
          <p className="text-gray-400 text-base leading-relaxed max-w-sm">
            Review and action anomaly flags raised by the GN-WAAS Sentinel engine.
            Manage underbilling, overbilling, and misclassification cases.
          </p>
        </div>

        <div className="relative space-y-4">
          {[
            { icon: <Shield size={18} />, title: 'Anomaly Review', desc: 'Sentinel-flagged billing discrepancies' },
            { icon: <BarChart3 size={18} />, title: 'Revenue Recovery', desc: 'Track confirmed losses and corrections' },
            { icon: <Users size={18} />, title: 'Field Coordination', desc: 'Assign and monitor field officer jobs' },
          ].map(f => (
            <div key={f.title} className="flex items-start gap-3">
              <div className="w-8 h-8 bg-gray-800 rounded-lg flex items-center justify-center text-blue-400 flex-shrink-0">
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
            <div className="w-10 h-10 bg-blue-600 rounded-xl flex items-center justify-center">
              <Droplets size={20} className="text-white" />
            </div>
            <div>
              <p className="font-bold text-gray-900">GWL Case Portal</p>
              <p className="text-xs text-gray-500">GN-WAAS</p>
            </div>
          </div>

          <div className="bg-white rounded-2xl shadow-card border border-gray-100 p-8">
            <div className="mb-8">
              <h2 className="text-2xl font-bold text-gray-900">Sign in</h2>
              <p className="text-sm text-gray-500 mt-1">GWL staff access only</p>
            </div>

            <form onSubmit={handleSubmit} className="space-y-5">
              <div>
                <label className="block text-sm font-semibold text-gray-700 mb-1.5">Email address</label>
                <input
                  type="email"
                  className="w-full px-3.5 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent placeholder:text-gray-400"
                  placeholder="supervisor@gwl.gov.gh"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  autoComplete="email"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-semibold text-gray-700 mb-1.5">Password</label>
                <div className="relative">
                  <input
                    type={showPw ? 'text' : 'password'}
                    className="w-full px-3.5 py-2.5 pr-10 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent placeholder:text-gray-400"
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
                className="w-full py-2.5 bg-blue-600 text-white rounded-xl font-semibold hover:bg-blue-700 transition-colors shadow-sm disabled:opacity-50 flex items-center justify-center gap-2"
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
                        className="text-left px-3 py-2 rounded-xl border border-gray-200 hover:border-blue-300 hover:bg-blue-50 transition-colors group"
                      >
                        <p className="text-xs font-semibold text-gray-700 group-hover:text-blue-700">{acc.label}</p>
                        <p className="text-[10px] text-gray-400 truncate">{acc.email}</p>
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </form>
          </div>

          <p className="text-center text-xs text-gray-400 mt-6">
            GN-WAAS v8 · GWL Case Management Portal
          </p>
        </div>
      </div>
    </div>
  );
}
