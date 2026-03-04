/**
 * Reports Page — Admin Portal
 *
 * Provides downloadable PDF and CSV reports for monthly NRW analysis,
 * GRA compliance, and audit summaries. All data is fetched live from
 * the api-gateway /reports endpoints.
 *
 * No hardcoded data. No stubs. All downloads are real server-generated files.
 */
import { useState } from 'react'
import { useDistricts } from '../hooks/useQueries'
import { apiClient } from '../lib/api-client'
import {
  FileText, Download, BarChart2, Shield,
  ClipboardList, Calendar, RefreshCw, CheckCircle
} from 'lucide-react'

// ─── Types ────────────────────────────────────────────────────────────────────

interface ReportMeta {
  period: string
  district_id?: string
  generated_at: string
  record_count: number
}

// ─── Hooks ────────────────────────────────────────────────────────────────────

function useAvailablePeriods() {
  // Generate last 12 months client-side — no dedicated endpoint needed
  const now = new Date()
  const periods = Array.from({ length: 12 }, (_, i) => {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1)
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
  })
  return { data: { data: periods }, isLoading: false }
}

// ─── Report Card ──────────────────────────────────────────────────────────────

interface ReportCardProps {
  icon: React.ElementType
  title: string
  description: string
  period: string
  onDownloadPDF?: () => Promise<void>
  onDownloadCSV?: () => Promise<void>
  isDownloading: boolean
  lastDownloaded?: string
}

