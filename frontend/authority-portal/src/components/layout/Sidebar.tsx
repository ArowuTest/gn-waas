import { NavLink } from 'react-router-dom'
import { LayoutDashboard, Search, BookOpen, AlertTriangle, Briefcase, BarChart3, LogOut, Droplets, UserCheck, FileDown } from 'lucide-react'
import { useAuth } from '../../contexts/AuthContext'

const navItems = [
  { to: '/district',      icon: LayoutDashboard, label: 'My District',     roles: ['DISTRICT_MANAGER','AUDIT_SUPERVISOR','FIELD_OFFICER','FINANCE_ANALYST','READONLY_VIEWER'] },
  { to: '/accounts',      icon: Search,           label: 'Account Search',  roles: ['DISTRICT_MANAGER','AUDIT_SUPERVISOR','FIELD_OFFICER'] },
  { to: '/meter-reading', icon: BookOpen,          label: 'Meter Reading',   roles: ['FIELD_OFFICER','AUDIT_SUPERVISOR'] },
  { to: '/report-issue',  icon: AlertTriangle,     label: 'Report Issue',    roles: ['DISTRICT_MANAGER','AUDIT_SUPERVISOR','FIELD_OFFICER'] },
  { to: '/my-jobs',       icon: Briefcase,         label: 'My Jobs',         roles: ['FIELD_OFFICER','AUDIT_SUPERVISOR'] },
  { to: '/nrw',            icon: BarChart3,         label: 'NRW Summary',     roles: ['DISTRICT_MANAGER','FINANCE_ANALYST','READONLY_VIEWER'] },
  { to: '/job-assignment', icon: UserCheck,         label: 'Job Assignment',  roles: ['DISTRICT_MANAGER','AUDIT_SUPERVISOR'] },
  { to: '/reporting',      icon: FileDown,          label: 'Reports',         roles: ['DISTRICT_MANAGER','FINANCE_ANALYST','READONLY_VIEWER'] },
]

export default function Sidebar() {
  const { user, logout, hasRole } = useAuth()

  return (
    <aside className="w-64 bg-green-900 min-h-screen flex flex-col">
      {/* Logo */}
      <div className="p-6 border-b border-green-800">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-yellow-400 rounded-lg flex items-center justify-center">
            <Droplets className="w-5 h-5 text-green-900" />
          </div>
          <div>
            <div className="text-white font-bold text-sm">GN-WAAS</div>
            <div className="text-green-400 text-xs">Authority Portal</div>
          </div>
        </div>
      </div>

      {/* User info */}
      {user && (
        <div className="px-4 py-4 border-b border-green-800">
          <div className="bg-green-800 rounded-xl p-3">
            <div className="text-white text-sm font-semibold truncate">{user.full_name}</div>
            <div className="text-green-400 text-xs mt-0.5">{user.role?.replace(/_/g, ' ')}</div>
          </div>
        </div>
      )}

      {/* Nav */}
      <nav className="flex-1 px-3 py-4 space-y-1">
        {navItems
          .filter(item => item.roles.some(r => hasRole(r)))
          .map(({ to, icon: Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                  isActive
                    ? 'bg-yellow-400 text-green-900'
                    : 'text-green-200 hover:bg-green-800 hover:text-white'
                }`
              }
            >
              <Icon className="w-4 h-4 flex-shrink-0" />
              {label}
            </NavLink>
          ))}
      </nav>

      {/* Logout */}
      <div className="p-4 border-t border-green-800">
        <button
          onClick={logout}
          className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium text-green-300 hover:bg-green-800 hover:text-white transition-colors"
        >
          <LogOut className="w-4 h-4" />
          Sign Out
        </button>
      </div>
    </aside>
  )
}
