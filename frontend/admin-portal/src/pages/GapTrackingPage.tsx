import React, { useState, useEffect } from 'react';
import { apiClient } from '../api/client';

// ── Types ─────────────────────────────────────────────────────────────────────

interface GapSummary {
  total_gaps_identified: number;
  total_gap_value_ghs: number;
  total_recovered_ghs: number;
  total_pending_ghs: number;
  recovery_rate_pct: number;
  success_fees_earned_ghs: number;
  gra_signed_audits: number;
  gra_provisional_audits: number;
  avg_days_to_recovery: number;
}

interface GapRow {
  id: string;
  audit_reference: string;
  district_name: string;
  account_number: string | null;
  customer_name: string | null;
  anomaly_type: string;
  variance_amount_ghs: number;
  gra_compliance_status: string;
  gra_sdc_id: string | null;
  created_at: string;
  recovery_id: string | null;
  recovered_amount_ghs: number | null;
  success_fee_ghs: number | null;
  recovery_status: string | null;
  confirmed_at: string | null;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

const fmt = (n: number) =>
  new Intl.NumberFormat('en-GH', { style: 'currency', currency: 'GHS', minimumFractionDigits: 2 }).format(n);

const graStatusBadge = (status: string) => {
  const map: Record<string, string> = {
    SIGNED: 'bg-green-100 text-green-700',
    PROVISIONAL: 'bg-yellow-100 text-yellow-700',
    RETRY_QUEUED: 'bg-orange-100 text-orange-700',
    PENDING: 'bg-gray-100 text-gray-600',
    FAILED: 'bg-red-100 text-red-700',
    EXEMPT_MANUAL: 'bg-purple-100 text-purple-700',
  };
  return map[status] || 'bg-gray-100 text-gray-600';
};

const recoveryStatusBadge = (status: string | null) => {
  if (!status) return 'bg-gray-100 text-gray-500';
  const map: Record<string, string> = {
    CONFIRMED: 'bg-green-100 text-green-700',
    PENDING: 'bg-yellow-100 text-yellow-700',
    DISPUTED: 'bg-red-100 text-red-700',
    WRITTEN_OFF: 'bg-gray-100 text-gray-500',
  };
  return map[status] || 'bg-gray-100 text-gray-600';
};

// ── Component ─────────────────────────────────────────────────────────────────

export default function GapTrackingPage() {
  const [summary, setSummary] = useState<GapSummary | null>(null);
  const [gaps, setGaps] = useState<GapRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [filterPeriod, setFilterPeriod] = useState('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchData();
  }, [page, filterPeriod]);

