import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Search, Filter, Download, RefreshCw, X } from 'lucide-react';
import { useCases, useDistricts } from '../hooks/useQueries';
import { api } from '../utils/api';
import { Badge, Button, Select, Input, Spinner, EmptyState, Table } from '../components/ui';
import {
  formatGHS, formatDate,
  GWL_STATUS_LABELS, GWL_STATUS_COLORS,
  SEVERITY_COLORS, FLAG_TYPE_LABELS, FLAG_TYPE_COLORS,
  exportToCSV,
} from '../utils/helpers';
import type { GWLCase, GWLStatus, Severity, FlagType } from '../types';

const STATUS_OPTIONS = [
  { value: '', label: 'All Statuses' },
  { value: 'PENDING_REVIEW', label: 'Pending Review' },
  { value: 'UNDER_INVESTIGATION', label: 'Under Investigation' },
  { value: 'FIELD_ASSIGNED', label: 'Field Assigned' },
  { value: 'EVIDENCE_SUBMITTED', label: 'Evidence Submitted' },
  { value: 'APPROVED_FOR_CORRECTION', label: 'Approved for Correction' },
  { value: 'DISPUTED', label: 'Disputed' },
  { value: 'CORRECTED', label: 'Corrected' },
  { value: 'CLOSED', label: 'Closed' },
];

const SEVERITY_OPTIONS = [
  { value: '', label: 'All Severities' },
  { value: 'CRITICAL', label: 'Critical' },
  { value: 'HIGH', label: 'High' },
  { value: 'MEDIUM', label: 'Medium' },
  { value: 'LOW', label: 'Low' },
];

const FLAG_TYPE_OPTIONS = [
  { value: '', label: 'All Flag Types' },
  { value: 'BILLING_VARIANCE', label: 'Billing Variance' },
  { value: 'CATEGORY_MISMATCH', label: 'Category Mismatch' },
  { value: 'OVERBILLING', label: 'Overbilling' },
  { value: 'PHANTOM_METER', label: 'Phantom Meter' },
  { value: 'NRW_SPIKE', label: 'NRW Spike' },
  { value: 'NIGHT_FLOW_ANOMALY', label: 'Night Flow Anomaly' },
  { value: 'METER_TAMPERING', label: 'Meter Tampering' },
];

