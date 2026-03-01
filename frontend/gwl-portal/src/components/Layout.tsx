import React, { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { cn } from '../utils/helpers';

const NAV_ITEMS = [
  { path: '/',                  label: 'Dashboard',           icon: '⊞' },
  { path: '/cases',             label: 'All Cases',           icon: '📋' },
  { path: '/underbilling',      label: 'Underbilling',        icon: '📉' },
  { path: '/overbilling',       label: 'Overbilling',         icon: '📈' },
  { path: '/misclassification', label: 'Misclassification',   icon: '🔄' },
  { path: '/field-assignments', label: 'Field Assignments',   icon: '🗺️' },
  { path: '/credits',           label: 'Credit Requests',     icon: '💳' },
  { path: '/reports',           label: 'Monthly Reports',     icon: '📊' },
];

export function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const navigate = useNavigate();
  const [sidebarOpen, setSidebarOpen] = useState(true);

  const handleLogout = () => {
    localStorage.removeItem('gwl_token');
    navigate('/login');
  };

  return (
    <div className="flex h-screen bg-gray-50 overflow-hidden">
      {/* Sidebar */}
      <aside className={cn(
        'flex flex-col bg-slate-900 text-white transition-all duration-200',
        sidebarOpen ? 'w-60' : 'w-16'
      )}>
        {/* Logo */}
        <div className="flex items-center gap-3 px-4 py-5 border-b border-slate-700">
          <div className="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center text-white font-bold text-sm flex-shrink-0">
            GW
          </div>
          {sidebarOpen && (
            <div>
              <p className="text-sm font-bold leading-tight">GWL Portal</p>
              <p className="text-xs text-slate-400">Case Management</p>
            </div>
          )}
        </div>

        {/* Nav */}
        <nav className="flex-1 py-4 overflow-y-auto">
          {NAV_ITEMS.map((item) => {
            const active = location.pathname === item.path ||
              (item.path !== '/' && location.pathname.startsWith(item.path));
            return (
              <Link
                key={item.path}
                to={item.path}
                className={cn(
                  'flex items-center gap-3 px-4 py-2.5 text-sm transition-colors',
                  active
                    ? 'bg-blue-600 text-white'
                    : 'text-slate-300 hover:bg-slate-800 hover:text-white'
                )}
                title={!sidebarOpen ? item.label : undefined}
              >
                <span className="text-base flex-shrink-0">{item.icon}</span>
                {sidebarOpen && <span>{item.label}</span>}
              </Link>
            );
          })}
        </nav>

        {/* Footer */}
        <div className="border-t border-slate-700 p-4">
          <button
            onClick={handleLogout}
            className="flex items-center gap-3 text-slate-400 hover:text-white text-sm w-full"
          >
            <span>🚪</span>
            {sidebarOpen && <span>Logout</span>}
          </button>
        </div>
      </aside>

      {/* Main */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Top bar */}
        <header className="bg-white border-b border-gray-200 px-6 py-3 flex items-center justify-between flex-shrink-0">
          <div className="flex items-center gap-4">
            <button
              onClick={() => setSidebarOpen(!sidebarOpen)}
              className="text-gray-500 hover:text-gray-700"
            >
              ☰
            </button>
            <div>
              <h1 className="text-sm font-semibold text-gray-900">
                Ghana National Water Audit & Assurance System
              </h1>
              <p className="text-xs text-gray-500">GWL Case Management Portal</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-xs text-gray-500">GWL Supervisor</span>
            <div className="w-8 h-8 bg-blue-600 rounded-full flex items-center justify-center text-white text-xs font-bold">
              GS
            </div>
          </div>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto p-6">
          {children}
        </main>
      </div>
    </div>
  );
}
