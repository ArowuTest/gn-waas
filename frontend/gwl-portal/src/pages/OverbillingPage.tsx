import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { TrendingUp, Download, RefreshCw } from 'lucide-react';
import { useCases, useCredits, useDistricts } from '../hooks/useQueries';
import { KPICard, Badge, Button, Select, Spinner, EmptyState, Table } from '../components/ui';
import {
  formatGHS, formatDate,
  GWL_STATUS_COLORS, GWL_STATUS_LABELS,
  SEVERITY_COLORS, exportToCSV,
} from '../utils/helpers';
import type { GWLCase, GWLStatus, Severity, CreditRequest } from '../types';

export default function OverbillingPage() {
  const navigate = useNavigate();
  const [districtId, setDistrictId] = useState('');
  const [creditStatus, setCreditStatus] = useState('');
  const [page, setPage] = useState(0);
  const limit = 25;

  const { data: districts } = useDistricts();
  const { data: casesData, isLoading, refetch } = useCases({
    flag_type: 'OVERBILLING',
    district_id: districtId || undefined,
    limit,
    offset: page * limit,
    sort_by: 'estimated_loss_ghs',
  });
  const { data: credits } = useCredits(
    creditStatus ? { status: creditStatus } : undefined
  );

  const cases: GWLCase[] = casesData?.cases || [];
  const total: number = casesData?.total || 0;
  const creditRequests: CreditRequest[] = credits || [];
  const totalOvercharge = cases.reduce((sum, c) => sum + c.estimated_loss_ghs, 0);
  const pendingCredits = creditRequests.filter((c) => c.status === 'PENDING').length;
  const creditsIssued = creditRequests.filter((c) => c.status === 'APPLIED_IN_GWL').reduce(
    (sum, c) => sum + c.credit_amount_ghs, 0
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
      { header: 'District', accessor: (r) => r.district_name },
      { header: 'Overcharge (GHS)', accessor: (r) => r.estimated_loss_ghs },
      { header: 'Status', accessor: (r) => GWL_STATUS_LABELS[r.gwl_status as GWLStatus] || r.gwl_status },
      { header: 'Flagged On', accessor: (r) => formatDate(r.created_at) },
    ], 'overbilling-cases.csv');
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3">
            <TrendingUp className="w-6 h-6 text-blue-500" />
            <h1 className="text-2xl font-bold text-gray-900">Overbilling Cases</h1>
          </div>
          <p className="text-sm text-gray-500 mt-1">
            Accounts where customers were charged more than the GN-WAAS shadow bill — GWL can issue credits
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
        <KPICard label="Open Overbilling Cases" value={total} color="bg-blue-50" />
        <KPICard
          label="Total Overcharge Detected"
          value={formatGHS(totalOvercharge)}
          sub="Customers overcharged"
          color="bg-blue-50"
        />
        <KPICard
          label="Pending Credit Requests"
          value={pendingCredits}
          sub="Awaiting approval"
          color="bg-yellow-50"
        />
        <KPICard
          label="Credits Issued (Applied)"
          value={formatGHS(creditsIssued)}
          sub="Applied in GWL system"
          color="bg-green-50"
        />
      </div>

      {/* Filters */}
      <div className="flex gap-3 flex-wrap">
        <Select
          options={districtOptions}
          value={districtId}
          onChange={(e) => { setDistrictId(e.target.value); setPage(0); }}
          className="w-48"
        />
      </div>

      {/* Overbilling Cases Table */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm">
        <div className="px-6 py-4 border-b border-gray-100">
          <h2 className="text-sm font-semibold text-gray-900">Overbilling Cases</h2>
          <p className="text-xs text-gray-500 mt-0.5">{total} cases · click a row to view details and issue a credit</p>
        </div>
        {isLoading ? (
          <Spinner />
        ) : cases.length === 0 ? (
          <EmptyState message="No overbilling cases found" />
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
                  header: 'Severity',
                  accessor: (r) => (
                    <Badge className={SEVERITY_COLORS[r.alert_level as Severity]}>{r.alert_level}</Badge>
                  ),
                },
                {
                  header: 'Overcharge',
                  accessor: (r) => (
                    <span className="font-bold text-blue-700">{formatGHS(r.estimated_loss_ghs)}</span>
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

      {/* Credit Requests Table */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <div>
            <h2 className="text-sm font-semibold text-gray-900">Credit Requests</h2>
            <p className="text-xs text-gray-500 mt-0.5">Track credit approvals and GWL system updates</p>
          </div>
          <Select
            options={[
              { value: '', label: 'All Statuses' },
              { value: 'PENDING', label: 'Pending' },
              { value: 'APPROVED', label: 'Approved' },
              { value: 'APPLIED_IN_GWL', label: 'Applied in GWL' },
              { value: 'REJECTED', label: 'Rejected' },
            ]}
            value={creditStatus}
            onChange={(e) => setCreditStatus(e.target.value)}
            className="w-44"
          />
        </div>
        {creditRequests.length === 0 ? (
          <EmptyState message="No credit requests yet" />
        ) : (
          <Table
            keyExtractor={(r) => r.id}
            data={creditRequests}
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
                header: 'Credit Amount',
                accessor: (r) => (
                  <span className="font-bold text-green-700">{formatGHS(r.credit_amount_ghs)}</span>
                ),
              },
              { header: 'Reason', accessor: (r) => <span className="text-xs text-gray-600">{r.reason}</span> },
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
