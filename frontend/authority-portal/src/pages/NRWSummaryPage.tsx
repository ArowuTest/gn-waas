import { exportNRWSummaryCSV, exportNRWSummaryPDF } from '../utils/exportUtils'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ReferenceLine, ResponsiveContainer } from 'recharts'
import { Loader2, AlertTriangle, RefreshCw } from 'lucide-react'
import { useNRWSummary } from '../hooks/useQueries'
import type { NRWSummaryRow } from '../hooks/useQueries'

const gradeColor: Record<string, string> = {
  A: 'bg-green-100 text-green-700',
  B: 'bg-blue-100 text-blue-700',
  C: 'bg-yellow-100 text-yellow-700',
  D: 'bg-orange-100 text-orange-700',
  F: 'bg-red-100 text-red-700',
  'N/A': 'bg-gray-100 text-gray-500',
}

function NRWStatusLabel({ lossRatio }: { lossRatio?: number }) {
  if (lossRatio === undefined || lossRatio === null) return <span className="text-gray-400">No data</span>
  if (lossRatio > 50) return <span className="text-red-600 font-medium">⚠ Above Ghana Average</span>
  if (lossRatio > 30) return <span className="text-orange-600 font-medium">↗ Above IWA Target</span>
  if (lossRatio > 20) return <span className="text-yellow-600 font-medium">→ Near Target</span>
  return <span className="text-green-700 font-medium">✓ World Class</span>
}

