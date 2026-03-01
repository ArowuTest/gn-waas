import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { RefreshCw, Plus, Eye } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { StatusBadge, GRAStatusBadge } from '../components/ui/Badge'
import { useAuditEvents, useDistricts } from '../hooks/useQueries'
import { formatCurrency, formatDate } from '../lib/utils'

export function AuditsPage() {
  const navigate = useNavigate()
  const [filters, setFilters] = useState({
    district_id: '',
    status: '',
    limit: 25,
    offset: 0,
  })

  const { data: districts } = useDistricts()
  const { data: auditsData, isLoading, refetch } = useAuditEvents(
    filters.district_id ? filters : { ...filters, district_id: '' }
  )

  const audits = auditsData?.data || []
  const total = auditsData?.meta?.total || 0

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1>Audit Events</h1>
          <p className="text-gray-500 text-sm mt-1">
            Full audit lifecycle — from anomaly detection to GRA sign-off
          </p>
        </div>
        <div className="flex gap-2">
          <button onClick={() => refetch()} className="btn-secondary btn-sm">
            <RefreshCw size={14} /> Refresh
          </button>
          <button
            onClick={() => navigate('/audits/new')}
            className="btn-primary btn-sm"
          >
            <Plus size={14} /> New Audit
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
            className="input w-40"
            value={filters.status}
            onChange={e => setFilters(f => ({ ...f, status: e.target.value, offset: 0 }))}
          >
            <option value="">All Statuses</option>
            <option value="PENDING">Pending</option>
            <option value="ASSIGNED">Assigned</option>
            <option value="IN_PROGRESS">In Progress</option>
            <option value="COMPLETED">Completed</option>
            <option value="CLOSED">Closed</option>
          </select>
        </div>
      </Card>

      {/* Table */}
      <Card noPadding>
        <div className="px-6 py-4 border-b border-gray-100">
          <h3 className="font-semibold text-gray-900">{total} audit events</h3>
        </div>

        {isLoading ? (
          <div className="p-12 text-center text-gray-400">Loading audits...</div>
        ) : audits.length === 0 ? (
          <div className="p-12 text-center">
            <p className="text-gray-500">No audit events found</p>
            <p className="text-gray-400 text-sm mt-1">
              {!filters.district_id ? 'Select a district to view audits' : 'No audits match your filters'}
            </p>
          </div>
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Reference</th>
                <th>Status</th>
                <th>GWL Billed</th>
                <th>Shadow Bill</th>
                <th>Variance</th>
                <th>GRA Status</th>
                <th>Created</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {audits.map(audit => (
                <tr
                  key={audit.id}
                  className="cursor-pointer"
                  onClick={() => navigate(`/audits/${audit.id}`)}
                >
                  <td>
                    <span className="font-mono text-sm font-medium text-brand-600">
                      {audit.audit_reference}
                    </span>
                  </td>
                  <td><StatusBadge status={audit.status} /></td>
                  <td className="font-mono text-sm">
                    {audit.gwl_billed_ghs ? formatCurrency(audit.gwl_billed_ghs) : '—'}
                  </td>
                  <td className="font-mono text-sm">
                    {audit.shadow_bill_ghs ? formatCurrency(audit.shadow_bill_ghs) : '—'}
                  </td>
                  <td>
                    {audit.variance_pct != null ? (
                      <span className={`font-mono text-sm font-medium ${
                        Math.abs(audit.variance_pct) > 15 ? 'text-danger' : 'text-success'
                      }`}>
                        {audit.variance_pct > 0 ? '+' : ''}{audit.variance_pct.toFixed(1)}%
                      </span>
                    ) : '—'}
                  </td>
                  <td><GRAStatusBadge status={audit.gra_status} /></td>
                  <td className="text-gray-400 text-xs">{formatDate(audit.created_at)}</td>
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
      </Card>
    </div>
  )
}
