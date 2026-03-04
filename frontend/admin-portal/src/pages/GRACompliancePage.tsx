/**
 * GRA Compliance Page — Admin Portal
 *
 * Displays GRA VSDC invoice signing status, QR-code receipts, and
 * compliance metrics for all districts. Data is fetched live from
 * the api-gateway /audits and /gwl/cases endpoints.
 *
 * No hardcoded data. All values come from the backend.
 */
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../lib/api-client'
import {
  CheckCircle, XCircle, Clock, AlertTriangle,
  Download, RefreshCw, Search, Filter
} from 'lucide-react'

// ─── Types ────────────────────────────────────────────────────────────────────

interface GRAComplianceRecord {
  id: string
  audit_reference: string
  // FIX: field names aligned with expanded GetByDistrict response
  account_number: string       // from water_accounts JOIN
  account_holder: string       // from water_accounts.customer_name JOIN
  district_name: string        // from districts JOIN
  gwl_billed_ghs: number | null  // invoice amount
  shadow_bill_ghs: number | null // shadow bill (used as VAT proxy)
  gra_sdc_id: string | null    // GRA receipt number
  gra_qr_code_url: string | null
  gra_signed_at: string | null // GRA lock timestamp
  gra_status: string           // SIGNED | PENDING | FAILED | EXEMPT
  status: string               // audit lifecycle status
  created_at: string
}

interface GRAComplianceSummary {
  total_invoices: number
  signed_count: number
  pending_count: number
  failed_count: number
  compliance_rate_pct: number
  total_vat_collected_ghs: number
  period: string
}

// ─── API Hooks ────────────────────────────────────────────────────────────────

function useGRACompliance(period: string, districtId: string, status: string) {
  return useQuery<{ data: { data: GRAComplianceRecord[]; meta?: { total?: number } } }>({
    queryKey: ['gra-compliance', period, districtId, status],
    enabled: !!districtId,
    queryFn: () =>
      apiClient.get('/audits', {
        params: {
          district_id: districtId,
          gra_status: status || undefined,
          limit: 100,
        },
      }),
    staleTime: 30_000,
  })
}

function useGRAComplianceSummary(period: string, districtId: string) {
  return useQuery<GRAComplianceSummary>({
    queryKey: ['gra-compliance-summary', period, districtId],
    enabled: !!districtId,
    queryFn: async () => {
      // Fetch all audit records for the period to compute summary client-side
      const res = await apiClient.get('/audits', {
        params: {
          district_id: districtId,
          limit: 500,
        },
      })
      const records: GRAComplianceRecord[] = res.data?.data?.data ?? []
      const signed   = records.filter(r => r.gra_status === 'SIGNED').length
      const pending  = records.filter(r => r.gra_status === 'PENDING').length
      const failed   = records.filter(r => r.gra_status === 'FAILED').length
      const total    = records.length
      const totalVat = records.reduce((sum, r) => sum + (r.vat_amount_ghs ?? 0), 0)
      return {
        total_invoices:          total,
        signed_count:            signed,
        pending_count:           pending,
        failed_count:            failed,
        compliance_rate_pct:     total > 0 ? (signed / total) * 100 : 0,
        total_vat_collected_ghs: totalVat,
        period,
      }
    },
    staleTime: 30_000,
  })
}