export default function NRWSummaryPage() {
  const { data: summaries, isLoading, isError, refetch, isFetching } = useNRWSummary()

  // Prepare chart data from real API response
  const chartData = (summaries ?? [])
    .filter(s => s.loss_ratio_pct !== undefined && s.loss_ratio_pct !== null)
    .map(s => ({
      district: s.district_name.replace(' District', '').replace(' Metropolitan', ''),
      nrw: Number((s.loss_ratio_pct ?? 0).toFixed(1)),
      grade: s.grade,
      estimated_loss: s.total_estimated_loss_ghs,
    }))
    .sort((a, b) => b.nrw - a.nrw)

  if (isLoading) {
    return (
      <div className="p-6 flex items-center justify-center min-h-64">
        <Loader2 className="w-8 h-8 animate-spin text-green-700" />
      </div>
    )
  }

  if (isError) {
    return (
      <div className="p-6 max-w-4xl mx-auto">
        <div className="bg-red-50 border border-red-200 rounded-xl p-6 text-center">
          <AlertTriangle className="w-8 h-8 text-red-500 mx-auto mb-2" />
          <p className="text-red-700 font-semibold">Failed to load NRW data</p>
          <button onClick={() => refetch()} className="mt-3 text-sm text-red-600 underline">
            Try again
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-black text-gray-900 mb-1">NRW Summary</h1>
          <p className="text-gray-500 text-sm">IWA/AWWA Water Balance — District Performance (Last 30 Days)</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => exportNRWSummaryCSV(summaries ?? [])}
            className="px-3 py-1.5 text-xs font-medium bg-white border border-gray-200 rounded-lg hover:bg-gray-50 flex items-center gap-1"
            title="Export to CSV"
          >
            📥 CSV
          </button>
          <button
            onClick={() => exportNRWSummaryPDF(summaries ?? [], 'District NRW Performance')}
            className="px-3 py-1.5 text-xs font-medium bg-white border border-gray-200 rounded-lg hover:bg-gray-50 flex items-center gap-1"
            title="Export to PDF"
          >
            🖨 PDF
          </button>
          <button
            onClick={() => refetch()}
            disabled={isFetching}
            className="p-2 rounded-lg border border-gray-200 hover:bg-gray-50 disabled:opacity-50"
            title="Refresh"
          >
            <RefreshCw className={`w-4 h-4 text-gray-500 ${isFetching ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Summary KPIs */}
      {summaries && summaries.length > 0 && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          {[
            {
              label: 'Total Open Anomalies',
              value: summaries.reduce((s, r) => s + r.open_anomalies, 0).toLocaleString(),
              color: 'text-red-600',
            },
            {
              label: 'Critical Flags',
              value: summaries.reduce((s, r) => s + r.critical_anomalies, 0).toLocaleString(),
              color: 'text-red-700',
            },
            {
              label: 'Est. Loss (GHS)',
              value: `₵${(summaries.reduce((s, r) => s + r.total_estimated_loss_ghs, 0) / 1000).toFixed(0)}K`,
              color: 'text-orange-600',
            },
            {
              label: 'Recovered (GHS)',
              value: `₵${(summaries.reduce((s, r) => s + r.total_recovered_ghs, 0) / 1000).toFixed(0)}K`,
              color: 'text-green-700',
            },
          ].map(({ label, value, color }) => (
            <div key={label} className="bg-white rounded-xl p-4 border border-gray-100 shadow-sm">
              <div className={`text-2xl font-black ${color}`}>{value}</div>
              <div className="text-xs text-gray-500 mt-1">{label}</div>
            </div>
          ))}
        </div>
      )}

      {/* Chart */}
      {chartData.length > 0 && (
        <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm mb-6">
          <h2 className="font-bold text-gray-900 mb-4">NRW % by District</h2>
          <ResponsiveContainer width="100%" height={280}>
            <BarChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis dataKey="district" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 12 }} unit="%" domain={[0, 70]} />
              <Tooltip
                formatter={(value) => [`${value ?? 0}%`, 'NRW']}
                labelFormatter={(label) => `District: ${label}`}
              />
              <ReferenceLine
                y={20}
                stroke="#16a34a"
                strokeDasharray="4 4"
                label={{ value: 'IWA Target 20%', fill: '#16a34a', fontSize: 11 }}
              />
              <ReferenceLine
                y={51.6}
                stroke="#dc2626"
                strokeDasharray="4 4"
                label={{ value: 'Ghana Avg 51.6%', fill: '#dc2626', fontSize: 11 }}
              />
              <Bar dataKey="nrw" fill="#2e7d32" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Table */}
      {summaries && summaries.length > 0 ? (
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                {['District', 'NRW %', 'Open Flags', 'Est. Loss (GHS)', 'Grade', 'Status'].map(h => (
                  <th key={h} className="text-left px-4 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {summaries.map((row: NRWSummaryRow) => (
                <tr key={row.district_id} className="hover:bg-gray-50">
                  <td className="px-4 py-3">
                    <div className="font-semibold text-gray-900">{row.district_name}</div>
                    <div className="text-xs text-gray-400">{row.region}</div>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`font-bold ${
                      (row.loss_ratio_pct ?? 0) > 50 ? 'text-red-600' :
                      (row.loss_ratio_pct ?? 0) > 30 ? 'text-orange-600' :
                      'text-green-700'
                    }`}>
                      {row.loss_ratio_pct !== undefined && row.loss_ratio_pct !== null
                        ? `${row.loss_ratio_pct.toFixed(1)}%`
                        : 'N/A'}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="font-semibold text-gray-900">{row.open_anomalies.toLocaleString()}</div>
                    {row.critical_anomalies > 0 && (
                      <div className="text-xs text-red-600">{row.critical_anomalies} critical</div>
                    )}
                  </td>
                  <td className="px-4 py-3 font-semibold text-gray-900">
                    ₵{row.total_estimated_loss_ghs.toLocaleString('en-GH', { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`font-bold px-2 py-0.5 rounded text-xs ${gradeColor[row.grade] ?? 'bg-gray-100 text-gray-500'}`}>
                      {row.grade}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <NRWStatusLabel lossRatio={row.loss_ratio_pct} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="bg-white rounded-2xl border border-gray-100 p-12 text-center">
          <p className="text-gray-500">No NRW data available for the current period</p>
        </div>
      )}
    </div>
  )
}
