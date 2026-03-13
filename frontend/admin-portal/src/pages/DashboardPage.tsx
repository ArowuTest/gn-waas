import { useState } from 'react'
import {
  AlertTriangle, TrendingDown, DollarSign, CheckCircle,
  Activity, RefreshCw, Droplets, ArrowUpRight, Zap,
  Users, MapPin, BarChart2, Shield, TrendingUp, Clock,
  Award, Target, Layers
} from 'lucide-react'
import { StatCard, Card } from '../components/ui/Card'
import { AlertLevelBadge, StatusBadge } from '../components/ui/Badge'
import {
  useDashboardStats, useAnomalies, useDistricts,
  useWaterBalance, useRevenueSummary, useWorkforceSummary,
  useActiveOfficers, useLeakagePipeline,
} from '../hooks/useQueries'
import { formatCurrency, formatRelativeTime, formatNumber } from '../lib/utils'
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell, Legend, AreaChart, Area
} from 'recharts'

// ── Colour constants ──────────────────────────────────────────────────────────
const ZONE_COLORS: Record<string, string> = {
  GREEN:  '#10b981',
  YELLOW: '#f59e0b',
  RED:    '#ef4444',
  GREY:   '#9ca3af',
}

const IWA_GRADE_COLORS: Record<string, string> = {
  A: '#10b981',
  B: '#3b82f6',
  C: '#f59e0b',
  D: '#ef4444',
}

// ── ILI Grade badge ───────────────────────────────────────────────────────────
function ILIGradeBadge({ grade }: { grade: string }) {
  const color = IWA_GRADE_COLORS[grade] ?? '#9ca3af'
  const labels: Record<string, string> = {
    A: 'Excellent (ILI < 1)',
    B: 'Good (ILI 1–2)',
    C: 'Fair (ILI 2–4)',
    D: 'Poor (ILI > 4)',
  }
  return (
    <span
      className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-bold text-white"
      style={{ backgroundColor: color }}
    >
      Grade {grade} — {labels[grade] ?? 'Unknown'}
    </span>
  )
}

