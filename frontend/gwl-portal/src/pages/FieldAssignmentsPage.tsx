import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { MapPin, RefreshCw, Download, UserPlus } from 'lucide-react';
import { useCases, useDistricts, useFieldOfficers } from '../hooks/useQueries';
import { KPICard, Badge, Button, Select, Spinner, EmptyState, Table } from '../components/ui';
import {
  formatGHS, formatDate, formatDateTime,
  GWL_STATUS_COLORS, GWL_STATUS_LABELS,
  SEVERITY_COLORS, exportToCSV,
} from '../utils/helpers';
import type { GWLCase, GWLStatus, Severity } from '../types';

export default function FieldAssignmentsPage() {
  const navigate = useNavigate();
  const [districtId, setDistrictId] = useState('');
  const [officerId, setOfficerId] = useState('');
  const [statusFilter, setStatusFilter] = useState('FIELD_ASSIGNED');

  const { data: districts } = useDistricts();
  const { data: officers } = useFieldOfficers();
  const { data: casesData, isLoading, refetch } = useCases({
    gwl_status: statusFilter || undefined,
    district_id: districtId || undefined,
    assigned_to_id: officerId || undefined,
    limit: 100,
    sort_by: 'created_at',
    sort_dir: 'ASC',
  });

  const cases: GWLCase[] = casesData?.cases || [];
  const total: number = casesData?.total || 0;

  const assignedCount = cases.filter((c) => c.gwl_status === 'FIELD_ASSIGNED').length;
  const evidenceSubmitted = cases.filter((c) => c.gwl_status === 'EVIDENCE_SUBMITTED').length;
  const overdueCount = cases.filter((c) => c.days_open > 7 && c.gwl_status === 'FIELD_ASSIGNED').length;

  // Group by officer
  const byOfficer = cases.reduce((acc, c) => {
    const key = c.assigned_officer_name || 'Unassigned';
    if (!acc[key]) acc[key] = [];
    acc[key].push(c);
    return acc;
  }, {} as Record<string, GWLCase[]>);

  const districtOptions = [
    { value: '', label: 'All Districts' },
    ...(districts || []).map((d: { id: string; district_name: string }) => ({
      value: d.id, label: d.district_name,
    })),
  ];

  const officerOptions = [
    { value: '', label: 'All Officers' },
    ...(officers || []).map((o: { id: string; full_name: string }) => ({
      value: o.id, label: o.full_name,
    })),
  ];

  const handleExport = () => {
    exportToCSV(cases, [
      { header: 'Account No', accessor: (r) => r.account_number || '' },
      { header: 'Account Holder', accessor: (r) => r.account_holder || '' },
      { header: 'District', accessor: (r) => r.district_name },
      { header: 'Flag Type', accessor: (r) => r.anomaly_type },
      { header: 'Severity', accessor: (r) => r.alert_level },
      { header: 'Assigned Officer', accessor: (r) => r.assigned_officer_name || '' },
      { header: 'Field Job Status', accessor: (r) => r.field_job_status || '' },
      { header: 'Days Open', accessor: (r) => r.days_open },
      { header: 'Assigned At', accessor: (r) => r.gwl_assigned_at ? formatDateTime(r.gwl_assigned_at) : '' },
    ], 'field-assignments.csv');
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3">
            <MapPin className="w-6 h-6 text-purple-500" />
            <h1 className="text-2xl font-bold text-gray-900">Field Assignments</h1>
          </div>
          <p className="text-sm text-gray-500 mt-1">
            Track field officers deployed to verify anomalies on-site
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => refetch()}>
            <RefreshCw className="w-4 h-4" />
          </Button>
          <Button variant="secondary" size="sm" onClick={handleExport}>
            <Download className="w-4 h-4" />
            Export
          </Button>
        </div>
      </div>

      {/* KPI Strip */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <KPICard label="Active Assignments" value={assignedCount} color="bg-purple-50" />
        <KPICard label="Evidence Submitted" value={evidenceSubmitted} sub="Awaiting review" color="bg-indigo-50" />
        <KPICard
          label="Overdue (>7 days)"
          value={overdueCount}
          sub="Need follow-up"
          color={overdueCount > 0 ? 'bg-red-50' : 'bg-green-50'}
        />
        <KPICard label="Total Officers" value={(officers || []).length} color="bg-gray-50" />
      </div>

      {/* Filters */}
      <div className="flex gap-3 flex-wrap">
        <Select
          options={[
            { value: 'FIELD_ASSIGNED', label: 'Field Assigned' },
            { value: 'EVIDENCE_SUBMITTED', label: 'Evidence Submitted' },
            { value: '', label: 'All Statuses' },
          ]}
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="w-48"
        />
        <Select
          options={districtOptions}
          value={districtId}
          onChange={(e) => setDistrictId(e.target.value)}
          className="w-48"
        />
        <Select
          options={officerOptions}
          value={officerId}
          onChange={(e) => setOfficerId(e.target.value)}
          className="w-48"
        />
      </div>

      {/* By Officer Summary */}
      {Object.keys(byOfficer).length > 0 && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {Object.entries(byOfficer).map(([name, officerCases]) => (
            <div key={name} className="bg-white rounded-xl border border-gray-200 p-4 shadow-sm">
              <div className="flex items-center gap-2 mb-2">
                <div className="w-8 h-8 bg-purple-100 rounded-full flex items-center justify-center text-purple-700 font-bold text-xs">
                  {name.charAt(0)}
                </div>
                <p className="text-sm font-medium text-gray-900 truncate">{name}</p>
              </div>
              <p className="text-2xl font-bold text-gray-900">{officerCases.length}</p>
              <p className="text-xs text-gray-500">active case{officerCases.length !== 1 ? 's' : ''}</p>
              <p className="text-xs text-red-500 mt-1">
                {officerCases.filter((c) => c.days_open > 7).length} overdue
              </p>
            </div>
          ))}
        </div>
      )}

      {/* Assignments Table */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm">
        <div className="px-6 py-4 border-b border-gray-100">
          <h2 className="text-sm font-semibold text-gray-900">Assignment Details</h2>
          <p className="text-xs text-gray-500 mt-0.5">{total} assignments · click a row to view full case</p>
        </div>
        {isLoading ? (
          <Spinner />
        ) : cases.length === 0 ? (
          <EmptyState message="No field assignments found" />
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
                    <p className="font-medium text-gray-900 text-sm">{r.account_holder || '—'}</p>
                    <p className="text-xs text-gray-500">{r.account_number} · {r.address || r.district_name}</p>
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
                header: 'Est. Loss',
                accessor: (r) => <span className="font-semibold text-red-700">{formatGHS(r.estimated_loss_ghs)}</span>,
              },
              {
                header: 'Assigned Officer',
                accessor: (r) => r.assigned_officer_name ? (
                  <div className="flex items-center gap-2">
                    <div className="w-6 h-6 bg-purple-100 rounded-full flex items-center justify-center text-purple-700 text-xs font-bold">
                      {r.assigned_officer_name.charAt(0)}
                    </div>
                    <span className="text-sm">{r.assigned_officer_name}</span>
                  </div>
                ) : (
                  <button
                    onClick={(e) => { e.stopPropagation(); navigate(`/cases/${r.id}`); }}
                    className="flex items-center gap-1 text-xs text-blue-600 hover:underline"
                  >
                    <UserPlus className="w-3 h-3" /> Assign
                  </button>
                ),
              },
              {
                header: 'Field Job',
                accessor: (r) => r.field_job_status ? (
                  <Badge className="bg-indigo-100 text-indigo-800 border-indigo-200">
                    {r.field_job_status}
                  </Badge>
                ) : <span className="text-gray-400 text-xs">—</span>,
              },
              {
                header: 'Case Status',
                accessor: (r) => (
                  <Badge className={GWL_STATUS_COLORS[r.gwl_status as GWLStatus]}>
                    {GWL_STATUS_LABELS[r.gwl_status as GWLStatus] || r.gwl_status}
                  </Badge>
                ),
              },
              {
                header: 'Assigned',
                accessor: (r) => r.gwl_assigned_at ? (
                  <span className="text-xs text-gray-500">{formatDate(r.gwl_assigned_at)}</span>
                ) : <span className="text-gray-400 text-xs">—</span>,
              },
              {
                header: 'Days',
                accessor: (r) => (
                  <span className={r.days_open > 7 ? 'text-red-600 font-semibold text-sm' : 'text-gray-600 text-sm'}>
                    {r.days_open}d
                  </span>
                ),
              },
            ]}
          />
        )}
      </div>
    </div>
  );
}
