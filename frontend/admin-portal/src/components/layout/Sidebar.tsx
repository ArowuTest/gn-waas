import { NavLink, useNavigate } from 'react-router-dom'
import {
  LayoutDashboard, AlertTriangle, ClipboardList, Users,
  MapPin, Settings, LogOut, Droplets, BarChart3,
  FileText, Shield, Smartphone, Map, Building2,
  ChevronLeft, ChevronRight
} from 'lucide-react'
import { useAuth } from '../../contexts/AuthContext'
import { cn } from '../../lib/utils'
import { useState } from 'react'

interface NavItem {
  label: string
  to: string
  icon: React.ReactNode
  roles?: string[]
  badge?: string
}

const navGroups = [
  {
    label: 'Overview',
    items: [
      { label: 'Dashboard',    to: '/dashboard',  icon: <LayoutDashboard size={17} /> },
      { label: 'DMA Map',      to: '/dma-map',    icon: <Map size={17} />, roles: ['SYSTEM_ADMIN', 'MOF_AUDITOR', 'GWL_MANAGER'] },
    ],
  },
  {
    label: 'Operations',
    items: [
      { label: 'Anomaly Flags', to: '/anomalies',  icon: <AlertTriangle size={17} /> },
      { label: 'Audit Events',  to: '/audits',     icon: <ClipboardList size={17} /> },
      { label: 'Field Jobs',    to: '/field-jobs', icon: <MapPin size={17} /> },
      { label: 'NRW Analysis',  to: '/nrw',        icon: <BarChart3 size={17} />, roles: ['SYSTEM_ADMIN', 'GWL_MANAGER', 'MOF_AUDITOR'] },
    ],
  },
  {
    label: 'Compliance',
    items: [
      { label: 'GRA Compliance', to: '/gra',     icon: <Shield size={17} />, roles: ['SYSTEM_ADMIN', 'GRA_OFFICER', 'MOF_AUDITOR'] },
      { label: 'Reports',        to: '/reports', icon: <FileText size={17} /> },
    ],
  },
  {
    label: 'Administration',
    items: [
      { label: 'Users',      to: '/users',      icon: <Users size={17} />,      roles: ['SYSTEM_ADMIN'] },
      { label: 'Districts',  to: '/districts',  icon: <Building2 size={17} />,  roles: ['SYSTEM_ADMIN'] },
      { label: 'Mobile App', to: '/mobile-app', icon: <Smartphone size={17} />, roles: ['SYSTEM_ADMIN'] },
      { label: 'Settings',   to: '/settings',   icon: <Settings size={17} />,   roles: ['SYSTEM_ADMIN'] },
      { label: 'Tariffs',    to: '/tariffs',    icon: <Settings size={17} />,   roles: ['SYSTEM_ADMIN'] },
      { label: 'Gap Tracking', to: '/gaps',      icon: <BarChart3 size={17} /> },
      { label: 'Whistleblower', to: '/whistleblower', icon: <Shield size={17} />, roles: ['SYSTEM_ADMIN'] },
      { label: 'Donor KPIs',   to: '/donor-kpis',    icon: <FileText size={17} />, roles: ['SYSTEM_ADMIN', 'MOF_AUDITOR'] },
      { label: 'Sync Status',  to: '/sync-status',   icon: <Smartphone size={17} />, roles: ['SYSTEM_ADMIN'] },
    ],
  },
]

export function Sidebar() {
  const { user, logout, hasRole } = useAuth()
  const navigate = useNavigate()
  const [collapsed, setCollapsed] = useState(false)

  return (
    <aside className={cn(
      'flex flex-col h-screen bg-gray-900 transition-all duration-300 ease-in-out flex-shrink-0',
      collapsed ? 'w-[68px]' : 'w-60'
    )}>
      {/* Logo */}
      <div className={cn(
        'flex items-center border-b border-gray-800 h-16 flex-shrink-0',
        collapsed ? 'justify-center px-0' : 'px-4 gap-3'
      )}>
        <div className="flex-shrink-0 w-9 h-9 bg-brand-600 rounded-xl flex items-center justify-center shadow-lg">
          <Droplets size={18} className="text-white" />
        </div>
        {!collapsed && (
          <div className="flex-1 min-w-0">
            <p className="text-sm font-bold text-white leading-tight">GN-WAAS</p>
            <p className="text-xs text-gray-400 leading-tight">Admin Portal</p>
          </div>
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto py-4 scrollbar-thin">
        {navGroups.map((group) => {
          const visibleItems = group.items.filter(item =>
            !item.roles || hasRole(...item.roles)
          )
          if (visibleItems.length === 0) return null

          return (
            <div key={group.label} className={cn('mb-1', collapsed ? 'px-2' : 'px-3')}>
              {!collapsed && (
                <p className="text-[10px] font-bold text-gray-500 uppercase tracking-widest px-3 mb-1.5 mt-2">
                  {group.label}
                </p>
              )}
              {collapsed && <div className="border-t border-gray-800 my-2" />}
              <div className="space-y-0.5">
                {visibleItems.map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    className={({ isActive }) =>
                      cn('sidebar-link', isActive && 'active')
                    }
                    title={collapsed ? item.label : undefined}
                  >
                    <span className="flex-shrink-0">{item.icon}</span>
                    {!collapsed && <span className="truncate">{item.label}</span>}
                  </NavLink>
                ))}
              </div>
            </div>
          )
        })}
      </nav>

      {/* Collapse toggle */}
      <div className="border-t border-gray-800 p-2">
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="w-full flex items-center justify-center gap-2 px-3 py-2 rounded-xl text-gray-400 hover:text-white hover:bg-gray-800 transition-colors text-xs font-medium"
          title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          {collapsed ? <ChevronRight size={16} /> : (
            <>
              <ChevronLeft size={16} />
              <span>Collapse</span>
            </>
          )}
        </button>
      </div>

      {/* User profile */}
      <div className="border-t border-gray-800 p-3">
        {!collapsed ? (
          <div className="flex items-center gap-3 px-1">
            <div className="w-8 h-8 bg-brand-600 rounded-xl flex items-center justify-center flex-shrink-0 shadow-sm">
              <span className="text-white text-xs font-bold">
                {user?.full_name?.charAt(0)?.toUpperCase() || 'U'}
              </span>
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-xs font-semibold text-white truncate">{user?.full_name || 'User'}</p>
              <p className="text-[10px] text-gray-400 truncate">{user?.role?.replace(/_/g, ' ')}</p>
            </div>
            <button
              onClick={logout}
              className="text-gray-500 hover:text-red-400 transition-colors p-1 rounded-lg hover:bg-gray-800"
              title="Sign out"
            >
              <LogOut size={15} />
            </button>
          </div>
        ) : (
          <button
            onClick={logout}
            className="w-full flex justify-center text-gray-500 hover:text-red-400 transition-colors p-2 rounded-xl hover:bg-gray-800"
            title="Sign out"
          >
            <LogOut size={17} />
          </button>
        )}
      </div>
    </aside>
  )
}
