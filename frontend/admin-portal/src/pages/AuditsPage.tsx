import { useState } from 'react'
import { RefreshCw, Eye, X, ExternalLink } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { StatusBadge, GRAStatusBadge } from '../components/ui/Badge'
import { useAuditEvents, useDistricts } from '../hooks/useQueries'
import { formatCurrency, formatDate } from '../lib/utils'

export function AuditsPage() {
  const [selectedAudit, setSelectedAudit] = useState<typeof audits[0] | null>(null)
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
    <>
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
          <span className="text-xs text-gray-400 italic">Audits are created automatically by Sentinel</span>
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
            <option value="IN_PROGRESS">In Progress</option>
            <option value="AWAITING_GRA">Awaiting GRA</option>
            <option value="GRA_CONFIRMED">GRA Confirmed</option>
            <option value="GRA_FAILED">GRA Failed</option>
            <option value="COMPLETED">Completed</option>
            <option value="DISPUTED">Disputed</option>
            <option value="ESCALATED">Escalated</option>
            <option value="CLOSED">Closed</option>
            <option value="PENDING_COMPLIANCE">Pending Compliance</option>
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
              {!filters.district_id ? 'Showing all districts — use the filter to narrow results' : 'No audits match your filters'}
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
                  onClick={() => setSelectedAudit(audit)}
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
                    <button className="btn-ghost btn-sm" onClick={() => setSelectedAudit(audit)}>
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

    {/* Audit Detail Modal */}
    {selectedAudit && (
      <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
        <div className="bg-white rounded-2xl shadow-2xl w-full max-w-2xl max-h-[90vh] overflow-y-auto">
          <div className="flex items-center justify-between p-6 border-b border-gray-100">
            <div>
              <h2 className="text-lg font-bold text-gray-900">Audit Detail</h2>
              <p className="text-sm font-mono text-brand-600 mt-0.5">{selectedAudit.audit_reference}</p>
            </div>
            <button onClick={() => setSelectedAudit(null)} className="text-gray-400 hover:text-gray-600">
              <X size={20} />
            </button>
          </div>
          <div className="p-6 space-y-4">
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div><span className="text-gray-500">Status</span><div className="mt-1"><StatusBadge status={selectedAudit.status} /></div></div>
              <div><span className="text-gray-500">GRA Status</span><div className="mt-1"><GRAStatusBadge status={selectedAudit.gra_status} /></div></div>
              <div><span className="text-gray-500">GWL Billed</span><div className="mt-1 font-mono font-medium">{selectedAudit.gwl_billed_ghs ? formatCurrency(selectedAudit.gwl_billed_ghs) : '—'}</div></div>
              <div><span className="text-gray-500">Shadow Bill</span><div className="mt-1 font-mono font-medium">{selectedAudit.shadow_bill_ghs ? formatCurrency(selectedAudit.shadow_bill_ghs) : '—'}</div></div>
              <div><span className="text-gray-500">Variance</span><div className={`mt-1 font-mono font-bold ${selectedAudit.variance_pct != null && Math.abs(selectedAudit.variance_pct) > 15 ? 'text-danger' : 'text-success'}`}>{selectedAudit.variance_pct != null ? `${selectedAudit.variance_pct > 0 ? '+' : ''}${selectedAudit.variance_pct.toFixed(1)}%` : '—'}</div></div>
              <div><span className="text-gray-500">Created</span><div className="mt-1">{formatDate(selectedAudit.created_at)}</div></div>
              {selectedAudit.confirmed_loss_ghs != null && (
                <div><span className="text-gray-500">Confirmed Loss</span><div className="mt-1 font-mono font-medium text-danger">{formatCurrency(selectedAudit.confirmed_loss_ghs)}</div></div>
              )}
              {selectedAudit.gra_receipt_number && (
                <div><span className="text-gray-500">GRA Receipt</span><div className="mt-1 font-mono text-xs">{selectedAudit.gra_receipt_number}</div></div>
              )}
            </div>
            {selectedAudit.notes && (
              <div className="bg-gray-50 rounded-lg p-3">
                <p className="text-xs text-gray-500 mb-1">Notes</p>
                <p className="text-sm text-gray-700">{selectedAudit.notes}</p>
              </div>
            )}
          </div>
          <div className="px-6 pb-6 flex justify-end">
            <button onClick={() => setSelectedAudit(null)} className="btn-secondary btn-sm">Close</button>
          </div>
        </div>
      </div>
    )}
    </>
  )
}