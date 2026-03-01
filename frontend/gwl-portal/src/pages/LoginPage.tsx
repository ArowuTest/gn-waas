import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Input } from '../components/ui';

export default function LoginPage() {
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    // In production: exchange credentials for Keycloak token
    // In development: accept any credentials and store a dev token
    try {
      if (import.meta.env.DEV) {
        // Dev mode: quick login
        localStorage.setItem('gwl_token', 'dev-gwl-token');
        navigate('/');
        return;
      }

      // Production: Keycloak token endpoint
      const keycloakUrl = import.meta.env.VITE_KEYCLOAK_URL || 'http://localhost:8080';
      const realm = import.meta.env.VITE_KEYCLOAK_REALM || 'gnwaas';
      const clientId = import.meta.env.VITE_KEYCLOAK_CLIENT_ID || 'gwl-portal';

      const res = await fetch(
        `${keycloakUrl}/realms/${realm}/protocol/openid-connect/token`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
          body: new URLSearchParams({
            grant_type: 'password',
            client_id: clientId,
            username: email,
            password,
          }),
        }
      );

      if (!res.ok) {
        setError('Invalid credentials. Please try again.');
        return;
      }

      const data = await res.json();
      localStorage.setItem('gwl_token', data.access_token);
      navigate('/');
    } catch {
      setError('Login failed. Please check your connection.');
    } finally {
      setLoading(false);
    }
  };

  const quickFill = (role: string) => {
    const accounts: Record<string, { email: string; password: string }> = {
      supervisor:      { email: 'supervisor@gwl.com.gh',      password: 'dev-password' },
      billing_officer: { email: 'billing@gwl.com.gh',         password: 'dev-password' },
      manager:         { email: 'manager@gwl.com.gh',         password: 'dev-password' },
    };
    const acc = accounts[role];
    if (acc) { setEmail(acc.email); setPassword(acc.password); }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-900 to-blue-900 flex items-center justify-center p-4">
      <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md p-8">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="w-16 h-16 bg-blue-600 rounded-2xl flex items-center justify-center text-white text-2xl font-bold mx-auto mb-4">
            GW
          </div>
          <h1 className="text-2xl font-bold text-gray-900">GWL Case Management</h1>
          <p className="text-sm text-gray-500 mt-1">Ghana Water Limited — GN-WAAS Portal</p>
        </div>

        <form onSubmit={handleLogin} className="space-y-4">
          <Input
            label="Email Address"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@gwl.com.gh"
            required
          />
          <Input
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="••••••••"
            required
          />

          {error && (
            <div className="bg-red-50 border border-red-200 rounded-lg px-4 py-3 text-sm text-red-700">
              {error}
            </div>
          )}

          <Button type="submit" className="w-full justify-center" loading={loading}>
            Sign In
          </Button>
        </form>

        {/* Dev quick-fill */}
        {import.meta.env.DEV && (
          <div className="mt-6 pt-6 border-t border-gray-100">
            <p className="text-xs text-gray-400 text-center mb-3">Development Quick Login</p>
            <div className="flex gap-2 flex-wrap justify-center">
              {['supervisor', 'billing_officer', 'manager'].map((role) => (
                <button
                  key={role}
                  type="button"
                  onClick={() => quickFill(role)}
                  className="text-xs px-3 py-1.5 bg-gray-100 hover:bg-gray-200 rounded-lg text-gray-600 capitalize"
                >
                  {role.replace('_', ' ')}
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
