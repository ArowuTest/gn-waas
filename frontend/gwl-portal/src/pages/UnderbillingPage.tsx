import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { TrendingDown, Download, RefreshCw } from 'lucide-react';
import { useCases, useDistricts } from '../hooks/useQueries';
import { KPICard, Badge, Button, Select, Spinner, EmptyState, Table } from '../components/ui';
import {
  formatGHS, formatDate,
  GWL_STATUS_COLORS, GWL_STATUS_LABELS,
  SEVERITY_COLORS, exportToCSV,
} from '../utils/helpers';
import type { GWLCase, GWLStatus, Severity } from '../types';

export default function UnderbillingPage() {
  const navigate = useNavigate();
  const [districtId, setDistrictId] = useState('');
  const [severity, setSeverity] = useState('');
  const [page, setPage] = useState(0);
  const limit = 25;

  const { data: districts } = useDistricts();
  const { data: casesData, isLoading, refetch } = useCases({
    flag_type: 'BILLING_VARIANCE',
    district_id: districtId || undefined,
    severity: severity || undefined,
    gwl_status: 'PENDING_REVIEW,UNDER_INVESTIGATION,FIELD_ASSIGNED,EVIDENCE_SUBMITTED',
    limit,
    offset: page * limit,
    sort_by: 'estimated_loss_ghs',
  });

  // Also fetch category mismatch cases (they are also underbilling)
  const { data: misclassData } = useCases({
    flag_type: 'CATEGORY_MISMATCH',
    district_id: districtId || undefined,
    limit: 100,
  });

  const cases: GWLCase[] = casesData?.cases || [];
  const total: number = casesData?.total || 0;
  const totalLoss = cases.reduce((sum, c) => sum + c.estimated_loss_ghs, 0);
  const criticalCount = cases.filter((c) => c.severity === 'CRITICAL').length;
  const misclassLoss = (misclassData?.cases || []).reduce(
    (sum: number, c: GWLCase) => sum + c.estimated_loss_ghs, 0
  );

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
      { header: 'Category', accessor: (r) => r.account_category || '' },
      { header: 'District', accessor: (r) => r.district_name },
      { header: 'Severity', accessor: (r) => r.alert_level },
      { header: 'Monthly Loss (GHS)', accessor: (r) => r.estimated_loss_ghs },
      { header: 'Annual Loss (GHS)', accessor: (r) => r.estimated_loss_ghs * 12 },
      { header: 'Status', accessor: (r) => GWL_STATUS_LABELS[r.gwl_status as GWLStatus] || r.gwl_status },
      { header: 'Days Open', accessor: (r) => r.days_open },
      { header: 'Flagged On', accessor: (r) => formatDate(r.created_at) },
    ], 'underbilling-cases.csv');
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3">
            <TrendingDown className="w-6 h-6 text-orange-500" />
            <h1 className="text-2xl font-bold text-gray-900">Underbilling Cases</h1>
          </div>
          <p className="text-sm text-gray-500 mt-1">
            Accounts where GWL is collecting less revenue than owed — ranked by monthly loss
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => refetch()}>
            <RefreshCw className="w-4 h-4" />
          </Button>
          <Button variant="secondary" size="sm" onClick={handleExport}>
            <Download className="w-4 h-4" />
            Export CSV
          </Button>
        </div>
      </div>

      {/* KPI Strip */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <KPICard
          label="Open Underbilling Cases"
          value={total}
          color="bg-orange-50"
        />
        <KPICard
          label="Monthly Revenue at Risk"
          value={formatGHS(totalLoss)}
          sub="Billing variance cases"
          color="bg-red-50"
        />
        <KPICard
          label="Annual Revenue at Risk"
          value={formatGHS(totalLoss * 12)}
          sub="If uncorrected"
          color="bg-red-50"
        />
        <KPICard
          label="Critical Cases"
          value={criticalCount}
          sub="Require immediate action"
          color="bg-red-50"
        />
      </div>

      {/* Misclassification revenue note */}
      {misclassLoss > 0 && (
        <div className="bg-violet-50 border border-violet-200 rounded-xl p-4 flex items-center justify-between">
          <div>
            <p className="text-sm font-semibold text-violet-900">
              Additional {formatGHS(misclassLoss)}/month from category misclassification
            </p>
            <p className="text-xs text-violet-700 mt-0.5">
              Accounts billed at wrong category rate — see Misclassification page for details
            </p>
          </div>
          <Button size="sm" variant="secondary" onClick={() => navigate('/misclassification')}>
            View Misclassification →
          </Button>
        </div>
      )}

      {/* Filters */}
      <div className="flex gap-3 flex-wrap">
        <Select
          options={districtOptions}
          value={districtId}
          onChange={(e) => { setDistrictId(e.target.value); setPage(0); }}
          className="w-48"
        />
        <Select
          options={[
            { value: '', label: 'All Severities' },
            { value: 'CRITICAL', label: 'Critical' },
            { value: 'HIGH', label: 'High' },
            { value: 'MEDIUM', label: 'Medium' },
          ]}
          value={severity}
          onChange={(e) => { setSeverity(e.target.value); setPage(0); }}
          className="w-40"
        />
      </div>

      {/* Table */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm">
        <div className="px-6 py-4 border-b border-gray-100">
          <p className="text-sm text-gray-500">{total} cases · sorted by monthly revenue loss</p>
        </div>
        {isLoading ? (
          <Spinner />
        ) : cases.length === 0 ? (
          <EmptyState message="No underbilling cases found" />
        ) : (
          <>
            <Table
              keyExtractor={(r) => r.id}
              data={cases}
              onRowClick={(r) => navigate(`/cases/${r.id}`)}
              columns={[
                {
                  header: 'Account',
                  accessor: (r) => (
                    <div>
                      <p className="font-medium text-gray-900 text-sm">{r.account_holder || '—'}</p>
                      <p className="text-xs text-gray-500">{r.account_number} · {r.account_category}</p>
                    </div>
                  ),
                },
                { header: 'District', accessor: (r) => r.district_name },
                {
                  header: 'Severity',
                  accessor: (r) => (
                    <Badge className={SEVERITY_COLORS[r.alert_level as Severity]}>{r.alert_level}</Badge>
                  ),
                },
                {
                  header: 'Monthly Loss',
                  accessor: (r) => (
                    <span className="font-bold text-red-700">{formatGHS(r.estimated_loss_ghs)}</span>
                  ),
                },
                {
                  header: 'Annual Loss',
                  accessor: (r) => (
                    <span className="text-red-600">{formatGHS(r.estimated_loss_ghs * 12)}</span>
                  ),
                },
                {
                  header: 'Status',
                  accessor: (r) => (
                    <Badge className={GWL_STATUS_COLORS[r.gwl_status as GWLStatus]}>
                      {GWL_STATUS_LABELS[r.gwl_status as GWLStatus] || r.gwl_status}
                    </Badge>
                  ),
                },
                {
                  header: 'Officer',
                  accessor: (r) => r.assigned_officer_name || <span className="text-gray-400 text-xs">Unassigned</span>,
                },
                {
                  header: 'Days Open',
                  accessor: (r) => (
                    <span className={r.days_open > 14 ? 'text-red-600 font-semibold' : 'text-gray-600'}>
                      {r.days_open}d
                    </span>
                  ),
                },
              ]}
            />
            <div className="flex items-center justify-between px-6 py-3 border-t border-gray-100">
              <p className="text-xs text-gray-500">
                Showing {page * limit + 1}–{Math.min((page + 1) * limit, total)} of {total}
              </p>
              <div className="flex gap-2">
                <Button variant="secondary" size="sm" disabled={page === 0} onClick={() => setPage(page - 1)}>Previous</Button>
                <Button variant="secondary" size="sm" disabled={(page + 1) * limit >= total} onClick={() => setPage(page + 1)}>Next</Button>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
