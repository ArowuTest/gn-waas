/**
 * FieldOfficersPage — District Field Officer Management
 *
 * Allows district managers and supervisors to view all field officers
 * assigned to their district, track their current job status, last
 * known location, and performance metrics. Provides a live operational
 * picture of field team deployment.
 */
import { useState } from 'react'
import { Users, MapPin, RefreshCw, Loader2, AlertTriangle, CheckCircle, Clock, Shield } from 'lucide-react'
import { useFieldOfficers } from '../hooks/useQueries'
import { useAuth } from '../contexts/AuthContext'
import type { User } from '../types'

const ROLE_LABELS: Record<string, string> = {
  FIELD_OFFICER:    'Field Officer',
  FIELD_SUPERVISOR: 'Field Supervisor',
}

const STATUS_COLORS: Record<string, string> = {
  ACTIVE:   'bg-green-100 text-green-700',
  INACTIVE: 'bg-gray-100 text-gray-500',
  PENDING:  'bg-yellow-100 text-yellow-700',
  SUSPENDED:'bg-red-100 text-red-700',
}

function OfficerCard({ officer }: { officer: User }) {
  const initials = officer.full_name
    .split(' ')
    .map(n => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)

  return (
    <div className="bg-white rounded-xl border border-gray-100 p-5 shadow-sm hover:border-green-200 transition-colors">
      {/* Header */}
      <div className="flex items-start gap-3 mb-4">
        <div className="w-10 h-10 bg-green-700 rounded-full flex items-center justify-center flex-shrink-0">
          <span className="text-white text-sm font-bold">{initials}</span>
        </div>
        <div className="flex-1 min-w-0">
          <p className="font-semibold text-gray-900 truncate">{officer.full_name}</p>
          <p className="text-xs text-gray-500 truncate">{officer.email}</p>
        </div>
        <span className={`inline-block px-2 py-0.5 rounded-full text-xs font-medium ${STATUS_COLORS[officer.status] ?? 'bg-gray-100 text-gray-500'}`}>
          {officer.status}
        </span>
      </div>

      {/* Details */}
      <div className="space-y-2 text-sm">
        <div className="flex items-center gap-2 text-gray-600">
          <Shield className="w-3.5 h-3.5 text-gray-400 flex-shrink-0" />
          <span>{ROLE_LABELS[officer.role] ?? officer.role}</span>
        </div>
        {officer.phone_number && (
          <div className="flex items-center gap-2 text-gray-600">
            <span className="text-gray-400 text-xs">📞</span>
            <span>{officer.phone_number}</span>
          </div>
        )}
        {officer.employee_id && (
          <div className="flex items-center gap-2 text-gray-600">
            <span className="text-gray-400 text-xs">🪪</span>
            <span className="font-mono text-xs">{officer.employee_id}</span>
          </div>
        )}
        {officer.last_login_at && (
          <div className="flex items-center gap-2 text-gray-500 text-xs">
            <Clock className="w-3 h-3 text-gray-400" />
            <span>Last active: {new Date(officer.last_login_at).toLocaleString('en-GH')}</span>
          </div>
        )}
        {officer.last_location_lat && officer.last_location_lng && (
          <div className="flex items-center gap-2 text-gray-500 text-xs">
            <MapPin className="w-3 h-3 text-gray-400" />
            <span>
              {(Number(officer.last_location_lat) || 0).toFixed(4)}, {(Number(officer.last_location_lng) || 0).toFixed(4)}
            </span>
          </div>
        )}
        {officer.is_mfa_enabled && (
          <div className="flex items-center gap-2 text-green-600 text-xs">
            <CheckCircle className="w-3 h-3" />
            <span>MFA Enabled</span>
          </div>
        )}
        {officer.device_id && (
          <div className="flex items-center gap-2 text-gray-500 text-xs">
            <span className="text-gray-400">📱</span>
            <span className="font-mono truncate">{officer.device_id}</span>
          </div>
        )}
      </div>
    </div>
  )
}

