/**
 * GN-WAAS GWL Portal — Credit Requests Page
 *
 * Dedicated page for managing customer credit requests raised by GWL supervisors
 * when overbilling is confirmed. Allows supervisors to view, filter, and action
 * credit requests, and track their application status in GWL's billing system.
 *
 * GAP-FIX-01: Previously the /credits route incorrectly rendered OverbillingPage.
 * This page is purpose-built for credit request management.
 */
import { useState } from 'react';
import { CreditCard, RefreshCw, Download, CheckCircle, Clock, XCircle, AlertTriangle } from 'lucide-react';
import { useCredits, useDistricts } from '../hooks/useQueries';
import { KPICard, Badge, Button, Select, Spinner, EmptyState, Table } from '../components/ui';
import { formatGHS, formatDate, exportToCSV } from '../utils/helpers';
import type { CreditRequest } from '../types';

const STATUS_COLORS: Record<string, string> = {
  PENDING:         'bg-yellow-100 text-yellow-800 border-yellow-200',
  APPROVED:        'bg-blue-100 text-blue-800 border-blue-200',
  APPLIED_IN_GWL:  'bg-green-100 text-green-800 border-green-200',
  REJECTED:        'bg-red-100 text-red-800 border-red-200',
};

const STATUS_LABELS: Record<string, string> = {
  PENDING:         'Pending Approval',
  APPROVED:        'Approved',
  APPLIED_IN_GWL:  'Applied in GWL',
  REJECTED:        'Rejected',
};