// ── AWWA Water Balance bar ────────────────────────────────────────────────────
function WaterBalanceBar({ wb }: { wb: ReturnType<typeof useWaterBalance>['data'] }) {
  if (!wb || wb.length === 0) {
    return (
      <div className="flex items-center justify-center h-24 text-gray-400 text-sm">
        No water balance data — run a sentinel scan to populate
      </div>
    )
  }
  const latest = wb[0]
  const total = latest.system_input_m3 || 1
  const billedPct   = (latest.billed_metered_m3 / total) * 100
  const unbilledPct = ((latest.unbilled_metered_m3 + latest.unbilled_unmetered_m3) / total) * 100
  const apparentPct = (latest.total_apparent_losses_m3 / total) * 100
  const realPct     = (latest.total_real_losses_m3 / total) * 100

  const segments = [
    { label: 'Billed Metered',    pct: billedPct,   color: '#10b981', m3: latest.billed_metered_m3 },
    { label: 'Unbilled Auth.',    pct: unbilledPct,  color: '#3b82f6', m3: latest.unbilled_metered_m3 + latest.unbilled_unmetered_m3 },
    { label: 'Apparent Losses',   pct: apparentPct,  color: '#f59e0b', m3: latest.total_apparent_losses_m3 },
    { label: 'Real Losses',       pct: realPct,      color: '#ef4444', m3: latest.total_real_losses_m3 },
  ]

  return (
    <div className="space-y-3">
      {/* Stacked bar */}
      <div className="flex h-8 rounded-lg overflow-hidden w-full">
        {segments.map(s => (
          <div
            key={s.label}
            style={{ width: `${Math.max(s.pct, 0.5)}%`, backgroundColor: s.color }}
            title={`${s.label}: ${s.pct.toFixed(1)}%`}
          />
        ))}
      </div>
      {/* Legend */}
      <div className="grid grid-cols-2 gap-x-6 gap-y-1.5">
        {segments.map(s => (
          <div key={s.label} className="flex items-center gap-2">
            <div className="w-3 h-3 rounded-sm flex-shrink-0" style={{ backgroundColor: s.color }} />
            <span className="text-xs text-gray-600 flex-1">{s.label}</span>
            <span className="text-xs font-mono font-semibold text-gray-800">
              {s.pct.toFixed(1)}%
            </span>
          </div>
        ))}
      </div>
      {/* ILI + NRW */}
      <div className="flex items-center justify-between pt-2 border-t border-gray-100">
        <div className="flex items-center gap-3">
          <div>
            <p className="text-xs text-gray-500">NRW</p>
            <p className="text-lg font-black text-gray-900">{latest.nrw_percent?.toFixed(1) ?? '—'}%</p>
          </div>
          <div className="w-px h-8 bg-gray-200" />
          <div>
            <p className="text-xs text-gray-500">ILI Score</p>
            <p className="text-lg font-black text-gray-900">{latest.ili?.toFixed(2) ?? '—'}</p>
          </div>
          <div className="w-px h-8 bg-gray-200" />
          <div>
            <p className="text-xs text-gray-500">System Input</p>
            <p className="text-lg font-black text-gray-900">{formatNumber(Math.round(latest.system_input_m3))} m³</p>
          </div>
        </div>
        <ILIGradeBadge grade={latest.iwa_grade} />
      </div>
      {/* Staleness indicator — warn if data is older than 7 days */}
      {(() => {
        const ts = latest.computed_at ?? latest.period_start
        if (!ts) return null
        const ageMs = Date.now() - new Date(ts).getTime()
        const ageDays = Math.floor(ageMs / 86_400_000)
        const label = new Date(ts).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })
        const isStale = ageDays > 7
        return (
          <p className={`text-xs mt-1 ${isStale ? 'text-amber-600 font-semibold' : 'text-gray-400'}`}>
            {isStale ? `⚠ Data is ${ageDays} days old — ` : 'Last computed: '}
            {label}{isStale ? '. Run a Sentinel scan to refresh.' : ''}
          </p>
        )
      })()}
    </div>
  )
}

