import { useState, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, UserPlus, CheckCircle, XCircle, AlertTriangle, RefreshCw } from 'lucide-react';
import { useCase, useCaseActions, useAssignFieldOfficer, useUpdateCaseStatus, useRequestReclassification, useRequestCredit, useFieldOfficers } from '../hooks/useQueries';
import { Badge, Button, Modal, Input, Select, Spinner } from '../components/ui';
import {
  formatGHS, formatDate, formatDateTime,
  GWL_STATUS_LABELS, GWL_STATUS_COLORS,
  SEVERITY_COLORS, FLAG_TYPE_LABELS, FLAG_TYPE_COLORS,
} from '../utils/helpers';
import type { GWLStatus, FlagType, Severity } from '../types';

// GAP-FIX-05: parse the logged-in user from the stored JWT so performed_by/role
// are real values instead of hardcoded 'GWL Supervisor' / 'GWL_SUPERVISOR'.
function parseStoredUser(): { name: string; role: string } {
  try {
    const raw = localStorage.getItem('gwl_user');
    if (raw) {
      const u = JSON.parse(raw) as { full_name?: string; name?: string; role?: string };
      return {
        name: u.full_name ?? u.name ?? 'GWL User',
        role: u.role ?? 'GWL_SUPERVISOR',
      };
    }
  } catch { /* ignore */ }
  return { name: 'GWL User', role: 'GWL_SUPERVISOR' };
}

