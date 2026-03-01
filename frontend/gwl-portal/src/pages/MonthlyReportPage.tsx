import { useState } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell, Legend } from 'recharts';
import { FileText, Download, RefreshCw } from 'lucide-react';
import { useMonthlyReport, useDistricts } from '../hooks/useQueries';
import { KPICard, Button, Select, Spinner } from '../components/ui';
import { formatGHS, exportToCSV } from '../utils/helpers';
import type { MonthlyReport } from '../types';

const CHART_COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6'];

export default function MonthlyReportPage() {
  const now = new Date();
  const defaultPeriod = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
  const [period, setPeriod] = useState(defaultPeriod);
  const [districtId, setDistrictId] = useState('');

  const { data: districts } = useDistricts();
  const { data: report, isLoading, refetch } = useMonthlyReport(period, districtId || undefined);

  const districtOptions = [
    { value: '', label: 'National Summary' },
    ...(districts || []).map((d: { id: string; district_name: string }) => ({
      value: d.id, label: d.district_name,
    })),
  ];

  // Generate last 12 months for period selector
  const periodOptions = Array.from({ length: 12 }, (_, i) => {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
    const value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`;
    const label = d.toLocaleDateString('en-GB', { month: 'long', year: 'numeric' });
    return { value, label };
  });

  const stats = (report as MonthlyReport)?.statistics;

  const caseBreakdownData = stats ? [
    { name: 'Resolved', value: stats.resolved, color: '#10b981' },
    { name: 'Pending', value: stats.pending, color: '#f59e0b' },
    { name: 'Disputed', value: stats.disputed, color: '#6b7280' },
    { name: 'Critical', value: stats.critical_cases, color: '#ef4444' },
  ] : [];

  const financialData = stats ? [
    { name: 'Underbilling', amount: stats.total_underbilling_ghs },
    { name: 'Overbilling', amount: stats.total_overbilling_ghs },
    { name: 'Recovered', amount: stats.revenue_recovered_ghs },
    { name: 'Credits Issued', amount: stats.credits_issued_ghs },
  ] : [];

  const handleExportCSV = () => {
    if (!stats) return;
    exportToCSV(
      [{ period: (report as MonthlyReport).period, ...stats }],
      [
        { header: 'Period', accessor: (r) => (r as { period: string }).period },
        { header: 'Total Flagged', accessor: (r) => (r as typeof stats & { period: string }).total_flagged },
        { header: 'Critical', accessor: (r) => (r as typeof stats & { period: string }).critical_cases },
        { header: 'Resolved', accessor: (r) => (r as typeof stats & { period: string }).resolved },
        { header: 'Pending', accessor: (r) => (r as typeof stats & { period: string }).pending },
        { header: 'Underbilling (GHS)', accessor: (r) => (r as typeof stats & { period: string }).total_underbilling_ghs },
        { header: 'Overbilling (GHS)', accessor: (r) => (r as typeof stats & { period: string }).total_overbilling_ghs },
        { header: 'Revenue Recovered (GHS)', accessor: (r) => (r as typeof stats & { period: string }).revenue_recovered_ghs },
        { header: 'Credits Issued (GHS)', accessor: (r) => (r as typeof stats & { period: string }).credits_issued_ghs },
        { header: 'Reclassifications Requested', accessor: (r) => (r as typeof stats & { period: string }).reclassifications_requested },
        { header: 'Reclassifications Applied', accessor: (r) => (r as typeof stats & { period: string }).reclassifications_applied },
        { header: 'Field Jobs Assigned', accessor: (r) => (r as typeof stats & { period: string }).field_jobs_assigned },
        { header: 'Field Jobs Completed', accessor: (r) => (r as typeof stats & { period: string }).field_jobs_completed },
      ],
      `gwl-monthly-report-${period}.csv`
    );
  };

  const handlePrint = () => window.print();

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3">
            <FileText className="w-6 h-6 text-blue-500" />
            <h1 className="text-2xl font-bold text-gray-900">Monthly Report</h1>
          </div>
          <p className="text-sm text-gray-500 mt-1">
            GWL case management summary — for management and Ministry of Finance
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => refetch()}>
            <RefreshCw className="w-4 h-4" />
          </Button>
          <Button variant="secondary" size="sm" onClick={handleExportCSV} disabled={!stats}>
            <Download className="w-4 h-4" />
            Export CSV
          </Button>
          <Button size="sm" onClick={handlePrint} disabled={!stats}>
            🖨️ Print / PDF
          </Button>
        </div>
      </div>

      {/* Period + District selectors */}
      <div className="flex gap-3 flex-wrap">
        <Select
          label="Report Period"
          options={periodOptions}
          value={period}
          onChange={(e) => setPeriod(e.target.value)}
          className="w-52"
        />
        <Select
          label="District"
          options={districtOptions}
          value={districtId}
          onChange={(e) => setDistrictId(e.target.value)}
          className="w-52"
        />
      </div>

      {isLoading ? (
        <Spinner />
      ) : !stats ? (
        <div className="text-center py-16 text-gray-400">No report data for this period</div>
      ) : (
        <>
          {/* Report header */}
          <div className="bg-gradient-to-r from-blue-700 to-blue-900 rounded-xl p-6 text-white print:bg-blue-700">
            <div className="flex items-start justify-between">
              <div>
                <p className="text-blue-200 text-sm font-medium uppercase tracking-wide">GN-WAAS Monthly Report</p>
                <h2 className="text-2xl font-bold mt-1">{(report as MonthlyReport).period}</h2>
                <p className="text-blue-200 text-sm mt-1">
                  {districtId ? districtOptions.find((d) => d.value === districtId)?.label : 'National Summary'}
                </p>
              </div>
              <div className="text-right">
                <p className="text-blue-200 text-xs">Generated</p>
                <p className="text-sm font-medium">{new Date().toLocaleDateString('en-GB', { day: '2-digit', month: 'long', year: 'numeric' })}</p>
              </div>
            </div>
          </div>

          {/* KPI Strip */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <KPICard label="Total Cases Flagged" value={stats.total_flagged} />
            <KPICard label="Critical Cases" value={stats.critical_cases} color="bg-red-50" />
            <KPICard label="Cases Resolved" value={stats.resolved} color="bg-green-50" />
            <KPICard label="Cases Pending" value={stats.pending} color="bg-yellow-50" />
          </div>

          {/* Financial Impact */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <KPICard
              label="Underbilling Detected"
              value={formatGHS(stats.total_underbilling_ghs)}
              sub="Revenue at risk"
              color="bg-orange-50"
            />
            <KPICard
              label="Overbilling Detected"
              value={formatGHS(stats.total_overbilling_ghs)}
              sub="Customer overcharges"
              color="bg-blue-50"
            />
            <KPICard
              label="Revenue Recovered"
              value={formatGHS(stats.revenue_recovered_ghs)}
              sub="Corrections applied"
              color="bg-green-50"
            />
            <KPICard
              label="Credits Issued"
              value={formatGHS(stats.credits_issued_ghs)}
              sub="Applied to customers"
              color="bg-teal-50"
            />
          </div>

          {/* Charts */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Case Breakdown Pie */}
            <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
              <h3 className="text-sm font-semibold text-gray-900 mb-4">Case Breakdown</h3>
              <ResponsiveContainer width="100%" height={220}>
                <PieChart>
                  <Pie
                    data={caseBreakdownData}
                    cx="50%"
                    cy="50%"
                    innerRadius={60}
                    outerRadius={90}
                    dataKey="value"
                    label={({ name, value }) => `${name}: ${value}`}
                  >
                    {caseBreakdownData.map((entry, i) => (
                      <Cell key={i} fill={entry.color} />
                    ))}
                  </Pie>
                  <Legend />
                  <Tooltip />
                </PieChart>
              </ResponsiveContainer>
            </div>

            {/* Financial Bar Chart */}
            <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
              <h3 className="text-sm font-semibold text-gray-900 mb-4">Financial Impact (GHS)</h3>
              <ResponsiveContainer width="100%" height={220}>
                <BarChart data={financialData} margin={{ top: 5, right: 10, left: 10, bottom: 5 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                  <XAxis dataKey="name" tick={{ fontSize: 11 }} />
                  <YAxis tick={{ fontSize: 11 }} tickFormatter={(v) => `₵${(v / 1000).toFixed(0)}k`} />
                  <Tooltip formatter={(v) => formatGHS(Number(v))} />
                  <Bar dataKey="amount" radius={[4, 4, 0, 0]}>
                    {financialData.map((_, i) => (
                      <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </div>
          </div>

          {/* Operations Summary */}
          <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
            <h3 className="text-sm font-semibold text-gray-900 mb-4">Operations Summary</h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-6 text-center">
              <div>
                <p className="text-2xl font-bold text-gray-900">{stats.reclassifications_requested}</p>
                <p className="text-xs text-gray-500 mt-1">Reclassifications Requested</p>
              </div>
              <div>
                <p className="text-2xl font-bold text-green-700">{stats.reclassifications_applied}</p>
                <p className="text-xs text-gray-500 mt-1">Reclassifications Applied</p>
              </div>
              <div>
                <p className="text-2xl font-bold text-purple-700">{stats.field_jobs_assigned}</p>
                <p className="text-xs text-gray-500 mt-1">Field Jobs Assigned</p>
              </div>
              <div>
                <p className="text-2xl font-bold text-blue-700">{stats.field_jobs_completed}</p>
                <p className="text-xs text-gray-500 mt-1">Field Jobs Completed</p>
              </div>
            </div>
          </div>

          {/* Disputed cases note */}
          {stats.disputed > 0 && (
            <div className="bg-gray-50 border border-gray-200 rounded-xl p-4">
              <p className="text-sm font-semibold text-gray-700">
                {stats.disputed} case{stats.disputed !== 1 ? 's' : ''} disputed this period
              </p>
              <p className="text-xs text-gray-500 mt-1">
                GWL has disputed these GN-WAAS flags. The audit team has been notified for review.
              </p>
            </div>
          )}
        </>
      )}
    </div>
  );
}
