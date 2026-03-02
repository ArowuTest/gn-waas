import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  AlertTriangle, TrendingDown, TrendingUp, Users,
  CheckCircle, Clock, RefreshCw, Download
} from 'lucide-react';
import { useCaseSummary, useCases, useDistricts } from '../hooks/useQueries';
import { KPICard, Badge, Button, Select, Spinner, EmptyState, Table } from '../components/ui';
import {
  formatGHS, formatDate,
  GWL_STATUS_LABELS, GWL_STATUS_COLORS,
  SEVERITY_COLORS, FLAG_TYPE_LABELS, FLAG_TYPE_COLORS,
  exportToCSV,
} from '../utils/helpers';
import type { GWLCase } from '../types';

export default function DashboardPage() {
  const navigate = useNavigate();
  const [districtId, setDistrictId] = useState('');

  const { data: summary, isLoading: summaryLoading, refetch: refetchSummary } = useCaseSummary(districtId || undefined);
  const { data: casesData, isLoading: casesLoading } = useCases({
    gwl_status: 'PENDING_REVIEW',
    district_id: districtId || undefined,
    limit: 10,
    sort_by: 'estimated_loss_ghs',
  });
  const { data: districts } = useDistricts();

  const cases: GWLCase[] = casesData?.cases || [];

  const districtOptions = [
    { value: '', label: 'All Districts' },
    ...(districts || []).map((d: { id: string; district_name: string }) => ({
      value: d.id, label: d.district_name,
    })),
  ];

  const handleExport = () => {
    exportToCSV(cases, [
      { header: 'Account No', accessor: (r) => r.account_number || '' },
      { header: 'Account Holder', accessor: (r) => r.account_holder || '' },
      { header: 'District', accessor: (r) => r.district_name },
      { header: 'Flag Type', accessor: (r) => FLAG_TYPE_LABELS[r.anomaly_type] || r.anomaly_type },
      { header: 'Severity', accessor: (r) => r.alert_level },
      { header: 'Estimated Loss (GHS)', accessor: (r) => r.estimated_loss_ghs },
      { header: 'Status', accessor: (r) => GWL_STATUS_LABELS[r.gwl_status] || r.gwl_status },
      { header: 'Days Open', accessor: (r) => r.days_open },
      { header: 'Flagged On', accessor: (r) => formatDate(r.created_at) },
    ], 'gwl-pending-cases.csv');
  };

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Case Management Dashboard</h1>
          <p className="text-sm text-gray-500 mt-0.5">
            GN-WAAS flagged anomalies requiring GWL action
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Select
            options={districtOptions}
            value={districtId}
            onChange={(e) => setDistrictId(e.target.value)}
            className="w-48"
          />
          <Button variant="secondary" size="sm" onClick={() => refetchSummary()}>
            <RefreshCw className="w-4 h-4" />
            Refresh
          </Button>
        </div>
      </div>

      {/* KPI Strip */}
      {summaryLoading ? (
        <Spinner />
      ) : summary ? (
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-4">
          <KPICard
            label="Total Open Cases"
            value={summary.total_open}
            icon={<Clock className="w-5 h-5" />}
            onClick={() => navigate('/cases')}
          />
          <KPICard
            label="Critical Cases"
            value={summary.critical_open}
            icon={<AlertTriangle className="w-5 h-5 text-red-500" />}
            color="bg-red-50"
            onClick={() => navigate('/cases?severity=CRITICAL')}
          />
          <KPICard
            label="Pending Review"
            value={summary.pending_review}
            sub="Awaiting GWL action"
            icon={<Clock className="w-5 h-5 text-yellow-500" />}
            color="bg-yellow-50"
          />
          <KPICard
            label="Field Assigned"
            value={summary.field_assigned}
            sub="Officers deployed"
            icon={<Users className="w-5 h-5 text-purple-500" />}
            color="bg-purple-50"
            onClick={() => navigate('/field-assignments')}
          />
          <KPICard
            label="Resolved This Month"
            value={summary.resolved_this_month}
            icon={<CheckCircle className="w-5 h-5 text-green-500" />}
            color="bg-green-50"
          />
        </div>
      ) : null}

      {/* Financial Impact Strip */}
      {summary && (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <KPICard
            label="Total Revenue at Risk"
            value={formatGHS(summary.total_estimated_loss_ghs)}
            sub="Open cases"
            icon={<AlertTriangle className="w-5 h-5 text-red-400" />}
            color="bg-red-50"
          />
          <KPICard
            label="Underbilling Detected"
            value={formatGHS(summary.underbilling_total_ghs)}
            sub="GWL collecting less than owed"
            icon={<TrendingDown className="w-5 h-5 text-orange-500" />}
            color="bg-orange-50"
            onClick={() => navigate('/underbilling')}
          />
          <KPICard
            label="Overbilling Detected"
            value={formatGHS(summary.overbilling_total_ghs)}
            sub="Customers overcharged"
            icon={<TrendingUp className="w-5 h-5 text-blue-500" />}
            color="bg-blue-50"
            onClick={() => navigate('/overbilling')}
          />
        </div>
      )}

      {/* Pending Review Queue */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <div>
            <h2 className="text-base font-semibold text-gray-900">Pending Review Queue</h2>
            <p className="text-xs text-gray-500 mt-0.5">Top 10 cases by estimated revenue loss</p>
          </div>
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" onClick={handleExport}>
              <Download className="w-4 h-4" />
              Export CSV
            </Button>
            <Button size="sm" onClick={() => navigate('/cases')}>
              View All Cases
            </Button>
          </div>
        </div>

        {casesLoading ? (
          <Spinner />
        ) : cases.length === 0 ? (
          <EmptyState message="No pending cases — all caught up!" />
        ) : (
          <Table
            keyExtractor={(r) => r.id}
            data={cases}
            onRowClick={(r) => navigate(`/cases/${r.id}`)}
            columns={[
              {
                header: 'Account',
                accessor: (r) => (
                  <div>
                    <p className="font-medium text-gray-900">{r.account_holder || '—'}</p>
                    <p className="text-xs text-gray-500">{r.account_number || '—'}</p>
                  </div>
                ),
              },
              {
                header: 'District',
                accessor: (r) => (
                  <div>
                    <p className="text-gray-700">{r.district_name}</p>
                    <p className="text-xs text-gray-400">{r.region}</p>
                  </div>
                ),
              },
              {
                header: 'Flag Type',
                accessor: (r) => (
                  <Badge className={FLAG_TYPE_COLORS[r.anomaly_type]}>
                    {FLAG_TYPE_LABELS[r.anomaly_type] || r.anomaly_type}
                  </Badge>
                ),
              },
              {
                header: 'Severity',
                accessor: (r) => (
                  <Badge className={SEVERITY_COLORS[r.alert_level]}>{r.alert_level}</Badge>
                ),
              },
              {
                header: 'Est. Loss',
                accessor: (r) => (
                  <span className="font-semibold text-red-700">{formatGHS(r.estimated_loss_ghs)}</span>
                ),
              },
              {
                header: 'Status',
                accessor: (r) => (
                  <Badge className={GWL_STATUS_COLORS[r.gwl_status]}>
                    {GWL_STATUS_LABELS[r.gwl_status] || r.gwl_status}
                  </Badge>
                ),
              },
              {
                header: 'Days Open',
                accessor: (r) => (
                  <span className={r.days_open > 14 ? 'text-red-600 font-semibold' : 'text-gray-600'}>
                    {r.days_open}d
                  </span>
                ),
              },
              {
                header: 'Flagged',
                accessor: (r) => formatDate(r.created_at),
              },
            ]}
          />
        )}
      </div>

      {/* Quick Stats Row */}
      {summary && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
            <h3 className="text-sm font-semibold text-gray-900 mb-3">Category Misclassification</h3>
            <div className="flex items-center gap-4">
              <div className="text-3xl font-bold text-violet-700">{summary.misclassified_count}</div>
              <div>
                <p className="text-sm text-gray-600">accounts likely in wrong billing category</p>
                <button
                  onClick={() => navigate('/misclassification')}
                  className="text-xs text-blue-600 hover:underline mt-1"
                >
                  Review misclassified accounts →
                </button>
              </div>
            </div>
          </div>
          <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
            <h3 className="text-sm font-semibold text-gray-900 mb-3">Quick Actions</h3>
            <div className="flex flex-wrap gap-2">
              <Button size="sm" onClick={() => navigate('/cases?gwl_status=PENDING_REVIEW')}>
                Review Pending
              </Button>
              <Button size="sm" variant="secondary" onClick={() => navigate('/field-assignments')}>
                Manage Field Jobs
              </Button>
              <Button size="sm" variant="secondary" onClick={() => navigate('/reports')}>
                Monthly Report
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
