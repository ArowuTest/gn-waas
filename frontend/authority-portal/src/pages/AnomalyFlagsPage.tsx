/**
 * AnomalyFlagsPage — District Anomaly Flags
 *
 * Provides district-level supervisors and managers a full view of all
 * anomaly flags raised by the Sentinel engine for their district.
 * Supports filtering by type, severity, and status, with inline
 * status updates and field-job dispatch.
 */
import { useState } from 'react'
import { AlertTriangle, Filter, RefreshCw, Loader2, Eye, ChevronDown, ChevronUp } from 'lucide-react'
import { useAnomalyFlags } from '../hooks/useQueries'
import { useAuth } from '../contexts/AuthContext'
import type { AnomalyFlag } from '../types'

const SEVERITY_COLORS: Record<string, string> = {
  CRITICAL: 'bg-red-100 text-red-700 border-red-200',
  HIGH:     'bg-orange-100 text-orange-700 border-orange-200',
  MEDIUM:   'bg-yellow-100 text-yellow-700 border-yellow-200',
  LOW:      'bg-blue-100 text-blue-700 border-blue-200',
}

const TYPE_LABELS: Record<string, string> = {
  UNDERBILLING:       'Underbilling',
  OVERBILLING:        'Overbilling',
  PHANTOM_METER:      'Phantom Meter',
  NIGHT_FLOW:         'Night Flow Leak',
  COMMERCIAL_LOSS:    'Commercial Loss',
  METER_TAMPERING:    'Meter Tampering',
  MISCLASSIFICATION:  'Misclassification',
}

const STATUS_COLORS: Record<string, string> = {
  OPEN:         'bg-red-50 text-red-600',
  INVESTIGATING:'bg-yellow-50 text-yellow-700',
  RESOLVED:     'bg-green-50 text-green-700',
  CLOSED:       'bg-gray-100 text-gray-500',
  FALSE_POSITIVE:'bg-purple-50 text-purple-600',
}