export default function CaseDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const currentUser = useMemo(() => parseStoredUser(), []);

  const { data, isLoading, refetch } = useCase(id!);
  const { data: actions } = useCaseActions(id!);
  const { data: officers } = useFieldOfficers();

  const assignMutation = useAssignFieldOfficer();
  const statusMutation = useUpdateCaseStatus();
  const reclassifyMutation = useRequestReclassification();
  const creditMutation = useRequestCredit();

  // Modal states
  const [assignModal, setAssignModal] = useState(false);
  const [statusModal, setStatusModal] = useState(false);
  const [reclassifyModal, setReclassifyModal] = useState(false);
  const [creditModal, setCreditModal] = useState(false);

  // Form states
  const [assignForm, setAssignForm] = useState({ officerId: '', priority: 'HIGH', dueDate: '', notes: '' });
  const [statusForm, setStatusForm] = useState({ status: '', notes: '', resolution: '' });
  const [reclassifyForm, setReclassifyForm] = useState({ recommendedCategory: '', justification: '' });
  const [creditForm, setCreditForm] = useState({ creditAmount: '', reason: '', notes: '' });

  if (isLoading) return <Spinner />;
  if (!data) return <div className="text-center py-16 text-gray-500">Case not found</div>;

  const gwlCase = data.case;
  const caseActions = actions || data.actions || [];

  const officerOptions = [
    { value: '', label: 'Select field officer...' },
    ...(officers || []).map((o: { id: string; full_name: string }) => ({
      value: o.id, label: o.full_name,
    })),
  ];

  const categoryOptions = [
    { value: '', label: 'Select category...' },
    { value: 'RESIDENTIAL', label: 'Residential' },
    { value: 'COMMERCIAL', label: 'Commercial' },
    { value: 'INDUSTRIAL', label: 'Industrial' },
    { value: 'GOVERNMENT', label: 'Government' },
    { value: 'STANDPIPE', label: 'Standpipe' },
  ];

  const handleAssign = async () => {
    if (!assignForm.officerId) return;
    await assignMutation.mutateAsync({
      id: id!,
      body: {
        officer_id: assignForm.officerId,
        account_id: gwlCase.account_id,
        priority: assignForm.priority,
        due_date: assignForm.dueDate,
        title: `Field Verification: ${gwlCase.title}`,
        description: gwlCase.description,
        performed_by: currentUser.name,
        role: currentUser.role,
      },
    });
    setAssignModal(false);
    refetch();
  };

  const handleStatusUpdate = async () => {
    if (!statusForm.status) return;
    await statusMutation.mutateAsync({
      id: id!,
      body: {
        status: statusForm.status,
        notes: statusForm.notes || undefined,
        resolution: statusForm.resolution || undefined,
        performed_by: currentUser.name,
        role: currentUser.role,
        action_type: statusForm.status,
        action_notes: statusForm.notes,
      },
    });
    setStatusModal(false);
    refetch();
  };

  const handleReclassify = async () => {
    if (!reclassifyForm.recommendedCategory) return;
    await reclassifyMutation.mutateAsync({
      id: id!,
      body: {
        account_id: gwlCase.account_id,
        district_id: gwlCase.district_id,
        current_category: gwlCase.account_category || 'RESIDENTIAL',
        recommended_category: reclassifyForm.recommendedCategory,
        justification: reclassifyForm.justification,
        monthly_revenue_impact_ghs: gwlCase.estimated_loss_ghs,
        requested_by_name: currentUser.name,
      },
    });
    setReclassifyModal(false);
    refetch();
  };

  const handleCredit = async () => {
    await creditMutation.mutateAsync({
      id: id!,
      body: {
        account_id: gwlCase.account_id,
        district_id: gwlCase.district_id,
        gwl_amount_ghs: gwlCase.estimated_loss_ghs,
        shadow_amount_ghs: 0,
        credit_amount_ghs: parseFloat(creditForm.creditAmount) || gwlCase.estimated_loss_ghs,
        reason: creditForm.reason,
        notes: creditForm.notes || undefined,
        requested_by_name: currentUser.name,
      },
    });
    setCreditModal(false);
    refetch();
  };

  const isResolved = ['CORRECTED', 'CLOSED', 'DISPUTED'].includes(gwlCase.gwl_status);

  return (
    <div className="space-y-6 max-w-6xl">
      {/* Back + Header */}
      <div className="flex items-start justify-between">
        <div className="flex items-start gap-4">
          <button onClick={() => navigate(-1)} className="mt-1 text-gray-400 hover:text-gray-600">
            <ArrowLeft className="w-5 h-5" />
          </button>
          <div>
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className="text-xl font-bold text-gray-900">{gwlCase.title}</h1>
              <Badge className={SEVERITY_COLORS[gwlCase.alert_level as Severity]}>{gwlCase.alert_level}</Badge>
              <Badge className={FLAG_TYPE_COLORS[gwlCase.anomaly_type as FlagType]}>
                {FLAG_TYPE_LABELS[gwlCase.anomaly_type as FlagType] || gwlCase.anomaly_type}
              </Badge>
              <Badge className={GWL_STATUS_COLORS[gwlCase.gwl_status as GWLStatus]}>
                {GWL_STATUS_LABELS[gwlCase.gwl_status as GWLStatus] || gwlCase.gwl_status}
              </Badge>
            </div>
            <p className="text-sm text-gray-500 mt-1">
              Case ID: {gwlCase.id.slice(0, 8)}... · Flagged {formatDate(gwlCase.created_at)} · {gwlCase.days_open} days open
            </p>
          </div>
        </div>
        <Button variant="ghost" size="sm" onClick={() => refetch()}>
          <RefreshCw className="w-4 h-4" />
        </Button>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Account + Evidence */}
        <div className="lg:col-span-2 space-y-5">
          {/* Account Details */}
          <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
            <h2 className="text-sm font-semibold text-gray-900 mb-4">Account Details</h2>
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <p className="text-xs text-gray-500 uppercase tracking-wide">Account Holder</p>
                <p className="font-medium text-gray-900 mt-0.5">{gwlCase.account_holder || '—'}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 uppercase tracking-wide">GWL Account No.</p>
                <p className="font-medium text-gray-900 mt-0.5">{gwlCase.account_number || '—'}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 uppercase tracking-wide">Current Category</p>
                <p className="font-medium text-gray-900 mt-0.5">{gwlCase.account_category || '—'}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 uppercase tracking-wide">Meter Number</p>
                <p className="font-medium text-gray-900 mt-0.5">{gwlCase.meter_number || '—'}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 uppercase tracking-wide">Address</p>
                <p className="font-medium text-gray-900 mt-0.5">{gwlCase.address || '—'}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 uppercase tracking-wide">District</p>
                <p className="font-medium text-gray-900 mt-0.5">{gwlCase.district_name} · {gwlCase.region}</p>
              </div>
            </div>
          </div>

          {/* Billing Evidence */}
          <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
            <h2 className="text-sm font-semibold text-gray-900 mb-4">Billing Evidence</h2>
            <div className="bg-amber-50 border border-amber-200 rounded-lg p-4 mb-4">
              <p className="text-sm text-amber-900 font-medium mb-1">GN-WAAS Finding</p>
              <p className="text-sm text-amber-800">{gwlCase.description}</p>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="bg-red-50 rounded-lg p-4 text-center">
                <p className="text-xs text-red-600 uppercase tracking-wide font-medium">Estimated Revenue Loss</p>
                <p className="text-2xl font-bold text-red-700 mt-1">{formatGHS(gwlCase.estimated_loss_ghs)}</p>
                <p className="text-xs text-red-500 mt-0.5">per month</p>
              </div>
              <div className="bg-blue-50 rounded-lg p-4 text-center">
                <p className="text-xs text-blue-600 uppercase tracking-wide font-medium">Annual Impact</p>
                <p className="text-2xl font-bold text-blue-700 mt-1">{formatGHS(gwlCase.estimated_loss_ghs * 12)}</p>
                <p className="text-xs text-blue-500 mt-0.5">if uncorrected</p>
              </div>
            </div>
          </div>

          {/* Field Officer Status */}
          {gwlCase.assigned_officer_name && (
            <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
              <h2 className="text-sm font-semibold text-gray-900 mb-3">Field Assignment</h2>
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 bg-purple-100 rounded-full flex items-center justify-center text-purple-700 font-bold text-sm">
                  {gwlCase.assigned_officer_name.charAt(0)}
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-900">{gwlCase.assigned_officer_name}</p>
                  <p className="text-xs text-gray-500">{gwlCase.assigned_officer_email}</p>
                </div>
                {gwlCase.field_job_status && (
                  <Badge className="ml-auto bg-purple-100 text-purple-800 border-purple-200">
                    {gwlCase.field_job_status}
                  </Badge>
                )}
              </div>
              {gwlCase.gwl_assigned_at && (
                <p className="text-xs text-gray-400 mt-2">
                  Assigned {formatDateTime(gwlCase.gwl_assigned_at)}
                </p>
              )}
            </div>
          )}

          {/* Audit Trail */}
          <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
            <h2 className="text-sm font-semibold text-gray-900 mb-4">Case History</h2>
            {caseActions.length === 0 ? (
              <p className="text-sm text-gray-400">No actions recorded yet</p>
            ) : (
              <div className="space-y-3">
                {caseActions.map((action: { id: string; action_type: string; performed_by_name: string; performed_by_role: string; action_notes: string | null; created_at: string }) => (
                  <div key={action.id} className="flex gap-3">
                    <div className="w-2 h-2 rounded-full bg-blue-400 mt-2 flex-shrink-0" />
                    <div>
                      <p className="text-sm text-gray-900">
                        <span className="font-medium">{action.performed_by_name}</span>
                        {' '}
                        <span className="text-gray-500">({action.performed_by_role})</span>
                        {' — '}
                        <span className="text-blue-700">{action.action_type.replace(/_/g, ' ')}</span>
                      </p>
                      {action.action_notes && (
                        <p className="text-xs text-gray-500 mt-0.5">{action.action_notes}</p>
                      )}
                      <p className="text-xs text-gray-400 mt-0.5">{formatDateTime(action.created_at)}</p>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Right: Actions Panel */}
        <div className="space-y-4">
          <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
            <h2 className="text-sm font-semibold text-gray-900 mb-4">Actions</h2>
            <div className="space-y-2">
              {!isResolved && (
                <>
                  <Button
                    className="w-full justify-center"
                    size="sm"
                    onClick={() => setAssignModal(true)}
                    disabled={gwlCase.gwl_status === 'FIELD_ASSIGNED'}
                  >
                    <UserPlus className="w-4 h-4" />
                    Assign Field Officer
                  </Button>

                  {gwlCase.anomaly_type === 'CATEGORY_MISMATCH' && (
                    <Button
                      className="w-full justify-center"
                      variant="secondary"
                      size="sm"
                      onClick={() => setReclassifyModal(true)}
                    >
                      🔄 Request Reclassification
                    </Button>
                  )}

                  {gwlCase.anomaly_type === 'OVERBILLING' && (
                    <Button
                      className="w-full justify-center"
                      variant="secondary"
                      size="sm"
                      onClick={() => setCreditModal(true)}
                    >
                      💳 Issue Credit to Customer
                    </Button>
                  )}

                  <Button
                    className="w-full justify-center"
                    variant="success"
                    size="sm"
                    onClick={() => { setStatusForm({ status: 'CORRECTED', notes: '', resolution: 'Correction applied in GWL billing system' }); setStatusModal(true); }}
                  >
                    <CheckCircle className="w-4 h-4" />
                    Mark as Corrected
                  </Button>

                  <Button
                    className="w-full justify-center"
                    variant="secondary"
                    size="sm"
                    onClick={() => { setStatusForm({ status: 'DISPUTED', notes: '', resolution: '' }); setStatusModal(true); }}
                  >
                    <XCircle className="w-4 h-4" />
                    Dispute Flag
                  </Button>

                  <Button
                    className="w-full justify-center"
                    variant="danger"
                    size="sm"
                    onClick={() => { setStatusForm({ status: 'UNDER_INVESTIGATION', notes: '', resolution: '' }); setStatusModal(true); }}
                  >
                    <AlertTriangle className="w-4 h-4" />
                    Escalate
                  </Button>
                </>
              )}

              {isResolved && (
                <div className="text-center py-4">
                  <CheckCircle className="w-8 h-8 text-green-500 mx-auto mb-2" />
                  <p className="text-sm text-gray-600">
                    Case {GWL_STATUS_LABELS[gwlCase.gwl_status as GWLStatus]}
                  </p>
                  {gwlCase.gwl_resolved_at && (
                    <p className="text-xs text-gray-400 mt-1">{formatDate(gwlCase.gwl_resolved_at)}</p>
                  )}
                </div>
              )}
            </div>
          </div>

          {/* Case metadata */}
          <div className="bg-gray-50 rounded-xl border border-gray-200 p-4 text-xs space-y-2">
            <div className="flex justify-between">
              <span className="text-gray-500">Case ID</span>
              <span className="font-mono text-gray-700">{gwlCase.id.slice(0, 8)}...</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-500">Flagged</span>
              <span className="text-gray-700">{formatDate(gwlCase.created_at)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-500">Days Open</span>
              <span className={gwlCase.days_open > 14 ? 'text-red-600 font-semibold' : 'text-gray-700'}>
                {gwlCase.days_open} days
              </span>
            </div>
            {gwlCase.gwl_notes && (
              <div>
                <span className="text-gray-500">Notes</span>
                <p className="text-gray-700 mt-1">{gwlCase.gwl_notes}</p>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* ── Modals ── */}

      {/* Assign Field Officer */}
      <Modal open={assignModal} onClose={() => setAssignModal(false)} title="Assign Field Officer">
        <div className="space-y-4">
          <Select
            label="Field Officer"
            options={officerOptions}
            value={assignForm.officerId}
            onChange={(e) => setAssignForm({ ...assignForm, officerId: e.target.value })}
          />
          <Select
            label="Priority"
            options={[
              { value: 'CRITICAL', label: 'Critical' },
              { value: 'HIGH', label: 'High' },
              { value: 'MEDIUM', label: 'Medium' },
              { value: 'LOW', label: 'Low' },
            ]}
            value={assignForm.priority}
            onChange={(e) => setAssignForm({ ...assignForm, priority: e.target.value })}
          />
          <Input
            label="Due Date"
            type="date"
            value={assignForm.dueDate}
            onChange={(e) => setAssignForm({ ...assignForm, dueDate: e.target.value })}
          />
          <div className="flex gap-3 pt-2">
            <Button variant="secondary" className="flex-1 justify-center" onClick={() => setAssignModal(false)}>
              Cancel
            </Button>
            <Button
              className="flex-1 justify-center"
              loading={assignMutation.isPending}
              onClick={handleAssign}
              disabled={!assignForm.officerId}
            >
              Assign Officer
            </Button>
          </div>
        </div>
      </Modal>

      {/* Update Status */}
      <Modal open={statusModal} onClose={() => setStatusModal(false)} title="Update Case Status">
        <div className="space-y-4">
          <Select
            label="New Status"
            options={[
              { value: 'UNDER_INVESTIGATION', label: 'Under Investigation' },
              { value: 'APPROVED_FOR_CORRECTION', label: 'Approved for Correction' },
              { value: 'DISPUTED', label: 'Disputed' },
              { value: 'CORRECTED', label: 'Corrected' },
              { value: 'CLOSED', label: 'Closed' },
            ]}
            value={statusForm.status}
            onChange={(e) => setStatusForm({ ...statusForm, status: e.target.value })}
          />
          <div>
            <label className="text-sm font-medium text-gray-700">Resolution Notes</label>
            <textarea
              className="mt-1 w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              rows={3}
              value={statusForm.notes}
              onChange={(e) => setStatusForm({ ...statusForm, notes: e.target.value })}
              placeholder="Describe the action taken..."
            />
          </div>
          <div className="flex gap-3 pt-2">
            <Button variant="secondary" className="flex-1 justify-center" onClick={() => setStatusModal(false)}>
              Cancel
            </Button>
            <Button
              className="flex-1 justify-center"
              loading={statusMutation.isPending}
              onClick={handleStatusUpdate}
              disabled={!statusForm.status}
            >
              Update Status
            </Button>
          </div>
        </div>
      </Modal>

      {/* Reclassification */}
      <Modal open={reclassifyModal} onClose={() => setReclassifyModal(false)} title="Request Account Reclassification">
        <div className="space-y-4">
          <div className="bg-violet-50 border border-violet-200 rounded-lg p-3 text-sm text-violet-800">
            Current category: <strong>{gwlCase.account_category || 'Unknown'}</strong>
          </div>
          <Select
            label="Recommended Category"
            options={categoryOptions}
            value={reclassifyForm.recommendedCategory}
            onChange={(e) => setReclassifyForm({ ...reclassifyForm, recommendedCategory: e.target.value })}
          />
          <div>
            <label className="text-sm font-medium text-gray-700">Justification</label>
            <textarea
              className="mt-1 w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              rows={3}
              value={reclassifyForm.justification}
              onChange={(e) => setReclassifyForm({ ...reclassifyForm, justification: e.target.value })}
              placeholder="Explain why this account should be reclassified..."
            />
          </div>
          <div className="flex gap-3 pt-2">
            <Button variant="secondary" className="flex-1 justify-center" onClick={() => setReclassifyModal(false)}>
              Cancel
            </Button>
            <Button
              className="flex-1 justify-center"
              loading={reclassifyMutation.isPending}
              onClick={handleReclassify}
              disabled={!reclassifyForm.recommendedCategory}
            >
              Submit Request
            </Button>
          </div>
        </div>
      </Modal>

      {/* Credit Request */}
      <Modal open={creditModal} onClose={() => setCreditModal(false)} title="Issue Credit to Customer">
        <div className="space-y-4">
          <div className="bg-blue-50 border border-blue-200 rounded-lg p-3 text-sm text-blue-800">
            Estimated overcharge: <strong>{formatGHS(gwlCase.estimated_loss_ghs)}</strong>
          </div>
          <Input
            label="Credit Amount (GHS)"
            type="number"
            step="0.01"
            value={creditForm.creditAmount}
            onChange={(e) => setCreditForm({ ...creditForm, creditAmount: e.target.value })}
            placeholder={gwlCase.estimated_loss_ghs.toFixed(2)}
          />
          <div>
            <label className="text-sm font-medium text-gray-700">Reason for Credit</label>
            <textarea
              className="mt-1 w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              rows={2}
              value={creditForm.reason}
              onChange={(e) => setCreditForm({ ...creditForm, reason: e.target.value })}
              placeholder="Describe the overbilling reason..."
            />
          </div>
          <div className="flex gap-3 pt-2">
            <Button variant="secondary" className="flex-1 justify-center" onClick={() => setCreditModal(false)}>
              Cancel
            </Button>
            <Button
              className="flex-1 justify-center"
              loading={creditMutation.isPending}
              onClick={handleCredit}
            >
              Submit Credit Request
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