// ── Revenue Recovery panel ────────────────────────────────────────────────────
function RevenuePanel({ districtId }: { districtId?: string }) {
  const { data: rev } = useRevenueSummary(districtId)

  const byTypeData = (rev?.by_type ?? []).map(t => ({
    name: t.recovery_type.replace(/_/g, ' '),
    recovered: t.recovered_ghs,
    fee: t.success_fee_ghs,
  }))

  return (
    <div className="space-y-4">
      {/* KPI row */}
      <div className="grid grid-cols-3 gap-3">
        <div className="bg-emerald-50 rounded-xl p-3 text-center">
          <p className="text-xs text-emerald-600 font-semibold uppercase tracking-wide">Recovered</p>
          <p className="text-xl font-black text-emerald-700 mt-0.5">
            {formatCurrency(rev?.total_recovered_ghs ?? 0)}
          </p>
          <p className="text-xs text-emerald-500 mt-0.5">{rev?.collected_count ?? 0} collected</p>
        </div>
        <div className="bg-blue-50 rounded-xl p-3 text-center">
          <p className="text-xs text-blue-600 font-semibold uppercase tracking-wide">Success Fee (3%)</p>
          <p className="text-xl font-black text-blue-700 mt-0.5">
            {formatCurrency(rev?.total_success_fee_ghs ?? 0)}
          </p>
          <p className="text-xs text-blue-500 mt-0.5">{rev?.confirmed_count ?? 0} confirmed</p>
        </div>
        <div className="bg-amber-50 rounded-xl p-3 text-center">
          <p className="text-xs text-amber-600 font-semibold uppercase tracking-wide">Variance Found</p>
          <p className="text-xl font-black text-amber-700 mt-0.5">
            {formatCurrency(rev?.total_variance_ghs ?? 0)}
          </p>
          <p className="text-xs text-amber-500 mt-0.5">{rev?.pending_count ?? 0} pending</p>
        </div>
      </div>

      {/* By-type chart */}
      {byTypeData.length > 0 ? (
        <ResponsiveContainer width="100%" height={140}>
          <BarChart data={byTypeData} margin={{ top: 0, right: 0, left: -20, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
            <XAxis dataKey="name" tick={{ fontSize: 10 }} />
            <YAxis tick={{ fontSize: 10 }} />
            <Tooltip formatter={((v: number) => formatCurrency(v)) as any} />
            <Bar dataKey="recovered" name="Recovered" fill="#10b981" radius={[3,3,0,0]} />
            <Bar dataKey="fee" name="Success Fee" fill="#3b82f6" radius={[3,3,0,0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <div className="flex items-center justify-center h-20 text-gray-400 text-sm">
          No recovery events yet — confirm audit findings to track revenue
        </div>
      )}
    </div>
  )
}

// ── Workforce panel ───────────────────────────────────────────────────────────
function WorkforcePanel({ districtId }: { districtId?: string }) {
  const { data: wf } = useWorkforceSummary()
  const { data: officers } = useActiveOfficers(districtId)

  const activeOfficers = officers ?? []

  return (
    <div className="space-y-4">
      {/* KPI row */}
      <div className="grid grid-cols-4 gap-2">
        {[
          { label: 'Total Officers', value: wf?.total_field_officers ?? 0, color: 'text-gray-900' },
          { label: 'Active Now',     value: wf?.active_now ?? 0,           color: 'text-emerald-600' },
          { label: 'On Active Job',  value: wf?.on_active_job ?? 0,        color: 'text-blue-600' },
          { label: 'Done Today',     value: wf?.jobs_completed_today ?? 0, color: 'text-purple-600' },
        ].map(k => (
          <div key={k.label} className="bg-gray-50 rounded-xl p-2.5 text-center">
            <p className={`text-xl font-black ${k.color}`}>{k.value}</p>
            <p className="text-xs text-gray-500 mt-0.5 leading-tight">{k.label}</p>
          </div>
        ))}
      </div>

      {/* Active officers list */}
      {activeOfficers.length > 0 ? (
        <div className="space-y-1.5 max-h-40 overflow-y-auto">
          {activeOfficers.slice(0, 6).map(o => (
            <div key={o.officer_id} className="flex items-center gap-2 p-2 bg-gray-50 rounded-lg">
              <div className="w-6 h-6 bg-emerald-100 rounded-full flex items-center justify-center flex-shrink-0">
                <div className="w-2 h-2 bg-emerald-500 rounded-full" />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-xs font-semibold text-gray-900 truncate">{o.full_name}</p>
                <p className="text-xs text-gray-400">{o.employee_id}</p>
              </div>
              <div className="text-right flex-shrink-0">
                <p className="text-xs text-gray-400">{formatRelativeTime(o.last_seen_at)}</p>
                {o.field_job_id && (
                  <span className="text-xs text-blue-600 font-semibold">On job</span>
                )}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="flex items-center justify-center h-16 text-gray-400 text-sm">
          No officers active in the last 30 minutes
        </div>
      )}
    </div>
  )
}

// ── District NRW heatmap summary ──────────────────────────────────────────────
function DistrictHeatmapSummary({ districts }: { districts: any[] }) {
  const counts = { RED: 0, YELLOW: 0, GREEN: 0, GREY: 0 }
  districts.forEach(d => {
    const z = (d.zone_type ?? 'GREY') as keyof typeof counts
    if (z in counts) counts[z]++
  })

  const items = [
    { zone: 'RED',    label: 'Critical (>40% NRW)',  count: counts.RED,    bg: 'bg-red-50',    text: 'text-red-700',    dot: 'bg-red-500' },
    { zone: 'YELLOW', label: 'Warning (20–40% NRW)', count: counts.YELLOW, bg: 'bg-amber-50',  text: 'text-amber-700',  dot: 'bg-amber-500' },
    { zone: 'GREEN',  label: 'Good (<20% NRW)',       count: counts.GREEN,  bg: 'bg-emerald-50',text: 'text-emerald-700',dot: 'bg-emerald-500' },
    { zone: 'GREY',   label: 'No data',               count: counts.GREY,   bg: 'bg-gray-50',   text: 'text-gray-600',   dot: 'bg-gray-400' },
  ]

  return (
    <div className="grid grid-cols-2 gap-2">
      {items.map(i => (
        <div key={i.zone} className={`${i.bg} rounded-xl p-3 flex items-center gap-3`}>
          <div className={`w-3 h-3 rounded-full ${i.dot} flex-shrink-0`} />
          <div>
            <p className={`text-lg font-black ${i.text}`}>{i.count}</p>
            <p className="text-xs text-gray-500 leading-tight">{i.label}</p>
          </div>
        </div>
      ))}
    </div>
  )
}

// ── Main Dashboard ────────────────────────────────────────────────────────────
export function DashboardPage() {
  const [selectedDistrict, setSelectedDistrict] = useState<string>('')
  const { data: districts } = useDistricts()
  const { data: stats, isLoading: statsLoading, refetch } = useDashboardStats(selectedDistrict || undefined)
  const { data: anomaliesData, isLoading: anomaliesLoading } = useAnomalies({
    district_id: selectedDistrict || undefined,
    limit: 8,
    status: 'OPEN',
  })
  const { data: waterBalance } = useWaterBalance(selectedDistrict || undefined)
  const { data: pipeline } = useLeakagePipeline(selectedDistrict || undefined)

  const anomalies = anomaliesData?.data || []

  // Separate revenue leakage from compliance flags for display
  const revenueLeakageAnomalies = anomalies.filter(a =>
    !['OUTAGE_CONSUMPTION', 'ADDRESS_UNVERIFIED'].includes(a.anomaly_type)
  )
  const complianceAnomalies = anomalies.filter(a => a.anomaly_type === 'OUTAGE_CONSUMPTION')
  const dataQualityAnomalies = anomalies.filter(a => a.anomaly_type === 'ADDRESS_UNVERIFIED')

  // Sort anomalies by monthly leakage GHS (highest first) — revenue impact ordering
  const sortedAnomalies = [...anomalies].sort((a, b) => {
    const aGHS = a.monthly_leakage_ghs ?? a.estimated_loss_ghs ?? 0
    const bGHS = b.monthly_leakage_ghs ?? b.estimated_loss_ghs ?? 0
    return bGHS - aGHS
  })

  const anomalyBreakdown = [
    { name: 'Critical', value: anomalies.filter(a => a.alert_level === 'CRITICAL').length, color: '#dc2626' },
    { name: 'High',     value: anomalies.filter(a => a.alert_level === 'HIGH').length,     color: '#ea580c' },
    { name: 'Medium',   value: anomalies.filter(a => a.alert_level === 'MEDIUM').length,   color: '#d97706' },
    { name: 'Low',      value: anomalies.filter(a => a.alert_level === 'LOW').length,      color: '#2563eb' },
  ].filter(d => d.value > 0)

  return (
    <div className="space-y-6">
      {/* ── Page Header ─────────────────────────────────────────────────────── */}
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
            Real-time water audit overview — Ghana National Water Audit &amp; Assurance System
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

      {/* ── Revenue Leakage Pipeline — PRIMARY MISSION METRIC ─────────────── */}
      {/* Every GHS figure = money GWL should be collecting but isn't.         */}
      {/* Pipeline: Detected → Field Verified → Confirmed → GRA Signed → Collected */}
      <div className="bg-gradient-to-r from-red-50 to-orange-50 border border-red-100 rounded-2xl p-4">
        <div className="flex items-center justify-between mb-3">
          <div>
            <h2 className="text-sm font-bold text-red-900 flex items-center gap-2">
              <TrendingDown size={16} className="text-red-600" />
              Revenue Leakage Pipeline
            </h2>
            <p className="text-xs text-red-600 mt-0.5">
              Monthly GHS being lost at each stage — GWL is delivering water but not collecting payment
            </p>
          </div>
          <div className="text-right">
            <p className="text-xs text-gray-500">Annual exposure</p>
            <p className="text-xl font-black text-red-700">
              {formatCurrency((pipeline?.total_detected_annual_ghs ?? 0))}
            </p>
          </div>
        </div>
        <div className="grid grid-cols-5 gap-2">
          {[
            { label: 'Detected', count: pipeline?.detected?.count ?? 0, ghs: pipeline?.detected?.ghs ?? 0, color: 'bg-red-500', textColor: 'text-red-700', desc: 'Open flags' },
            { label: 'Field Verified', count: pipeline?.field_verified?.count ?? 0, ghs: pipeline?.field_verified?.ghs ?? 0, color: 'bg-orange-500', textColor: 'text-orange-700', desc: 'Outcome recorded' },
            { label: 'Confirmed', count: pipeline?.confirmed?.count ?? 0, ghs: pipeline?.confirmed?.ghs ?? 0, color: 'bg-amber-500', textColor: 'text-amber-700', desc: 'Fraud confirmed' },
            { label: 'GRA Signed', count: pipeline?.gra_signed?.count ?? 0, ghs: pipeline?.gra_signed?.ghs ?? 0, color: 'bg-blue-500', textColor: 'text-blue-700', desc: 'Legally binding' },
            { label: 'Collected', count: pipeline?.collected?.count ?? 0, ghs: pipeline?.collected?.ghs ?? 0, color: 'bg-emerald-500', textColor: 'text-emerald-700', desc: 'Money recovered' },
          ].map((stage, i) => (
            <div key={stage.label} className="bg-white rounded-xl p-3 text-center shadow-sm">
              <div className={`w-2 h-2 rounded-full ${stage.color} mx-auto mb-1.5`} />
              <p className={`text-lg font-black ${stage.textColor}`}>{formatCurrency(stage.ghs)}</p>
              <p className="text-xs font-bold text-gray-700">{stage.label}</p>
              <p className="text-xs text-gray-400">{stage.count} flags</p>
              <p className="text-xs text-gray-400 mt-0.5">{stage.desc}</p>
            </div>
          ))}
        </div>
        {/* Compliance and data quality flags — separate from revenue leakage */}
        {((pipeline?.compliance_flags_open ?? 0) > 0 || (pipeline?.data_quality_flags_open ?? 0) > 0) && (
          <div className="mt-3 pt-3 border-t border-red-100 flex items-center gap-4 text-xs text-gray-500">
            <span className="flex items-center gap-1">
              <Shield size={12} className="text-blue-500" />
              <strong className="text-blue-700">{pipeline?.compliance_flags_open ?? 0}</strong> compliance flags (PURC violations — not revenue leakage)
            </span>
            <span className="flex items-center gap-1">
              <MapPin size={12} className="text-gray-400" />
              <strong className="text-gray-600">{pipeline?.data_quality_flags_open ?? 0}</strong> address verification pending (field job required)
            </span>
          </div>
        )}
      </div>

      {/* ── KPI Row — Audit Operations ──────────────────────────────────────── */}
      <div className="grid grid-cols-2 xl:grid-cols-4 gap-4">
        <StatCard
          title="Revenue Leakage Flags"
          value={statsLoading ? '—' : formatNumber(revenueLeakageAnomalies.length)}
          subtitle="Sorted by GHS impact"
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
          title="Confirmed & Collected"
          value={statsLoading ? '—' : formatCurrency(pipeline?.total_collected_ghs ?? 0)}
          subtitle="Money actually recovered"
          icon={<DollarSign size={18} />}
          variant="success"
        />
        <StatCard
          title="Recovery Rate"
          value={statsLoading ? '—' : `${(pipeline?.recovery_rate_pct ?? 0).toFixed(1)}%`}
          subtitle="Collected / confirmed"
          icon={<TrendingUp size={18} />}
          variant="success"
        />
      </div>

      {/* ── IWA/AWWA Water Balance + District Heatmap ───────────────────────── */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
        <Card
          title="IWA/AWWA M36 Water Balance"
          subtitle="System Input Volume decomposition"
          className="xl:col-span-2"
          action={
            <a href="/nrw" className="text-xs text-brand-600 hover:text-brand-700 font-semibold flex items-center gap-1">
              Full analysis <ArrowUpRight size={12} />
            </a>
          }
        >
          <WaterBalanceBar wb={waterBalance} />
        </Card>

        <Card
          title="District NRW Heatmap"
          subtitle="Zone classification by NRW %"
          action={
            <a href="/dma-map" className="text-xs text-brand-600 hover:text-brand-700 font-semibold flex items-center gap-1">
              View map <ArrowUpRight size={12} />
            </a>
          }
        >
          <DistrictHeatmapSummary districts={districts ?? []} />
          <div className="mt-3 pt-3 border-t border-gray-100 flex items-center justify-between text-xs text-gray-500">
            <span>{districts?.length ?? 0} total districts</span>
            <span className="font-semibold text-gray-700">
              {districts?.filter(d => d.is_active).length ?? 0} active
            </span>
          </div>
        </Card>
      </div>

      {/* ── Revenue Recovery + Workforce ────────────────────────────────────── */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
        <Card
          title="Revenue Recovery"
          subtitle="Managed-service monetisation — 3% success fee model"
          action={
            <a href="/reports" className="text-xs text-brand-600 hover:text-brand-700 font-semibold flex items-center gap-1">
              Full report <ArrowUpRight size={12} />
            </a>
          }
        >
          <RevenuePanel districtId={selectedDistrict || undefined} />
        </Card>

        <Card
          title="Workforce Oversight"
          subtitle="Field officer GPS tracking — last 30 minutes"
          action={
            <a href="/field-jobs" className="text-xs text-brand-600 hover:text-brand-700 font-semibold flex items-center gap-1">
              Field jobs <ArrowUpRight size={12} />
            </a>
          }
        >
          <WorkforcePanel districtId={selectedDistrict || undefined} />
        </Card>
      </div>

      {/* ── Anomaly Breakdown + Recent Anomalies ────────────────────────────── */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
        {/* Pie chart */}
        <Card title="Anomaly Severity" subtitle="Open flags by alert level">
          {anomalyBreakdown.length > 0 ? (
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
                >
                  {anomalyBreakdown.map((entry, i) => (
                    <Cell key={i} fill={entry.color} />
                  ))}
                </Pie>
                <Legend
                  formatter={(value, entry: any) => (
                    <span className="text-xs text-gray-600">
                      {value} ({entry.payload.value})
                    </span>
                  )}
                />
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          ) : (
            <div className="flex flex-col items-center justify-center h-40 text-center">
              <CheckCircle size={32} className="text-emerald-400 mb-2" />
              <p className="text-sm text-gray-500">No open anomalies</p>
              <p className="text-xs text-gray-400 mt-0.5">System is operating normally</p>
            </div>
          )}
        </Card>

        {/* Recent anomalies table */}
        <Card
          title="Revenue Leakage Flags"
          subtitle="Sorted by monthly GHS impact — highest first"
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

      {/* ── IWA Framework Banner ─────────────────────────────────────────────── */}
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
              <h3 className="text-white font-bold text-base">IWA/AWWA M36 Water Balance Framework</h3>
              <p className="text-gray-400 text-sm mt-1 max-w-lg">
                GN-WAAS implements the international water audit standard.
                System Input Volume = Authorised Consumption + Water Losses (Real + Apparent).
                ILI (Infrastructure Leakage Index) grades performance A–D.
              </p>
            </div>
          </div>
          <div className="text-right flex-shrink-0 space-y-1">
            <div>
              <p className="text-gray-500 text-xs font-semibold uppercase tracking-wider">IWA Target NRW</p>
              <p className="text-white text-3xl font-black">≤ 20%</p>
            </div>
            <div>
              <p className="text-gray-500 text-xs">Ghana avg: 51.6%</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
