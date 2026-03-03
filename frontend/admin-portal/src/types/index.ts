// ============================================================
// GN-WAAS TypeScript Type Definitions
// All enum values MUST match the PostgreSQL ENUM types defined
// in database/migrations/001_extensions_and_types.sql exactly.
// ============================================================

// ── Alert / Severity ──────────────────────────────────────────────────────────
export type AlertLevel = 'INFO' | 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL'

// ── Audit Event lifecycle (matches audit_status SQL enum) ─────────────────────
export type AuditStatus =
  | 'PENDING'
  | 'IN_PROGRESS'
  | 'AWAITING_GRA'
  | 'GRA_CONFIRMED'
  | 'GRA_FAILED'
  | 'COMPLETED'
  | 'DISPUTED'
  | 'ESCALATED'
  | 'CLOSED'
  | 'PENDING_COMPLIANCE'

// ── GRA VSDC compliance (matches gra_compliance_status SQL enum) ──────────────
export type GRAStatus = 'PENDING' | 'SIGNED' | 'FAILED' | 'RETRYING' | 'EXEMPT'

// ── Field Job lifecycle (matches field_job_status SQL enum + migration 009) ───
export type FieldJobStatus =
  | 'QUEUED'
  | 'ASSIGNED'
  | 'DISPATCHED'
  | 'EN_ROUTE'
  | 'ON_SITE'
  | 'COMPLETED'
  | 'FAILED'
  | 'CANCELLED'
  | 'ESCALATED'
  | 'SOS'

// ── RBAC User Roles (matches user_role SQL enum) ──────────────────────────────
// FE-FIX-05: UserRole must include GWL_SUPERVISOR (added via migration 015).
// Omitting it caused TypeScript type errors when the backend returned that role.
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

export interface District {
  id: string
  district_code: string
  district_name: string
  region: string
  population_estimate: number
  total_connections: number
  supply_status: string
  zone_type: string
  geographic_zone: string
  loss_ratio_pct?: number
  data_confidence_grade?: number
  is_pilot_district: boolean
  is_active: boolean
  created_at: string
}

export interface WaterAccount {
  id: string
  gwl_account_number: string
  account_holder_name: string
  account_holder_tin?: string
  category: string
  status: string
  district_id: string
  meter_number: string
  address_line1: string
  gps_latitude: number
  gps_longitude: number
  is_within_network?: boolean
  monthly_avg_consumption: number
  is_phantom_flagged: boolean
  created_at: string
  updated_at: string
}

export interface AnomalyFlag {
  id: string
  account_id?: string
  district_id: string
  anomaly_type: string
  alert_level: AlertLevel
  fraud_type?: string
  title: string
  description: string
  estimated_loss_ghs?: number
  evidence_data?: Record<string, unknown>
  status: string
  detection_hash?: string
  sentinel_version: string
  created_at: string
  updated_at: string
}

export interface AuditEvent {
  id: string
  audit_reference: string
  account_id: string
  district_id: string
  anomaly_flag_id?: string
  status: AuditStatus
  assigned_officer_id?: string
  assigned_supervisor_id?: string
  assigned_at?: string
  due_date?: string
  field_job_id?: string
  meter_photo_url?: string
  surroundings_photo_url?: string
  photo_urls?: string[]
  evidence_object_keys?: string[]
  photo_hashes?: string[]
  ocr_reading_value?: number
  manual_reading_value?: number
  ocr_status?: string
  gps_latitude?: number
  gps_longitude?: number
  gps_precision_m?: number
  tamper_evidence_detected: boolean
  tamper_evidence_url?: string
  gra_status: GRAStatus
  gra_sdc_id?: string
  gra_qr_code_url?: string
  gra_qr_code?: string
  gra_receipt_number?: string
  gra_locked_at?: string
  gra_signed_at?: string
  gwl_billed_ghs?: number
  shadow_bill_ghs?: number
  variance_pct?: number
  confirmed_loss_ghs?: number
  recovery_invoice_ghs?: number
  success_fee_ghs?: number
  is_locked: boolean
  locked_at?: string
  notes?: string
  created_at: string
  updated_at: string
}

export interface FieldJob {
  id: string
  job_reference: string
  audit_event_id?: string
  account_id: string
  district_id: string
  assigned_officer_id: string
  status: FieldJobStatus
  is_blind_audit: boolean
  target_gps_lat: number
  target_gps_lng: number
  gps_fence_radius_m: number
  dispatched_at?: string
  arrived_at?: string
  completed_at?: string
  officer_gps_lat?: number
  officer_gps_lng?: number
  gps_verified?: boolean
  biometric_verified: boolean
  priority: number
  requires_security_escort: boolean
  sos_triggered: boolean
  sos_triggered_at?: string
  evidence_submitted_at?: string
  evidence_photo_count?: number
  notes?: string
  created_at: string
  updated_at: string
}

export interface User {
  id: string
  email: string
  full_name: string
  phone_number: string
  role: UserRole
  status: string
  organisation: string
  employee_id: string
  district_id?: string
  keycloak_id?: string
  last_login_at?: string
  created_at: string
  updated_at: string
}

export interface TariffRate {
  id: string
  category: string
  tier_name: string
  min_consumption_m3: number
  max_consumption_m3?: number
  rate_per_m3_ghs: number
  fixed_charge_ghs: number
  effective_from: string
  effective_to?: string
  is_active: boolean
}

export interface ShadowBill {
  id: string
  gwl_bill_id: string
  account_id: string
  billing_period_start: string
  billing_period_end: string
  consumption_m3: number
  correct_category: string
  shadow_amount_ghs: number
  shadow_vat_ghs: number
  total_shadow_bill_ghs: number
  gwl_total_ghs: number
  variance_ghs: number
  variance_pct: number
  is_flagged: boolean
  created_at: string
}

export interface DashboardStats {
  total: number
  pending: number
  in_progress: number
  completed: number
  gra_signed: number
  total_confirmed_loss_ghs: number
  total_success_fees_ghs: number
}

export interface NRWSummary {
  district_id: string
  district_name: string
  production_m3: number
  billed_m3: number
  nrw_m3: number
  nrw_pct: number
  data_confidence_grade: string
  period_start: string
  period_end: string
}

export interface ApiResponse<T> {
  data: T
  meta?: {
    total?: number
    page?: number
    page_size?: number
  }
  error?: string
  code?: string
}