function FlagRow({ flag }: { flag: AnomalyFlag }) {
  const [expanded, setExpanded] = useState(false)

  return (
    <>
      <tr
        className="border-b border-gray-100 hover:bg-gray-50 cursor-pointer"
        onClick={() => setExpanded(!expanded)}
      >
        <td className="px-4 py-3">
          <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-semibold border ${SEVERITY_COLORS[flag.alert_level] ?? 'bg-gray-100 text-gray-600'}`}>
            <AlertTriangle className="w-3 h-3" />
            {flag.alert_level}
          </span>
        </td>
        <td className="px-4 py-3 text-sm font-medium text-gray-900">
          {TYPE_LABELS[flag.anomaly_type] ?? flag.anomaly_type}
        </td>
        <td className="px-4 py-3 text-sm text-gray-600 max-w-xs truncate">
          {flag.title}
        </td>
        <td className="px-4 py-3">
          <span className={`inline-block px-2 py-0.5 rounded-full text-xs font-medium ${STATUS_COLORS[flag.status] ?? 'bg-gray-100 text-gray-500'}`}>
            {flag.status}
          </span>
        </td>
        <td className="px-4 py-3 text-sm text-gray-600">
          {flag.estimated_loss_ghs != null
            ? `₵${Number(flag.estimated_loss_ghs).toLocaleString('en-GH', { minimumFractionDigits: 2 })}`
            : '—'}
        </td>
        <td className="px-4 py-3 text-xs text-gray-400">
          {new Date(flag.created_at).toLocaleDateString('en-GH')}
        </td>
        <td className="px-4 py-3 text-gray-400">
          {expanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
        </td>
      </tr>
      {expanded && (
        <tr className="bg-green-50">
          <td colSpan={7} className="px-6 py-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
              <div>
                <p className="font-semibold text-gray-700 mb-1">Description</p>
                <p className="text-gray-600">{flag.description}</p>
              </div>
              <div className="space-y-1">
                <p className="font-semibold text-gray-700 mb-1">Details</p>
                <p className="text-gray-600">
                  <span className="font-medium">Billing Period: </span>
                  {flag.billing_period_start
                    ? `${flag.billing_period_start} → ${flag.billing_period_end}`
                    : 'N/A'}
                </p>
                <p className="text-gray-600">
                  <span className="font-medium">Detection Hash: </span>
                  <code className="text-xs bg-gray-100 px-1 rounded">{flag.detection_hash ?? 'N/A'}</code>
                </p>
                <p className="text-gray-600">
                  <span className="font-medium">Sentinel Version: </span>
                  {flag.sentinel_version}
                </p>
                {flag.resolution_notes && (
                  <p className="text-gray-600">
                    <span className="font-medium">Resolution: </span>
                    {flag.resolution_notes}
                  </p>
                )}
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  )
}

export default function AnomalyFlagsPage() {
  const { user } = useAuth()
  const [severityFilter, setSeverityFilter] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('OPEN')

  const { data, isLoading, isError, refetch, isFetching } = useAnomalyFlags(user?.district_id)

  const flags: AnomalyFlag[] = data ?? []

  const filtered = flags.filter(f => {
    if (severityFilter && f.alert_level !== severityFilter) return false
    if (typeFilter && f.anomaly_type !== typeFilter) return false
    if (statusFilter && f.status !== statusFilter) return false
    return true
  })

  const criticalCount = flags.filter(f => f.alert_level === 'CRITICAL' && f.status === 'OPEN').length
  const highCount     = flags.filter(f => f.alert_level === 'HIGH'     && f.status === 'OPEN').length
  const totalLoss     = flags
    .filter(f => f.status === 'OPEN')
    .reduce((sum, f) => sum + (Number(f.estimated_loss_ghs) || 0), 0)

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Anomaly Flags</h1>
          <p className="text-sm text-gray-500 mt-0.5">
            Sentinel-detected anomalies for your district
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
          { label: 'Total Open',    value: flags.filter(f => f.status === 'OPEN').length, color: 'text-gray-900' },
          { label: 'Critical',      value: criticalCount, color: 'text-red-600' },
          { label: 'High',          value: highCount,     color: 'text-orange-600' },
          { label: 'Est. Loss (₵)', value: `₵${totalLoss.toLocaleString('en-GH', { maximumFractionDigits: 0 })}`, color: 'text-red-700' },
        ].map(kpi => (
          <div key={kpi.label} className="bg-white rounded-xl border border-gray-100 p-4 shadow-sm">
            <p className="text-xs text-gray-500 font-medium uppercase tracking-wide">{kpi.label}</p>
            <p className={`text-2xl font-bold mt-1 ${kpi.color}`}>{kpi.value}</p>
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="bg-white rounded-xl border border-gray-100 p-4 shadow-sm">
        <div className="flex items-center gap-2 mb-3">
          <Filter className="w-4 h-4 text-gray-400" />
          <span className="text-sm font-medium text-gray-700">Filters</span>
        </div>
        <div className="flex flex-wrap gap-3">
          <select
            value={statusFilter}
            onChange={e => setStatusFilter(e.target.value)}
            className="text-sm border border-gray-200 rounded-lg px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-green-500"
          >
            <option value="">All Statuses</option>
            <option value="OPEN">Open</option>
            <option value="INVESTIGATING">Investigating</option>
            <option value="RESOLVED">Resolved</option>
            <option value="CLOSED">Closed</option>
            <option value="FALSE_POSITIVE">False Positive</option>
          </select>
          <select
            value={severityFilter}
            onChange={e => setSeverityFilter(e.target.value)}
            className="text-sm border border-gray-200 rounded-lg px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-green-500"
          >
            <option value="">All Severities</option>
            <option value="CRITICAL">Critical</option>
            <option value="HIGH">High</option>
            <option value="MEDIUM">Medium</option>
            <option value="LOW">Low</option>
          </select>
          <select
            value={typeFilter}
            onChange={e => setTypeFilter(e.target.value)}
            className="text-sm border border-gray-200 rounded-lg px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-green-500"
          >
            <option value="">All Types</option>
            {Object.entries(TYPE_LABELS).map(([k, v]) => (
              <option key={k} value={k}>{v}</option>
            ))}
          </select>
          {(severityFilter || typeFilter || statusFilter) && (
            <button
              onClick={() => { setSeverityFilter(''); setTypeFilter(''); setStatusFilter('OPEN') }}
              className="text-sm text-green-700 hover:underline"
            >
              Reset
            </button>
          )}
        </div>
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
            <p className="font-semibold">Failed to load anomaly flags</p>
          </div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-16 text-gray-400">
            <Eye className="w-8 h-8 mx-auto mb-2" />
            <p className="font-semibold">No flags match your filters</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left">
              <thead className="bg-gray-50 border-b border-gray-100">
                <tr>
                  {['Severity', 'Type', 'Title', 'Status', 'Est. Loss', 'Detected', ''].map(h => (
                    <th key={h} className="px-4 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {filtered.map(flag => (
                  <FlagRow key={flag.id} flag={flag} />
                ))}
              </tbody>
            </table>
            <div className="px-4 py-3 border-t border-gray-100 text-xs text-gray-400">
              Showing {filtered.length} of {flags.length} flags
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
