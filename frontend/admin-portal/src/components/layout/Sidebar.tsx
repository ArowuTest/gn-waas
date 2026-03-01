import { NavLink, useNavigate } from 'react-router-dom'
import {
  LayoutDashboard, AlertTriangle, ClipboardList, Users,
  MapPin, Settings, LogOut, Droplets, BarChart3,
  FileText, Shield, ChevronDown, Smartphone
} from 'lucide-react'
import { useAuth } from '../../contexts/AuthContext'
import { cn } from '../../lib/utils'
import { useState } from 'react'

interface NavItem {
  label: string
  to: string
  icon: React.ReactNode
  roles?: string[]
  children?: { label: string; to: string }[]
}

const navItems: NavItem[] = [
  {
    label: 'Dashboard',
    to: '/dashboard',
    icon: <LayoutDashboard size={18} />,
  },
  {
    label: 'Anomaly Flags',
    to: '/anomalies',
    icon: <AlertTriangle size={18} />,
  },
  {
    label: 'Audit Events',
    to: '/audits',
    icon: <ClipboardList size={18} />,
  },
  {
    label: 'Field Jobs',
    to: '/field-jobs',
    icon: <MapPin size={18} />,
  },
  {
    label: 'Mobile App',
    to: '/mobile-app',
    icon: <Smartphone size={18} />,
    roles: ['SYSTEM_ADMIN'],
  },
  {
    label: 'NRW Analysis',
    to: '/nrw',
    icon: <BarChart3 size={18} />,
    roles: ['SYSTEM_ADMIN', 'DISTRICT_MANAGER', 'FINANCE_ANALYST'],
  },
  {
    label: 'GRA Compliance',
    to: '/gra',
    icon: <Shield size={18} />,
    roles: ['SYSTEM_ADMIN', 'GRA_LIAISON', 'FINANCE_ANALYST'],
  },
  {
    label: 'Reports',
    to: '/reports',
    icon: <FileText size={18} />,
  },
  {
    label: 'Users',
    to: '/users',
    icon: <Users size={18} />,
    roles: ['SYSTEM_ADMIN'],
  },
  {
    label: 'Settings',
    to: '/settings',
    icon: <Settings size={18} />,
    roles: ['SYSTEM_ADMIN'],
  },
]

export function Sidebar() {
  const { user, logout, hasRole } = useAuth()
  const navigate = useNavigate()
  const [collapsed, setCollapsed] = useState(false)

  const visibleItems = navItems.filter(item =>
    !item.roles || hasRole(...item.roles)
  )

  return (
    <aside className={cn(
      'flex flex-col h-screen bg-white border-r border-gray-200 transition-all duration-200',
      collapsed ? 'w-16' : 'w-64'
    )}>
      {/* Logo */}
      <div className="flex items-center gap-3 px-4 py-5 border-b border-gray-100">
        <div className="flex-shrink-0 w-8 h-8 bg-brand-500 rounded-lg flex items-center justify-center">
          <Droplets size={18} className="text-white" />
        </div>
        {!collapsed && (
          <div>
            <p className="text-sm font-bold text-gray-900">GN-WAAS</p>
            <p className="text-xs text-gray-400">Water Audit System</p>
          </div>
        )}
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="ml-auto text-gray-400 hover:text-gray-600"
        >
          <ChevronDown size={16} className={cn('transition-transform', collapsed ? '-rotate-90' : 'rotate-90')} />
        </button>
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto py-4 px-2 space-y-1">
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
            {!collapsed && <span>{item.label}</span>}
          </NavLink>
        ))}
      </nav>

      {/* User profile */}
      <div className="border-t border-gray-100 p-3">
        {!collapsed ? (
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-brand-100 rounded-full flex items-center justify-center flex-shrink-0">
              <span className="text-brand-700 text-xs font-bold">
                {user?.full_name?.charAt(0) || 'U'}
              </span>
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-gray-900 truncate">{user?.full_name}</p>
              <p className="text-xs text-gray-400 truncate">{user?.role?.replace(/_/g, ' ')}</p>
            </div>
            <button
              onClick={logout}
              className="text-gray-400 hover:text-danger transition-colors"
              title="Logout"
            >
              <LogOut size={16} />
            </button>
          </div>
        ) : (
          <button
            onClick={logout}
            className="w-full flex justify-center text-gray-400 hover:text-danger"
            title="Logout"
          >
            <LogOut size={18} />
          </button>
        )}
      </div>
    </aside>
  )
}
