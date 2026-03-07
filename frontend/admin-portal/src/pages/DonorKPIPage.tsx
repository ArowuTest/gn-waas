import React, { useState, useEffect } from 'react';

interface WaterBalance {
  system_input_volume_m3: number;
  authorised_consumption_m3: number;
  billed_authorised_m3: number;
  unbilled_authorised_m3: number;
  water_losses_m3: number;
  apparent_losses_m3: number;
  real_losses_m3: number;
  nrw_percent: number;
  nrw_target_percent: number;
  infrastructure_leakage_index: number;
}

interface RevenueKPIs {
  total_billed_ghs: number;
  total_collected_ghs: number;
  collection_efficiency_pct: number;
  revenue_gap_ghs: number;
  recovered_ghs: number;
  recovery_rate_pct: number;
  success_fees_ghs: number;
  ghs_usd_rate: number;
  recovered_usd: number;
}

interface AuditKPIs {
  total_audits: number;
  completed_audits: number;
  gra_signed_audits: number;
  provisional_audits: number;
  gra_compliance_rate_pct: number;
  anomalies_detected: number;
  anomalies_confirmed: number;
  confirmation_rate_pct: number;
}

interface FieldKPIs {
  total_jobs: number;
  completed_jobs: number;
  completion_rate_pct: number;
  active_officers: number;
  jobs_per_officer: number;
  gps_confirmed_accounts: number;
}

interface MoMoKPIs {
  total_transactions: number;
  matched_transactions: number;
  ghost_accounts: number;
  fraud_flags: number;
  total_amount_ghs: number;
  unmatched_amount_ghs: number;
}

interface WhistleblowerKPIs {
  total_tips: number;
  investigated_tips: number;
  confirmed_fraud: number;
  rewards_issued_ghs: number;
}

interface KPIReport {
  period: string;
  generated_at: string;
  water_balance: WaterBalance;
  revenue: RevenueKPIs;
  audits: AuditKPIs;
  field_ops: FieldKPIs;
  momo: MoMoKPIs;
  whistleblower: WhistleblowerKPIs;
}

interface TrendPoint {
  month: string;
  nrw_percent: number;
  recovered_ghs: number;
  audits_completed: number;
  gra_signed_pct: number;
}

const fmt = (n: number, decimals = 0) =>
  n.toLocaleString('en-GH', { minimumFractionDigits: decimals, maximumFractionDigits: decimals });

const pct = (n: number) => `${n.toFixed(1)}%`;

