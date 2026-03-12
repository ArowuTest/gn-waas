import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { RefreshCw, Download } from 'lucide-react';
import { useCases, useReclassifications, useDistricts } from '../hooks/useQueries';
import { KPICard, Badge, Button, Select, Spinner, EmptyState, Table } from '../components/ui';
import {
  formatGHS, formatDate,
  GWL_STATUS_COLORS, GWL_STATUS_LABELS,
  exportToCSV,
} from '../utils/helpers';
import type { GWLCase, GWLStatus, ReclassificationRequest } from '../types';

// Category colour mapping
const CATEGORY_COLORS: Record<string, string> = {
  RESIDENTIAL:  'bg-green-100 text-green-800 border-green-200',
  COMMERCIAL:   'bg-blue-100 text-blue-800 border-blue-200',
  INDUSTRIAL:   'bg-orange-100 text-orange-800 border-orange-200',
  GOVERNMENT:   'bg-purple-100 text-purple-800 border-purple-200',
  STANDPIPE:    'bg-gray-100 text-gray-700 border-gray-200',
};

export default function MisclassificationPage() {
  const navigate = useNavigate();
  const [districtId, setDistrictId] = useState('');
  const [reclassStatus, setReclassStatus] = useState('');
  const [page, setPage] = useState(0);
  const limit = 25;

  const { data: districts } = useDistricts();
  const { data: casesData, isLoading, refetch } = useCases({
    flag_type: 'CATEGORY_MISMATCH',
    district_id: districtId || undefined,
    limit,
    offset: page * limit,
    sort_by: 'estimated_loss_ghs',
  });
  const { data: reclassifications } = useReclassifications(
    reclassStatus ? { status: reclassStatus } : undefined
  );

  const cases: GWLCase[] = casesData?.cases || [];
  const total: number = casesData?.total || 0;
  const reclassList: ReclassificationRequest[] = reclassifications || [];

  const totalMonthlyLoss = cases.reduce((sum, c) => sum + c.estimated_loss_ghs, 0);
  const pendingReclass = reclassList.filter((r) => r.status === 'PENDING').length;
  const appliedReclass = reclassList.filter((r) => r.status === 'APPLIED_IN_GWL').length;

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
      { header: 'Current Category', accessor: (r) => r.account_category || '' },
      { header: 'District', accessor: (r) => r.district_name },
      { header: 'Monthly Revenue Loss (GHS)', accessor: (r) => r.estimated_loss_ghs },
      { header: 'Annual Revenue Loss (GHS)', accessor: (r) => r.estimated_loss_ghs * 12 },
      { header: 'Status', accessor: (r) => GWL_STATUS_LABELS[r.gwl_status as GWLStatus] || r.gwl_status },
      { header: 'Flagged On', accessor: (r) => formatDate(r.created_at) },
    ], 'misclassification-cases.csv');
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3">
            <span className="text-2xl">🔄</span>
            <h1 className="text-2xl font-bold text-gray-900">Category Misclassification</h1>
          </div>
          <p className="text-sm text-gray-500 mt-1">
            Accounts billed at the wrong tariff category — consumption patterns indicate a different classification
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

      {/* Explanation banner */}
      <div className="bg-violet-50 border border-violet-200 rounded-xl p-4">
        <p className="text-sm font-semibold text-violet-900 mb-1">How misclassification is detected</p>
        <p className="text-sm text-violet-800">
          GN-WAAS compares each account's monthly consumption against typical usage ranges per category.
          An account consuming 400m³/month billed as Residential (avg 8.5m³) is almost certainly a
          commercial premises. The shadow bill is recalculated at the correct tariff rate to show the
          revenue gap.
        </p>
      </div>

      {/* KPI Strip */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <KPICard label="Misclassified Accounts" value={total} color="bg-violet-50" />
        <KPICard
          label="Monthly Revenue Gap"
          value={formatGHS(totalMonthlyLoss)}
          sub="Billed at wrong rate"
          color="bg-red-50"
        />
        <KPICard
          label="Annual Revenue Gap"
          value={formatGHS(totalMonthlyLoss * 12)}
          sub="If uncorrected"
          color="bg-red-50"
        />
        <KPICard
          label="Reclassifications Applied"
          value={appliedReclass}
          sub={`${pendingReclass} pending approval`}
          color="bg-green-50"
        />
      </div>

      {/* Filters */}
      <div className="flex gap-3">
        <Select
          options={districtOptions}
          value={districtId}
          onChange={(e) => { setDistrictId(e.target.value); setPage(0); }}
          className="w-48"
        />
      </div>

      {/* Misclassified Accounts Table */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm">
        <div className="px-6 py-4 border-b border-gray-100">
          <h2 className="text-sm font-semibold text-gray-900">Misclassified Accounts</h2>
          <p className="text-xs text-gray-500 mt-0.5">
            {total} accounts · click a row to view evidence and request reclassification
          </p>
        </div>
        {isLoading ? (
          <Spinner />
        ) : cases.length === 0 ? (
          <EmptyState message="No misclassification cases found" />
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
                      <p className="text-xs text-gray-500">{r.account_number}</p>
                    </div>
                  ),
                },
                { header: 'District', accessor: (r) => r.district_name },
                {
                  header: 'Current Category',
                  accessor: (r) => (
                    <Badge className={CATEGORY_COLORS[r.account_category || ''] || 'bg-gray-100 text-gray-700 border-gray-200'}>
                      {r.account_category || '—'}
                    </Badge>
                  ),
                },
                {
                  header: 'GN-WAAS Finding',
                  accessor: (r) => (
                    <span className="text-xs text-gray-600 max-w-xs block truncate" title={r.description}>
                      {r.description}
                    </span>
                  ),
                },
                {
                  header: 'Monthly Gap',
                  accessor: (r) => (
                    <span className="font-bold text-violet-700">{formatGHS(r.estimated_loss_ghs)}</span>
                  ),
                },
                {
                  header: 'Annual Gap',
                  accessor: (r) => (
                    <span className="text-violet-600">{formatGHS(r.estimated_loss_ghs * 12)}</span>
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
                  header: 'Flagged',
                  accessor: (r) => <span className="text-xs text-gray-500">{formatDate(r.created_at)}</span>,
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

      {/* Reclassification Requests Table */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <div>
            <h2 className="text-sm font-semibold text-gray-900">Reclassification Requests</h2>
            <p className="text-xs text-gray-500 mt-0.5">Track formal reclassification requests submitted to GWL billing team</p>
          </div>
          <Select
            options={[
              { value: '', label: 'All Statuses' },
              { value: 'PENDING', label: 'Pending' },
              { value: 'APPROVED', label: 'Approved' },
              { value: 'APPLIED_IN_GWL', label: 'Applied in GWL' },
              { value: 'REJECTED', label: 'Rejected' },
            ]}
            value={reclassStatus}
            onChange={(e) => setReclassStatus(e.target.value)}
            className="w-44"
          />
        </div>
        {reclassList.length === 0 ? (
          <EmptyState message="No reclassification requests yet — open a case and click 'Request Reclassification'" />
        ) : (
          <Table
            keyExtractor={(r) => r.id}
            data={reclassList}
            columns={[
              {
                header: 'Account',
                accessor: (r) => (
                  <div>
                    <p className="font-medium text-gray-900 text-sm">{r.account_holder || '—'}</p>
                    <p className="text-xs text-gray-500">{r.account_number}</p>
                  </div>
                ),
              },
              { header: 'District', accessor: (r) => r.district_name },
              {
                header: 'Change',
                accessor: (r) => (
                  <div className="flex items-center gap-2">
                    <Badge className={CATEGORY_COLORS[r.current_category] || 'bg-gray-100 text-gray-700 border-gray-200'}>
                      {r.current_category}
                    </Badge>
                    <span className="text-gray-400">→</span>
                    <Badge className={CATEGORY_COLORS[r.recommended_category] || 'bg-gray-100 text-gray-700 border-gray-200'}>
                      {r.recommended_category}
                    </Badge>
                  </div>
                ),
              },
              {
                header: 'Monthly Impact',
                accessor: (r) => (
                  <span className="font-semibold text-violet-700">{formatGHS(r.monthly_revenue_impact_ghs)}</span>
                ),
              },
              {
                header: 'Status',
                accessor: (r) => (
                  <Badge className={
                    r.status === 'APPLIED_IN_GWL' ? 'bg-green-100 text-green-800 border-green-200' :
                    r.status === 'PENDING' ? 'bg-yellow-100 text-yellow-800 border-yellow-200' :
                    r.status === 'REJECTED' ? 'bg-red-100 text-red-800 border-red-200' :
                    'bg-gray-100 text-gray-700 border-gray-200'
                  }>
                    {(r.status ?? '').replace(/_/g, ' ')}
                  </Badge>
                ),
              },
              { header: 'Requested By', accessor: (r) => r.requested_by_name },
              {
                header: 'Date',
                accessor: (r) => <span className="text-xs text-gray-500">{formatDate(r.created_at)}</span>,
              },
            ]}
          />
        )}
      </div>
    </div>
  );
}
