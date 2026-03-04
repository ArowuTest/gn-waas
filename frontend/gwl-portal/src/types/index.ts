// ─── GWL Portal Type Definitions ─────────────────────────────────────────────

// ── RBAC User Roles (matches user_role SQL enum + Keycloak realm roles) ───────
// SEC-C01 fix: canonical role set must be identical across all layers:
//   PostgreSQL user_role enum, backend knownRoles map, Keycloak realm-export.json,
//   and all frontend UserRole types.
export type UserRole =
  | 'SUPER_ADMIN'
  | 'SYSTEM_ADMIN'
  | 'MINISTER_VIEW'
  | 'GRA_OFFICER'
  | 'MOF_AUDITOR'
  | 'GWL_EXECUTIVE'
  | 'GWL_MANAGER'
  | 'GWL_SUPERVISOR'
  | 'GWL_ANALYST'
  | 'FIELD_SUPERVISOR'
  | 'FIELD_OFFICER'
  | 'MDA_USER'


export type Severity = 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW';
export type GWLStatus =
  | 'PENDING_REVIEW'
  | 'UNDER_INVESTIGATION'
  | 'FIELD_ASSIGNED'
  | 'EVIDENCE_SUBMITTED'
  | 'APPROVED_FOR_CORRECTION'
  | 'DISPUTED'
  | 'CORRECTED'
  | 'CLOSED';

export type FlagType =
  | 'BILLING_VARIANCE'
  | 'CATEGORY_MISMATCH'
  | 'OVERBILLING'
  | 'PHANTOM_METER'
  | 'NRW_SPIKE'
  | 'NIGHT_FLOW_ANOMALY'
  | 'METER_TAMPERING';

export interface GWLCase {
  id: string;
  account_id: string | null;
  district_id: string;
  // Backend now returns anomaly_type and alert_level (renamed from flag_type/severity)
  anomaly_type: FlagType;
  alert_level: Severity;
  // Legacy aliases for backward compatibility
  flag_type?: FlagType;
  severity?: Severity;
  title: string;
  description: string;
  evidence: Record<string, unknown>;
  estimated_loss_ghs: number;
  created_at: string;

  // GWL workflow
  gwl_status: GWLStatus;
  gwl_assigned_to_id: string | null;
  gwl_assigned_at: string | null;
  gwl_resolved_at: string | null;
  gwl_resolution: string | null;
  gwl_notes: string | null;
  days_open: number;

  // Account
  account_number: string | null;
  account_holder: string | null;
  account_category: string | null;
  meter_number: string | null;
  address: string | null;

  // District
  district_name: string;
  district_code: string;
  region: string;

  // Field officer
  assigned_officer_name: string | null;
  assigned_officer_email: string | null;
  field_job_status: string | null;
  field_job_id: string | null;
}

export interface GWLCaseSummary {
  total_open: number;
  critical_open: number;
  pending_review: number;
  field_assigned: number;
  resolved_this_month: number;
  total_estimated_loss_ghs: number;
  underbilling_total_ghs: number;
  overbilling_total_ghs: number;
  misclassified_count: number;
}

export interface ReclassificationRequest {
  id: string;
  anomaly_flag_id: string;
  account_id: string;
  district_id: string;
  current_category: string;
  recommended_category: string;
  justification: string;
  monthly_revenue_impact_ghs: number;
  annual_revenue_impact_ghs: number;
  status: string;
  requested_by_name: string;
  approved_by_name: string | null;
  approved_at: string | null;
  applied_in_gwl_at: string | null;
  gwl_reference: string | null;
  created_at: string;
  account_number: string | null;
  account_holder: string | null;
  district_name: string;
}

export interface CreditRequest {
  id: string;
  anomaly_flag_id: string;
  account_id: string;
  district_id: string;
  billing_period_start: string;
  billing_period_end: string;
  gwl_amount_ghs: number;
  shadow_amount_ghs: number;
  overcharge_amount_ghs: number;
  credit_amount_ghs: number;
  reason: string;
  notes: string | null;
  status: string;
  requested_by_name: string;
  approved_by_name: string | null;
  approved_at: string | null;
  applied_in_gwl_at: string | null;
  gwl_credit_reference: string | null;
  created_at: string;
  account_number: string | null;
  account_holder: string | null;
  district_name: string;
}

export interface CaseAction {
  id: string;
  anomaly_flag_id: string;
  performed_by_name: string;
  performed_by_role: string;
  action_type: string;
  action_notes: string | null;
  action_metadata: Record<string, unknown>;
  created_at: string;
}

export interface District {
  id: string;
  district_name: string;
  district_code: string;
  region: string;
}

export interface FieldOfficer {
  id: string;
  full_name: string;
  email: string;
  district_id: string;
}

export interface MonthlyReport {
  period: string;
  generated: string;
  statistics: {
    total_flagged: number;
    critical_cases: number;
    resolved: number;
    pending: number;
    disputed: number;
    total_underbilling_ghs: number;
    total_overbilling_ghs: number;
    revenue_recovered_ghs: number;
    credits_issued_ghs: number;
    reclassifications_requested: number;
    reclassifications_applied: number;
    field_jobs_assigned: number;
    field_jobs_completed: number;
  };
}
