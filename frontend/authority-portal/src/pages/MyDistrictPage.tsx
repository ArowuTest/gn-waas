import { useAuth } from '../contexts/AuthContext'
import { TrendingDown, Users, AlertTriangle, CheckCircle, Clock, Droplets, Loader2, RefreshCw } from 'lucide-react'
import { useMyDistrict, useNRWTrend } from '../hooks/useQueries'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'

export default function MyDistrictPage() {
  const { user } = useAuth()
  const { data, isLoading, isError, refetch, isFetching } = useMyDistrict()

  const district = data?.district
  const summary = data?.summary

  const { data: trend } = useNRWTrend(district?.id ?? '')

  if (isLoading) {
    return (
      <div className="p-6 flex items-center justify-center min-h-64">
        <Loader2 className="w-8 h-8 animate-spin text-green-700" />
      </div>
    )
  }

  if (isError || !district) {
    return (
      <div className="p-6 max-w-6xl mx-auto">
        <div className="bg-red-50 border border-red-200 rounded-xl p-6 text-center">
          <AlertTriangle className="w-8 h-8 text-red-500 mx-auto mb-2" />
          <p className="text-red-700 font-semibold">Failed to load district data</p>
          <p className="text-red-500 text-sm mt-1">
            Your account may not be assigned to a district. Contact your administrator.
          </p>
          <button onClick={() => refetch()} className="mt-3 text-sm text-red-600 underline">
            Try again
          </button>
        </div>
      </div>
    )
  }

  const nrwPct = district.loss_ratio_pct ?? 0
  const dataGrade = summary?.grade ?? 'N/A'
  const totalAccounts = summary?.total_accounts ?? district.total_connections
  const openAnomalies = summary?.open_anomalies ?? 0
  const criticalAnomalies = summary?.critical_anomalies ?? 0
  const estimatedLoss = summary?.total_estimated_loss_ghs ?? 0
  const confirmedLoss = summary?.total_confirmed_loss_ghs ?? 0
  const recovered = summary?.total_recovered_ghs ?? 0
  const successFee = recovered * 0.03

  return (
    <div className="p-6 max-w-6xl mx-auto">
      {/* Header */}
      <div className="mb-8 flex items-start justify-between">
        <div>
          <div className="flex items-center gap-2 text-sm text-gray-500 mb-2">
            <Droplets className="w-4 h-4 text-green-700" />
            <span>Ghana Water Limited — GN-WAAS Authority Portal</span>
          </div>
          <h1 className="text-2xl font-black text-gray-900">
            Welcome back, {user?.full_name?.split(' ')[0] || user?.email?.split('@')[0] || 'Officer'}
          </h1>
          <p className="text-gray-500 mt-1">
            {district.district_name} · {district.region}
            {district.is_pilot_district && (
              <span className="ml-2 text-xs bg-green-100 text-green-700 px-2 py-0.5 rounded-full font-medium">
                Pilot District
              </span>
            )}
          </p>
        </div>
        <button
          onClick={() => refetch()}
          disabled={isFetching}
          className="p-2 rounded-lg border border-gray-200 hover:bg-gray-50 disabled:opacity-50"
          title="Refresh"
        >
          <RefreshCw className={`w-4 h-4 text-gray-500 ${isFetching ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {/* NRW Banner */}
      <div className={`rounded-2xl p-6 mb-6 text-white ${
        nrwPct > 40 ? 'bg-red-700' :
        nrwPct > 25 ? 'bg-orange-600' : 'bg-green-700'
      }`}>
        <div className="flex items-center justify-between">
          <div>
            <div className="text-sm font-medium opacity-80 mb-1">
              Current NRW Rate — {district.district_name}
            </div>
            <div className="text-5xl font-black">
              {nrwPct > 0 ? `${nrwPct.toFixed(1)}%` : 'No data'}
            </div>
            <div className="text-sm opacity-80 mt-1">
              IWA Target: 20% · Ghana Average: 51.6% · Data Grade:{' '}
              <strong>{dataGrade}</strong>
            </div>
          </div>
          <TrendingDown className="w-16 h-16 opacity-30" />
        </div>
        {nrwPct > 0 && (
          <div className="mt-4 bg-white/20 rounded-full h-2">
            <div
              className="bg-white rounded-full h-2 transition-all"
              style={{ width: `${Math.min(nrwPct, 100)}%` }}
            />
          </div>
        )}
      </div>

      {/* KPI Grid */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {[
          {
            label: 'Total Connections',
            value: totalAccounts.toLocaleString(),
            sub: `${district.total_connections.toLocaleString()} registered`,
            icon: Users,
            color: 'blue',
          },
          {
            label: 'Open Anomalies',
            value: openAnomalies.toLocaleString(),
            sub: criticalAnomalies > 0 ? `${criticalAnomalies} critical` : 'Require attention',
            icon: AlertTriangle,
            color: 'red',
          },
          {
            label: 'Est. Loss (GHS)',
            value: `₵${(estimatedLoss / 1000).toFixed(0)}K`,
            sub: `₵${(confirmedLoss / 1000).toFixed(0)}K confirmed`,
            icon: CheckCircle,
            color: 'green',
          },
          {
            label: 'Zone Type',
            value: district.zone_type,
            sub: district.supply_status,
            icon: Clock,
            color: 'yellow',
          },
        ].map(({ label, value, sub, icon: Icon, color }) => (
          <div key={label} className="bg-white rounded-xl p-5 border border-gray-100 shadow-sm">
            <div className={`w-9 h-9 rounded-lg flex items-center justify-center mb-3 ${
              color === 'blue' ? 'bg-blue-100' :
              color === 'red' ? 'bg-red-100' :
              color === 'green' ? 'bg-green-100' : 'bg-yellow-100'
            }`}>
              <Icon className={`w-5 h-5 ${
                color === 'blue' ? 'text-blue-600' :
                color === 'red' ? 'text-red-600' :
                color === 'green' ? 'text-green-700' : 'text-yellow-600'
              }`} />
            </div>
            <div className="text-2xl font-black text-gray-900">{value}</div>
            <div className="text-sm font-semibold text-gray-700">{label}</div>
            <div className="text-xs text-gray-400 mt-0.5">{sub}</div>
          </div>
        ))}
      </div>

      {/* 12-month trend chart */}
      {trend && trend.length > 0 && (
        <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm mb-6">
          <h2 className="font-bold text-gray-900 mb-4">12-Month Anomaly Trend</h2>
          <ResponsiveContainer width="100%" height={200}>
            <LineChart data={trend} margin={{ top: 5, right: 10, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis dataKey="month" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip />
              <Line type="monotone" dataKey="open_flags" stroke="#dc2626" strokeWidth={2} dot={false} name="Open Flags" />
              <Line type="monotone" dataKey="resolved_flags" stroke="#16a34a" strokeWidth={2} dot={false} name="Resolved" />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Recovery summary */}
      {recovered > 0 && (
        <div className="bg-green-50 border border-green-200 rounded-2xl p-6">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm font-semibold text-green-700 mb-1">Revenue Recovered This Period</div>
              <div className="text-3xl font-black text-green-900">
                GHS {recovered.toLocaleString('en-GH', { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
              </div>
              <div className="text-sm text-green-600 mt-1">
                From confirmed audit events · GRA-signed
              </div>
            </div>
            <div className="text-right">
              <div className="text-sm text-green-600 font-medium">Success Fee (3%)</div>
              <div className="text-xl font-bold text-green-800">
                GHS {successFee.toLocaleString('en-GH', { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
