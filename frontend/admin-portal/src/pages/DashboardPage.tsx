import { useState } from 'react'
import {
  AlertTriangle, TrendingDown, DollarSign, CheckCircle,
  Activity, RefreshCw, Droplets, ArrowUpRight, Zap
} from 'lucide-react'
import { StatCard, Card } from '../components/ui/Card'
import { AlertLevelBadge, StatusBadge, GRAStatusBadge } from '../components/ui/Badge'
import { useDashboardStats, useAnomalies, useDistricts } from '../hooks/useQueries'
import { formatCurrency, formatRelativeTime, formatNumber } from '../lib/utils'
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell, Legend, AreaChart, Area
} from 'recharts'

const SEVERITY_COLORS = ['#dc2626', '#ea580c', '#d97706', '#2563eb']
const SEVERITY_LABELS = ['Critical', 'High', 'Medium', 'Low']

export function DashboardPage() {
  const [selectedDistrict, setSelectedDistrict] = useState<string>('')
  const { data: districts } = useDistricts()
  const { data: stats, isLoading: statsLoading, refetch } = useDashboardStats(selectedDistrict || undefined)
  const { data: anomaliesData, isLoading: anomaliesLoading } = useAnomalies({
    district_id: selectedDistrict || undefined,
    limit: 8,
    status: 'OPEN',
  })

  const anomalies = anomaliesData?.data || []

  const anomalyBreakdown = [
    { name: 'Critical', value: anomalies.filter(a => a.alert_level === 'CRITICAL').length, color: '#dc2626' },
    { name: 'High',     value: anomalies.filter(a => a.alert_level === 'HIGH').length,     color: '#ea580c' },
    { name: 'Medium',   value: anomalies.filter(a => a.alert_level === 'MEDIUM').length,   color: '#d97706' },
    { name: 'Low',      value: anomalies.filter(a => a.alert_level === 'LOW').length,      color: '#2563eb' },
  ].filter(d => d.value > 0)

  const totalAnomalies = anomalyBreakdown.reduce((s, d) => s + d.value, 0)

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-start justify-between">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <div className="w-6 h-6 bg-brand-600 rounded-lg flex items-center justify-center">
              <Droplets size={13} className="text-white" />
            </div>
            <span className="text-xs font-semibold text-brand-600 uppercase tracking-wider">GN-WAAS Admin</span>
          </div>
          <h1 className="text-2xl font-black text-gray-900">Operations Dashboard</h1>
          <p className="text-sm text-gray-500 mt-0.5">
            Real-time water audit overview — Ghana National Water Audit & Assurance System
          </p>
        </div>
        <div className="flex items-center gap-3">
          <select
            className="input w-52 text-sm"
            value={selectedDistrict}
            onChange={e => setSelectedDistrict(e.target.value)}
          >
            <option value="">All Districts</option>
            {districts?.map(d => (
              <option key={d.id} value={d.id}>{d.district_name}</option>
            ))}
          </select>
          <button onClick={() => refetch()} className="btn-secondary btn-sm gap-1.5">
            <RefreshCw size={13} />
            Refresh
          </button>
        </div>
      </div>

      {/* KPI Cards — Row 1 */}
      <div className="grid grid-cols-2 xl:grid-cols-4 gap-4">
        <StatCard
          title="Open Anomalies"
          value={statsLoading ? '—' : formatNumber(stats?.pending ?? 0)}
          subtitle="Requiring investigation"
          icon={<AlertTriangle size={18} />}
          variant="danger"
        />
        <StatCard
          title="Audits In Progress"
          value={statsLoading ? '—' : formatNumber(stats?.in_progress ?? 0)}
          subtitle="Field officers deployed"
          icon={<Activity size={18} />}
          variant="warning"
        />
        <StatCard
          title="Confirmed Revenue Loss"
          value={statsLoading ? '—' : formatCurrency(stats?.total_confirmed_loss_ghs ?? 0)}
          subtitle="This period"
          icon={<TrendingDown size={18} />}
          variant="danger"
        />
        <StatCard
          title="Success Fees Earned"
          value={statsLoading ? '—' : formatCurrency(stats?.total_success_fees_ghs ?? 0)}
          subtitle="3% of recovered revenue"
          icon={<DollarSign size={18} />}
          variant="success"
        />
      </div>

      {/* KPI Cards — Row 2 */}
      <div className="grid grid-cols-2 xl:grid-cols-4 gap-4">
        <StatCard
          title="GRA Signed Audits"
          value={statsLoading ? '—' : formatNumber(stats?.gra_signed ?? 0)}
          subtitle="Legally compliant"
          icon={<CheckCircle size={18} />}
          variant="success"
        />
        <StatCard
          title="Completed Audits"
          value={statsLoading ? '—' : formatNumber(stats?.completed ?? 0)}
          subtitle="Closed this period"
          icon={<CheckCircle size={18} />}
          variant="brand"
        />
        <StatCard
          title="Total Recovered"
          value={statsLoading ? '—' : formatCurrency(stats?.total_recovered_ghs ?? 0)}
          subtitle="Revenue recovered"
          icon={<DollarSign size={18} />}
          variant="success"
        />
        <StatCard
          title="Pending Assignment"
          value={statsLoading ? '—' : formatNumber(stats?.pending ?? 0)}
          subtitle="Awaiting field officer"
          icon={<AlertTriangle size={18} />}
          variant="warning"
        />
      </div>

      {/* Charts + Recent Anomalies */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        {/* Anomaly breakdown */}
        <Card
          title="Anomaly Severity"
          subtitle={`${totalAnomalies} open flags`}
          className="xl:col-span-1"
        >
          {anomalyBreakdown.length > 0 ? (
            <>
              <ResponsiveContainer width="100%" height={200}>
                <PieChart>
                  <Pie
                    data={anomalyBreakdown}
                    cx="50%"
                    cy="50%"
                    innerRadius={55}
                    outerRadius={80}
                    paddingAngle={3}
                    dataKey="value"
                    strokeWidth={0}
                  >
                    {anomalyBreakdown.map((entry, index) => (
                      <Cell key={index} fill={entry.color} />
                    ))}
                  </Pie>
                  <Tooltip
                    formatter={(value, name) => [`${value} flags`, name]}
                    contentStyle={{ borderRadius: '12px', border: '1px solid #f1f5f9', boxShadow: '0 4px 6px -1px rgba(0,0,0,0.1)' }}
                  />
                </PieChart>
              </ResponsiveContainer>
              <div className="grid grid-cols-2 gap-2 mt-2">
                {anomalyBreakdown.map(d => (
                  <div key={d.name} className="flex items-center gap-2">
                    <span className="w-2.5 h-2.5 rounded-full flex-shrink-0" style={{ backgroundColor: d.color }} />
                    <span className="text-xs text-gray-600">{d.name}</span>
                    <span className="text-xs font-bold text-gray-900 ml-auto">{d.value}</span>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div className="h-[200px] flex flex-col items-center justify-center text-gray-400">
              <CheckCircle size={32} className="text-emerald-400 mb-2" />
              <p className="text-sm font-medium text-gray-500">No open anomalies</p>
              <p className="text-xs text-gray-400 mt-0.5">System is operating normally</p>
            </div>
          )}
        </Card>

        {/* Recent anomalies table */}
        <Card
          title="Recent Open Anomalies"
          subtitle="Latest flags requiring attention"
          className="xl:col-span-2"
          noPadding
          action={
            <a href="/anomalies" className="text-xs text-brand-600 hover:text-brand-700 font-semibold flex items-center gap-1">
              View all <ArrowUpRight size={12} />
            </a>
          }
        >
          {anomaliesLoading ? (
            <div className="p-8 text-center">
              <div className="w-6 h-6 border-2 border-brand-500 border-t-transparent rounded-full animate-spin mx-auto" />
            </div>
          ) : anomalies.length === 0 ? (
            <div className="p-8 text-center">
              <CheckCircle size={28} className="text-emerald-400 mx-auto mb-2" />
              <p className="text-sm text-gray-500 font-medium">No open anomalies</p>
            </div>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Anomaly</th>
                  <th>Level</th>
                  <th>Est. Loss</th>
                  <th>Detected</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {anomalies.map(flag => (
                  <tr key={flag.id}>
                    <td>
                      <div>
                        <p className="font-semibold text-gray-900 text-xs">
                          {flag.anomaly_type.replace(/_/g, ' ')}
                        </p>
                        <p className="text-gray-400 text-xs truncate max-w-[180px]">{flag.title}</p>
                      </div>
                    </td>
                    <td><AlertLevelBadge level={flag.alert_level} /></td>
                    <td className="font-mono text-xs font-semibold text-gray-700">
                      {flag.estimated_loss_ghs ? formatCurrency(flag.estimated_loss_ghs) : '—'}
                    </td>
                    <td className="text-gray-400 text-xs">{formatRelativeTime(flag.created_at)}</td>
                    <td><StatusBadge status={flag.status} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Card>
      </div>

      {/* IWA Water Balance Banner */}
      <div className="relative overflow-hidden rounded-2xl bg-gray-900 p-6">
        <div className="absolute inset-0 opacity-10">
          <div className="absolute top-0 right-0 w-64 h-64 bg-brand-500 rounded-full translate-x-1/3 -translate-y-1/3" />
          <div className="absolute bottom-0 left-0 w-48 h-48 bg-gold-500 rounded-full -translate-x-1/3 translate-y-1/3" />
        </div>
        <div className="relative flex items-center justify-between gap-6">
          <div className="flex items-start gap-4">
            <div className="w-10 h-10 bg-brand-600 rounded-xl flex items-center justify-center flex-shrink-0">
              <Zap size={18} className="text-white" />
            </div>
            <div>
              <h3 className="text-white font-bold text-base">IWA/AWWA Water Balance Framework</h3>
              <p className="text-gray-400 text-sm mt-1 max-w-lg">
                GN-WAAS aligns with international water audit standards.
                System Input Volume → Authorised Consumption + Water Losses (Real + Apparent).
              </p>
            </div>
          </div>
          <div className="text-right flex-shrink-0">
            <p className="text-gray-500 text-xs font-semibold uppercase tracking-wider">Target NRW</p>
            <p className="text-white text-3xl font-black mt-0.5">≤ 20%</p>
            <p className="text-gray-500 text-xs mt-0.5">Ghana avg: 51.6%</p>
          </div>
        </div>
      </div>
    </div>
  )
}
