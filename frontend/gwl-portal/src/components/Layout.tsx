import React, { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { cn } from '../utils/helpers';
import {
  LayoutDashboard, ClipboardList, TrendingDown, TrendingUp,
  RefreshCw, MapPin, CreditCard, BarChart3, LogOut,
  Droplets, ChevronLeft, ChevronRight, Menu
} from 'lucide-react';

const NAV_GROUPS = [
  {
    label: 'Overview',
    items: [
      { path: '/',       label: 'Dashboard',     icon: LayoutDashboard },
      { path: '/cases',  label: 'All Cases',     icon: ClipboardList },
    ],
  },
  {
    label: 'Case Types',
    items: [
      { path: '/underbilling',      label: 'Underbilling',      icon: TrendingDown },
      { path: '/overbilling',       label: 'Overbilling',       icon: TrendingUp },
      { path: '/misclassification', label: 'Misclassification', icon: RefreshCw },
    ],
  },
  {
    label: 'Operations',
    items: [
      { path: '/field-assignments', label: 'Field Assignments', icon: MapPin },
      { path: '/credits',           label: 'Credit Requests',   icon: CreditCard },
      { path: '/reports',           label: 'Monthly Reports',   icon: BarChart3 },
    ],
  },
];

export function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const navigate = useNavigate();
  const [collapsed, setCollapsed] = useState(false);

  const handleLogout = () => {
    localStorage.removeItem('gwl_token');
    navigate('/login');
  };

  return (
    <div className="flex h-screen bg-slate-50 overflow-hidden">
      {/* Sidebar */}
      <aside className={cn(
        'flex flex-col bg-gray-900 transition-all duration-300 ease-in-out flex-shrink-0',
        collapsed ? 'w-[68px]' : 'w-60'
      )}>
        {/* Logo */}
        <div className={cn(
          'flex items-center border-b border-gray-800 h-16 flex-shrink-0',
          collapsed ? 'justify-center' : 'px-4 gap-3'
        )}>
          <div className="flex-shrink-0 w-9 h-9 bg-blue-600 rounded-xl flex items-center justify-center shadow-lg">
            <Droplets size={18} className="text-white" />
          </div>
          {!collapsed && (
            <div className="flex-1 min-w-0">
              <p className="text-sm font-bold text-white leading-tight">GWL Portal</p>
              <p className="text-xs text-gray-400 leading-tight">Case Management</p>
            </div>
          )}
        </div>

        {/* Nav */}
        <nav className="flex-1 overflow-y-auto py-4 scrollbar-thin">
          {NAV_GROUPS.map((group) => (
            <div key={group.label} className={cn('mb-1', collapsed ? 'px-2' : 'px-3')}>
              {!collapsed && (
                <p className="text-[10px] font-bold text-gray-500 uppercase tracking-widest px-3 mb-1.5 mt-2">
                  {group.label}
                </p>
              )}
              {collapsed && <div className="border-t border-gray-800 my-2" />}
              <div className="space-y-0.5">
                {group.items.map(({ path, label, icon: Icon }) => {
                  const active = location.pathname === path ||
                    (path !== '/' && location.pathname.startsWith(path));
                  return (
                    <Link
                      key={path}
                      to={path}
                      title={collapsed ? label : undefined}
                      className={cn(
                        'flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium transition-all duration-150',
                        active
                          ? 'bg-blue-600 text-white shadow-sm'
                          : 'text-gray-400 hover:bg-gray-800 hover:text-white'
                      )}
                    >
                      <Icon size={17} className="flex-shrink-0" />
                      {!collapsed && <span className="truncate">{label}</span>}
                    </Link>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>

        {/* Collapse toggle */}
        <div className="border-t border-gray-800 p-2">
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="w-full flex items-center justify-center gap-2 px-3 py-2 rounded-xl text-gray-400 hover:text-white hover:bg-gray-800 transition-colors text-xs font-medium"
          >
            {collapsed ? <ChevronRight size={16} /> : (
              <>
                <ChevronLeft size={16} />
                <span>Collapse</span>
              </>
            )}
          </button>
        </div>

        {/* Logout */}
        <div className="border-t border-gray-800 p-3">
          {!collapsed ? (
            <button
              onClick={handleLogout}
              className="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium text-gray-400 hover:text-red-400 hover:bg-gray-800 transition-colors"
            >
              <LogOut size={17} />
              <span>Sign Out</span>
            </button>
          ) : (
            <button
              onClick={handleLogout}
              className="w-full flex justify-center text-gray-500 hover:text-red-400 transition-colors p-2 rounded-xl hover:bg-gray-800"
              title="Sign out"
            >
              <LogOut size={17} />
            </button>
          )}
        </div>
      </aside>

      {/* Main */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Top bar */}
        <header className="bg-white border-b border-gray-100 px-6 h-14 flex items-center justify-between flex-shrink-0 shadow-sm">
          <div className="flex items-center gap-3">
            <div>
              <p className="text-sm font-bold text-gray-900 leading-tight">
                Ghana National Water Audit & Assurance System
              </p>
              <p className="text-xs text-gray-400">GWL Case Management Portal</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2 bg-gray-50 border border-gray-200 rounded-xl px-3 py-1.5">
              <div className="w-6 h-6 bg-blue-600 rounded-lg flex items-center justify-center text-white text-[10px] font-bold">
                GS
              </div>
              <span className="text-xs font-semibold text-gray-700">GWL Supervisor</span>
            </div>
          </div>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto scrollbar-thin animate-fade-in">
          {children}
        </main>
      </div>
    </div>
  );
}
