import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Search, Filter, RefreshCw, Eye } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { AlertLevelBadge, StatusBadge } from '../components/ui/Badge'
import { useAnomalies, useDistricts } from '../hooks/useQueries'
import { formatCurrency, formatRelativeTime } from '../lib/utils'
import apiClient from '../lib/api-client'

export function AnomaliesPage() {
  const navigate = useNavigate()
  const [filters, setFilters] = useState({
    district_id: '',
    level: '',
    type: '',
    status: 'OPEN',
    limit: 25,
    offset: 0,
  })

  const { data: districts } = useDistricts()
  const { data: anomaliesData, isLoading, refetch } = useAnomalies(filters)

  const anomalies = anomaliesData?.data || []
  const total = anomaliesData?.meta?.total || 0

  const handleTriggerScan = async () => {
    if (!filters.district_id) {
      alert('Please select a district first')
      return
    }
    try {
      await apiClient.post(`/sentinel/scan/${filters.district_id}`)
      alert('Sentinel scan triggered. Results will appear shortly.')
      refetch()
    } catch (err) {
      alert('Failed to trigger scan')
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1>Anomaly Flags</h1>
          <p className="text-gray-500 text-sm mt-1">
            Sentinel-detected billing anomalies and fraud indicators
          </p>
        </div>
        <div className="flex gap-2">
          <button onClick={() => refetch()} className="btn-secondary btn-sm">
            <RefreshCw size={14} /> Refresh
          </button>
          <button onClick={handleTriggerScan} className="btn-primary btn-sm">
            Run Sentinel Scan
          </button>
        </div>
      </div>

      {/* Filters */}
      <Card>
        <div className="flex flex-wrap gap-3">
          <select
            className="input w-48"
            value={filters.district_id}
            onChange={e => setFilters(f => ({ ...f, district_id: e.target.value, offset: 0 }))}
          >
            <option value="">All Districts</option>
            {districts?.map(d => (
              <option key={d.id} value={d.id}>{d.district_name}</option>
            ))}
          </select>

          <select
            className="input w-36"
            value={filters.level}
            onChange={e => setFilters(f => ({ ...f, level: e.target.value, offset: 0 }))}
          >
            <option value="">All Levels</option>
            <option value="CRITICAL">Critical</option>
            <option value="HIGH">High</option>
            <option value="MEDIUM">Medium</option>
            <option value="LOW">Low</option>
          </select>

          <select
            className="input w-48"
            value={filters.type}
            onChange={e => setFilters(f => ({ ...f, type: e.target.value, offset: 0 }))}
          >
            <option value="">All Types</option>
            <option value="SHADOW_BILL_VARIANCE">Shadow Bill Variance</option>
            <option value="PHANTOM_METER">Phantom Meter</option>
            <option value="GHOST_ACCOUNT">Ghost Account</option>
            <option value="CATEGORY_MISMATCH">Category Mismatch</option>
            <option value="DISTRICT_BALANCE">District Balance</option>
            <option value="ZERO_CONSUMPTION">Zero Consumption</option>
          </select>

          <select
            className="input w-36"
            value={filters.status}
            onChange={e => setFilters(f => ({ ...f, status: e.target.value, offset: 0 }))}
          >
            <option value="">All Statuses</option>
            <option value="OPEN">Open</option>
            <option value="IN_PROGRESS">In Progress</option>
            <option value="RESOLVED">Resolved</option>
            <option value="FALSE_POSITIVE">False Positive</option>
          </select>
        </div>
      </Card>

      {/* Table */}
      <Card noPadding>
        <div className="px-6 py-4 border-b border-gray-100 flex items-center justify-between">
          <h3 className="font-semibold text-gray-900">
            {total} anomalies found
          </h3>
        </div>

        {isLoading ? (
          <div className="p-12 text-center text-gray-400">Loading anomalies...</div>
        ) : anomalies.length === 0 ? (
          <div className="p-12 text-center">
            <div className="text-4xl mb-3">✓</div>
            <p className="text-gray-500 font-medium">No anomalies found</p>
            <p className="text-gray-400 text-sm mt-1">
              {filters.status === 'OPEN' ? 'All clear for selected filters' : 'Try adjusting your filters'}
            </p>
          </div>
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Anomaly</th>
                <th>Level</th>
                <th>Type</th>
                <th>Est. Loss (GHS)</th>
                <th>Status</th>
                <th>Detected</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {anomalies.map(flag => (
                <tr
                  key={flag.id}
                  className="cursor-pointer"
                  onClick={() => navigate(`/anomalies/${flag.id}`)}
                >
                  <td>
                    <div className="max-w-xs">
                      <p className="font-medium text-gray-900 text-sm">{flag.title}</p>
                      <p className="text-gray-400 text-xs mt-0.5 truncate">{flag.description}</p>
                    </div>
                  </td>
                  <td><AlertLevelBadge level={flag.alert_level} /></td>
                  <td>
                    <span className="text-xs font-mono bg-gray-100 px-2 py-0.5 rounded">
                      {flag.anomaly_type}
                    </span>
                  </td>
                  <td className="font-mono text-sm font-medium text-danger">
                    {flag.estimated_loss_ghs ? formatCurrency(flag.estimated_loss_ghs) : '—'}
                  </td>
                  <td><StatusBadge status={flag.status} /></td>
                  <td className="text-gray-400 text-xs">{formatRelativeTime(flag.created_at)}</td>
                  <td>
                    <button className="btn-ghost btn-sm">
                      <Eye size={14} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        {/* Pagination — always shown when there are results */}
        {total > 0 && (
          <div className="px-6 py-4 border-t border-gray-100 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <p className="text-sm text-gray-500">
                Showing {Math.min(filters.offset + 1, total)}–{Math.min(filters.offset + filters.limit, total)} of {total}
              </p>
              <select
                className="input input-sm w-24 text-xs"
                value={filters.limit}
                onChange={e => setFilters(f => ({ ...f, limit: Number(e.target.value), offset: 0 }))}
              >
                <option value={10}>10 / page</option>
                <option value={25}>25 / page</option>
                <option value={50}>50 / page</option>
                <option value={100}>100 / page</option>
              </select>
            </div>
            <div className="flex items-center gap-2">
              <button
                className="btn-secondary btn-sm"
                disabled={filters.offset === 0}
                onClick={() => setFilters(f => ({ ...f, offset: 0 }))}
              >
                «
              </button>
              <button
                className="btn-secondary btn-sm"
                disabled={filters.offset === 0}
                onClick={() => setFilters(f => ({ ...f, offset: Math.max(0, f.offset - f.limit) }))}
              >
                Previous
              </button>
              <span className="text-sm text-gray-500 px-2">
                Page {Math.floor(filters.offset / filters.limit) + 1} of {Math.ceil(total / filters.limit)}
              </span>
              <button
                className="btn-secondary btn-sm"
                disabled={filters.offset + filters.limit >= total}
                onClick={() => setFilters(f => ({ ...f, offset: f.offset + f.limit }))}
              >
                Next
              </button>
              <button
                className="btn-secondary btn-sm"
                disabled={filters.offset + filters.limit >= total}
                onClick={() => setFilters(f => ({ ...f, offset: (Math.ceil(total / f.limit) - 1) * f.limit }))}
              >
                »
              </button>
            </div>
          </div>
        )}
      </Card>
    </div>
  )
}
