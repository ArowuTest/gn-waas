/**
 * AuditEventsPage — District Audit Events Tracker
 *
 * Allows district managers and supervisors to track all audit events
 * in their district: pending, in-progress, awaiting GRA confirmation,
 * and completed. Provides a full lifecycle view with financial outcomes.
 */
import { useState } from 'react'
import { ClipboardList, RefreshCw, Loader2, AlertTriangle, CheckCircle, Clock, DollarSign } from 'lucide-react'
import { useAuditEvents } from '../hooks/useQueries'
import { useAuth } from '../contexts/AuthContext'
import type { AuditEvent } from '../types'

const STATUS_CONFIG: Record<string, { label: string; color: string; icon: React.ReactNode }> = {
  PENDING:             { label: 'Pending',           color: 'bg-gray-100 text-gray-600',    icon: <Clock className="w-3 h-3" /> },
  IN_PROGRESS:         { label: 'In Progress',       color: 'bg-blue-100 text-blue-700',    icon: <ClipboardList className="w-3 h-3" /> },
  AWAITING_GRA:        { label: 'Awaiting GRA',      color: 'bg-yellow-100 text-yellow-700',icon: <Clock className="w-3 h-3" /> },
  GRA_CONFIRMED:       { label: 'GRA Confirmed',     color: 'bg-green-100 text-green-700',  icon: <CheckCircle className="w-3 h-3" /> },
  GRA_FAILED:          { label: 'GRA Failed',        color: 'bg-red-100 text-red-700',      icon: <AlertTriangle className="w-3 h-3" /> },
  COMPLETED:           { label: 'Completed',         color: 'bg-green-100 text-green-700',  icon: <CheckCircle className="w-3 h-3" /> },
  DISPUTED:            { label: 'Disputed',          color: 'bg-orange-100 text-orange-700',icon: <AlertTriangle className="w-3 h-3" /> },
  ESCALATED:           { label: 'Escalated',         color: 'bg-red-100 text-red-700',      icon: <AlertTriangle className="w-3 h-3" /> },
  CLOSED:              { label: 'Closed',            color: 'bg-gray-100 text-gray-500',    icon: <CheckCircle className="w-3 h-3" /> },
  PENDING_COMPLIANCE:  { label: 'Pending Compliance',color: 'bg-purple-100 text-purple-700',icon: <Clock className="w-3 h-3" /> },
}

const GRA_STATUS_COLORS: Record<string, string> = {
  PENDING:  'text-gray-500',
  SIGNED:   'text-green-600',
  FAILED:   'text-red-600',
  RETRYING: 'text-yellow-600',
  EXEMPT:   'text-blue-600',
}