export default function CaseQueuePage() {
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [districtId, setDistrictId] = useState('');
  const [status, setStatus] = useState('');
  const [severity, setSeverity] = useState('');
  const [flagType, setFlagType] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [showFilters, setShowFilters] = useState(false);
  const [page, setPage] = useState(0);
  const limit = 25;

  const { data: districts } = useDistricts();
  const { data: casesData, isLoading, refetch } = useCases({
    search: search || undefined,
    district_id: districtId || undefined,
    gwl_status: status || undefined,
    severity: severity || undefined,
    flag_type: flagType || undefined,
    date_from: dateFrom || undefined,
    date_to: dateTo || undefined,
    limit,
    offset: page * limit,
    sort_by: 'estimated_loss_ghs',
  });

  const cases: GWLCase[] = casesData?.cases || [];
  const total: number = casesData?.total || 0;

  const districtOptions = [
    { value: '', label: 'All Districts' },
    ...(districts || []).map((d: { id: string; district_name: string }) => ({
      value: d.id, label: d.district_name,
    })),
  ];

  const activeFilters = [districtId, status, severity, flagType, dateFrom, dateTo].filter(Boolean).length;

  const clearFilters = () => {
    setDistrictId(''); setStatus(''); setSeverity('');
    setFlagType(''); setDateFrom(''); setDateTo('');
    setPage(0);
  };

  const [isExporting, setIsExporting] = useState(false);

  const handleExport = async () => {
    setIsExporting(true);
    try {
      // Fetch ALL matching cases (not just the current page) for a complete export.
      const params: Record<string, string | number> = { limit: 5000, offset: 0, sort_by: 'estimated_loss_ghs' };
      if (search)     params.search      = search;
      if (districtId) params.district_id = districtId;
      if (status)     params.gwl_status  = status;
      if (severity)   params.severity    = severity;
      if (flagType)   params.flag_type   = flagType;
      if (dateFrom)   params.date_from   = dateFrom;
      if (dateTo)     params.date_to     = dateTo;
      const res = await api.get('/gwl/cases', { params });
      const allCases = res.data?.data?.cases ?? res.data?.cases ?? [];
      exportToCSV(allCases, [
      { header: 'Account No', accessor: (r) => r.account_number || '' },
      { header: 'Account Holder', accessor: (r) => r.account_holder || '' },
      { header: 'Category', accessor: (r) => r.account_category || '' },
      { header: 'District', accessor: (r) => r.district_name },
      { header: 'Region', accessor: (r) => r.region },
      { header: 'Flag Type', accessor: (r) => FLAG_TYPE_LABELS[r.anomaly_type as FlagType] || r.anomaly_type },
      { header: 'Severity', accessor: (r) => r.alert_level },
      { header: 'Estimated Loss (GHS)', accessor: (r) => r.estimated_loss_ghs },
      { header: 'GWL Status', accessor: (r) => GWL_STATUS_LABELS[r.gwl_status as GWLStatus] || r.gwl_status },
      { header: 'Assigned Officer', accessor: (r) => r.assigned_officer_name || '' },
      { header: 'Days Open', accessor: (r) => r.days_open },
      { header: 'Flagged On', accessor: (r) => formatDate(r.created_at) },
      ], 'gwl-cases.csv');
    } catch (err) {
      console.error('Export failed', err);
      alert('Export failed. Please try again.');
    } finally {
      setIsExporting(false);
    }
  };

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">All Cases</h1>
          <p className="text-sm text-gray-500 mt-0.5">
            {total} case{total !== 1 ? 's' : ''} found
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => refetch()}>
            <RefreshCw className="w-4 h-4" />
          </Button>
          <Button variant="secondary" size="sm" onClick={handleExport} disabled={isExporting}>
            <Download className={`w-4 h-4 ${isExporting ? 'animate-spin' : ''}`} />
            {isExporting ? 'Exporting…' : 'Export CSV'}
          </Button>
          <Button
            variant={showFilters ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => setShowFilters(!showFilters)}
          >
            <Filter className="w-4 h-4" />
            Filters {activeFilters > 0 && `(${activeFilters})`}
          </Button>
        </div>
      </div>

      {/* Search bar */}
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
        <input
          type="text"
          placeholder="Search by account number or account holder name..."
          value={search}
          onChange={(e) => { setSearch(e.target.value); setPage(0); }}
          className="w-full pl-10 pr-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      {/* Filter panel */}
      {showFilters && (
        <div className="bg-gray-50 border border-gray-200 rounded-xl p-4">
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
            <Select
              label="District"
              options={districtOptions}
              value={districtId}
              onChange={(e) => { setDistrictId(e.target.value); setPage(0); }}
            />
            <Select
              label="Status"
              options={STATUS_OPTIONS}
              value={status}
              onChange={(e) => { setStatus(e.target.value); setPage(0); }}
            />
            <Select
              label="Severity"
              options={SEVERITY_OPTIONS}
              value={severity}
              onChange={(e) => { setSeverity(e.target.value); setPage(0); }}
            />
            <Select
              label="Flag Type"
              options={FLAG_TYPE_OPTIONS}
              value={flagType}
              onChange={(e) => { setFlagType(e.target.value); setPage(0); }}
            />
            <Input
              label="Date From"
              type="date"
              value={dateFrom}
              onChange={(e) => { setDateFrom(e.target.value); setPage(0); }}
            />
            <Input
              label="Date To"
              type="date"
              value={dateTo}
              onChange={(e) => { setDateTo(e.target.value); setPage(0); }}
            />
          </div>
          {activeFilters > 0 && (
            <button
              onClick={clearFilters}
              className="mt-3 flex items-center gap-1 text-xs text-red-600 hover:text-red-800"
            >
              <X className="w-3 h-3" /> Clear all filters
            </button>
          )}
        </div>
      )}

      {/* Table */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm">
        {isLoading ? (
          <Spinner />
        ) : cases.length === 0 ? (
          <EmptyState message="No cases match your filters" />
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
                      <p className="text-xs text-gray-500">{r.account_number || '—'} · {r.account_category || '—'}</p>
                    </div>
                  ),
                },
                {
                  header: 'District',
                  accessor: (r) => (
                    <div>
                      <p className="text-sm text-gray-700">{r.district_name}</p>
                      <p className="text-xs text-gray-400">{r.region}</p>
                    </div>
                  ),
                },
                {
                  header: 'Flag',
                  accessor: (r) => (
                    <Badge className={FLAG_TYPE_COLORS[r.anomaly_type as FlagType]}>
                      {FLAG_TYPE_LABELS[r.anomaly_type as FlagType] || r.anomaly_type}
                    </Badge>
                  ),
                },
                {
                  header: 'Severity',
                  accessor: (r) => (
                    <Badge className={SEVERITY_COLORS[r.alert_level as Severity]}>{r.alert_level}</Badge>
                  ),
                },
                {
                  header: 'Est. Loss',
                  accessor: (r) => (
                    <span className="font-semibold text-red-700 text-sm">{formatGHS(r.estimated_loss_ghs)}</span>
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
                  header: 'Days',
                  accessor: (r) => (
                    <span className={r.days_open > 14 ? 'text-red-600 font-semibold text-sm' : 'text-gray-600 text-sm'}>
                      {r.days_open}d
                    </span>
                  ),
                },
                {
                  header: 'Flagged',
                  accessor: (r) => <span className="text-xs text-gray-500">{formatDate(r.created_at)}</span>,
                },
              ]}
            />

            {/* Pagination */}
            <div className="flex items-center justify-between px-6 py-3 border-t border-gray-100">
              <p className="text-xs text-gray-500">
                Showing {page * limit + 1}–{Math.min((page + 1) * limit, total)} of {total}
              </p>
              <div className="flex gap-2">
                <Button
                  variant="secondary" size="sm"
                  disabled={page === 0}
                  onClick={() => setPage(page - 1)}
                >
                  Previous
                </Button>
                <Button
                  variant="secondary" size="sm"
                  disabled={(page + 1) * limit >= total}
                  onClick={() => setPage(page + 1)}
                >
                  Next
                </Button>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
