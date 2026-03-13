import { useState, useMemo } from 'react'
import { exportNRWSummaryCSV, exportNRWSummaryPDF } from '../utils/exportUtils'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ReferenceLine, ResponsiveContainer } from 'recharts'
import { Loader2, AlertTriangle, RefreshCw, Filter } from 'lucide-react'
import { useNRWSummary, useDistricts } from '../hooks/useQueries'
import type { NRWSummaryRow } from '../hooks/useQueries'

const gradeColor: Record<string, string> = {
  A: 'bg-green-100 text-green-700',
  B: 'bg-blue-100 text-blue-700',
  C: 'bg-yellow-100 text-yellow-700',
  D: 'bg-orange-100 text-orange-700',
  F: 'bg-red-100 text-red-700',
  'N/A': 'bg-gray-100 text-gray-500',
}

const PERIODS = [
  { label: 'Last 30 Days',  value: '30d' },
  { label: 'Last 90 Days',  value: '90d' },
  { label: 'Last 6 Months', value: '6m'  },
  { label: 'Last 12 Months',value: '12m' },
]

const GRADES = ['A', 'B', 'C', 'D', 'F']

function NRWStatusLabel({ lossRatio }: { lossRatio?: number }) {
  if (lossRatio === undefined || lossRatio === null) return <span className="text-gray-400">No data</span>
  if (lossRatio > 50) return <span className="text-red-600 font-medium">⚠ Above Ghana Average</span>
  if (lossRatio > 30) return <span className="text-orange-600 font-medium">↗ Above IWA Target</span>
  if (lossRatio > 20) return <span className="text-yellow-600 font-medium">→ Near Target</span>
  return <span className="text-green-700 font-medium">✓ World Class</span>
}