export default function CreditRequestsPage() {
  const [districtId, setDistrictId] = useState('');
  const [statusFilter, setStatusFilter] = useState('');

  const { data: districts } = useDistricts();
  const { data: credits, isLoading, refetch } = useCredits(
    Object.fromEntries(
      Object.entries({
        status:      statusFilter || undefined,
        district_id: districtId  || undefined,
      }).filter(([, v]) => v !== undefined) as [string, string][]
    )
  );

  const creditList: CreditRequest[] = credits || [];

  // ── KPI calculations ──────────────────────────────────────────────────────
  const totalRequested  = creditList.reduce((s, c) => s + c.credit_amount_ghs, 0);
  const pendingCount    = creditList.filter(c => c.status === 'PENDING').length;
  const appliedCount    = creditList.filter(c => c.status === 'APPLIED_IN_GWL').length;
  const appliedAmount   = creditList
    .filter(c => c.status === 'APPLIED_IN_GWL')
    .reduce((s, c) => s + c.credit_amount_ghs, 0);

  const districtOptions = [
    { value: '', label: 'All Districts' },
    ...(districts || []).map((d: { id: string; district_name: string }) => ({
      value: d.id, label: d.district_name,
    })),
  ];

  const statusOptions = [
    { value: '',               label: 'All Statuses' },
    { value: 'PENDING',        label: 'Pending Approval' },
    { value: 'APPROVED',       label: 'Approved' },
    { value: 'APPLIED_IN_GWL', label: 'Applied in GWL' },
    { value: 'REJECTED',       label: 'Rejected' },
  ];

  const handleExport = () => {
    exportToCSV(creditList, [
      { header: 'Reference',        accessor: (r) => (r as CreditRequest).gwl_credit_reference ?? '' },
      { header: 'Account Number',   accessor: (r) => (r as CreditRequest).account_number ?? '' },
      { header: 'Account Holder',   accessor: (r) => (r as CreditRequest).account_holder ?? '' },
      { header: 'District',         accessor: (r) => (r as CreditRequest).district_name ?? '' },
      { header: 'Overcharge (GHS)', accessor: (r) => (r as CreditRequest).overcharge_amount_ghs.toFixed(2) },
      { header: 'Credit (GHS)',     accessor: (r) => (r as CreditRequest).credit_amount_ghs.toFixed(2) },
      { header: 'Status',           accessor: (r) => STATUS_LABELS[(r as CreditRequest).status] ?? (r as CreditRequest).status },
      { header: 'Requested By',     accessor: (r) => (r as CreditRequest).requested_by_name ?? '' },
      { header: 'Requested At',     accessor: (r) => formatDate((r as CreditRequest).created_at) },
    ], `credit-requests-${new Date().toISOString().slice(0, 10)}.csv`);
  };

  const columns = [
    {
      header: 'Reference',
      accessor: (r: CreditRequest) => (
        <span className="font-mono text-xs text-gray-600">
          {r.gwl_credit_reference ?? `CR-${r.id.slice(0, 8).toUpperCase()}`}
        </span>
      ),
    },
    {
      header: 'Account',
      accessor: (r: CreditRequest) => (
        <div>
          <p className="text-sm font-semibold text-gray-900">{r.account_holder ?? '—'}</p>
          <p className="text-xs text-gray-500 font-mono">{r.account_number ?? '—'}</p>
        </div>
      ),
    },
    {
      header: 'District',
      accessor: (r: CreditRequest) => (
        <span className="text-sm text-gray-700">{r.district_name ?? '—'}</span>
      ),
    },
    {
      header: 'Overcharge',
      accessor: (r: CreditRequest) => (
        <span className="text-sm font-semibold text-red-600">{formatGHS(r.overcharge_amount_ghs)}</span>
      ),
    },
    {
      header: 'Credit Amount',
      accessor: (r: CreditRequest) => (
        <span className="text-sm font-bold text-green-700">{formatGHS(r.credit_amount_ghs)}</span>
      ),
    },
    {
      header: 'Status',
      accessor: (r: CreditRequest) => (
        <Badge className={STATUS_COLORS[r.status] ?? 'bg-gray-100 text-gray-700 border-gray-200'}>
          {STATUS_LABELS[r.status] ?? r.status}
        </Badge>
      ),
    },
    {
      header: 'Requested By',
      accessor: (r: CreditRequest) => (
        <div>
          <p className="text-sm text-gray-700">{r.requested_by_name ?? '—'}</p>
          <p className="text-xs text-gray-400">{formatDate(r.created_at)}</p>
        </div>
      ),
    },
    {
      header: 'Reason',
      accessor: (r: CreditRequest) => (
        <p className="text-xs text-gray-500 max-w-xs truncate" title={r.reason ?? ''}>
          {r.reason ?? '—'}
        </p>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      {/* ── Header ─────────────────────────────────────────────────────────── */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-black text-gray-900">Credit Requests</h1>
          <p className="text-sm text-gray-500 mt-1">
            Customer credit requests raised for confirmed overbilling cases
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => refetch()}>
            <RefreshCw size={14} /> Refresh
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={handleExport}
            disabled={creditList.length === 0}
          >
            <Download size={14} /> Export CSV
          </Button>
        </div>
      </div>

      {/* ── KPI Cards ──────────────────────────────────────────────────────── */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <KPICard
          label="Total Requests"
          value={creditList.length}
          icon={<CreditCard size={18} />}
        />
        <KPICard
          label="Pending Approval"
          value={pendingCount}
          icon={<Clock size={18} />}
        />
        <KPICard
          label="Applied in GWL"
          value={appliedCount}
          icon={<CheckCircle size={18} />}
        />
        <KPICard
          label="Total Credit Value"
          value={formatGHS(totalRequested)}
          icon={<CreditCard size={18} />}
          sub={`${formatGHS(appliedAmount)} applied`}
        />
      </div>

      {/* ── Filters ────────────────────────────────────────────────────────── */}
      <div className="bg-white rounded-xl border border-gray-200 p-4">
        <div className="flex flex-wrap gap-3 items-center">
          <Select
            value={districtId}
            onChange={(e) => setDistrictId(e.target.value)}
            options={districtOptions}
            className="w-48"
          />
          <Select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            options={statusOptions}
            className="w-48"
          />
          {(districtId || statusFilter) && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => { setDistrictId(''); setStatusFilter(''); }}
            >
              Clear filters
            </Button>
          )}
          <span className="ml-auto text-sm text-gray-500">
            {creditList.length} request{creditList.length !== 1 ? 's' : ''}
          </span>
        </div>
      </div>

      {/* ── Table ──────────────────────────────────────────────────────────── */}
      {isLoading ? (
        <div className="flex justify-center py-16"><Spinner /></div>
      ) : creditList.length === 0 ? (
        <EmptyState
          message={
            statusFilter || districtId
              ? 'No credit requests match your filters. Try adjusting them.'
              : 'No credit requests yet. They appear here when GWL supervisors raise them for confirmed overbilling cases.'
          }
        />
      ) : (
        <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
          <Table
            columns={columns}
            data={creditList}
            keyExtractor={(r) => r.id}
          />
        </div>
      )}

      {/* ── Info Banner ────────────────────────────────────────────────────── */}
      <div className="bg-blue-50 border border-blue-200 rounded-xl p-4 flex gap-3">
        <AlertTriangle className="w-5 h-5 text-blue-500 flex-shrink-0 mt-0.5" />
        <div>
          <p className="text-sm font-semibold text-blue-800">Credit Request Workflow</p>
          <p className="text-sm text-blue-600 mt-0.5">
            Credits are raised from the Case Detail page when overbilling is confirmed.
            Once approved, the credit reference must be applied in GWL's billing system
            and the status updated to <strong>Applied in GWL</strong> to close the loop.
          </p>
        </div>
      </div>
    </div>
  );
}
