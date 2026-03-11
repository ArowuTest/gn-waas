import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard, Search, BookOpen, AlertTriangle, Briefcase,
  BarChart3, LogOut, Droplets, UserCheck, FileDown, Flag,
  ClipboardList, Users, ChevronLeft, ChevronRight
} from 'lucide-react'
import { useAuth } from '../../contexts/AuthContext'
import { useState } from 'react'
import { cn } from '../../lib/utils'

const navGroups = [
  {
    label: 'Overview',
    items: [
      {
        to: '/district',
        icon: LayoutDashboard,
        label: 'My District',
        roles: ['GRA_OFFICER', 'GWL_MANAGER', 'FIELD_SUPERVISOR', 'FIELD_OFFICER', 'MOF_AUDITOR', 'MINISTER_VIEW', 'GWL_ANALYST', 'GWL_EXECUTIVE'],
      },
      {
        to: '/nrw',
        icon: BarChart3,
        label: 'NRW Summary',
        roles: ['GRA_OFFICER', 'GWL_MANAGER', 'MOF_AUDITOR', 'MINISTER_VIEW', 'GWL_ANALYST', 'GWL_EXECUTIVE'],
      },
    ],
  },
  {
    label: 'Field Work',
    items: [
      {
        to: '/accounts',
        icon: Search,
        label: 'Account Search',
        roles: ['GRA_OFFICER', 'GWL_MANAGER', 'FIELD_SUPERVISOR', 'FIELD_OFFICER', 'GWL_ANALYST'],
      },
      {
        to: '/meter-reading',
        icon: BookOpen,
        label: 'Meter Reading',
        roles: ['FIELD_OFFICER', 'FIELD_SUPERVISOR'],
      },
      {
        to: '/report-issue',
        icon: AlertTriangle,
        label: 'Report Issue',
        roles: ['GRA_OFFICER', 'GWL_MANAGER', 'FIELD_SUPERVISOR', 'FIELD_OFFICER'],
      },
      {
        to: '/my-jobs',
        icon: Briefcase,
        label: 'My Jobs',
        roles: ['FIELD_OFFICER', 'FIELD_SUPERVISOR'],
      },
    ],
  },
  {
    label: 'Management',
    items: [
      {
        to: '/job-assignment',
        icon: UserCheck,
        label: 'Job Assignment',
        roles: ['GRA_OFFICER', 'GWL_MANAGER', 'FIELD_SUPERVISOR'],
      },
      {
        to: '/field-officers',
        icon: Users,
        label: 'Field Officers',
        roles: ['GRA_OFFICER', 'GWL_MANAGER', 'FIELD_SUPERVISOR'],
      },
      {
        to: '/anomaly-flags',
        icon: Flag,
        label: 'Anomaly Flags',
        roles: ['GRA_OFFICER', 'GWL_MANAGER', 'FIELD_SUPERVISOR', 'MOF_AUDITOR', 'GWL_ANALYST', 'GWL_EXECUTIVE'],
      },
      {
        to: '/audit-events',
        icon: ClipboardList,
        label: 'Audit Events',
        roles: ['GRA_OFFICER', 'GWL_MANAGER', 'MOF_AUDITOR', 'MINISTER_VIEW', 'GWL_ANALYST', 'GWL_EXECUTIVE'],
      },
      {
        to: '/reporting',
        icon: FileDown,
        label: 'Reports',
        roles: ['GRA_OFFICER', 'GWL_MANAGER', 'MOF_AUDITOR', 'MINISTER_VIEW', 'GWL_ANALYST', 'GWL_EXECUTIVE'],
      },
    ],
  },
]

export default function Sidebar() {
  const { user, logout, hasRole } = useAuth()
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
        <div className="flex-shrink-0 w-9 h-9 bg-emerald-600 rounded-xl flex items-center justify-center shadow-lg">
          <Droplets size={18} className="text-white" />
        </div>
        {!collapsed && (
          <div className="flex-1 min-w-0">
            <p className="text-sm font-bold text-white leading-tight">GN-WAAS</p>
            <p className="text-xs text-gray-400 leading-tight">Authority Portal</p>
          </div>
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto py-4 scrollbar-thin">
        {navGroups.map((group) => {
          const visibleItems = group.items.filter(item =>
            item.roles.some(r => hasRole(r))
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
                {visibleItems.map(({ to, icon: Icon, label }) => (
                  <NavLink
                    key={to}
                    to={to}
                    className={({ isActive }) =>
                      cn(
                        'flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium transition-all duration-150',
                        isActive
                          ? 'bg-emerald-600 text-white shadow-sm'
                          : 'text-gray-400 hover:bg-gray-800 hover:text-white'
                      )
                    }
                    title={collapsed ? label : undefined}
                  >
                    <Icon size={17} className="flex-shrink-0" />
                    {!collapsed && <span className="truncate">{label}</span>}
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
            <div className="w-8 h-8 bg-emerald-600 rounded-xl flex items-center justify-center flex-shrink-0 shadow-sm">
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
