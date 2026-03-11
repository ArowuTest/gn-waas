import { useState } from 'react'
import { Card, StatCard } from '../components/ui/Card'
import { useDistricts, useNRWSummary, useWaterBalance } from '../hooks/useQueries'
import { formatNumber, formatCurrency, getDataConfidenceGrade } from '../lib/utils'
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip,
  ResponsiveContainer, ReferenceLine, Cell, Legend,
  RadarChart, Radar, PolarGrid, PolarAngleAxis, PolarRadiusAxis,
} from 'recharts'
import { Target, TrendingDown, Droplets, Award, AlertTriangle, Info } from 'lucide-react'

// ── ILI explanation tooltip ───────────────────────────────────────────────────
const ILI_GRADES = [
  { grade: 'A', range: 'ILI < 1.0',  label: 'Excellent',  color: '#10b981', bg: 'bg-emerald-50', text: 'text-emerald-700', desc: 'Best practice — losses are at or below unavoidable minimum' },
  { grade: 'B', range: 'ILI 1–2',    label: 'Good',       color: '#3b82f6', bg: 'bg-blue-50',    text: 'text-blue-700',    desc: 'Well-managed system — some improvement possible' },
  { grade: 'C', range: 'ILI 2–4',    label: 'Fair',       color: '#f59e0b', bg: 'bg-amber-50',   text: 'text-amber-700',   desc: 'Moderate losses — active leakage control recommended' },
  { grade: 'D', range: 'ILI > 4',    label: 'Poor',       color: '#ef4444', bg: 'bg-red-50',     text: 'text-red-700',     desc: 'High losses — urgent infrastructure investment needed' },
]

function ILIGradeTable() {
  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
      {ILI_GRADES.map(g => (
        <div key={g.grade} className={`${g.bg} rounded-xl p-3`}>
          <div className="flex items-center gap-2 mb-1">
            <span className={`text-2xl font-black ${g.text}`}>{g.grade}</span>
            <div>
              <p className={`text-xs font-bold ${g.text}`}>{g.label}</p>
              <p className="text-xs text-gray-500">{g.range}</p>
            </div>
          </div>
          <p className="text-xs text-gray-600 leading-relaxed">{g.desc}</p>
        </div>
      ))}
    </div>
  )
}