export default function FieldOfficersPage() {
  const { user } = useAuth()
  const [roleFilter, setRoleFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('ACTIVE')
  const [search, setSearch] = useState('')

  const { data, isLoading, isError, refetch, isFetching } = useFieldOfficers(user?.district_id)

  const officers: User[] = data ?? []

  const filtered = officers.filter(o => {
    if (roleFilter && o.role !== roleFilter) return false
    if (statusFilter && o.status !== statusFilter) return false
    if (search) {
      const q = search.toLowerCase()
      return (
        o.full_name.toLowerCase().includes(q) ||
        o.email.toLowerCase().includes(q) ||
        (o.employee_id ?? '').toLowerCase().includes(q)
      )
    }
    return true
  })

  const activeCount   = officers.filter(o => o.status === 'ACTIVE').length
  const mfaCount      = officers.filter(o => o.is_mfa_enabled).length
  const supervisorCount = officers.filter(o => o.role === 'FIELD_SUPERVISOR').length

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Field Officers</h1>
          <p className="text-sm text-gray-500 mt-0.5">
            District field team — {officers.length} officers registered
          </p>
        </div>
        <button
          onClick={() => refetch()}
          disabled={isFetching}
          className="flex items-center gap-2 px-4 py-2 bg-green-700 text-white rounded-lg text-sm font-medium hover:bg-green-800 disabled:opacity-50"
        >
          <RefreshCw className={`w-4 h-4 ${isFetching ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {/* KPI Strip */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          { label: 'Total Officers',  value: officers.length,   color: 'text-gray-900' },
          { label: 'Active',          value: activeCount,       color: 'text-green-700' },
          { label: 'Supervisors',     value: supervisorCount,   color: 'text-blue-700' },
          { label: 'MFA Enabled',     value: mfaCount,          color: 'text-purple-700' },
        ].map(kpi => (
          <div key={kpi.label} className="bg-white rounded-xl border border-gray-100 p-4 shadow-sm">
            <p className="text-xs text-gray-500 font-medium uppercase tracking-wide">{kpi.label}</p>
            <p className={`text-2xl font-bold mt-1 ${kpi.color}`}>{kpi.value}</p>
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="bg-white rounded-xl border border-gray-100 p-4 shadow-sm">
        <div className="flex flex-wrap gap-3 items-center">
          <input
            type="text"
            placeholder="Search by name, email, or employee ID…"
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="text-sm border border-gray-200 rounded-lg px-3 py-1.5 w-64 focus:outline-none focus:ring-2 focus:ring-green-500"
          />
          <select
            value={roleFilter}
            onChange={e => setRoleFilter(e.target.value)}
            className="text-sm border border-gray-200 rounded-lg px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-green-500"
          >
            <option value="">All Roles</option>
            <option value="FIELD_OFFICER">Field Officer</option>
            <option value="FIELD_SUPERVISOR">Field Supervisor</option>
          </select>
          <select
            value={statusFilter}
            onChange={e => setStatusFilter(e.target.value)}
            className="text-sm border border-gray-200 rounded-lg px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-green-500"
          >
            <option value="">All Statuses</option>
            <option value="ACTIVE">Active</option>
            <option value="INACTIVE">Inactive</option>
            <option value="PENDING">Pending</option>
            <option value="SUSPENDED">Suspended</option>
          </select>
          {(roleFilter || statusFilter !== 'ACTIVE' || search) && (
            <button
              onClick={() => { setRoleFilter(''); setStatusFilter('ACTIVE'); setSearch('') }}
              className="text-sm text-green-700 hover:underline"
            >
              Reset
            </button>
          )}
          <span className="text-xs text-gray-400 ml-auto">{filtered.length} shown</span>
        </div>
      </div>

      {/* Grid */}
      {isLoading ? (
        <div className="flex items-center justify-center py-16">
          <Loader2 className="w-8 h-8 animate-spin text-green-700" />
        </div>
      ) : isError ? (
        <div className="text-center py-16 text-red-500">
          <AlertTriangle className="w-8 h-8 mx-auto mb-2" />
          <p className="font-semibold">Failed to load field officers</p>
        </div>
      ) : filtered.length === 0 ? (
        <div className="text-center py-16 text-gray-400">
          <Users className="w-8 h-8 mx-auto mb-2" />
          <p className="font-semibold">No officers match your filters</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {filtered.map(officer => (
            <OfficerCard key={officer.id} officer={officer} />
          ))}
        </div>
      )}
    </div>
  )
}
