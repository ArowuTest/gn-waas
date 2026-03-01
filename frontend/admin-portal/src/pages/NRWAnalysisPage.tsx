import { useState } from 'react'
import { Card, StatCard } from '../components/ui/Card'
import { useDistricts, useNRWSummary } from '../hooks/useQueries'
import { formatNumber, getDataConfidenceGrade } from '../lib/utils'
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip,
  ResponsiveContainer, ReferenceLine, Cell
} from 'recharts'

export function NRWAnalysisPage() {
  const [selectedDistrict, setSelectedDistrict] = useState<string>('')
  const { data: districts } = useDistricts()
  const { data: nrwData } = useNRWSummary(selectedDistrict || undefined)

  const summaries = nrwData || []

  // Chart data
  const chartData = summaries.map(s => ({
    name: s.district_name.replace(' District', '').substring(0, 12),
    nrw_pct: parseFloat(s.nrw_pct.toFixed(1)),
    production: parseFloat((s.production_m3 / 1000).toFixed(1)),
    billed: parseFloat((s.billed_m3 / 1000).toFixed(1)),
  }))

  const avgNRW = summaries.length > 0
    ? summaries.reduce((sum, s) => sum + s.nrw_pct, 0) / summaries.length
    : 0

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1>NRW Analysis</h1>
          <p className="text-gray-500 text-sm mt-1">
            IWA/AWWA Water Balance — Non-Revenue Water tracking by district
          </p>
        </div>
        <select
          className="input w-48"
          value={selectedDistrict}
          onChange={e => setSelectedDistrict(e.target.value)}
        >
          <option value="">All Districts</option>
          {districts?.map(d => (
            <option key={d.id} value={d.id}>{d.district_name}</option>
          ))}
        </select>
      </div>

      {/* Summary KPIs */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Average NRW"
          value={`${formatNumber(avgNRW, 1)}%`}
          subtitle="Across all districts"
          variant={avgNRW > 30 ? 'danger' : avgNRW > 20 ? 'warning' : 'success'}
        />
        <StatCard
          title="Districts Above 30%"
          value={summaries.filter(s => s.nrw_pct > 30).length}
          subtitle="High-loss districts"
          variant="danger"
        />
        <StatCard
          title="Districts Below 20%"
          value={summaries.filter(s => s.nrw_pct <= 20).length}
          subtitle="Meeting IWA target"
          variant="success"
        />
        <StatCard
          title="Districts Monitored"
          value={summaries.length}
          subtitle="With data this period"
        />
      </div>

      {/* NRW Bar Chart */}
      <Card title="NRW % by District" subtitle="IWA target: ≤20% | Ghana average: 51.6%">
        {chartData.length > 0 ? (
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={chartData} margin={{ top: 10, right: 20, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis dataKey="name" tick={{ fontSize: 11 }} />
              <YAxis tickFormatter={v => `${v}%`} tick={{ fontSize: 11 }} />
              <Tooltip formatter={(v) => [`${v}%`, 'NRW']} />
              <ReferenceLine y={20} stroke="#16a34a" strokeDasharray="4 4" label={{ value: 'IWA Target 20%', fill: '#16a34a', fontSize: 11 }} />
              <ReferenceLine y={51.6} stroke="#dc2626" strokeDasharray="4 4" label={{ value: 'Ghana Avg 51.6%', fill: '#dc2626', fontSize: 11 }} />
              <Bar dataKey="nrw_pct" radius={[4, 4, 0, 0]}>
                {chartData.map((entry, index) => (
                  <Cell
                    key={index}
                    fill={entry.nrw_pct > 40 ? '#dc2626' : entry.nrw_pct > 25 ? '#d97706' : '#16a34a'}
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

      {/* District Table */}
      <Card title="District NRW Summary" noPadding>
        <table className="table">
          <thead>
            <tr>
              <th>District</th>
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
                <td colSpan={6} className="text-center text-gray-400 py-8">
                  No data available
                </td>
              </tr>
            ) : summaries.map(s => {
              const grade = getDataConfidenceGrade(s.nrw_pct)
              return (
                <tr key={s.district_id}>
                  <td className="font-medium">{s.district_name}</td>
                  <td className="font-mono text-sm">{formatNumber(s.production_m3, 0)}</td>
                  <td className="font-mono text-sm">{formatNumber(s.billed_m3, 0)}</td>
                  <td className="font-mono text-sm text-danger">{formatNumber(s.nrw_m3, 0)}</td>
                  <td>
                    <span className={`font-mono font-bold text-sm ${
                      s.nrw_pct > 40 ? 'text-danger' :
                      s.nrw_pct > 25 ? 'text-warning' : 'text-success'
                    }`}>
                      {s.nrw_pct.toFixed(1)}%
                    </span>
                  </td>
                  <td>
                    <span className={grade.className}>
                      {grade.grade} — {grade.label}
                    </span>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </Card>

      {/* AWWA Framework Reference */}
      <Card className="bg-gray-50 border-gray-200">
        <h3 className="text-gray-700 mb-3">IWA/AWWA Water Balance Components</h3>
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 text-sm">
          {[
            { label: 'System Input Volume', desc: 'Total water entering the distribution system', color: 'bg-blue-100 text-blue-800' },
            { label: 'Authorised Consumption', desc: 'Billed + unbilled authorised use', color: 'bg-green-100 text-green-800' },
            { label: 'Real Losses', desc: 'Physical leakage from pipes and storage', color: 'bg-orange-100 text-orange-800' },
            { label: 'Apparent Losses', desc: 'Meter errors + unauthorised consumption (theft)', color: 'bg-red-100 text-red-800' },
          ].map(item => (
            <div key={item.label} className="p-3 rounded-lg bg-white border border-gray-200">
              <span className={`badge text-xs mb-2 ${item.color}`}>{item.label}</span>
              <p className="text-gray-500 text-xs">{item.desc}</p>
            </div>
          ))}
        </div>
      </Card>
    </div>
  )
}
