import { useState } from 'react'
import { AlertTriangle, TrendingDown, DollarSign, CheckCircle, Activity, RefreshCw } from 'lucide-react'
import { StatCard, Card } from '../components/ui/Card'
import { AlertLevelBadge, StatusBadge, GRAStatusBadge } from '../components/ui/Badge'
import { useDashboardStats, useAnomalies, useDistricts } from '../hooks/useQueries'
import { formatCurrency, formatRelativeTime, formatNumber } from '../lib/utils'
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell, Legend
} from 'recharts'

const COLORS = ['#dc2626', '#d97706', '#2563eb', '#16a34a']

export function DashboardPage() {
  const [selectedDistrict, setSelectedDistrict] = useState<string>('')
  const { data: districts } = useDistricts()
  const { data: stats, isLoading: statsLoading } = useDashboardStats(selectedDistrict || undefined)
  const { data: anomaliesData, isLoading: anomaliesLoading } = useAnomalies({
    district_id: selectedDistrict || undefined,
    limit: 8,
    status: 'OPEN',
  })

  const anomalies = anomaliesData?.data || []

  // Anomaly breakdown for pie chart
  const anomalyBreakdown = [
    { name: 'CRITICAL', value: anomalies.filter(a => a.alert_level === 'CRITICAL').length },
    { name: 'HIGH',     value: anomalies.filter(a => a.alert_level === 'HIGH').length },
    { name: 'MEDIUM',   value: anomalies.filter(a => a.alert_level === 'MEDIUM').length },
    { name: 'LOW',      value: anomalies.filter(a => a.alert_level === 'LOW').length },
  ].filter(d => d.value > 0)

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1>Operations Dashboard</h1>
          <p className="text-gray-500 text-sm mt-1">
            Ghana National Water Audit & Assurance System — Real-time overview
          </p>
        </div>
        <div className="flex items-center gap-3">
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
          <button className="btn-secondary btn-sm">
            <RefreshCw size={14} />
            Refresh
          </button>
        </div>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Open Anomalies"
          value={statsLoading ? '—' : (stats?.pending ?? 0)}
          subtitle="Requiring investigation"
          icon={<AlertTriangle size={20} />}
          variant="danger"
        />
        <StatCard
          title="Audits In Progress"
          value={statsLoading ? '—' : (stats?.in_progress ?? 0)}
          subtitle="Field officers deployed"
          icon={<Activity size={20} />}
          variant="warning"
        />
        <StatCard
          title="Confirmed Revenue Loss"
          value={statsLoading ? '—' : formatCurrency(stats?.total_confirmed_loss_ghs ?? 0)}
          subtitle="This period"
          icon={<TrendingDown size={20} />}
          variant="danger"
        />
        <StatCard
          title="Success Fees Earned"
          value={statsLoading ? '—' : formatCurrency(stats?.total_success_fees_ghs ?? 0)}
          subtitle="3% of recovered revenue"
          icon={<DollarSign size={20} />}
          variant="success"
        />
      </div>

      {/* Second row */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="GRA Signed Audits"
          value={statsLoading ? '—' : (stats?.gra_signed ?? 0)}
          subtitle="Legally compliant"
          icon={<CheckCircle size={20} />}
          variant="success"
        />
        <StatCard
          title="Completed Audits"
          value={statsLoading ? '—' : (stats?.completed ?? 0)}
          subtitle="All time"
          icon={<CheckCircle size={20} />}
        />
        <StatCard
          title="Total Audits"
          value={statsLoading ? '—' : (stats?.total ?? 0)}
          subtitle="All time"
          icon={<Activity size={20} />}
        />
        <StatCard
          title="Pending Assignment"
          value={statsLoading ? '—' : (stats?.pending ?? 0)}
          subtitle="Awaiting field officer"
          icon={<AlertTriangle size={20} />}
          variant="warning"
        />
      </div>

      {/* Charts + Recent Anomalies */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Anomaly breakdown pie */}
        <Card title="Anomaly Severity Breakdown" className="lg:col-span-1">
          {anomalyBreakdown.length > 0 ? (
            <ResponsiveContainer width="100%" height={220}>
              <PieChart>
                <Pie
                  data={anomalyBreakdown}
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={90}
                  paddingAngle={3}
                  dataKey="value"
                >
                  {anomalyBreakdown.map((_, index) => (
                    <Cell key={index} fill={COLORS[index % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip formatter={(value) => [`${value} flags`, '']} />
                <Legend />
              </PieChart>
            </ResponsiveContainer>
          ) : (
            <div className="h-[220px] flex items-center justify-center text-gray-400 text-sm">
              No open anomalies
            </div>
          )}
        </Card>

        {/* Recent anomalies table */}
        <Card title="Recent Open Anomalies" className="lg:col-span-2" noPadding>
          {anomaliesLoading ? (
            <div className="p-6 text-center text-gray-400 text-sm">Loading...</div>
          ) : anomalies.length === 0 ? (
            <div className="p-6 text-center text-gray-400 text-sm">
              ✓ No open anomalies
            </div>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Type</th>
                  <th>Level</th>
                  <th>Est. Loss</th>
                  <th>Detected</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {anomalies.map(flag => (
                  <tr key={flag.id} className="cursor-pointer hover:bg-gray-50">
                    <td>
                      <div>
                        <p className="font-medium text-gray-900 text-xs">{flag.anomaly_type.replace(/_/g, ' ')}</p>
                        <p className="text-gray-400 text-xs truncate max-w-[200px]">{flag.title}</p>
                      </div>
                    </td>
                    <td><AlertLevelBadge level={flag.alert_level} /></td>
                    <td className="font-mono text-sm">
                      {flag.estimated_loss_ghs
                        ? formatCurrency(flag.estimated_loss_ghs)
                        : '—'}
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

      {/* AWWA/IWA Water Balance Banner */}
      <Card className="bg-gradient-to-r from-brand-500 to-brand-700 text-white border-0">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-white font-bold">IWA/AWWA Water Balance Framework</h3>
            <p className="text-brand-100 text-sm mt-1">
              GN-WAAS aligns with international water audit standards.
              System Input Volume → Authorised Consumption + Water Losses (Real + Apparent).
            </p>
          </div>
          <div className="text-right">
            <p className="text-brand-100 text-xs">Target NRW</p>
            <p className="text-white text-2xl font-bold">≤ 20%</p>
            <p className="text-brand-100 text-xs">Current Ghana avg: 51.6%</p>
          </div>
        </div>
      </Card>
    </div>
  )
}