  const fetchData = async () => {
    setLoading(true);
    setError(null);
    try {
      const params: Record<string, string> = { page: String(page), limit: '50' };
      if (filterPeriod) params.period = filterPeriod;

      const [summaryRes, gapsRes] = await Promise.all([
        apiClient.get('/gaps/summary', { params: filterPeriod ? { period: filterPeriod } : {} }),
        apiClient.get('/gaps', { params }),
      ]);
      setSummary(summaryRes.data);
      setGaps(gapsRes.data.gaps || []);
    } catch (err: any) {
      setError('Failed to load gap data: ' + (err.response?.data?.error || err.message));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Revenue Gap Tracking</h1>
          <p className="text-sm text-gray-500 mt-1">
            All identified revenue gaps, GRA compliance status, and recovery progress.
          </p>
        </div>
        <div className="flex gap-3 items-center">
          <label className="text-sm text-gray-600">Period:</label>
          <input
            type="month"
            value={filterPeriod}
            onChange={e => { setFilterPeriod(e.target.value); setPage(1); }}
            className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm"
          />
          {filterPeriod && (
            <button
              onClick={() => { setFilterPeriod(''); setPage(1); }}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
              Clear
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-red-700 text-sm">
          {error}
        </div>
      )}

      {/* Summary Cards */}
      {summary && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          <SummaryCard
            label="Total Gaps Identified"
            value={summary.total_gaps_identified.toLocaleString()}
            sub={fmt(summary.total_gap_value_ghs) + ' total value'}
            color="blue"
          />
          <SummaryCard
            label="Recovered"
            value={fmt(summary.total_recovered_ghs)}
            sub={`${summary.recovery_rate_pct.toFixed(1)}% recovery rate`}
            color="green"
          />
          <SummaryCard
            label="Pending Recovery"
            value={fmt(summary.total_pending_ghs)}
            sub={`Avg ${summary.avg_days_to_recovery.toFixed(0)} days to recover`}
            color="yellow"
          />
          <SummaryCard
            label="Success Fees Earned"
            value={fmt(summary.success_fees_earned_ghs)}
            sub={`${summary.gra_signed_audits} GRA-signed · ${summary.gra_provisional_audits} provisional`}
            color="purple"
          />
        </div>
      )}

      {/* Recovery Rate Progress Bar */}
      {summary && (
        <div className="bg-white rounded-xl border border-gray-200 p-4 mb-6">
          <div className="flex justify-between text-sm text-gray-600 mb-2">
            <span>Recovery Progress</span>
            <span className="font-semibold">{summary.recovery_rate_pct.toFixed(1)}%</span>
          </div>
          <div className="h-3 bg-gray-100 rounded-full overflow-hidden">
            <div
              className="h-full bg-gradient-to-r from-blue-500 to-green-500 rounded-full transition-all"
              style={{ width: `${Math.min(summary.recovery_rate_pct, 100)}%` }}
            />
          </div>
          <div className="flex justify-between text-xs text-gray-400 mt-1">
            <span>{fmt(summary.total_recovered_ghs)} recovered</span>
            <span>{fmt(summary.total_gap_value_ghs)} total identified</span>
          </div>
        </div>
      )}

      {/* Gaps Table */}
      <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
        <div className="px-4 py-3 border-b border-gray-200 flex items-center justify-between">
          <h2 className="font-semibold text-gray-900">Gap Detail</h2>
          <span className="text-sm text-gray-500">{gaps.length} records</span>
        </div>
        {loading ? (
          <div className="text-center py-12 text-gray-400">Loading gaps…</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Audit Ref</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">District</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Account</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Type</th>
                  <th className="text-right px-4 py-3 font-medium text-gray-600">Gap (GH₵)</th>
                  <th className="text-center px-4 py-3 font-medium text-gray-600">GRA Status</th>
                  <th className="text-right px-4 py-3 font-medium text-gray-600">Recovered</th>
                  <th className="text-center px-4 py-3 font-medium text-gray-600">Recovery</th>
                  <th className="text-right px-4 py-3 font-medium text-gray-600">Fee (3%)</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Date</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {gaps.length === 0 ? (
                  <tr>
                    <td colSpan={10} className="text-center py-8 text-gray-400">
                      No gaps found for the selected period.
                    </td>
                  </tr>
                ) : gaps.map(gap => (
                  <tr key={gap.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3 font-mono text-xs text-blue-600">{gap.audit_reference}</td>
                    <td className="px-4 py-3 text-gray-700">{gap.district_name}</td>
                    <td className="px-4 py-3">
                      {gap.account_number ? (
                        <div>
                          <div className="font-medium text-gray-900 text-xs">{gap.account_number}</div>
                          <div className="text-gray-500 text-xs">{gap.customer_name}</div>
                        </div>
                      ) : (
                        <span className="text-gray-400 text-xs">District-level</span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <span className="px-2 py-0.5 bg-orange-100 text-orange-700 rounded text-xs">
                        {gap.anomaly_type.replace(/_/g, ' ')}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right font-semibold text-red-600">
                      {fmt(gap.variance_amount_ghs)}
                    </td>
                    <td className="px-4 py-3 text-center">
                      <span className={`px-2 py-0.5 rounded text-xs font-medium ${graStatusBadge(gap.gra_compliance_status)}`}>
                        {gap.gra_compliance_status}
                      </span>
                      {gap.gra_sdc_id && (
                        <div className="text-xs text-gray-400 mt-0.5 font-mono">{gap.gra_sdc_id.slice(0, 16)}…</div>
                      )}
                    </td>
                    <td className="px-4 py-3 text-right font-semibold text-green-600">
                      {gap.recovered_amount_ghs != null ? fmt(gap.recovered_amount_ghs) : '—'}
                    </td>
                    <td className="px-4 py-3 text-center">
                      {gap.recovery_status ? (
                        <span className={`px-2 py-0.5 rounded text-xs font-medium ${recoveryStatusBadge(gap.recovery_status)}`}>
                          {gap.recovery_status}
                        </span>
                      ) : (
                        <span className="text-gray-300 text-xs">No event</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-right text-purple-600 font-medium">
                      {gap.success_fee_ghs != null ? fmt(gap.success_fee_ghs) : '—'}
                    </td>
                    <td className="px-4 py-3 text-gray-500 text-xs">
                      {new Date(gap.created_at).toLocaleDateString('en-GH')}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        <div className="px-4 py-3 border-t border-gray-200 flex items-center justify-between">
          <button
            onClick={() => setPage(p => Math.max(1, p - 1))}
            disabled={page === 1}
            className="text-sm text-gray-600 hover:text-gray-900 disabled:opacity-40"
          >
            ← Previous
          </button>
          <span className="text-sm text-gray-500">Page {page}</span>
          <button
            onClick={() => setPage(p => p + 1)}
            disabled={gaps.length < 50}
            className="text-sm text-gray-600 hover:text-gray-900 disabled:opacity-40"
          >
            Next →
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Summary Card ──────────────────────────────────────────────────────────────

function SummaryCard({
  label, value, sub, color,
}: {
  label: string;
  value: string;
  sub: string;
  color: 'blue' | 'green' | 'yellow' | 'purple';
}) {
  const colors = {
    blue:   'bg-blue-50 border-blue-200',
    green:  'bg-green-50 border-green-200',
    yellow: 'bg-yellow-50 border-yellow-200',
    purple: 'bg-purple-50 border-purple-200',
  };
  const textColors = {
    blue:   'text-blue-900',
    green:  'text-green-900',
    yellow: 'text-yellow-900',
    purple: 'text-purple-900',
  };
  return (
    <div className={`rounded-xl border p-4 ${colors[color]}`}>
      <div className="text-xs text-gray-500 mb-1">{label}</div>
      <div className={`text-xl font-bold ${textColors[color]}`}>{value}</div>
      <div className="text-xs text-gray-500 mt-1">{sub}</div>
    </div>
  );
}