export default function NRWSummaryPage() {
  // ── Filter state ──────────────────────────────────────────────────────────
  const [period, setPeriod]           = useState('30d')
  const [districtFilter, setDistrict] = useState('')
  const [gradeFilter, setGrade]       = useState('')
  const [minNRW, setMinNRW]           = useState('')
  const [maxNRW, setMaxNRW]           = useState('')
  const [showFilters, setShowFilters] = useState(false)
  const [sortBy, setSortBy]           = useState<'nrw_desc' | 'nrw_asc' | 'name' | 'loss_desc'>('nrw_desc')

  const { data: summaries, isLoading, isError, refetch, isFetching } = useNRWSummary(
    districtFilter || undefined,
    period,
  )
  const { data: districts = [] } = useDistricts()

  // ── Client-side filtering & sorting ──────────────────────────────────────
  const filtered = useMemo(() => {
    let rows = summaries ?? []
    if (gradeFilter) rows = rows.filter(r => r.grade === gradeFilter)
    if (minNRW !== '') rows = rows.filter(r => (r.loss_ratio_pct ?? 0) >= parseFloat(minNRW))
    if (maxNRW !== '') rows = rows.filter(r => (r.loss_ratio_pct ?? 0) <= parseFloat(maxNRW))

    return [...rows].sort((a, b) => {
      switch (sortBy) {
        case 'nrw_desc':  return (b.loss_ratio_pct ?? -Infinity) - (a.loss_ratio_pct ?? -Infinity)
        case 'nrw_asc':   return (a.loss_ratio_pct ?? Infinity)  - (b.loss_ratio_pct ?? Infinity)
        case 'name':      return a.district_name.localeCompare(b.district_name)
        case 'loss_desc': return b.total_estimated_loss_ghs - a.total_estimated_loss_ghs
        default:          return 0
      }
    })
  }, [summaries, gradeFilter, minNRW, maxNRW, sortBy])

  const chartData = filtered
    .filter(s => s.loss_ratio_pct != null)
    .slice(0, 15)
    .map(s => ({
      district: s.district_name.replace(' District', '').replace(' Metropolitan', ''),
      nrw: Number((s.loss_ratio_pct ?? 0).toFixed(1)),
      grade: s.grade,
    }))

  const activeFilterCount = [districtFilter, gradeFilter, minNRW, maxNRW].filter(Boolean).length

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
          <button onClick={() => refetch()} className="mt-3 text-sm text-red-600 underline">Try again</button>
        </div>
      </div>
    )
  }

  return (
    <div className="p-6 max-w-5xl mx-auto">

      {/* ── Header ── */}
      <div className="mb-4 flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-black text-gray-900 mb-1">NRW Summary</h1>
          <p className="text-gray-500 text-sm">IWA/AWWA Water Balance — District Performance</p>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          {/* Period selector */}
          <select
            value={period}
            onChange={e => setPeriod(e.target.value)}
            className="text-xs border border-gray-200 rounded-lg px-3 py-1.5 bg-white focus:outline-none focus:ring-2 focus:ring-green-500"
          >
            {PERIODS.map(p => <option key={p.value} value={p.value}>{p.label}</option>)}
          </select>

          {/* Filter toggle */}
          <button
            onClick={() => setShowFilters(f => !f)}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium border rounded-lg transition-colors ${
              showFilters || activeFilterCount > 0
                ? 'bg-green-50 border-green-300 text-green-700'
                : 'bg-white border-gray-200 text-gray-600 hover:bg-gray-50'
            }`}
          >
            <Filter className="w-3.5 h-3.5" />
            Filters {activeFilterCount > 0 && <span className="bg-green-600 text-white rounded-full w-4 h-4 flex items-center justify-center text-[10px]">{activeFilterCount}</span>}
          </button>

          {/* Sort */}
          <select
            value={sortBy}
            onChange={e => setSortBy(e.target.value as any)}
            className="text-xs border border-gray-200 rounded-lg px-3 py-1.5 bg-white focus:outline-none"
          >
            <option value="nrw_desc">Sort: NRW % ↓</option>
            <option value="nrw_asc">Sort: NRW % ↑</option>
            <option value="name">Sort: Name A–Z</option>
            <option value="loss_desc">Sort: Est. Loss ↓</option>
          </select>

          {/* Export */}
          <button
            onClick={() => exportNRWSummaryCSV(filtered)}
            className="px-3 py-1.5 text-xs font-medium bg-white border border-gray-200 rounded-lg hover:bg-gray-50"
            title="Export to CSV"
          >📥 CSV</button>
          <button
            onClick={() => exportNRWSummaryPDF(filtered, `NRW Performance — ${PERIODS.find(p2 => p2.value === period)?.label}`)}
            className="px-3 py-1.5 text-xs font-medium bg-white border border-gray-200 rounded-lg hover:bg-gray-50"
            title="Export to PDF"
          >🖨 PDF</button>
          <button
            onClick={() => refetch()}
            disabled={isFetching}
            className="p-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 text-gray-500 ${isFetching ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* ── Filter Panel ── */}
      {showFilters && (
        <div className="mb-4 bg-gray-50 border border-gray-200 rounded-xl p-4 grid grid-cols-2 md:grid-cols-4 gap-3">
          <div>
            <label className="block text-xs font-semibold text-gray-600 mb-1">District</label>
            <select
              value={districtFilter}
              onChange={e => setDistrict(e.target.value)}
              className="w-full text-sm border border-gray-200 rounded-lg px-2 py-1.5 bg-white focus:outline-none focus:ring-2 focus:ring-green-500"
            >
              <option value="">All Districts</option>
              {districts.map((d: any) => (
                <option key={d.id} value={d.id}>{d.district_name}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-xs font-semibold text-gray-600 mb-1">IWA Grade</label>
            <select
              value={gradeFilter}
              onChange={e => setGrade(e.target.value)}
              className="w-full text-sm border border-gray-200 rounded-lg px-2 py-1.5 bg-white focus:outline-none focus:ring-2 focus:ring-green-500"
            >
              <option value="">All Grades</option>
              {GRADES.map(g => <option key={g} value={g}>Grade {g}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs font-semibold text-gray-600 mb-1">Min NRW %</label>
            <input
              type="number" min="0" max="100" value={minNRW}
              onChange={e => setMinNRW(e.target.value)}
              placeholder="e.g. 30"
              className="w-full text-sm border border-gray-200 rounded-lg px-2 py-1.5 bg-white focus:outline-none focus:ring-2 focus:ring-green-500"
            />
          </div>
          <div>
            <label className="block text-xs font-semibold text-gray-600 mb-1">Max NRW %</label>
            <input
              type="number" min="0" max="100" value={maxNRW}
              onChange={e => setMaxNRW(e.target.value)}
              placeholder="e.g. 60"
              className="w-full text-sm border border-gray-200 rounded-lg px-2 py-1.5 bg-white focus:outline-none focus:ring-2 focus:ring-green-500"
            />
          </div>
          {activeFilterCount > 0 && (
            <div className="col-span-full flex justify-end">
              <button
                onClick={() => { setDistrict(''); setGrade(''); setMinNRW(''); setMaxNRW('') }}
                className="text-xs text-red-600 hover:underline"
              >
                ✕ Clear all filters
              </button>
            </div>
          )}
        </div>
      )}

      {/* ── Result count ── */}
      <div className="mb-3 text-xs text-gray-500">
        Showing <strong>{filtered.length}</strong> of <strong>{summaries?.length ?? 0}</strong> districts
        {activeFilterCount > 0 && ' (filtered)'}
      </div>

      {/* ── KPI strip ── */}
      {filtered.length > 0 && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          {[
            { label: 'Total Open Flags',    value: filtered.reduce((s, r) => s + r.open_anomalies, 0).toLocaleString(),                                icon: '🚩' },
            { label: 'Critical Flags',      value: filtered.reduce((s, r) => s + r.critical_anomalies, 0).toLocaleString(),                            icon: '🚨' },
            { label: 'Est. Revenue Loss',   value: `₵${(filtered.reduce((s, r) => s + r.total_estimated_loss_ghs, 0) / 1000).toFixed(0)}K`,            icon: '💸' },
            { label: 'Recovered',           value: `₵${(filtered.reduce((s, r) => s + r.total_recovered_ghs, 0) / 1000).toFixed(0)}K`,                 icon: '✅' },
          ].map(k => (
            <div key={k.label} className="bg-white rounded-xl border border-gray-100 shadow-sm p-4 flex items-center gap-3">
              <span className="text-2xl">{k.icon}</span>
              <div>
                <div className="text-xl font-black text-gray-900">{k.value}</div>
                <div className="text-xs text-gray-500">{k.label}</div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* ── Bar chart ── */}
      {chartData.length > 0 && (
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 mb-4">
            NRW % by District {chartData.length < (filtered.length) && `(top ${chartData.length} shown)`}
          </h2>
          <ResponsiveContainer width="100%" height={260}>
            <BarChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis dataKey="district" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 11 }} unit="%" domain={[0, 70]} />
              <Tooltip
                formatter={(value) => [`${value ?? 0}%`, 'NRW']}
                labelFormatter={(label) => `District: ${label}`}
              />
              <ReferenceLine y={20} stroke="#16a34a" strokeDasharray="4 4"
                label={{ value: 'IWA Target 20%', fill: '#16a34a', fontSize: 10 }} />
              <ReferenceLine y={51.6} stroke="#dc2626" strokeDasharray="4 4"
                label={{ value: 'Ghana Avg 51.6%', fill: '#dc2626', fontSize: 10 }} />
              <Bar dataKey="nrw" fill="#2e7d32" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* ── Table ── */}
      {filtered.length > 0 ? (
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                {['District', 'Zone', 'NRW %', 'Open Flags', 'Est. Loss (GHS)', 'Grade', 'Status'].map(h => (
                  <th key={h} className="text-left px-4 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {filtered.map((row: NRWSummaryRow) => (
                <tr key={row.district_id} className="hover:bg-gray-50">
                  <td className="px-4 py-3">
                    <div className="font-semibold text-gray-900">{row.district_name}</div>
                    <div className="text-xs text-gray-400">{row.region}</div>
                  </td>
                  <td className="px-4 py-3">
                    {row.zone_type ? (
                      <span className={`text-xs font-bold px-2 py-0.5 rounded-full ${
                        row.zone_type === 'RED'    ? 'bg-red-100 text-red-700' :
                        row.zone_type === 'YELLOW' ? 'bg-amber-100 text-amber-700' :
                        row.zone_type === 'GREEN'  ? 'bg-emerald-100 text-emerald-700' :
                        'bg-gray-100 text-gray-500'
                      }`}>{row.zone_type}</span>
                    ) : <span className="text-gray-400 text-xs">—</span>}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`font-bold ${
                      (row.loss_ratio_pct ?? 0) > 50 ? 'text-red-600' :
                      (row.loss_ratio_pct ?? 0) > 30 ? 'text-orange-600' : 'text-green-700'
                    }`}>
                      {row.loss_ratio_pct != null ? `${row.loss_ratio_pct.toFixed(1)}%` : 'N/A'}
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
          <p className="text-gray-500">No districts match the current filters</p>
          {activeFilterCount > 0 && (
            <button
              onClick={() => { setDistrict(''); setGrade(''); setMinNRW(''); setMaxNRW('') }}
              className="mt-2 text-sm text-green-700 underline"
            >Clear filters</button>
          )}
        </div>
      )}
    </div>
  )
}