function AuditRow({ event }: { event: AuditEvent }) {
  const cfg = STATUS_CONFIG[event.status] ?? { label: event.status, color: 'bg-gray-100 text-gray-600', icon: null }
  const variancePct = event.variance_pct != null ? Number(event.variance_pct) : null
  const isHighVariance = variancePct != null && variancePct > 15

  return (
    <tr className="border-b border-gray-100 hover:bg-gray-50">
      <td className="px-4 py-3">
        <span className="font-mono text-xs text-green-700 font-semibold">{event.audit_reference}</span>
      </td>
      <td className="px-4 py-3">
        <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${cfg.color}`}>
          {cfg.icon}
          {cfg.label}
        </span>
      </td>
      <td className="px-4 py-3 text-sm text-gray-600">
        {event.gwl_billed_ghs != null
          ? `₵${Number(event.gwl_billed_ghs).toLocaleString('en-GH', { minimumFractionDigits: 2 })}`
          : '—'}
      </td>
      <td className="px-4 py-3 text-sm text-gray-600">
        {event.shadow_bill_ghs != null
          ? `₵${Number(event.shadow_bill_ghs).toLocaleString('en-GH', { minimumFractionDigits: 2 })}`
          : '—'}
      </td>
      <td className="px-4 py-3 text-sm">
        {variancePct != null ? (
          <span className={`font-semibold ${isHighVariance ? 'text-red-600' : 'text-gray-600'}`}>
            {(variancePct ?? 0).toFixed(1)}%
          </span>
        ) : '—'}
      </td>
      <td className="px-4 py-3 text-sm">
        <span className={`text-xs font-medium ${GRA_STATUS_COLORS[event.gra_status] ?? 'text-gray-500'}`}>
          {event.gra_status}
        </span>
      </td>
      <td className="px-4 py-3 text-xs text-gray-400">
        {new Date(event.created_at).toLocaleDateString('en-GH')}
      </td>
      <td className="px-4 py-3 text-xs text-gray-400">
        {event.due_date ? new Date(event.due_date).toLocaleDateString('en-GH') : '—'}
      </td>
    </tr>
  )
}

export default function AuditEventsPage() {
  const { user } = useAuth()
  const [statusFilter, setStatusFilter] = useState('')
  const [page, setPage] = useState(1)

  const { data, isLoading, isError, refetch, isFetching } = useAuditEvents(user?.district_id, page)

  const events: AuditEvent[] = data?.events ?? []
  const total: number = data?.total ?? 0
  const pageSize = 20
  const totalPages = Math.ceil(total / pageSize)

  const filtered = statusFilter ? events.filter(e => e.status === statusFilter) : events

  // KPIs
  const pending     = events.filter(e => e.status === 'PENDING').length
  const inProgress  = events.filter(e => e.status === 'IN_PROGRESS').length
  const awaitingGRA = events.filter(e => e.status === 'AWAITING_GRA').length
  const completed   = events.filter(e => e.status === 'COMPLETED').length
  const totalRecovered = events
    .filter(e => e.confirmed_loss_ghs != null)
    .reduce((sum, e) => sum + (Number(e.confirmed_loss_ghs) || 0), 0)

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Audit Events</h1>
          <p className="text-sm text-gray-500 mt-0.5">Full lifecycle tracking for your district</p>
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
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
        {[
          { label: 'Pending',      value: pending,     color: 'text-gray-700' },
          { label: 'In Progress',  value: inProgress,  color: 'text-blue-700' },
          { label: 'Awaiting GRA', value: awaitingGRA, color: 'text-yellow-700' },
          { label: 'Completed',    value: completed,   color: 'text-green-700' },
          { label: 'Confirmed Loss', value: `₵${totalRecovered.toLocaleString('en-GH', { maximumFractionDigits: 0 })}`, color: 'text-red-700' },
        ].map(kpi => (
          <div key={kpi.label} className="bg-white rounded-xl border border-gray-100 p-4 shadow-sm">
            <p className="text-xs text-gray-500 font-medium uppercase tracking-wide">{kpi.label}</p>
            <p className={`text-2xl font-bold mt-1 ${kpi.color}`}>{kpi.value}</p>
          </div>
        ))}
      </div>

      {/* Filter */}
      <div className="bg-white rounded-xl border border-gray-100 p-4 shadow-sm flex items-center gap-3">
        <ClipboardList className="w-4 h-4 text-gray-400" />
        <span className="text-sm font-medium text-gray-700">Filter by status:</span>
        <select
          value={statusFilter}
          onChange={e => { setStatusFilter(e.target.value); setPage(1) }}
          className="text-sm border border-gray-200 rounded-lg px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-green-500"
        >
          <option value="">All Statuses</option>
          {Object.entries(STATUS_CONFIG).map(([k, v]) => (
            <option key={k} value={k}>{v.label}</option>
          ))}
        </select>
        <span className="text-xs text-gray-400 ml-auto">{total} total events</span>
      </div>

      {/* Table */}
      <div className="bg-white rounded-xl border border-gray-100 shadow-sm overflow-hidden">
        {isLoading ? (
          <div className="flex items-center justify-center py-16">
            <Loader2 className="w-8 h-8 animate-spin text-green-700" />
          </div>
        ) : isError ? (
          <div className="text-center py-16 text-red-500">
            <AlertTriangle className="w-8 h-8 mx-auto mb-2" />
            <p className="font-semibold">Failed to load audit events</p>
          </div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-16 text-gray-400">
            <DollarSign className="w-8 h-8 mx-auto mb-2" />
            <p className="font-semibold">No audit events found</p>
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full text-left">
                <thead className="bg-gray-50 border-b border-gray-100">
                  <tr>
                    {['Reference', 'Status', 'GWL Bill', 'Shadow Bill', 'Variance', 'GRA', 'Created', 'Due'].map(h => (
                      <th key={h} className="px-4 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {filtered.map(event => (
                    <AuditRow key={event.id} event={event} />
                  ))}
                </tbody>
              </table>
            </div>
            {/* Pagination */}
            {totalPages > 1 && (
              <div className="flex items-center justify-between px-4 py-3 border-t border-gray-100">
                <button
                  onClick={() => setPage(p => Math.max(1, p - 1))}
                  disabled={page === 1}
                  className="text-sm text-green-700 hover:underline disabled:text-gray-300"
                >
                  ← Previous
                </button>
                <span className="text-xs text-gray-500">Page {page} of {totalPages}</span>
                <button
                  onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                  disabled={page === totalPages}
                  className="text-sm text-green-700 hover:underline disabled:text-gray-300"
                >
                  Next →
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
