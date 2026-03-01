import { useAuth } from '../contexts/AuthContext'
import { TrendingDown, Users, AlertTriangle, CheckCircle, Clock, Droplets } from 'lucide-react'

const mockStats = {
  districtName: 'Accra West',
  region: 'Greater Accra',
  nrwPct: 38.2,
  totalAccounts: 12847,
  activeAccounts: 11203,
  openAnomalies: 47,
  resolvedThisMonth: 23,
  pendingJobs: 8,
  recoveredGHS: 84320,
  dataGrade: 'B',
}

export default function MyDistrictPage() {
  const { user } = useAuth()

  return (
    <div className="p-6 max-w-6xl mx-auto">
      {/* Header */}
      <div className="mb-8">
        <div className="flex items-center gap-2 text-sm text-gray-500 mb-2">
          <Droplets className="w-4 h-4 text-green-700" />
          <span>Ghana Water Limited — GN-WAAS Authority Portal</span>
        </div>
        <h1 className="text-2xl font-black text-gray-900">
          Welcome back, {user?.email?.split(' ')[0] || 'Officer'}
        </h1>
        <p className="text-gray-500 mt-1">
          {mockStats.districtName} District · {mockStats.region} · Last scan: 2 hours ago
        </p>
      </div>

      {/* NRW Banner */}
      <div className={`rounded-2xl p-6 mb-6 text-white ${
        mockStats.nrwPct > 40 ? 'bg-red-700' :
        mockStats.nrwPct > 25 ? 'bg-orange-600' : 'bg-green-700'
      }`}>
        <div className="flex items-center justify-between">
          <div>
            <div className="text-sm font-medium opacity-80 mb-1">Current NRW Rate — {mockStats.districtName}</div>
            <div className="text-5xl font-black">{mockStats.nrwPct}%</div>
            <div className="text-sm opacity-80 mt-1">
              IWA Target: 20% · Ghana Average: 51.6% · Data Grade: <strong>{mockStats.dataGrade}</strong>
            </div>
          </div>
          <TrendingDown className="w-16 h-16 opacity-30" />
        </div>
        <div className="mt-4 bg-white/20 rounded-full h-2">
          <div className="bg-white rounded-full h-2" style={{ width: `${Math.min(mockStats.nrwPct, 100)}%` }}></div>
        </div>
      </div>

      {/* KPI Grid */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {[
          { label: 'Total Accounts', value: mockStats.totalAccounts.toLocaleString(), sub: `${mockStats.activeAccounts.toLocaleString()} active`, icon: Users, color: 'blue' },
          { label: 'Open Anomalies', value: mockStats.openAnomalies, sub: 'Require attention', icon: AlertTriangle, color: 'red' },
          { label: 'Resolved This Month', value: mockStats.resolvedThisMonth, sub: 'Confirmed & closed', icon: CheckCircle, color: 'green' },
          { label: 'Pending Field Jobs', value: mockStats.pendingJobs, sub: 'Awaiting officers', icon: Clock, color: 'yellow' },
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

      {/* Recovery summary */}
      <div className="bg-green-50 border border-green-200 rounded-2xl p-6">
        <div className="flex items-center justify-between">
          <div>
            <div className="text-sm font-semibold text-green-700 mb-1">Revenue Recovered This Month</div>
            <div className="text-3xl font-black text-green-900">
              GHS {mockStats.recoveredGHS.toLocaleString()}
            </div>
            <div className="text-sm text-green-600 mt-1">
              From {mockStats.resolvedThisMonth} confirmed audit events · GRA-signed
            </div>
          </div>
          <div className="text-right">
            <div className="text-sm text-green-600 font-medium">Success Fee (3%)</div>
            <div className="text-xl font-bold text-green-800">
              GHS {(mockStats.recoveredGHS * 0.03).toLocaleString()}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