// ── AWWA Water Balance breakdown for a single district ────────────────────────
function AWWABreakdown({ wb }: { wb: any }) {
  if (!wb) return null

  const total = wb.system_input_m3 || 1

  const components = [
    {
      category: 'Authorised Consumption',
      items: [
        { label: 'Billed Metered',       m3: wb.billed_metered_m3,     color: '#10b981' },
        { label: 'Billed Unmetered',     m3: wb.billed_unmetered_m3,   color: '#34d399' },
        { label: 'Unbilled Metered',     m3: wb.unbilled_metered_m3,   color: '#6ee7b7' },
        { label: 'Unbilled Unmetered',   m3: wb.unbilled_unmetered_m3, color: '#a7f3d0' },
      ],
      totalM3: wb.total_authorised_m3,
      color: '#10b981',
    },
    {
      category: 'Apparent Losses (Commercial)',
      items: [
        { label: 'Unauthorised Consumption', m3: wb.unauthorised_consumption_m3, color: '#f59e0b' },
        { label: 'Metering Inaccuracies',    m3: wb.metering_inaccuracies_m3,    color: '#fbbf24' },
        { label: 'Data Handling Errors',     m3: wb.data_handling_errors_m3,     color: '#fcd34d' },
      ],
      totalM3: wb.total_apparent_losses_m3,
      color: '#f59e0b',
    },
    {
      category: 'Real Losses (Physical)',
      items: [
        { label: 'Main Leakage',              m3: wb.main_leakage_m3,            color: '#ef4444' },
        { label: 'Storage Overflow',          m3: wb.storage_overflow_m3,        color: '#f87171' },
        { label: 'Service Connection Leaks',  m3: wb.service_connection_leak_m3, color: '#fca5a5' },
      ],
      totalM3: wb.total_real_losses_m3,
      color: '#ef4444',
    },
  ]

  return (
    <div className="space-y-4">
      {/* System Input header */}
      <div className="flex items-center justify-between p-3 bg-blue-50 rounded-xl">
        <div className="flex items-center gap-2">
          <Droplets size={16} className="text-blue-600" />
          <span className="text-sm font-bold text-blue-800">System Input Volume</span>
        </div>
        <span className="text-sm font-mono font-black text-blue-900">
          {formatNumber(Math.round(wb.system_input_m3))} m³
        </span>
      </div>

      {/* Components */}
      {components.map(cat => (
        <div key={cat.category} className="border border-gray-100 rounded-xl overflow-hidden">
          <div
            className="flex items-center justify-between px-4 py-2.5"
            style={{ backgroundColor: cat.color + '18' }}
          >
            <span className="text-sm font-bold" style={{ color: cat.color }}>
              {cat.category}
            </span>
            <div className="text-right">
              <span className="text-sm font-mono font-black" style={{ color: cat.color }}>
                {formatNumber(Math.round(cat.totalM3))} m³
              </span>
              <span className="text-xs text-gray-500 ml-2">
                ({((cat.totalM3 / total) * 100).toFixed(1)}%)
              </span>
            </div>
          </div>
          <div className="divide-y divide-gray-50">
            {cat.items.map(item => (
              <div key={item.label} className="flex items-center justify-between px-4 py-2 bg-white">
                <div className="flex items-center gap-2">
                  <div className="w-2.5 h-2.5 rounded-sm" style={{ backgroundColor: item.color }} />
                  <span className="text-xs text-gray-600">{item.label}</span>
                </div>
                <div className="text-right">
                  <span className="text-xs font-mono text-gray-800">
                    {formatNumber(Math.round(item.m3))} m³
                  </span>
                  <span className="text-xs text-gray-400 ml-1.5">
                    ({((item.m3 / total) * 100).toFixed(1)}%)
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      ))}

      {/* NRW total */}
      <div className="flex items-center justify-between p-3 bg-red-50 rounded-xl border border-red-100">
        <div className="flex items-center gap-2">
          <TrendingDown size={16} className="text-red-600" />
          <span className="text-sm font-bold text-red-800">Total NRW (Water Losses)</span>
        </div>
        <div className="text-right">
          <span className="text-sm font-mono font-black text-red-900">
            {formatNumber(Math.round(wb.nrw_m3))} m³
          </span>
          <span className="text-sm font-black text-red-700 ml-2">
            ({wb.nrw_percent.toFixed(1)}%)
          </span>
        </div>
      </div>
    </div>
  )
}

// ── Main NRW Analysis Page ────────────────────────────────────────────────────
export function NRWAnalysisPage() {
  const [selectedDistrict, setSelectedDistrict] = useState<string>('')
  const { data: districts } = useDistricts()
  const { data: nrwData } = useNRWSummary(selectedDistrict || undefined)
  const { data: waterBalance } = useWaterBalance(selectedDistrict || undefined)

  const summaries = nrwData || []
  const latestWB = waterBalance?.[0]

  // Chart data
  const chartData = summaries.map(s => ({
    name: s.district_name.replace(' District', '').substring(0, 12),
    // nrw_pct is the canonical field; fall back to loss_ratio_pct for older API responses
    nrw_pct: parseFloat(((s.nrw_pct ?? (s as any).loss_ratio_pct ?? 0) as number).toFixed(1)),
    production: parseFloat(((s.production_m3 ?? 0) / 1000).toFixed(1)),
    billed: parseFloat(((s.billed_m3 ?? 0) / 1000).toFixed(1)),
  }))

  const nrwPct = (s: typeof summaries[0]) => (s.nrw_pct ?? (s as any).loss_ratio_pct ?? 0) as number
  const avgNRW = summaries.length > 0
    ? summaries.reduce((sum, s) => sum + nrwPct(s), 0) / summaries.length
    : 0

  const aboveTarget = summaries.filter(s => nrwPct(s) > 20).length
  const criticalCount = summaries.filter(s => nrwPct(s) > 40).length

  return (
    <div className="space-y-6">
      {/* ── Header ──────────────────────────────────────────────────────────── */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-black text-gray-900">NRW Analysis</h1>
          <p className="text-gray-500 text-sm mt-1">
            IWA/AWWA M36 Water Balance — Non-Revenue Water tracking with ILI scoring
          </p>
        </div>
        <select
          className="input w-52"
          value={selectedDistrict}
          onChange={e => setSelectedDistrict(e.target.value)}
        >
          <option value="">All Districts</option>
          {districts?.map(d => (
            <option key={d.id} value={d.id}>{d.district_name}</option>
          ))}
        </select>
      </div>

      {/* ── KPI Row ─────────────────────────────────────────────────────────── */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Average NRW"
          value={`${formatNumber(avgNRW, 1)}%`}
          subtitle="Across monitored districts"
          variant={avgNRW > 40 ? 'danger' : avgNRW > 20 ? 'warning' : 'success'}
          icon={<TrendingDown size={18} />}
        />
        <StatCard
          title="ILI Score"
          value={latestWB ? latestWB.ili.toFixed(2) : '—'}
          subtitle={`Grade ${latestWB?.iwa_grade ?? '—'} — ${
            latestWB?.iwa_grade === 'A' ? 'Excellent' :
            latestWB?.iwa_grade === 'B' ? 'Good' :
            latestWB?.iwa_grade === 'C' ? 'Fair' :
            latestWB?.iwa_grade === 'D' ? 'Poor' : 'No data'
          }`}
          variant={
            latestWB?.iwa_grade === 'A' ? 'success' :
            latestWB?.iwa_grade === 'B' ? 'success' :
            latestWB?.iwa_grade === 'C' ? 'warning' : 'danger'
          }
          icon={<Award size={18} />}
        />
        <StatCard
          title="Above IWA Target"
          value={aboveTarget}
          subtitle="Districts with NRW > 20%"
          variant={aboveTarget > 0 ? 'warning' : 'success'}
          icon={<Target size={18} />}
        />
        <StatCard
          title="Critical Districts"
          value={criticalCount}
          subtitle="NRW > 40% (RED zone)"
          variant={criticalCount > 0 ? 'danger' : 'success'}
          icon={<AlertTriangle size={18} />}
        />
      </div>

      {/* ── NRW Bar Chart ────────────────────────────────────────────────────── */}
      <Card title="NRW % by District" subtitle="IWA target: ≤20% | Ghana average: ~51.6%">
        {chartData.length > 0 ? (
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={chartData} margin={{ top: 10, right: 20, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis dataKey="name" tick={{ fontSize: 11 }} />
              <YAxis tickFormatter={v => `${v}%`} tick={{ fontSize: 11 }} domain={[0, 100]} />
              <Tooltip formatter={((v: number) => [`${v}%`, 'NRW']) as any} />
              <ReferenceLine y={20} stroke="#16a34a" strokeDasharray="4 4"
                label={{ value: 'IWA Target 20%', fill: '#16a34a', fontSize: 11, position: 'insideTopRight' }} />
              <ReferenceLine y={51.6} stroke="#dc2626" strokeDasharray="4 4"
                label={{ value: 'Ghana Avg 51.6%', fill: '#dc2626', fontSize: 11, position: 'insideTopRight' }} />
              <Bar dataKey="nrw_pct" radius={[4, 4, 0, 0]}>
                {chartData.map((entry, index) => (
                  <Cell
                    key={index}
                    fill={(entry.nrw_pct ?? 0) > 40 ? '#ef4444' : (entry.nrw_pct ?? 0) > 20 ? '#f59e0b' : '#10b981'}
                  />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        ) : (
          <div className="h-[300px] flex items-center justify-center text-gray-400 text-sm">
            No NRW data available. Run sentinel scans to populate.
          </div>
        )}
      </Card>

      {/* ── AWWA Water Balance Breakdown ─────────────────────────────────────── */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
        <Card
          title="IWA/AWWA M36 Water Balance"
          subtitle={selectedDistrict
            ? `Detailed breakdown for selected district`
            : 'Select a district to see the full AWWA breakdown'}
        >
          {latestWB ? (
            <AWWABreakdown wb={latestWB} />
          ) : (
            <div className="flex flex-col items-center justify-center h-48 text-center text-gray-400">
              <Droplets size={32} className="mb-2 opacity-40" />
              <p className="text-sm">Select a district and run a sentinel scan</p>
              <p className="text-xs mt-1">to see the full AWWA water balance breakdown</p>
            </div>
          )}
        </Card>

        {/* ILI Grade Reference */}
        <Card
          title="Infrastructure Leakage Index (ILI)"
          subtitle="IWA standard metric — Current Annual Real Losses ÷ Unavoidable Annual Real Losses"
        >
          <ILIGradeTable />
          {latestWB && (
            <div className="mt-4 p-3 bg-gray-50 rounded-xl">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-xs text-gray-500 font-semibold uppercase tracking-wide">Current ILI</p>
                  <p className="text-3xl font-black text-gray-900 mt-0.5">{latestWB.ili.toFixed(2)}</p>
                </div>
                <div className="text-right">
                  <p className="text-xs text-gray-500">Data Confidence</p>
                  <p className="text-lg font-bold text-gray-700">{latestWB.data_confidence_score.toFixed(0)}%</p>
                  <p className="text-xs text-gray-400">
                    Computed {new Date(latestWB.computed_at).toLocaleDateString()}
                  </p>
                </div>
              </div>
              <div className="mt-2 pt-2 border-t border-gray-200">
                <p className="text-xs text-gray-500">
                  Est. Revenue Recovery Potential:{' '}
                  <span className="font-bold text-gray-800">
                    {formatCurrency(latestWB.estimated_revenue_recovery_ghs)}
                  </span>
                </p>
              </div>
            </div>
          )}
        </Card>
      </div>

      {/* ── District NRW Summary Table ───────────────────────────────────────── */}
      <Card title="District NRW Summary" noPadding>
        <table className="table">
          <thead>
            <tr>
              <th>District</th>
              <th>Zone</th>
              <th>Production (m³)</th>
              <th>Billed (m³)</th>
              <th>NRW (m³)</th>
              <th>NRW %</th>
              <th>Data Confidence</th>
            </tr>
          </thead>
          <tbody>
            {summaries.length === 0 ? (
              <tr>
                <td colSpan={7} className="text-center text-gray-400 py-8">
                  No data available — run sentinel scans to populate
                </td>
              </tr>
            ) : summaries.map(s => {
              const grade = getDataConfidenceGrade(s.nrw_pct)
              const zone = s.zone_type ?? 'GREY'
              const zoneColors: Record<string, string> = {
                RED: 'bg-red-100 text-red-700',
                YELLOW: 'bg-amber-100 text-amber-700',
                GREEN: 'bg-emerald-100 text-emerald-700',
                GREY: 'bg-gray-100 text-gray-600',
              }
              return (
                <tr key={s.district_id}>
                  <td className="font-medium">{s.district_name}</td>
                  <td>
                    <span className={`badge text-xs ${zoneColors[zone] ?? zoneColors.GREY}`}>
                      {zone}
                    </span>
                  </td>
                  <td className="font-mono text-sm">{formatNumber(s.production_m3, 0)}</td>
                  <td className="font-mono text-sm">{formatNumber(s.billed_m3, 0)}</td>
                  <td className="font-mono text-sm text-red-600">{formatNumber(s.nrw_m3, 0)}</td>
                  <td>
                    <span className={`font-mono font-bold text-sm ${
                      s.nrw_pct > 40 ? 'text-red-600' :
                      s.nrw_pct > 20 ? 'text-amber-600' : 'text-emerald-600'
                    }`}>
                      {s.nrw_pct.toFixed(1)}%
                    </span>
                  </td>
                  <td>
                    <span className={grade.className}>{grade.grade} — {grade.label}</span>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </Card>

      {/* ── AWWA Framework Reference ─────────────────────────────────────────── */}
      <Card className="bg-gray-50 border-gray-200">
        <div className="flex items-center gap-2 mb-3">
          <Info size={16} className="text-gray-500" />
          <h3 className="text-gray-700 font-bold">IWA/AWWA M36 Water Balance Components</h3>
        </div>
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 text-sm">
          {[
            { label: 'System Input Volume',    desc: 'Total water entering the distribution system from all sources', color: 'bg-blue-100 text-blue-800' },
            { label: 'Authorised Consumption', desc: 'Billed metered + billed unmetered + unbilled authorised use',   color: 'bg-emerald-100 text-emerald-800' },
            { label: 'Real Losses',            desc: 'Physical leakage from mains, service connections, and storage', color: 'bg-orange-100 text-orange-800' },
            { label: 'Apparent Losses',        desc: 'Meter inaccuracies + unauthorised consumption (commercial theft)', color: 'bg-red-100 text-red-800' },
          ].map(item => (
            <div key={item.label} className="p-3 rounded-lg bg-white border border-gray-200">
              <span className={`badge text-xs mb-2 ${item.color}`}>{item.label}</span>
              <p className="text-gray-500 text-xs leading-relaxed">{item.desc}</p>
            </div>
          ))}
        </div>
        <p className="text-xs text-gray-400 mt-3">
          ILI = Current Annual Real Losses (CARL) ÷ Unavoidable Annual Real Losses (UARL).
          UARL is calculated from pipe length, number of service connections, and average pressure.
          GN-WAAS uses statistical estimation when pressure data is unavailable.
        </p>
      </Card>
    </div>
  )
}