const DonorKPIPage: React.FC = () => {
  const [report, setReport] = useState<KPIReport | null>(null);
  const [trend, setTrend] = useState<TrendPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [period, setPeriod] = useState(() => {
    const now = new Date();
    return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
  });
  const [districtCode, setDistrictCode] = useState('');

  const apiBase = import.meta.env.VITE_API_URL || '';

  const fetchReport = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ period });
      if (districtCode) params.set('district_code', districtCode);

      const [kpiRes, trendRes] = await Promise.all([
        fetch(`${apiBase}/api/v1/reports/donor/kpis?${params}`, {
          headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
        }),
        fetch(`${apiBase}/api/v1/reports/donor/trend?months=12`, {
          headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
        }),
      ]);

      const kpiData = await kpiRes.json();
      const trendData = await trendRes.json();
      setReport(kpiData);
      setTrend(trendData.trend || []);
    } catch (e) {
      console.error('Failed to fetch donor KPIs', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchReport(); }, [period, districtCode]);

  if (loading) {
    return <div className="p-8 text-center text-gray-500">Loading donor KPI report...</div>;
  }

  if (!report) {
    return <div className="p-8 text-center text-red-500">Failed to load report</div>;
  }

  const wb = report.water_balance;
  const rev = report.revenue;
  const aud = report.audits;
  const field = report.field_ops;
  const momo = report.momo;
  const wb_tip = report.whistleblower;

  const nrwProgress = Math.min(100, (wb.nrw_percent / 60) * 100);
  const nrwColor = wb.nrw_percent > 40 ? 'bg-red-500' : wb.nrw_percent > 25 ? 'bg-yellow-500' : 'bg-green-500';

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Donor KPI Report</h1>
          <p className="text-sm text-gray-500 mt-1">
            IWA/AWWA M36 Water Balance Framework · Ghana National Water Audit & Assurance System
          </p>
        </div>
        <div className="flex gap-3">
          <input
            type="month"
            value={period}
            onChange={e => setPeriod(e.target.value)}
            className="border rounded px-3 py-2 text-sm"
          />
          <input
            type="text"
            value={districtCode}
            onChange={e => setDistrictCode(e.target.value)}
            placeholder="District code (optional)"
            className="border rounded px-3 py-2 text-sm w-48"
          />
          <button
            onClick={fetchReport}
            className="px-4 py-2 bg-blue-600 text-white rounded text-sm"
          >
            Refresh
          </button>
        </div>
      </div>

      <p className="text-xs text-gray-400">Period: {report.period} · Generated: {new Date(report.generated_at).toLocaleString()}</p>

      {/* NRW Hero */}
      <div className="bg-white rounded-xl border p-6">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-lg font-bold text-gray-900">Non-Revenue Water (NRW)</h2>
            <p className="text-sm text-gray-500">IWA/AWWA M36 Water Balance</p>
          </div>
          <div className="text-right">
            <div className={`text-4xl font-bold ${wb.nrw_percent > 40 ? 'text-red-600' : wb.nrw_percent > 25 ? 'text-yellow-600' : 'text-green-600'}`}>
              {pct(wb.nrw_percent)}
            </div>
            <div className="text-sm text-gray-500">Target: {pct(wb.nrw_target_percent)}</div>
          </div>
        </div>
        <div className="w-full bg-gray-200 rounded-full h-4 mb-2">
          <div className={`h-4 rounded-full ${nrwColor}`} style={{ width: `${nrwProgress}%` }} />
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4">
          {[
            { label: 'System Input', value: `${fmt(wb.system_input_volume_m3)} m³` },
            { label: 'Authorised Consumption', value: `${fmt(wb.authorised_consumption_m3)} m³` },
            { label: 'Apparent Losses', value: `${fmt(wb.apparent_losses_m3)} m³` },
            { label: 'Real Losses (ILI: ' + wb.infrastructure_leakage_index.toFixed(2) + ')', value: `${fmt(wb.real_losses_m3)} m³` },
          ].map(item => (
            <div key={item.label} className="bg-gray-50 rounded p-3">
              <p className="text-xs text-gray-500">{item.label}</p>
              <p className="text-sm font-bold text-gray-900">{item.value}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Revenue */}
      <div className="bg-white rounded-xl border p-6">
        <h2 className="text-lg font-bold text-gray-900 mb-4">Revenue Recovery</h2>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {[
            { label: 'Total Billed', value: `GH₵${fmt(rev.total_billed_ghs, 2)}`, sub: '' },
            { label: 'Revenue Gap', value: `GH₵${fmt(rev.revenue_gap_ghs, 2)}`, sub: 'Lost to fraud/NRW', color: 'text-red-600' },
            { label: 'Recovered', value: `GH₵${fmt(rev.recovered_ghs, 2)}`, sub: `USD ${fmt(rev.recovered_usd, 0)} @ ${rev.ghs_usd_rate.toFixed(2)}`, color: 'text-green-600' },
            { label: 'Recovery Rate', value: pct(rev.recovery_rate_pct), sub: `Success fees: GH₵${fmt(rev.success_fees_ghs, 2)}`, color: rev.recovery_rate_pct > 50 ? 'text-green-600' : 'text-orange-600' },
          ].map(item => (
            <div key={item.label} className="bg-gray-50 rounded p-4">
              <p className="text-xs text-gray-500">{item.label}</p>
              <p className={`text-xl font-bold ${item.color || 'text-gray-900'}`}>{item.value}</p>
              {item.sub && <p className="text-xs text-gray-400 mt-1">{item.sub}</p>}
            </div>
          ))}
        </div>
        <div className="mt-4 flex items-center gap-2">
          <span className="text-sm text-gray-600">Collection Efficiency:</span>
          <div className="flex-1 bg-gray-200 rounded-full h-2">
            <div className="bg-blue-500 h-2 rounded-full" style={{ width: `${Math.min(100, rev.collection_efficiency_pct)}%` }} />
          </div>
          <span className="text-sm font-medium">{pct(rev.collection_efficiency_pct)}</span>
        </div>
      </div>

      {/* Audits + Field Ops */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Audits */}
        <div className="bg-white rounded-xl border p-6">
          <h2 className="text-lg font-bold text-gray-900 mb-4">Audit Performance</h2>
          <div className="space-y-3">
            {[
              { label: 'Total Audits', value: fmt(aud.total_audits) },
              { label: 'Completed', value: fmt(aud.completed_audits) },
              { label: 'GRA Signed', value: `${fmt(aud.gra_signed_audits)} (${pct(aud.gra_compliance_rate_pct)})`, color: aud.gra_compliance_rate_pct > 80 ? 'text-green-600' : 'text-orange-600' },
              { label: 'Provisional', value: fmt(aud.provisional_audits), color: 'text-yellow-600' },
              { label: 'Anomalies Detected', value: fmt(aud.anomalies_detected) },
              { label: 'Confirmed Fraud', value: `${fmt(aud.anomalies_confirmed)} (${pct(aud.confirmation_rate_pct)})`, color: 'text-red-600' },
            ].map(item => (
              <div key={item.label} className="flex justify-between items-center py-1 border-b border-gray-100">
                <span className="text-sm text-gray-600">{item.label}</span>
                <span className={`text-sm font-bold ${item.color || 'text-gray-900'}`}>{item.value}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Field Ops */}
        <div className="bg-white rounded-xl border p-6">
          <h2 className="text-lg font-bold text-gray-900 mb-4">Field Operations</h2>
          <div className="space-y-3">
            {[
              { label: 'Total Jobs', value: fmt(field.total_jobs) },
              { label: 'Completed', value: `${fmt(field.completed_jobs)} (${pct(field.completion_rate_pct)})` },
              { label: 'Active Officers', value: fmt(field.active_officers) },
              { label: 'Jobs per Officer', value: field.jobs_per_officer.toFixed(1) },
              { label: 'GPS-Confirmed Accounts', value: fmt(field.gps_confirmed_accounts) },
            ].map(item => (
              <div key={item.label} className="flex justify-between items-center py-1 border-b border-gray-100">
                <span className="text-sm text-gray-600">{item.label}</span>
                <span className="text-sm font-bold text-gray-900">{item.value}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* MoMo + Whistleblower */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="bg-white rounded-xl border p-6">
          <h2 className="text-lg font-bold text-gray-900 mb-4">Mobile Money Reconciliation</h2>
          <div className="space-y-3">
            {[
              { label: 'Total Transactions', value: fmt(momo.total_transactions) },
              { label: 'Matched', value: fmt(momo.matched_transactions) },
              { label: 'Ghost Accounts', value: fmt(momo.ghost_accounts), color: 'text-red-600' },
              { label: 'Fraud Flags', value: fmt(momo.fraud_flags), color: 'text-red-600' },
              { label: 'Total Amount', value: `GH₵${fmt(momo.total_amount_ghs, 2)}` },
              { label: 'Unmatched Amount', value: `GH₵${fmt(momo.unmatched_amount_ghs, 2)}`, color: 'text-orange-600' },
            ].map(item => (
              <div key={item.label} className="flex justify-between items-center py-1 border-b border-gray-100">
                <span className="text-sm text-gray-600">{item.label}</span>
                <span className={`text-sm font-bold ${item.color || 'text-gray-900'}`}>{item.value}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="bg-white rounded-xl border p-6">
          <h2 className="text-lg font-bold text-gray-900 mb-4">Whistleblower Programme</h2>
          <div className="space-y-3">
            {[
              { label: 'Total Tips', value: fmt(wb_tip.total_tips) },
              { label: 'Under Investigation', value: fmt(wb_tip.investigated_tips) },
              { label: 'Confirmed Fraud', value: fmt(wb_tip.confirmed_fraud), color: 'text-red-600' },
              { label: 'Rewards Issued', value: `GH₵${fmt(wb_tip.rewards_issued_ghs, 2)}`, color: 'text-green-600' },
            ].map(item => (
              <div key={item.label} className="flex justify-between items-center py-1 border-b border-gray-100">
                <span className="text-sm text-gray-600">{item.label}</span>
                <span className={`text-sm font-bold ${item.color || 'text-gray-900'}`}>{item.value}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* 12-Month NRW Trend */}
      {trend.length > 0 && (
        <div className="bg-white rounded-xl border p-6">
          <h2 className="text-lg font-bold text-gray-900 mb-4">12-Month NRW Trend</h2>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  {['Month','NRW %','Recovered (GH₵)','Audits','GRA Signed %'].map(h => (
                    <th key={h} className="px-3 py-2 text-left text-xs font-medium text-gray-500">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {trend.map(pt => (
                  <tr key={pt.month} className="border-b hover:bg-gray-50">
                    <td className="px-3 py-2 font-medium">{pt.month}</td>
                    <td className={`px-3 py-2 font-bold ${pt.nrw_percent > 40 ? 'text-red-600' : pt.nrw_percent > 25 ? 'text-yellow-600' : 'text-green-600'}`}>
                      {pct(pt.nrw_percent)}
                    </td>
                    <td className="px-3 py-2 text-green-600">{fmt(pt.recovered_ghs, 2)}</td>
                    <td className="px-3 py-2">{fmt(pt.audits_completed)}</td>
                    <td className="px-3 py-2">{pct(pt.gra_signed_pct)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Footer */}
      <div className="text-xs text-gray-400 text-center pb-4">
        GN-WAAS · IWA/AWWA M36 Water Balance Framework · Deployed on NITA Sovereign Infrastructure
      </div>
    </div>
  );
};

export default DonorKPIPage;