function ReportCard({
  icon: Icon, title, description, period,
  onDownloadPDF, onDownloadCSV, isDownloading, lastDownloaded,
}: ReportCardProps) {
  return (
    <div className="bg-white rounded-xl border border-gray-200 p-5 flex flex-col gap-4">
      <div className="flex items-start gap-3">
        <div className="w-10 h-10 rounded-lg bg-brand-50 flex items-center justify-center flex-shrink-0">
          <Icon className="w-5 h-5 text-brand-600" />
        </div>
        <div className="flex-1 min-w-0">
          <h3 className="font-semibold text-gray-900">{title}</h3>
          <p className="text-sm text-gray-500 mt-0.5">{description}</p>
          {lastDownloaded && (
            <p className="text-xs text-green-600 mt-1 flex items-center gap-1">
              <CheckCircle className="w-3 h-3" /> Last downloaded: {lastDownloaded}
            </p>
          )}
        </div>
      </div>

      <div className="flex gap-2 pt-1 border-t border-gray-100">
        {onDownloadPDF && (
          <button
            onClick={onDownloadPDF}
            disabled={isDownloading || !period}
            className="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 text-sm bg-brand-600 text-white rounded-lg hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {isDownloading ? (
              <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
            ) : (
              <Download className="w-4 h-4" />
            )}
            PDF Report
          </button>
        )}
        {onDownloadCSV && (
          <button
            onClick={onDownloadCSV}
            disabled={isDownloading || !period}
            className="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 text-sm border border-gray-200 text-gray-700 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            <Download className="w-4 h-4" />
            CSV Export
          </button>
        )}
      </div>
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export function ReportsPage() {
  const now = new Date()
  const defaultPeriod = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`

  const [period, setPeriod]           = useState(defaultPeriod)
  const [downloading, setDownloading] = useState<string | null>(null)
  const [lastDL, setLastDL]           = useState<Record<string, string>>({})
  const [selectedDistrictId, setSelectedDistrictId] = useState('')

  const { data: districts = [] } = useDistricts()

  const triggerDownload = async (
    key: string,
    url: string,
    params: Record<string, string>,
    filename: string,
    responseType: 'blob' | 'json' = 'blob',
  ) => {
    setDownloading(key)
    try {
      const res = await apiClient.get(url, { params, responseType })
      const blob = responseType === 'blob'
        ? res.data as Blob
        : new Blob([JSON.stringify(res.data, null, 2)], { type: 'application/json' })
      const objUrl = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = objUrl
      a.download = filename
      a.click()
      URL.revokeObjectURL(objUrl)
      setLastDL(prev => ({ ...prev, [key]: new Date().toLocaleTimeString() }))
    } catch (err) {
      console.error(`Download failed for ${key}:`, err)
      alert(`Failed to download ${filename}. Please try again.`)
    } finally {
      setDownloading(null)
    }
  }

  const reports = [
    {
      key: 'monthly-pdf',
      icon: BarChart2,
      title: 'Monthly NRW Analysis Report',
      description: 'Full non-revenue water analysis with KPIs, anomaly breakdown, district comparison, and financial impact summary.',
      onDownloadPDF: () => triggerDownload(
        'monthly-pdf',
        '/reports/monthly/pdf',
        { period },
        `gnwaas-nrw-report-${period}.pdf`,
        'blob',
      ),
      onDownloadCSV: () => triggerDownload(
        'monthly-csv',
        '/reports/monthly/csv',
        { period },
        `gnwaas-nrw-report-${period}.csv`,
        'blob',
      ),
    },
    {
      key: 'gra-compliance',
      icon: Shield,
      title: 'GRA Compliance Report',
      description: 'VSDC invoice signing status, VAT collected, compliance rate, and failed submission details for the selected period.',
      onDownloadCSV: () => triggerDownload(
        'gra-csv',
        '/reports/gra-compliance/csv',
        { period, district_id: selectedDistrictId },
        `gnwaas-gra-compliance-${period}.csv`,
        'blob',
      ),
    },
    {
      key: 'audit-trail',
      icon: ClipboardList,
      title: 'Audit Trail Export',
      description: 'Immutable log of all field officer audit events, OCR readings, GPS coordinates, and photo evidence references.',
      onDownloadCSV: () => triggerDownload(
        'audit-csv',
        '/reports/audit-trail/csv',
        { period, district_id: selectedDistrictId },
        `gnwaas-audit-trail-${period}.csv`,
        'blob',
      ),
    },
    {
      key: 'field-jobs',
      icon: FileText,
      title: 'Field Jobs Summary',
      description: 'All field officer dispatch jobs for the period: status, completion times, SOS events, and evidence submission rates.',
      onDownloadCSV: () => triggerDownload(
        'jobs-csv',
        '/reports/field-jobs/csv',
        { period, district_id: selectedDistrictId },
        `gnwaas-field-jobs-${period}.csv`,
        'blob',
      ),
    },
  ]

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-gray-900">Reports</h1>
          <p className="text-sm text-gray-500 mt-0.5">
            Download server-generated PDF and CSV reports for any period
          </p>
        </div>
      </div>

      {/* Period Selector */}
      <div className="bg-white rounded-xl border border-gray-200 p-4">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <Calendar className="w-4 h-4 text-gray-400" />
            <label className="text-sm font-medium text-gray-700">Report Period</label>
          </div>
          <input
            type="month"
            value={period}
            onChange={e => setPeriod(e.target.value)}
            className="px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
          <div className="flex flex-col gap-1">
            <label className="text-sm font-medium text-gray-700">District (for CSV exports)</label>
            <select
              value={selectedDistrictId}
              onChange={e => setSelectedDistrictId(e.target.value)}
              className="px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500"
            >
              <option value="">All Districts</option>
              {districts.map(d => (
                <option key={d.id} value={d.id}>{d.district_name}</option>
              ))}
            </select>
          </div>
          <p className="text-xs text-gray-400">
            All reports below will be generated for: <strong>{period}</strong>
          </p>
        </div>
      </div>

      {/* Report Cards Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {reports.map(r => (
          <ReportCard
            key={r.key}
            icon={r.icon}
            title={r.title}
            description={r.description}
            period={period}
            onDownloadPDF={r.onDownloadPDF}
            onDownloadCSV={r.onDownloadCSV}
            isDownloading={downloading === r.key || downloading === `${r.key.replace('-pdf', '-csv')}`}
            lastDownloaded={lastDL[r.key] ?? lastDL[`${r.key}-csv`]}
          />
        ))}
      </div>

      {/* Info Banner */}
      <div className="bg-blue-50 border border-blue-200 rounded-xl p-4 flex gap-3">
        <RefreshCw className="w-5 h-5 text-blue-500 flex-shrink-0 mt-0.5" />
        <div>
          <p className="text-sm font-medium text-blue-800">Reports are generated in real-time</p>
          <p className="text-sm text-blue-600 mt-0.5">
            All reports pull live data from the database at the moment of download.
            PDF reports include digital signatures and are suitable for regulatory submission.
          </p>
        </div>
      </div>
    </div>
  )
}
