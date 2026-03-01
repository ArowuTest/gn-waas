/**
 * GN-WAAS Authority Portal — Reporting Page
 *
 * Download official PDF and CSV monthly audit reports from the server.
 * Reports are generated server-side with real DB data and are suitable
 * for regulatory submission to PURC, GRA, and Ministry of Finance.
 */

import { useState } from 'react'
import { FileText, Download, Calendar, AlertTriangle, CheckCircle, Loader2 } from 'lucide-react'
import apiClient from '../lib/api-client'

function getPeriodOptions(): { label: string; value: string }[] {
  const options = []
  const now = new Date()
  for (let i = 0; i < 12; i++) {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1)
    const value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
    const label = d.toLocaleDateString('en-GB', { month: 'long', year: 'numeric' })
    options.push({ label, value })
  }
  return options
}

type DownloadStatus = 'idle' | 'loading' | 'success' | 'error'

export default function ReportingPage() {
  const [period, setPeriod] = useState(() => {
    const d = new Date()
    d.setMonth(d.getMonth() - 1)
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
  })
  const [pdfStatus, setPdfStatus] = useState<DownloadStatus>('idle')
  const [csvStatus, setCsvStatus] = useState<DownloadStatus>('idle')
  const [errorMsg, setErrorMsg] = useState('')

  const periodOptions = getPeriodOptions()

  const downloadFile = async (format: 'pdf' | 'csv') => {
    const setter = format === 'pdf' ? setPdfStatus : setCsvStatus
    setter('loading')
    setErrorMsg('')
    try {
      const response = await apiClient.get(`/reports/monthly/${format}`, {
        params: { period },
        responseType: 'blob',
      })
      const blob = new Blob([response.data], {
        type: format === 'pdf' ? 'application/pdf' : 'text/csv',
      })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `GN-WAAS-Monthly-Report-${period}.${format}`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      setter('success')
      setTimeout(() => setter('idle'), 3000)
    } catch (err) {
      setter('error')
      setErrorMsg(`Failed to download ${format.toUpperCase()} report. Please try again.`)
      setTimeout(() => setter('idle'), 5000)
    }
  }

  const selectedPeriodLabel = periodOptions.find(p => p.value === period)?.label ?? period

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Official Reports</h1>
        <p className="text-gray-500 text-sm mt-1">
          Download server-generated audit reports for regulatory submission
        </p>
      </div>

      {/* Period Selector */}
      <div className="bg-white rounded-xl border border-gray-100 p-6 shadow-sm">
        <div className="flex items-center gap-3 mb-4">
          <Calendar className="w-5 h-5 text-green-700" />
          <h2 className="text-base font-semibold text-gray-900">Select Report Period</h2>
        </div>
        <select
          value={period}
          onChange={e => setPeriod(e.target.value)}
          className="w-full md:w-72 border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-green-500"
        >
          {periodOptions.map(opt => (
            <option key={opt.value} value={opt.value}>{opt.label}</option>
          ))}
        </select>
      </div>

      {/* Download Cards */}
      <div className="grid md:grid-cols-2 gap-4">
        {/* PDF Report */}
        <div className="bg-white rounded-xl border border-gray-100 p-6 shadow-sm">
          <div className="flex items-center gap-3 mb-3">
            <div className="w-10 h-10 bg-red-50 rounded-xl flex items-center justify-center">
              <FileText className="w-5 h-5 text-red-600" />
            </div>
            <div>
              <h3 className="font-semibold text-gray-900">PDF Report</h3>
              <p className="text-xs text-gray-500">Official regulatory document</p>
            </div>
          </div>
          <p className="text-sm text-gray-600 mb-4">
            Branded PDF with KPI summary, case breakdown table, financial impact analysis,
            and digital audit trail footer. Suitable for PURC, GRA, and MoF submission.
          </p>
          <div className="text-xs text-gray-400 mb-4 space-y-1">
            <p>✓ Total anomaly flags &amp; severity breakdown</p>
            <p>✓ Revenue recovered vs. unrecovered</p>
            <p>✓ Field job completion rate</p>
            <p>✓ Regulatory compliance statement</p>
          </div>
          <button
            onClick={() => downloadFile('pdf')}
            disabled={pdfStatus === 'loading'}
            className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-red-600 text-white text-sm rounded-lg hover:bg-red-700 disabled:opacity-50 transition-colors"
          >
            {pdfStatus === 'loading' ? (
              <><Loader2 className="w-4 h-4 animate-spin" /> Generating PDF...</>
            ) : pdfStatus === 'success' ? (
              <><CheckCircle className="w-4 h-4" /> Downloaded!</>
            ) : (
              <><Download className="w-4 h-4" /> Download PDF — {selectedPeriodLabel}</>
            )}
          </button>
        </div>

        {/* CSV Report */}
        <div className="bg-white rounded-xl border border-gray-100 p-6 shadow-sm">
          <div className="flex items-center gap-3 mb-3">
            <div className="w-10 h-10 bg-green-50 rounded-xl flex items-center justify-center">
              <FileText className="w-5 h-5 text-green-700" />
            </div>
            <div>
              <h3 className="font-semibold text-gray-900">CSV Export</h3>
              <p className="text-xs text-gray-500">Excel-compatible data export</p>
            </div>
          </div>
          <p className="text-sm text-gray-600 mb-4">
            BOM-prefixed CSV with all KPIs, financial metrics, and case counts.
            Opens directly in Microsoft Excel and Google Sheets for further analysis.
          </p>
          <div className="text-xs text-gray-400 mb-4 space-y-1">
            <p>✓ All KPI metrics in tabular format</p>
            <p>✓ Financial summary with recovery rate</p>
            <p>✓ Excel-compatible (BOM prefix)</p>
            <p>✓ Suitable for data analysis &amp; dashboards</p>
          </div>
          <button
            onClick={() => downloadFile('csv')}
            disabled={csvStatus === 'loading'}
            className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-green-700 text-white text-sm rounded-lg hover:bg-green-800 disabled:opacity-50 transition-colors"
          >
            {csvStatus === 'loading' ? (
              <><Loader2 className="w-4 h-4 animate-spin" /> Generating CSV...</>
            ) : csvStatus === 'success' ? (
              <><CheckCircle className="w-4 h-4" /> Downloaded!</>
            ) : (
              <><Download className="w-4 h-4" /> Download CSV — {selectedPeriodLabel}</>
            )}
          </button>
        </div>
      </div>

      {/* Error Message */}
      {errorMsg && (
        <div className="bg-red-50 border border-red-200 rounded-xl p-4 flex items-center gap-3">
          <AlertTriangle className="w-5 h-5 text-red-500 flex-shrink-0" />
          <p className="text-red-700 text-sm">{errorMsg}</p>
        </div>
      )}

      {/* Info Box */}
      <div className="bg-blue-50 border border-blue-100 rounded-xl p-4">
        <p className="text-blue-800 text-sm font-medium mb-1">About These Reports</p>
        <p className="text-blue-700 text-sm">
          Reports are generated server-side using live data from the GN-WAAS database.
          All figures are based on the PURC 2026 tiered tariff schedule and GRA E-VAT compliance rules.
          Reports are suitable for official submission to regulatory bodies.
        </p>
      </div>
    </div>
  )
}