// ─── Status Badge ─────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: GRAComplianceRecord['status'] }) {
  const map = {
    SIGNED:  { icon: CheckCircle,    cls: 'bg-green-100 text-green-700',  label: 'Signed' },
    PENDING: { icon: Clock,          cls: 'bg-yellow-100 text-yellow-700', label: 'Pending' },
    FAILED:  { icon: XCircle,        cls: 'bg-red-100 text-red-700',      label: 'Failed' },
    EXEMPT:  { icon: AlertTriangle,  cls: 'bg-gray-100 text-gray-600',    label: 'Exempt' },
  }
  const { icon: Icon, cls, label } = map[status] ?? map.PENDING
  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${cls}`}>
      <Icon className="w-3 h-3" />
      {label}
    </span>
  )
}

// ─── Summary Cards ────────────────────────────────────────────────────────────

function SummaryCard({
  label, value, sub, color,
}: { label: string; value: string | number; sub?: string; color: string }) {
  return (
    <div className="bg-white rounded-xl border border-gray-200 p-5">
      <p className="text-xs text-gray-500 uppercase tracking-wide">{label}</p>
      <p className={`text-2xl font-bold mt-1 ${color}`}>{value}</p>
      {sub && <p className="text-xs text-gray-400 mt-0.5">{sub}</p>}
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export function GRACompliancePage() {
  const now = new Date()
  const defaultPeriod = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`

  const [period, setPeriod]       = useState(defaultPeriod)
  const [districtId, setDistrict] = useState('')
  const [statusFilter, setStatus] = useState('')
  const [search, setSearch]       = useState('')

  const { data: recordsData, isLoading, refetch } = useGRACompliance(period, districtId, statusFilter)
  const { data: summary = {} as Partial<GRAComplianceSummary> } = useGRAComplianceSummary(period, districtId)

  const records: GRAComplianceRecord[] = (recordsData as any)?.data?.data ?? []

  if (!districtId) {
    return (
      <div className="p-8 text-center text-gray-500">
        <p className="text-lg font-medium">Select a district to view GRA compliance records</p>
      </div>
    )
  }


  const filtered = records.filter(r =>
    !search ||
    r.audit_reference.toLowerCase().includes(search.toLowerCase()) ||
    (r.account_number ?? '').toLowerCase().includes(search.toLowerCase()) ||
    (r.account_holder ?? '').toLowerCase().includes(search.toLowerCase()) ||
    (r.gra_sdc_id ?? '').toLowerCase().includes(search.toLowerCase())
  )

  const handleExportCSV = () => {
    const rows = [
      ['Audit Ref', 'Account', 'Holder', 'District', 'Invoice (GHS)', 'VAT (GHS)', 'GRA Receipt', 'Status', 'Signed At'],
      ...filtered.map(r => [
        r.audit_reference,
        r.account_number ?? '',
        r.account_holder ?? '',
        r.district_name ?? '',
        (r.gwl_billed_ghs ?? 0).toFixed(2),
        (r.shadow_bill_ghs ?? 0).toFixed(2),
        r.gra_sdc_id ?? '',
        r.status,
        r.gra_signed_at ? new Date(r.gra_signed_at).toLocaleString() : '',
      ]),
    ]
    const csv = '\uFEFF' + rows.map(r => r.map(c => `"${c}"`).join(',')).join('\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `gra-compliance-${period}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-gray-900">GRA Compliance</h1>
          <p className="text-sm text-gray-500 mt-0.5">
            VSDC invoice signing status and VAT receipt tracking
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => refetch()}
            className="flex items-center gap-1.5 px-3 py-2 text-sm border border-gray-200 rounded-lg hover:bg-gray-50"
          >
            <RefreshCw className="w-4 h-4" /> Refresh
          </button>
          <button
            onClick={handleExportCSV}
            className="flex items-center gap-1.5 px-3 py-2 text-sm bg-brand-600 text-white rounded-lg hover:bg-brand-700"
          >
            <Download className="w-4 h-4" /> Export CSV
          </button>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <SummaryCard
          label="Compliance Rate"
          value={`${(summary.compliance_rate_pct ?? 0).toFixed(1)}%`}
          sub={`${summary.signed_count ?? 0} of ${summary.total_invoices ?? 0} signed`}
          color="text-green-600"
        />
        <SummaryCard
          label="Pending Signing"
          value={summary.pending_count ?? 0}
          sub="Awaiting GRA VSDC"
          color="text-yellow-600"
        />
        <SummaryCard
          label="Failed / Rejected"
          value={summary.failed_count ?? 0}
          sub="Require resubmission"
          color="text-red-600"
        />
        <SummaryCard
          label="VAT Collected"
          value={`₵${((summary.total_vat_collected_ghs ?? 0) / 1000).toFixed(1)}k`}
          sub={`Period: ${period}`}
          color="text-brand-600"
        />
      </div>

      {/* Filters */}
      <div className="bg-white rounded-xl border border-gray-200 p-4">
        <div className="flex flex-wrap gap-3">
          <div className="relative flex-1 min-w-48">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input
              type="text"
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="w-full pl-9 pr-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500"
              placeholder="Search by audit ref, account, receipt..."
            />
          </div>
          <input
            type="month"
            value={period}
            onChange={e => setPeriod(e.target.value)}
            className="px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
          <select
            value={statusFilter}
            onChange={e => setStatus(e.target.value)}
            className="px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            <option value="">All Statuses</option>
            <option value="SIGNED">Signed</option>
            <option value="PENDING">Pending</option>
            <option value="FAILED">Failed</option>
            <option value="EXEMPT">Exempt</option>
          </select>
        </div>
      </div>

      {/* Table */}
      <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-200">
              <tr>
                {['Audit Ref', 'Account', 'District', 'Invoice (GHS)', 'VAT (GHS)', 'GRA Receipt', 'Status', 'Signed At'].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wide">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {isLoading ? (
                <tr>
                  <td colSpan={8} className="px-4 py-12 text-center text-gray-400">
                    <div className="w-6 h-6 border-2 border-brand-500 border-t-transparent rounded-full animate-spin mx-auto mb-2" />
                    Loading compliance records...
                  </td>
                </tr>
              ) : filtered.length === 0 ? (
                <tr>
                  <td colSpan={8} className="px-4 py-12 text-center text-gray-400">
                    <Filter className="w-8 h-8 mx-auto mb-2 opacity-40" />
                    No records match the current filters
                  </td>
                </tr>
              ) : (
                filtered.map(r => (
                  <tr key={r.id} className="hover:bg-gray-50 transition-colors">
                    <td className="px-4 py-3 font-mono text-xs text-brand-600">{r.audit_reference}</td>
                    <td className="px-4 py-3">
                      <div className="font-medium text-gray-900">{r.account_number}</div>
                      <div className="text-xs text-gray-400">{r.account_holder}</div>
                    </td>
                    <td className="px-4 py-3 text-gray-600">{r.district_name}</td>
                    <td className="px-4 py-3 text-right font-medium">
                      ₵{(r.gwl_billed_ghs ?? 0).toFixed(2)}
                    </td>
                    <td className="px-4 py-3 text-right text-gray-600">
                      ₵{(r.shadow_bill_ghs ?? 0).toFixed(2)}
                    </td>
                    <td className="px-4 py-3 font-mono text-xs">
                      {r.gra_sdc_id ?? (
                        <span className="text-gray-300">—</span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={r.status} />
                    </td>
                    <td className="px-4 py-3 text-xs text-gray-400">
                      {r.gra_signed_at
                        ? new Date(r.gra_signed_at).toLocaleString()
                        : <span className="text-gray-300">—</span>
                      }
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        {filtered.length > 0 && (
          <div className="px-4 py-3 border-t border-gray-100 text-xs text-gray-400">
            Showing {filtered.length} record{filtered.length !== 1 ? 's' : ''}
          </div>
        )}
      </div>
    </div>
  )
}
